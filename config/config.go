// Package config contains configuration options for the server as well as
// various config contstants.
package config

import (
	"fmt"
	"strings"

	"github.com/dekarrin/jelly/logging"
	"github.com/dekarrin/jelly/types"
)

type ILog interface {
	Create() (types.Logger, error)
	FillDefaults() ILog
	Validate() error
}

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
	Provider types.LogProvider

	// File to log to. If not set, all logging will be done to stderr and it
	// will display all logging statements. If set, the file will receive all
	// levels of log messages and stderr will show only those of Info level or
	// higher.
	File string
}

func (log Log) Create() (types.Logger, error) {
	return logging.New(log.Provider, log.File)
}

func (log Log) FillDefaults() Log {
	newLog := log

	if newLog.Provider == types.NoLog {
		newLog.Provider = types.Jellog
	}

	return newLog
}

func (g Log) Validate() error {
	if g.Provider == types.NoLog {
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
	APIs map[string]types.APIConfig

	// Log is used to configure the built-in logging system. It can be left
	// blank to disable logging entirely.
	Log Log

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
		if Get[string](api, types.ConfigKeyAPIName) == "" {
			if err := api.Set(types.ConfigKeyAPIName, name); err != nil {
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
		if Get[bool](authConf, types.ConfigKeyAPIEnabled) {
			dbs := Get[[]string](authConf, types.ConfigKeyAPIUsesDBs)
			if len(dbs) > 0 {
				// make shore this DB exists
				if _, ok := newCFG.DBs[dbs[0]]; !ok {
					newCFG.DBs[dbs[0]] = Database{Type: types.DatabaseInMemory, Connector: "authuser"}.FillDefaults()
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

// Get returns a value from an types.APIConfig. Panics if the given value is not of
// the given type or if there is an error retrieving it or if the given key does
// not exist.
func Get[E any](api types.APIConfig, key string) E {
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

func apiHas(api types.APIConfig, key string) bool {
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
