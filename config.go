package jelly

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	ConfigKeyAPIName    = "name"
	ConfigKeyAPIBase    = "base"
	ConfigKeyAPIEnabled = "enabled"
	ConfigKeyAPIUsesDBs = "uses"
)

const (
	DatabaseNone     DBType = "none"
	DatabaseSQLite   DBType = "sqlite"
	DatabaseOWDB     DBType = "owdb"
	DatabaseInMemory DBType = "inmem"
)

const (
	NoFormat Format = iota
	JSON
	YAML
)

// TypedSlice takes a value that is passed to Set that is expected to be a slice
// of the given type and performs the required conversions. If a non-nil error
// is returned it will contain the key name automatically in its error string.
func TypedSlice[E any](key string, value interface{}) ([]E, error) {
	var typed E
	var typedValues []E

	if valueStr, ok := value.([]E); ok {
		typedValues = valueStr
		return typedValues, nil
	} else if valueSlice, ok := value.([]interface{}); ok {
		var ok bool
		for i := range valueSlice {
			if typed, ok = valueSlice[i].(E); ok {
				typedValues = append(typedValues, typed)
			} else {
				return nil, fmt.Errorf("%s[%d]: %q is not a valid string", key, i, valueSlice[i])
			}
		}
		return typedValues, nil
	} else {
		return nil, fmt.Errorf("key '%s' requires a %T but got a %T", key, typedValues, value)
	}
}

type Format int

func (f Format) String() string {
	switch f {
	case NoFormat:
		return "NoFormat"
	case JSON:
		return "JSON"
	case YAML:
		return "YAML"
	default:
		return fmt.Sprintf("Format(%d)", int(f))
	}
}

func (f Format) Extensions() []string {
	switch f {
	case JSON:
		return []string{"json", "jsn"}
	case YAML:
		return []string{"yaml", "yml"}
	default:
		return nil
	}
}

// DBType is the type of a Database connection.
type DBType string

func (dbt DBType) String() string {
	return string(dbt)
}

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
	Common() CommonConfig

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

// CommonConfig holds configuration options common to all APIs.
type CommonConfig struct {
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

	// UsesDBs is a list of names of data stores and authenticators that the API
	// uses directly. When Init is called, it is passed active connections to
	// each of the DBs. There must be a corresponding entry for each DB name in
	// the root DBs listing in the Config this API is a part of. The
	// Authenticators slice should contain only authenticators that are provided
	// by other APIs; see their documentation for which they provide.
	UsesDBs []string
}

// FillDefaults returns a new *Common identical to cc but with unset values set
// to their defaults and values normalized.
func (cc *CommonConfig) FillDefaults() APIConfig {
	newCC := new(CommonConfig)
	*newCC = *cc

	if newCC.Base == "" {
		newCC.Base = "/"
	}

	return newCC
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

// Validate returns an error if the Config has invalid field values set. Empty
// and unset values are considered invalid; if defaults are intended to be used,
// call Validate on the return value of FillDefaults.
func (cc *CommonConfig) Validate() error {
	if err := validateBaseURI(cc.Base); err != nil {
		return fmt.Errorf(ConfigKeyAPIBase+": %w", err)
	}

	return nil
}

func (cc *CommonConfig) Common() CommonConfig {
	return *cc
}

func (cc *CommonConfig) Keys() []string {
	return []string{ConfigKeyAPIName, ConfigKeyAPIEnabled, ConfigKeyAPIBase, ConfigKeyAPIUsesDBs}
}

func (cc *CommonConfig) Get(key string) interface{} {
	switch strings.ToLower(key) {
	case ConfigKeyAPIName:
		return cc.Name
	case ConfigKeyAPIEnabled:
		return cc.Enabled
	case ConfigKeyAPIBase:
		return cc.Base
	case ConfigKeyAPIUsesDBs:
		return cc.UsesDBs
	default:
		return nil
	}
}

func (cc *CommonConfig) Set(key string, value interface{}) error {
	switch strings.ToLower(key) {
	case ConfigKeyAPIName:
		if valueStr, ok := value.(string); ok {
			cc.Name = valueStr
			return nil
		} else {
			return fmt.Errorf("key '"+ConfigKeyAPIName+"' requires a string but got a %T", value)
		}
	case ConfigKeyAPIEnabled:
		if valueBool, ok := value.(bool); ok {
			cc.Enabled = valueBool
			return nil
		} else {
			return fmt.Errorf("key '"+ConfigKeyAPIEnabled+"' requires a bool but got a %T", value)
		}
	case ConfigKeyAPIBase:
		if valueStr, ok := value.(string); ok {
			cc.Base = valueStr
			return nil
		} else {
			return fmt.Errorf("key '"+ConfigKeyAPIBase+"' requires a string but got a %T", value)
		}
	case ConfigKeyAPIUsesDBs:
		if valueStrSlice, ok := value.([]string); ok {
			cc.UsesDBs = valueStrSlice
			return nil
		} else {
			return fmt.Errorf("key '"+ConfigKeyAPIUsesDBs+"' requires a []string but got a %T", value)
		}
	default:
		return fmt.Errorf("not a valid key: %q", key)
	}
}

func (cc *CommonConfig) SetFromString(key string, value string) error {
	switch strings.ToLower(key) {
	case ConfigKeyAPIName, ConfigKeyAPIBase:
		return cc.Set(key, value)
	case ConfigKeyAPIEnabled:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		return cc.Set(key, b)
	case ConfigKeyAPIUsesDBs:
		if value == "" {
			return cc.Set(key, []string{})
		}
		dbsStrSlice := strings.Split(value, ",")
		return cc.Set(key, dbsStrSlice)
	default:
		return fmt.Errorf("not a valid key: %q", key)
	}
}

// LogConfig contains logging options. Loggers are provided to APIs in the form of
// sub-components of the primary logger. If logging is enabled, the Jelly server
// will configure the logger of the chosen provider and use it for messages
// about the server itself, and will pass a sub-component logger to each API to
// use for its own logging.
type LogConfig struct {
	// Enabled is whether to enable built-in logging statements.
	Enabled bool

	// Provider must be the name of one of the logging providers. If set to
	// None or unset, it will default to logging.Jellog.
	Provider LogProvider

	// File to log to. If not set, all logging will be done to stderr and it
	// will display all logging statements. If set, the file will receive all
	// levels of log messages and stderr will show only those of Info level or
	// higher.
	File string
}

func (log LogConfig) FillDefaults() LogConfig {
	newLog := log

	if newLog.Provider == NoLog {
		newLog.Provider = Jellog
	}

	return newLog
}

func (g LogConfig) Validate() error {
	if g.Provider == NoLog {
		return fmt.Errorf("provider: must not be empty")
	}

	return nil
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

	// The main auth provider to use for the project. Must be the
	// fully-qualified name of it, e.g. COMPONENT.PROVIDER format.
	MainAuthProvider string
}

func (g Globals) FillDefaults() Globals {
	newG := g

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

// Database contains configuration settings for connecting to a persistence
// layer.
type DatabaseConfig struct {
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
// types.DatabaseInMemory. If OWDB File is not set, it is changed to "db.owv".
func (db DatabaseConfig) FillDefaults() DatabaseConfig {
	newDB := db

	if newDB.Type == DatabaseNone {
		newDB = DatabaseConfig{Type: DatabaseInMemory}
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
func (db DatabaseConfig) Validate() error {
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
func ParseDBConnString(s string) (DatabaseConfig, error) {
	var paramStr string
	dbParts := strings.SplitN(s, ":", 2)

	if len(dbParts) == 2 {
		paramStr = strings.TrimSpace(dbParts[1])
	}

	// parse the first section into a type, from there we can determine if
	// further params are required.
	dbEng, err := ParseDBType(strings.TrimSpace(dbParts[0]))
	if err != nil {
		return DatabaseConfig{}, fmt.Errorf("unsupported DB engine: %w", err)
	}

	switch dbEng {
	case DatabaseInMemory:
		// there cannot be any other options
		if paramStr != "" {
			return DatabaseConfig{}, fmt.Errorf("unsupported param(s) for in-memory DB engine: %s", paramStr)
		}

		return DatabaseConfig{Type: DatabaseInMemory}, nil
	case DatabaseSQLite:
		// there must be options
		if paramStr == "" {
			return DatabaseConfig{}, fmt.Errorf("sqlite DB engine requires path to data directory after ':'")
		}

		// the only option is the DB path, as long as the param str isn't
		// literally blank, it can be used.

		// convert slashes to correct type
		dd := filepath.FromSlash(paramStr)
		return DatabaseConfig{Type: DatabaseSQLite, DataDir: dd}, nil
	case DatabaseOWDB:
		// there must be options
		if paramStr == "" {
			return DatabaseConfig{}, fmt.Errorf("owdb DB engine requires qualified path to data directory after ':'")
		}

		// split the arguments, simply go through and ignore unescaped
		params, err := parseParamsMap(paramStr)
		if err != nil {
			return DatabaseConfig{}, err
		}

		db := DatabaseConfig{Type: DatabaseOWDB}

		if val, ok := params["dir"]; ok {
			db.DataDir = filepath.FromSlash(val)
		} else {
			return DatabaseConfig{}, fmt.Errorf("owdb DB engine params missing qualified path to data directory in key 'dir'")
		}

		if val, ok := params["file"]; ok {
			db.DataFile = val
		} else {
			db.DataFile = "db.owv"
		}
		return db, nil
	case DatabaseNone:
		// not allowed
		return DatabaseConfig{}, fmt.Errorf("cannot specify DB engine 'none' (perhaps you wanted 'inmem'?)")
	default:
		// unknown
		return DatabaseConfig{}, fmt.Errorf("unknown DB engine: %q", dbEng.String())
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

// Config is a complete configuration for a server. It contains all parameters
// that can be used to configure its operation.
type Config struct {

	// Globals is all variables shared with initialization of all APIs.
	Globals Globals

	// DBs is the configurations to use for connecting to databases and other
	// persistence layers. If not provided, it will be set to a configuration
	// for using an in-memory persistence layer.
	DBs map[string]DatabaseConfig

	// APIs is the configuration for each API that will be included in a
	// configured jelly framework server. Each APIConfig must return a
	// CommonConfig whose Name is either set to blank or to the key that maps to
	// it.
	APIs map[string]APIConfig

	// Log is used to configure the built-in logging system. It can be left
	// blank to disable logging entirely.
	Log LogConfig

	// Format is the format of config, used in Dump. It will only be
	// automatically set if the Config was created via a call to Load.
	Format Format
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

// configGet returns a value from an types.APIConfig. Panics if the given value is not of
// the given type or if there is an error retrieving it or if the given key does
// not exist.
func configGet[E any](api APIConfig, key string) E {

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

// FillDefaults returns a new Config identical to cfg but with unset values
// set to their defaults.
func (cfg Config) FillDefaults() Config {
	newCFG := cfg

	for name, db := range newCFG.DBs {
		newCFG.DBs[name] = db.FillDefaults()
	}
	newCFG.Globals = newCFG.Globals.FillDefaults()
	for name, api := range newCFG.APIs {
		if configGet[string](api, ConfigKeyAPIName) == "" {
			if err := api.Set(ConfigKeyAPIName, name); err != nil {
				panic(fmt.Sprintf("setting a config global failed; should never happen: %v", err))
			}
		}
		api = api.FillDefaults()
		newCFG.APIs[name] = api
	}
	newCFG.Log = newCFG.Log.FillDefaults()

	// if the user has enabled the jellyauth API, set defaults now.
	if authConf, ok := newCFG.APIs["jellyauth"]; ok {
		// make shore the first DB exists
		if configGet[bool](authConf, ConfigKeyAPIEnabled) {
			dbs := configGet[[]string](authConf, ConfigKeyAPIUsesDBs)
			if len(dbs) > 0 {
				// make shore this DB exists
				if _, ok := newCFG.DBs[dbs[0]]; !ok {
					newCFG.DBs[dbs[0]] = DatabaseConfig{Type: DatabaseInMemory, Connector: "authuser"}.FillDefaults()
				}
			}
			if newCFG.Globals.MainAuthProvider == "" {
				newCFG.Globals.MainAuthProvider = "jellyauth.jwt"
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
	if err := cfg.Log.Validate(); err != nil {
		return fmt.Errorf("logging: %w", err)
	}
	for name, db := range cfg.DBs {
		if err := db.Validate(); err != nil {
			return fmt.Errorf("dbs: %s: %w", name, err)
		}
	}
	for name, api := range cfg.APIs {
		com := cfg.APIs[name].Common()

		if name != com.Name && com.Name != "" {
			return fmt.Errorf("%s: name mismatch; API.Name is set to %q", name, com.Name)
		}
		if err := api.Validate(); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}

	// all possible values for UnauthDelayMS are valid, so no need to check it

	return nil
}
