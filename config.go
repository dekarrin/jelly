package jelly

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dekarrin/jelly/jeldao"
	"github.com/dekarrin/jelly/jeldao/jelinmem"
	"github.com/dekarrin/jelly/jeldao/jelite"
	"github.com/dekarrin/jelly/jeldao/owdb"
)

// DBType is the type of a Database connection.
type DBType string

func (dbt DBType) String() string {
	return string(dbt)
}

const (
	DatabaseNone     DBType = "none"
	DatabaseSQLite   DBType = "sqlite"
	DatabaseOWDB     DBType = "owdb"
	DatabaseInMemory DBType = "inmem"
)

const (
	MaxSecretSize = 64
	MinSecretSize = 32
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
		return DatabaseNone, fmt.Errorf("DB type not one of 'sqlite', 'owdb', or 'inmem': %q", s)
	}
}

// Database contains configuration settings for connecting to a persistence
// layer.
type Database struct {
	// Type is the type of database the config refers to. It also determines
	// which of its other fields are valid.
	Type DBType

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

// Config is a configuration for a server. It contains all parameters that can
// be used to configure the operation of a Server.
type Config struct {

	// TokenSecret is the secret used for signing tokens. If not provided, a
	// default key is used.
	TokenSecret []byte

	// DBs is the configurations to use for connecting to databases and other
	// persistence layers. If not provided, it will be set to a configuration
	// for using an in-memory persistence layer.
	DBs map[string]Database

	// UnauthDelayMillis is the amount of additional time to wait
	// (in milliseconds) before sending a response that indicates either that
	// the client was unauthorized or the client was unauthenticated. This is
	// something of an "anti-flood" measure for naive clients attempting
	// non-parallel connections. If not set it will default to 1 second
	// (1000ms). Set this to any negative number to disable the delay.
	UnauthDelayMillis int

	// DBConnector is a custom DB connector for overriding the defaults, which
	// only provide jeldao.Store instances that are associated with the built-in
	// authentication and login functionality.
	//
	// Any fields in the Connector that are set to nil will use the default
	// connection method for that DB type.
	DBConnector Connector
}

// UnauthDelay returns the configured time for the UnauthDelay as a
// time.Duration. If cfg.UnauthDelayMS is set to a number less than 0, this will
// return a zero-valued time.Duration.
func (cfg Config) UnauthDelay() time.Duration {
	if cfg.UnauthDelayMillis < 1 {
		var dur time.Duration
		return dur
	}
	return time.Millisecond * time.Duration(cfg.UnauthDelayMillis)
}

// FillDefaults returns a new Config identical to cfg but with unset values
// set to their defaults.
func (cfg Config) FillDefaults() Config {
	newCFG := cfg

	if newCFG.TokenSecret == nil {
		newCFG.TokenSecret = []byte("DEFAULT_TOKEN_SECRET-DO_NOT_USE_IN_PROD!")
	}
	for name, db := range newCFG.DBs {
		newCFG.DBs[name] = db.FillDefaults()
	}
	if newCFG.UnauthDelayMillis == 0 {
		newCFG.UnauthDelayMillis = 1000
	}

	return newCFG
}

// Validate returns an error if the Config has invalid field values set. Empty
// and unset values are considered invalid; if defaults are intended to be used,
// call Validate on the return value of FillDefaults.
func (cfg Config) Validate() error {
	if len(cfg.TokenSecret) < MinSecretSize {
		return fmt.Errorf("token secret: must be at least %d bytes, but is %d", MinSecretSize, len(cfg.TokenSecret))
	}
	if len(cfg.TokenSecret) > MaxSecretSize {
		return fmt.Errorf("token secret: must be no more than %d bytes, but is %d", MaxSecretSize, len(cfg.TokenSecret))
	}
	for name, db := range cfg.DBs {
		if err := db.Validate(); err != nil {
			return fmt.Errorf("dbs: %s: %w", name, err)
		}
	}

	// all possible values for UnauthDelayMS are valid, so no need to check it

	return nil
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
type Connector struct {
	InMem  func() (jeldao.Store, error)
	SQLite func(dir string) (jeldao.Store, error)
	OWDB   func(dir string, file string) (jeldao.Store, error)
}

// Connect performs all logic needed to connect to the configured DB and
// initialize the store for use.
func (conr Connector) Connect(db Database) (jeldao.Store, error) {
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
		InMem: func() (jeldao.Store, error) {
			return jelinmem.NewAuthUserStore(), nil
		},
		SQLite: func(dir string) (jeldao.Store, error) {
			return jelite.NewAuthUserStore(dir)
		},
		// TODO: actually have Connector accept unique configs for each, as
		// this will build over time. also, owdb has its own in-mem mode it can
		// independently use as specified by config; this should be allowed if
		// discouraged.
		OWDB: func(dir, file string) (jeldao.Store, error) {
			fullPath := filepath.Join(dir, file)
			return owdb.Open(fullPath)
		},
	}
}
