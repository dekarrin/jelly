// Package config contains configuration options for the server as well as
// various config contstants.
package config

import (
	"fmt"
	"strings"

	"github.com/dekarrin/jelly/logging"
)

// Log contains logging options. Loggers are provided to APIs in the form of
// sub-components of the primary logger. If logging is enabled, the Jelly server
// will configure the logger of the chosen provider and use it for messages
// about the server itself, and will pass a sub-component logger to each API to
// use for its own logging.
type Log struct {
	// Enabled is whether to enable built-in logging statements.
	Enabled bool

	// Provider must be the name of one of the logging providers. If set to
	// None or unset, it will default to logging.Jellog.
	Provider logging.Provider

	// File to log to. If not set, all logging will be done to stderr and it
	// will display all logging statements. If set, the file will receive all
	// levels of log messages and stderr will show only those of Info level or
	// higher.
	File string
}

func (log Log) Create() (logging.Logger, error) {
	return logging.New(log.Provider, log.File)
}

func (log Log) FillDefaults() Log {
	newLog := log

	if newLog.Provider == logging.None {
		newLog.Provider = logging.Jellog
	}

	return newLog
}

func (g Log) Validate() error {
	if g.Provider == logging.None {
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

	// Log is used to configure the built-in logging system. It can be left
	// blank to disable logging entirely.
	Log Log

	// DBConnector is a custom DB connector for overriding the defaults, which
	// only provide jeldao.Store instances that are associated with the built-in
	// authentication and login functionality.
	//
	// Any fields in the Connector that are set to nil will use the default
	// connection method for that DB type.
	DBConnector Connector

	// origFormat is the format of config, used in Dump.
	origFormat Format
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
		if Get[string](api, KeyAPIName) == "" {
			if err := api.Set(KeyAPIName, name); err != nil {
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
		if Get[bool](authConf, KeyAPIEnabled) {
			dbs := Get[[]string](authConf, KeyAPIUsesDBs)
			if len(dbs) > 0 {
				// make shore this DB exists
				if _, ok := newCFG.DBs[dbs[0]]; !ok {
					newCFG.DBs[dbs[0]] = Database{Type: DatabaseInMemory}.FillDefaults()
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

// Get returns a value from an APIConfig. Panics if the given value is not of
// the given type or if there is an error retrieving it or if the given key does
// not exist.
func Get[E any](api APIConfig, key string) E {
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

func apiHas(api APIConfig, key string) bool {
	needle := strings.ToLower(key)

	for _, k := range api.Keys() {
		if strings.ToLower(k) == needle {
			return true
		}
	}
	return false
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
