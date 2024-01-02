// Package config contains configuration options for the server as well as
// various config contstants.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

const (
	KeyAPIName    = "name"
	KeyAPIBase    = "base"
	KeyAPIEnabled = "enabled"
	KeyAPIDBs     = "uses"
)

// Common holds configuration options common to all APIs.
type Common struct {
	// Name is the name of the API. Must be unique.
	Name string

	// Enabled is whether the API is to be enabled. By default, this is false in
	// all cases.
	Enabled bool

	// Base is the base URI that all paths will be rooted at, relative to the
	// server base path. This can be "/" (or "", which is equivalent) to
	// indicate that the API is to be based directly at the URIBase of the
	// server config that this API is a part of.
	Base string

	// UsesDBs is a list of names of data stores that the API uses directly.
	// When Init is called, it is passed active connections to each of the DBs.
	// There must be a corresponding entry for each name in the root DBs listing
	// in the Config this API is a part of.
	UsesDBs []string
}

// FillDefaults returns a new *Common identical to cc but with unset values set
// to their defaults and values normalized.
func (cc *Common) FillDefaults() *Common {
	newCC := new(Common)

	if newCC.Base == "" {
		newCC.Base = "/"
	}

	return newCC
}

// Validate returns an error if the Config has invalid field values set. Empty
// and unset values are considered invalid; if defaults are intended to be used,
// call Validate on the return value of FillDefaults.
func (cc *Common) Validate() error {
	if err := validateBaseURI(cc.Base); err != nil {
		return fmt.Errorf(KeyAPIBase+": %w", err)
	}

	return nil
}

func (cc *Common) Common() Common {
	return *cc
}

func (cc *Common) Keys() []string {
	return []string{KeyAPIName, KeyAPIEnabled, KeyAPIBase, KeyAPIDBs}
}

func (cc *Common) Get(key string) interface{} {
	switch strings.ToLower(key) {
	case KeyAPIName:
		return cc.Name
	case KeyAPIEnabled:
		return cc.Enabled
	case KeyAPIBase:
		return cc.Base
	case KeyAPIDBs:
		return cc.UsesDBs
	default:
		return nil
	}
}

func (cc *Common) Set(key string, value interface{}) error {
	switch strings.ToLower(key) {
	case KeyAPIName:
		if valueStr, ok := value.(string); ok {
			cc.Name = valueStr
			return nil
		} else {
			return fmt.Errorf("key '"+KeyAPIName+"' requires a string but got a %T", value)
		}
	case KeyAPIEnabled:
		if valueBool, ok := value.(bool); ok {
			cc.Enabled = valueBool
			return nil
		} else {
			return fmt.Errorf("key '"+KeyAPIEnabled+"' requires a bool but got a %T", value)
		}
	case KeyAPIBase:
		if valueStr, ok := value.(string); ok {
			cc.Base = valueStr
			return nil
		} else {
			return fmt.Errorf("key '"+KeyAPIBase+"' requires a string but got a %T", value)
		}
	case KeyAPIDBs:
		if valueStrSlice, ok := value.([]string); ok {
			cc.UsesDBs = valueStrSlice
			return nil
		} else {
			return fmt.Errorf("key '"+KeyAPIDBs+"' requires a []string but got a %T", value)
		}
	default:
		return fmt.Errorf("not a valid key: %q", key)
	}
}

func (cc *Common) SetFromString(key string, value string) error {
	switch strings.ToLower(key) {
	case KeyAPIName, KeyAPIBase:
		return cc.Set(key, value)
	case KeyAPIEnabled:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		return cc.Set(key, b)
	case KeyAPIDBs:
		if value == "" {
			return cc.Set(key, []string{})
		}
		dbsStrSlice := strings.Split(value, ",")
		return cc.Set(key, dbsStrSlice)
	default:
		return fmt.Errorf("not a valid key: %q", key)
	}
}

// Globals are the values of global configuration values from the top level
// config. These values are shared with every API.
type Globals struct {

	// Port is the port that the server will listen on. It will default to 8080
	// if none is given.
	Port int

	// Address is the internet address that the server will listen on. It will
	// default to "localhost" if none is given.
	Address string

	// URIBase is the base path that all APIs are rooted on. It will default to
	// "/", which is equivalent to being directly on root.
	URIBase string

	// UnauthDelayMillis is the amount of additional time to wait
	// (in milliseconds) before sending a response that indicates either that
	// the client was unauthorized or the client was unauthenticated. This is
	// something of an "anti-flood" measure for naive clients attempting
	// non-parallel connections. If not set it will default to 1 second
	// (1000ms). Set this to any negative number to disable the delay.
	UnauthDelayMillis int
}

func (g Globals) FillDefaults() Globals {
	newG := g

	if newG.UnauthDelayMillis == 0 {
		newG.UnauthDelayMillis = 1000
	}
	if newG.Port == 0 {
		newG.Port = 8080
	}
	if newG.Address == "" {
		newG.Address = "localhost"
	}
	if newG.URIBase == "" {
		newG.URIBase = "/"
	}

	return newG
}

func (g Globals) Validate() error {
	if g.Port < 1 {
		return fmt.Errorf("port: must be greater than 0")
	}
	if g.Address == "" {
		return fmt.Errorf("address: must not be empty")
	}
	if err := validateBaseURI(g.URIBase); err != nil {
		return fmt.Errorf("base: %w", err)
	}

	return nil
}

// UnauthDelay returns the configured time for the UnauthDelay as a
// time.Duration. If cfg.UnauthDelayMS is set to a number less than 0, this will
// return a zero-valued time.Duration.
func (g Globals) UnauthDelay() time.Duration {
	if g.UnauthDelayMillis < 1 {
		var dur time.Duration
		return dur
	}
	return time.Millisecond * time.Duration(g.UnauthDelayMillis)
}

type APIConfig interface {
	// Common returns the parts of the API configuration that all APIs are
	// required to have. Its keys should be considered part of the configuration
	// held within the APIConfig and any function that accepts keys will accept
	// the Common keys; additionally, FillDefaults and Validate will both
	// perform their operations on the Common's keys.
	//
	// Performing mutation operations on the Common() returned will not
	// necessarily affect the APIConfig it came from. Affecting one of its key's
	// values should be done by calling the appropriate method on the APIConfig
	// with the key name.
	Common() Common

	// Keys returns a list of strings, each of which is a valid key that this
	// configuration contains. These keys may be passed to other methods to
	// access values in this config.
	//
	// Each key returned should be alpha-numeric, and snake-case is preferred
	// (though not required). If a key contains an illegal character for a
	// particular format of a config source, it will be replaced with an
	// underscore in that format; e.g. a key called "test!" would be retrieved
	// from an envvar called "APPNAME_TEST_" as opposed to "APPNAME_TEST!", as
	// the exclamation mark is not allowed in most environment variable names.
	//
	// The returned slice will contain the values returned by Common()'s Keys()
	// function as well as any other keys provided by the APIConfig. Each item
	// in the returned slice must be non-empty and unique when all keys are
	// converted to lowercase.
	Keys() []string

	// Get gets the current value of a config key. The parameter key should be a
	// string that is returned from Keys(). If key is not a string that was
	// returned from Keys, this function must return nil.
	//
	// The key is not case-sensitive.
	Get(key string) interface{}

	// Set sets the current value of a config key directly. The value must be of
	// the correct type; no parsing is done in Set.
	//
	// The key is not case-sensitive.
	Set(key string, value interface{}) error

	// SetFromString sets the current value of a config key by parsing the given
	// string for its value.
	//
	// The key is not case-sensitive.
	SetFromString(key string, value string) error

	// FillDefaults returns a copy of the APIConfig with any unset values set to
	// default values, if possible. It need not be a brand new copy; it is legal
	// for implementers to returns the same APIConfig that FillDefaults was
	// called on.
	//
	// Implementors must ensure that the returned APIConfig's Common() returns a
	// common config that has had its keys set to their defaults as well.
	FillDefaults() APIConfig

	// Validate checks all current values of the APIConfig and returns whether
	// there is any issues with them.
	//
	// Implementors must ensure that calling Validate() also calls validation on
	// the common keys as well as those that they provide.
	Validate() error
}

// GetString returns a string value from an APIConfig. If the given value is not
// a string or there is an error retrieving it, panics.
func GetString(api APIConfig, key string) string {
	return apiGetTyped[string](api, key)
}

// GetBool returns a bool value from an APIConfig. If the given value is not a
// bool or there is an error retrieving it, panics.
func GetBool(api APIConfig, key string) bool {
	return apiGetTyped[bool](api, key)
}

// GetInt returns an int value from an APIConfig. If the given value is not an
// int or there is an error retrieving it, panics.
func GetInt(api APIConfig, key string) int {
	return apiGetTyped[int](api, key)
}

// GetStringSlice returns a string slice value from an APIConfig. If the given
// value is not a []string or there is an error retrieving it, panics.
func GetStringSlice(api APIConfig, key string) []string {
	return apiGetTyped[[]string](api, key)
}

func apiGetTyped[E any](api APIConfig, key string) E {
	if !apiHas(api, key) {
		panic(fmt.Sprintf("config does not contain key %q", key))
	}
	v := api.Get(key)
	if typed, ok := v.(E); ok {
		return typed
	}

	var check E
	panic(fmt.Sprintf("key %q is not of type %T", key, check))
}

func apiHas(api APIConfig, key string) bool {
	needle := strings.ToLower(key)

	for _, k := range api.Keys() {
		if strings.ToLower(k) == needle {
			return true
		}
	}
	return false
}

// Config is a complete configuration for a server. It contains all parameters
// that can be used to configure its operation.
type Config struct {

	// Globals is all variables shared with initialization of all APIs.
	Globals Globals

	// DBs is the configurations to use for connecting to databases and other
	// persistence layers. If not provided, it will be set to a configuration
	// for using an in-memory persistence layer.
	DBs map[string]Database

	// APIs is the configuration for each API that will be included in a
	// configured jelly framework server. Each APIConfig must return a
	// CommonConfig whose Name is either set to blank or to the key that maps to
	// it.
	APIs map[string]APIConfig

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
	return cfg.Globals.UnauthDelay()
}

// FillDefaults returns a new Config identical to cfg but with unset values
// set to their defaults.
func (cfg Config) FillDefaults() Config {
	newCFG := cfg

	for name, db := range newCFG.DBs {
		newCFG.DBs[name] = db.FillDefaults()
	}
	newCFG.Globals = newCFG.Globals.FillDefaults()
	for name, api := range newCFG.APIs {
		if GetString(api, KeyAPIName) == "" {
			if err := api.Set(KeyAPIName, name); err != nil {
				panic(fmt.Sprintf("setting a common property failed; should never happen: %v", err))
			}
		}
		api = api.FillDefaults()
		newCFG.APIs[name] = api
	}

	// if the user has enabled the jellyauth API, set defaults now.
	if authConf, ok := newCFG.APIs["jellyauth"]; ok {
		// make shore the first DB exists
		if GetBool(authConf, KeyAPIEnabled) {
			dbs := GetStringSlice(authConf, KeyAPIDBs)
			if len(dbs) > 0 {
				// make shore this DB exists
				if _, ok := newCFG.DBs[dbs[0]]; !ok {
					newCFG.DBs[dbs[0]] = Database{Type: DatabaseInMemory}.FillDefaults()
				}
			}
		}
	}

	return newCFG
}

// Validate returns an error if the Config has invalid field values set. Empty
// and unset values are considered invalid; if defaults are intended to be used,
// call Validate on the return value of FillDefaults.
func (cfg Config) Validate() error {
	if err := cfg.Globals.Validate(); err != nil {
		return err
	}
	for name, db := range cfg.DBs {
		if err := db.Validate(); err != nil {
			return fmt.Errorf("dbs: %s: %w", name, err)
		}
	}
	for name, api := range cfg.APIs {
		com := cfg.APIs[name].Common()

		if name != com.Name && com.Name != "" {
			return fmt.Errorf("apis: %s: name mismatch; API.Name is set to %q", name, com.Name)
		}
		if err := api.Validate(); err != nil {
			return fmt.Errorf("apis: %s: %w", name, err)
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

func validateBaseURI(base string) error {
	if strings.ContainsRune(base, '{') {
		return fmt.Errorf("contains disallowed char \"{\"")
	}
	if strings.ContainsRune(base, '}') {
		return fmt.Errorf("contains disallowed char \"}\"")
	}
	if strings.Contains(base, "//") {
		return fmt.Errorf("contains disallowed double-slash \"//\"")
	}
	return nil
}
