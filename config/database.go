package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dekarrin/jelly/db/inmem"
	"github.com/dekarrin/jelly/db/owdb"
	"github.com/dekarrin/jelly/db/sqlite"
	"github.com/dekarrin/jelly/types"
)

// Database contains configuration settings for connecting to a persistence
// layer.
type Database struct {
	// Type is the type of database the config refers to, primarily for data
	// validation purposes. It also determines which of its other fields are
	// valid.
	Type types.DBType

	// Connector is the name of the registered connector function that should be
	// used. The function name must be registered for DBs of the given type.
	Connector string

	// DataDir is the path on disk to a directory to use to store data in. This
	// is only applicable for certain DB types: SQLite, OWDB.
	DataDir string

	// DataFile is the name of the DB file to use for an OrbweaverDB (OWDB)
	// persistence store. By default, it is "db.owv". This is only applicable
	// for certain DB types: OWDB.
	DataFile string
}

// FillDefaults returns a new Database identical to db but with unset values
// set to their defaults. In this case, if the type is not set, it is changed to
// types.DatabaseInMemory. If OWDB File is not set, it is changed to "db.owv".
func (db Database) FillDefaults() Database {
	newDB := db

	if newDB.Type == types.DatabaseNone {
		newDB = Database{Type: types.DatabaseInMemory}
	}
	if newDB.Type == types.DatabaseOWDB && newDB.DataFile == "" {
		newDB.DataFile = "db.owv"
	}
	if newDB.Connector == "" {
		newDB.Connector = "*"
	}

	return newDB
}

// Validate returns an error if the Database does not have the correct fields
// set. Its type will be checked to ensure that it is a valid type to use and
// any fields necessary for connecting to that type of DB are also checked.
func (db Database) Validate() error {
	switch db.Type {
	case types.DatabaseInMemory:
		// nothing else to check
		return nil
	case types.DatabaseSQLite:
		if db.DataDir == "" {
			return fmt.Errorf("DataDir not set to path")
		}
		return nil
	case types.DatabaseOWDB:
		if db.DataDir == "" {
			return fmt.Errorf("DataDir not set to path")
		}
		return nil
	case types.DatabaseNone:
		return fmt.Errorf("'none' DB is not valid")
	default:
		return fmt.Errorf("unknown database type: %q", db.Type.String())
	}
}

// ParseDBConnString parses a database connection string of the form
// "engine:params" (or just "engine" if no other params are required) into a
// valid Database config object.
//
// Supported database types and a sample string containing valid configurations
// for each are shown below. Placeholder values are between angle brackets,
// optional parts are between square brackets. Ordering of parameters does not
// matter.
//
// * In-memory database: "inmem"
// * SQLite3 DB file: "sqlite:</path/to/db/dir>""
// * OrbweaverDB: "owdb:dir=<path/to/db/dir>[,file=<new-db-file-name.owv>]"
func ParseDBConnString(s string) (Database, error) {
	var paramStr string
	dbParts := strings.SplitN(s, ":", 2)

	if len(dbParts) == 2 {
		paramStr = strings.TrimSpace(dbParts[1])
	}

	// parse the first section into a type, from there we can determine if
	// further params are required.
	dbEng, err := types.ParseDBType(strings.TrimSpace(dbParts[0]))
	if err != nil {
		return Database{}, fmt.Errorf("unsupported DB engine: %w", err)
	}

	switch dbEng {
	case types.DatabaseInMemory:
		// there cannot be any other options
		if paramStr != "" {
			return Database{}, fmt.Errorf("unsupported param(s) for in-memory DB engine: %s", paramStr)
		}

		return Database{Type: types.DatabaseInMemory}, nil
	case types.DatabaseSQLite:
		// there must be options
		if paramStr == "" {
			return Database{}, fmt.Errorf("sqlite DB engine requires path to data directory after ':'")
		}

		// the only option is the DB path, as long as the param str isn't
		// literally blank, it can be used.

		// convert slashes to correct type
		dd := filepath.FromSlash(paramStr)
		return Database{Type: types.DatabaseSQLite, DataDir: dd}, nil
	case types.DatabaseOWDB:
		// there must be options
		if paramStr == "" {
			return Database{}, fmt.Errorf("owdb DB engine requires qualified path to data directory after ':'")
		}

		// split the arguments, simply go through and ignore unescaped
		params, err := parseParamsMap(paramStr)
		if err != nil {
			return Database{}, err
		}

		db := Database{Type: types.DatabaseOWDB}

		if val, ok := params["dir"]; ok {
			db.DataDir = filepath.FromSlash(val)
		} else {
			return Database{}, fmt.Errorf("owdb DB engine params missing qualified path to data directory in key 'dir'")
		}

		if val, ok := params["file"]; ok {
			db.DataFile = val
		} else {
			db.DataFile = "db.owv"
		}
		return db, nil
	case types.DatabaseNone:
		// not allowed
		return Database{}, fmt.Errorf("cannot specify DB engine 'none' (perhaps you wanted 'inmem'?)")
	default:
		// unknown
		return Database{}, fmt.Errorf("unknown DB engine: %q", dbEng.String())
	}
}

func parseParamsMap(paramStr string) (map[string]string, error) {
	seqs := splitWithEscaped(paramStr, ",")
	if len(seqs) < 1 {
		return nil, fmt.Errorf("not a map format string: %q", paramStr)
	}

	params := map[string]string{}
	for idx, kv := range seqs {
		parsed := splitWithEscaped(kv, "=")
		if len(parsed) != 2 {
			return nil, fmt.Errorf("param %d: not a kv-pair: %q", idx, kv)
		}
		k := parsed[0]
		v := parsed[1]
		params[strings.ToLower(k)] = v
	}

	return params, nil
}

// if sep contains a backslash, nil is returned.
func splitWithEscaped(s, sep string) []string {
	if strings.Contains(s, "\\") {
		return nil
	}
	var split []string
	var cur strings.Builder
	sepr := []rune(sep)
	sr := []rune(s)
	var seprPos int
	for i := 0; i < len(sr); i++ {
		ch := sr[i]

		if ch == sepr[seprPos] {
			if seprPos+1 >= len(sepr) {
				split = append(split, cur.String())
				cur.Reset()
				seprPos = 0
			} else {
				seprPos++
			}
		} else {
			seprPos = 0
		}

		if ch == '\\' {
			cur.WriteRune(ch)
			cur.WriteRune(sr[i+1])
			i++
		}
	}

	var preSepStr string
	if seprPos > 0 {
		preSepStr = string(sepr[0:seprPos])
	}
	if cur.Len() > 0 {
		split = append(split, preSepStr+cur.String())
	}

	return split
}

// ConnectorRegistry holds registered connecter functions for opening store
// structs on database connections.
//
// The zero value can be immediately used and will have the built-in default and
// pre-rolled connectors available. This can be disabled by setting
// DisableDefaults to true before attempting to use it.
type ConnectorRegistry struct {
	DisableDefaults bool
	reg             map[types.DBType]map[string]func(Database) (types.Store, error)
}

func (cr *ConnectorRegistry) initDefaults() {
	// TODO: follow initDefaults pattern on all env-y structs

	if cr.reg == nil {
		cr.reg = map[types.DBType]map[string]func(Database) (types.Store, error){
			types.DatabaseInMemory: {},
			types.DatabaseSQLite:   {},
			types.DatabaseOWDB:     {},
		}

		if !cr.DisableDefaults {
			cr.reg[types.DatabaseInMemory]["authuser"] = func(d Database) (types.Store, error) {
				return inmem.NewAuthUserStore(), nil
			}
			cr.reg[types.DatabaseSQLite]["authuser"] = func(db Database) (types.Store, error) {
				err := os.MkdirAll(db.DataDir, 0770)
				if err != nil {
					return nil, fmt.Errorf("create data dir: %w", err)
				}

				store, err := sqlite.NewAuthUserStore(db.DataDir)
				if err != nil {
					return nil, fmt.Errorf("initialize sqlite: %w", err)
				}

				return store, nil
			}
			cr.reg[types.DatabaseOWDB]["*"] = func(db Database) (types.Store, error) {
				err := os.MkdirAll(db.DataDir, 0770)
				if err != nil {
					return nil, fmt.Errorf("create data dir: %w", err)
				}

				fullPath := filepath.Join(db.DataDir, db.DataFile)
				store, err := owdb.Open(fullPath)
				if err != nil {
					return nil, fmt.Errorf("initialize owdb: %w", err)
				}

				return store, nil
			}
		}
	}
}

func (cr *ConnectorRegistry) Register(engine types.DBType, name string, connector func(Database) (types.Store, error)) error {
	if connector == nil {
		return fmt.Errorf("connector function cannot be nil")
	}

	cr.initDefaults()

	engConns, ok := cr.reg[engine]
	if !ok {
		return fmt.Errorf("%q is not a supported DB type", engine)
	}

	normName := strings.ToLower(name)
	if _, ok := engConns[normName]; ok && normName != "*" {
		return fmt.Errorf("duplicate connector registration; %q/%q already has a registered connector", engine, normName)
	}

	engConns[normName] = connector
	cr.reg[engine] = engConns
	return nil
}

// List returns an alphabetized list of all currently registered connector
// names for an engine.
func (cr *ConnectorRegistry) List(engine types.DBType) []string {
	cr.initDefaults()

	engConns := cr.reg[engine]

	names := make([]string, len(engConns))

	var cur int
	for k := range engConns {
		names[cur] = k
	}

	sort.Strings(names)
	return names
}

// Connect opens a connection to the configured database, returning a generic
// db.Store. The Store can then be cast to the appropriate type by APIs in
// their init method.
func (cr *ConnectorRegistry) Connect(db Database) (types.Store, error) {
	cr.initDefaults()

	engConns := cr.reg[db.Type]

	normName := strings.ToLower(db.Connector)
	connector, ok := engConns[normName]
	if !ok {
		connector, ok = engConns["*"]
		if !ok {
			var additionalInfo = "DB does not specify connector"
			if normName != "" && normName != "*" {
				additionalInfo = fmt.Sprintf("%q/%q is not a registered connector", db.Type, normName)
			}
			return nil, fmt.Errorf("%s and %q has no default \"*\" connector registered", additionalInfo, db.Type)
		}
	}

	return connector(db)
}
