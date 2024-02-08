package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dekarrin/jelly/dao"
	"github.com/dekarrin/jelly/dao/inmem"
	"github.com/dekarrin/jelly/dao/owdb"
	"github.com/dekarrin/jelly/dao/sqlite"
)

// DBType is the type of a Database connection.
type DBType string

func (dbt DBType) String() string {
	return string(dbt)
}

var dbTypeOrdering = []DBType{
	DatabaseNone, DatabaseSQLite, DatabaseOWDB, DatabaseInMemory,
}

const (
	DatabaseNone     DBType = "none"
	DatabaseSQLite   DBType = "sqlite"
	DatabaseOWDB     DBType = "owdb"
	DatabaseInMemory DBType = "inmem"
)

// ParseDBType parses a string found in a connection string into a DBType.
func ParseDBType(s string) (DBType, error) {
	sLower := strings.ToLower(s)

	switch sLower {
	case DatabaseSQLite.String():
		return DatabaseSQLite, nil
	case DatabaseInMemory.String():
		return DatabaseInMemory, nil
	case DatabaseOWDB.String():
		return DatabaseOWDB, nil
	default:
		return DatabaseNone, fmt.Errorf("DB type %q is not one of 'sqlite', 'owdb', or 'inmem'", s)
	}
}

// Database contains configuration settings for connecting to a persistence
// layer.
type Database struct {
	// Type is the type of database the config refers to, primarily for data
	// validation purposes. It also determines which of its other fields are
	// valid.
	Type DBType

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
// DatabaseInMemory. If OWDB File is not set, it is changed to "db.owv".
func (db Database) FillDefaults() Database {
	newDB := db

	if newDB.Type == DatabaseNone {
		newDB = Database{Type: DatabaseInMemory}
	}
	if newDB.Type == DatabaseOWDB && newDB.DataFile == "" {
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
	case DatabaseInMemory:
		// nothing else to check
		return nil
	case DatabaseSQLite:
		if db.DataDir == "" {
			return fmt.Errorf("DataDir not set to path")
		}
		return nil
	case DatabaseOWDB:
		if db.DataDir == "" {
			return fmt.Errorf("DataDir not set to path")
		}
		return nil
	case DatabaseNone:
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
	dbEng, err := ParseDBType(strings.TrimSpace(dbParts[0]))
	if err != nil {
		return Database{}, fmt.Errorf("unsupported DB engine: %w", err)
	}

	switch dbEng {
	case DatabaseInMemory:
		// there cannot be any other options
		if paramStr != "" {
			return Database{}, fmt.Errorf("unsupported param(s) for in-memory DB engine: %s", paramStr)
		}

		return Database{Type: DatabaseInMemory}, nil
	case DatabaseSQLite:
		// there must be options
		if paramStr == "" {
			return Database{}, fmt.Errorf("sqlite DB engine requires path to data directory after ':'")
		}

		// the only option is the DB path, as long as the param str isn't
		// literally blank, it can be used.

		// convert slashes to correct type
		dd := filepath.FromSlash(paramStr)
		return Database{Type: DatabaseSQLite, DataDir: dd}, nil
	case DatabaseOWDB:
		// there must be options
		if paramStr == "" {
			return Database{}, fmt.Errorf("owdb DB engine requires qualified path to data directory after ':'")
		}

		// split the arguments, simply go through and ignore unescaped
		params, err := parseParamsMap(paramStr)
		if err != nil {
			return Database{}, err
		}

		db := Database{Type: DatabaseOWDB}

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
	case DatabaseNone:
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

// Connector holds functions for establishing a connection and opening a store
// that implements jeldao.Store. It's included in Config objects so the default
// connector functions can be overriden. The default connectors only support
// opening a Store that provides access to entities assocaited with the built-in
// authentication and login management of the jelly framework.
//
// Custom Connectors do not need to provide a value for all of the DB connection
// functions; any that are left as nil will default to the built-in
// implementations.
//
// TODO: remove this once DBConnectorRegistry is complete.
type Connector struct {
	InMem  func() (dao.Store, error)
	SQLite func(dir string) (dao.Store, error)
	OWDB   func(dir string, file string) (dao.Store, error)
}

// Connect performs all logic needed to connect to the configured DB and
// initialize the store for use.
func (conr Connector) Connect(db Database) (dao.Store, error) {
	conr = conr.FillDefaults()
	switch db.Type {
	case DatabaseInMemory:
		return conr.InMem()
	case DatabaseOWDB:
		err := os.MkdirAll(db.DataDir, 0770)
		if err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}

		store, err := conr.OWDB(db.DataDir, db.DataFile)
		if err != nil {
			return nil, fmt.Errorf("initialize owdb: %w", err)
		}

		return store, nil
	case DatabaseSQLite:
		err := os.MkdirAll(db.DataDir, 0770)
		if err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}

		store, err := conr.SQLite(db.DataDir)
		if err != nil {
			return nil, fmt.Errorf("initialize sqlite: %w", err)
		}

		return store, nil
	case DatabaseNone:
		return nil, fmt.Errorf("cannot connect to 'none' DB")
	default:
		return nil, fmt.Errorf("unknown database type: %q", db.Type.String())
	}
}

// FillDefaults returns a new Config identitical to cfg but with unset values
// set to their defaults.
func (conr Connector) FillDefaults() Connector {
	def := DefaultDBConnector()
	newConr := conr

	if newConr.InMem == nil {
		newConr.InMem = def.InMem
	}
	if newConr.SQLite == nil {
		newConr.SQLite = def.SQLite
	}
	if newConr.OWDB == nil {
		newConr.OWDB = def.OWDB
	}

	return newConr
}

func DefaultDBConnector() Connector {
	return Connector{
		InMem: func() (dao.Store, error) {
			return inmem.NewAuthUserStore(), nil
		},
		SQLite: func(dir string) (dao.Store, error) {
			return sqlite.NewAuthUserStore(dir)
		},
		// TODO: actually have Connector accept unique configs for each, as
		// this will build over time. also, owdb has its own in-mem mode it can
		// independently use as specified by config; this should be allowed if
		// discouraged.
		OWDB: func(dir, file string) (dao.Store, error) {
			fullPath := filepath.Join(dir, file)
			return owdb.Open(fullPath)
		},
	}
}

// DBConnectorRegistry holds registered connecter functions for opening store
// structs on database connections.
//
// The zero value can be immediately used and will have the built-in default and
// pre-rolled connectors available. This can be disabled by setting
// DisableDefaults to true before attempting to use it.
type DBConnectorRegistry struct {
	DisableDefaults bool
	reg             map[DBType]map[string]func(Database) (dao.Store, error)
}

func (cr *DBConnectorRegistry) initDefaults() {
	// TODO: follow initDefaults pattern on all env-y structs

	if cr.reg == nil {
		cr.reg = map[DBType]map[string]func(Database) (dao.Store, error){
			DatabaseInMemory: {},
			DatabaseSQLite:   {},
			DatabaseOWDB:     {},
		}

		if !cr.DisableDefaults {
			cr.reg[DatabaseInMemory]["authuser"] = func(d Database) (dao.Store, error) {
				return inmem.NewAuthUserStore(), nil
			}
			cr.reg[DatabaseSQLite]["authuser"] = func(db Database) (dao.Store, error) {
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
			cr.reg[DatabaseOWDB]["*"] = func(db Database) (dao.Store, error) {
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

func (cr *DBConnectorRegistry) Register(engine DBType, name string, connector func(Database) (dao.Store, error)) error {
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
func (cr *DBConnectorRegistry) List(engine DBType) []string {
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
// dao.Store. The Store can then be cast to the appropriate type by APIs in
// their init method.
func (cr *DBConnectorRegistry) Connect(db Database) (dao.Store, error) {
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
