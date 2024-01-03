package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var apiConfigProviders = map[string]func() APIConfig{}

type marshaledDatabase struct {
	Type string `yaml:"type" json:"type"`
	Dir  string `yaml:"dir" json:"dir"`
	File string `yaml:"file" json:"file"`
}

type marshaledUses struct {
	DBs            []string `yaml:"dbs" json:"dbs"`
	Authenticators []string `yaml:"authenticators" json:"authenticators"`
}

type marshaledAPI struct {
	Base    string        `yaml:"base" json:"base"`
	Enabled bool          `yaml:"enabled" json:"enabled"`
	Uses    marshaledUses `yaml:"uses" json:"uses"`

	others map[string]interface{}
}

type marshaledConfig struct {
	Listen      string                       `yaml:"listen" json:"listen"`
	Base        string                       `yaml:"base" json:"base"`
	DBs         map[string]marshaledDatabase `yaml:"dbs" json:"dbs"`
	UnauthDelay int                          `yaml:"unauth_delay" json:"unauth_delay"`
	APIs        map[string]marshaledAPI      `yaml:"apis" json:"apis"`
}

// Load loads a configuration from a JSON or YAML file. The format of the file
// is determined by examining its extension; files ending in .json are parsed as
// JSON files, and files ending in .yaml or .yml are parsed as YAML files. Other
// extensions are not supported. The extension is not case-sensitive.
//
// Ensure Register is called with all config sections that will be present
// before calling Load.
func Load(file string) (Config, error) {
	var cfg Config
	var mc marshaledConfig

	switch filepath.Ext(strings.ToLower(file)) {
	case ".json":
		// json file
		data, err := os.ReadFile(file)
		if err != nil {
			return cfg, fmt.Errorf("%q: %w", file, err)
		}
		err = json.Unmarshal(data, &mc)
		if err != nil {
			return cfg, fmt.Errorf("%q: %w", file, err)
		}
	case ".yaml", ".yml":
		// yaml file
		data, err := os.ReadFile(file)
		if err != nil {
			return cfg, fmt.Errorf("%q: %w", file, err)
		}
		err = yaml.Unmarshal(data, &mc)
		if err != nil {
			return cfg, fmt.Errorf("%q: %w", file, err)
		}
	default:
		return cfg, fmt.Errorf("%q: incompatible format; must be .json, .yml, or .yaml file", file)
	}

	err := cfg.unmarshal(mc)
	return cfg, err
}

// Register marks an API config name as being in use and gives a provider
// function to create instances of an APIConfig to use for that config section.
//
// It must be called in order to use custom config sections for APIs.
func Register(name string, provider func() APIConfig) error {
	normName := strings.ToLower(name)
	if _, ok := apiConfigProviders[normName]; ok {
		return fmt.Errorf("duplicate config section name: %q is already registered", name)
	}
	if provider == nil {
		return fmt.Errorf("APIConfig provider function cannot be nil")
	}
	apiConfigProviders[normName] = provider
	return nil
}

func unmarshalAPI(ma marshaledAPI, name string) (APIConfig, error) {
	nameNorm := strings.ToLower(name)

	prov, ok := apiConfigProviders[nameNorm]
	if !ok {
		return nil, fmt.Errorf("no provider exists for API config section named %q", nameNorm)
	}

	api := prov()

	if err := api.Set(KeyAPIName, nameNorm); err != nil {
		return nil, fmt.Errorf(KeyAPIName+": %w", err)
	}
	if err := api.Set(KeyAPIEnabled, ma.Enabled); err != nil {
		return nil, fmt.Errorf(KeyAPIEnabled+": %w", err)
	}
	if err := api.Set(KeyAPIBase, ma.Base); err != nil {
		return nil, fmt.Errorf(KeyAPIBase+": %w", err)
	}
	if err := api.Set(KeyAPIUsesDBs, ma.Uses.DBs); err != nil {
		return nil, fmt.Errorf(KeyAPIUsesDBs+": %w", err)
	}
	if err := api.Set(KeyAPIUsesAuthenticators, ma.Uses.Authenticators); err != nil {
		return nil, fmt.Errorf(KeyAPIUsesAuthenticators+": %w", err)
	}

	for k, v := range ma.others {
		kNorm := strings.ToLower(k)
		if err := api.Set(kNorm, v); err != nil {
			return nil, fmt.Errorf("%s: %w", kNorm, err)
		}
	}

	return api, nil
}

// unmarshal completely replaces all attributes.
//
// does no validation except that which is required for parsing.
func (cfg *Globals) unmarshal(m marshaledConfig) error {
	var err error

	// listen address part...
	listenAddr := m.Listen
	bindParts := strings.SplitN(listenAddr, ":", 2)
	if len(bindParts) != 2 {
		return fmt.Errorf("listen: not in \"ADDRESS:PORT\" or \":PORT\" format")
	}
	cfg.Address = bindParts[0]
	cfg.Port, err = strconv.Atoi(bindParts[1])
	if err != nil {
		return fmt.Errorf("listen: %q is not a valid port number", bindParts[1])
	}

	// ...and the rest
	cfg.URIBase = m.Base
	cfg.UnauthDelayMillis = m.UnauthDelay

	return nil
}

// unmarshal completely replaces all attributes except DBConnector with the
// values or missing values in the marshaledConfig.
//
// does no validation except that which is required for parsing.
func (cfg *Config) unmarshal(m marshaledConfig) error {
	if err := cfg.Globals.unmarshal(m); err != nil {
		return err
	}
	cfg.DBs = map[string]Database{}
	for n, marshaledDB := range m.DBs {
		var db Database
		err := db.unmarshal(marshaledDB)
		if err != nil {
			return fmt.Errorf("dbs: %s: %w", n, err)
		}
		cfg.DBs[n] = db
	}
	cfg.APIs = map[string]APIConfig{}
	for n, mAPI := range m.APIs {
		api, err := unmarshalAPI(mAPI, n)
		if err != nil {
			return fmt.Errorf("%s: %w", n, err)
		}
		cfg.APIs[n] = api
	}

	return nil
}

// unmarshal completely replaces all attributes with the values or missing
// values in the marshaledDatabase.
//
// does no validation except that which is required for parsing.
func (db *Database) unmarshal(m marshaledDatabase) error {
	var err error

	db.Type, err = ParseDBType(m.Type)
	if err != nil {
		return fmt.Errorf("type: %w", err)
	}

	db.DataDir = m.Dir
	db.DataFile = m.File

	return nil
}

func (mc *marshaledConfig) UnmarshalYAML(n *yaml.Node) error {
	var m map[string]interface{}
	if err := yaml.Unmarshal([]byte(n.Value), &m); err != nil {
		return err
	}

	for k, v := range m {
		delete(m, k)
		m[strings.ToLower(k)] = v
	}

	if listen, ok := m["listen"]; ok {
		listenStr, convOk := listen.(string)
		if !convOk {
			return fmt.Errorf("listen: should be a string but was of type %T", listen)
		}
		mc.Listen = listenStr
		delete(m, "listen")
	}
	if base, ok := m["base"]; ok {
		baseStr, convOk := base.(string)
		if !convOk {
			return fmt.Errorf("base: should be a string but was of type %T", base)
		}
		mc.Base = baseStr
		delete(m, "base")
	}
	if unauthDelay, ok := m["unauth_delay"]; ok {
		unauthDelayInt, convOk := unauthDelay.(int)
		if !convOk {
			return fmt.Errorf("unauth_delay: should be an int but was of type %T", unauthDelay)
		}
		mc.UnauthDelay = unauthDelayInt
		delete(m, "unauth_delay")
	}

	mc.DBs = map[string]marshaledDatabase{}
	if dbs, ok := m["dbs"]; ok {
		dbsSlice, convOk := dbs.(map[string]interface{})
		if !convOk {
			return fmt.Errorf("dbs: should be an object but was of type %T", dbs)
		}
		for name, dbUntyped := range dbsSlice {
			encoded, err := yaml.Marshal(dbUntyped)
			if err != nil {
				return fmt.Errorf("dbs: %s: re-encode: %w", name, err)
			}
			var db marshaledDatabase
			err = yaml.Unmarshal(encoded, &db)
			if err != nil {
				return fmt.Errorf("dbs: %s: %w", name, err)
			}
			mc.DBs[name] = db
		}
		delete(m, "dbs")
	}

	// ...then, all the rest are API sections that are their own config
	mc.APIs = map[string]marshaledAPI{}
	for name, apiUntyped := range m {
		apiMap, convOk := apiUntyped.(map[string]interface{})
		if !convOk {
			return fmt.Errorf("%s: should be an object but was of type %T", name, apiUntyped)
		}

		encoded, err := yaml.Marshal(apiMap)
		if err != nil {
			return fmt.Errorf("%s: re-encode: %w", name, err)
		}
		var api marshaledAPI
		err = yaml.Unmarshal(encoded, &api)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}

		// make everyfin case-insensitive
		for k, v := range apiMap {
			delete(apiMap, k)
			apiMap[strings.ToLower(k)] = v
		}

		// delete the base attributes from the map
		delete(apiMap, "base")
		delete(apiMap, "uses")
		delete(apiMap, "enabled")

		api.others = map[string]interface{}{}
		for k, v := range apiMap {
			api.others[k] = v
		}

		mc.APIs[name] = api
	}

	return nil
}

func (mc *marshaledConfig) UnmarshalJSON(b []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	for k, v := range m {
		delete(m, k)
		m[strings.ToLower(k)] = v
	}

	if listen, ok := m["listen"]; ok {
		listenStr, convOk := listen.(string)
		if !convOk {
			return fmt.Errorf("listen: should be a string but was of type %T", listen)
		}
		mc.Listen = listenStr
		delete(m, "listen")
	}
	if base, ok := m["base"]; ok {
		baseStr, convOk := base.(string)
		if !convOk {
			return fmt.Errorf("base: should be a string but was of type %T", base)
		}
		mc.Base = baseStr
		delete(m, "base")
	}
	if unauthDelay, ok := m["unauth_delay"]; ok {
		unauthDelayInt, convOk := unauthDelay.(int)
		if !convOk {
			return fmt.Errorf("unauth_delay: should be an int but was of type %T", unauthDelay)
		}
		mc.UnauthDelay = unauthDelayInt
		delete(m, "unauth_delay")
	}

	mc.DBs = map[string]marshaledDatabase{}
	if dbs, ok := m["dbs"]; ok {
		dbsSlice, convOk := dbs.(map[string]interface{})
		if !convOk {
			return fmt.Errorf("dbs: should be an object but was of type %T", dbs)
		}
		for name, dbUntyped := range dbsSlice {
			encoded, err := json.Marshal(dbUntyped)
			if err != nil {
				return fmt.Errorf("dbs: %s: re-encode: %w", name, err)
			}
			var db marshaledDatabase
			err = json.Unmarshal(encoded, &db)
			if err != nil {
				return fmt.Errorf("dbs: %s: %w", name, err)
			}
			mc.DBs[name] = db
		}
		delete(m, "dbs")
	}

	// ...then, all the rest are API sections that are their own config
	mc.APIs = map[string]marshaledAPI{}
	for name, apiUntyped := range m {
		apiMap, convOk := apiUntyped.(map[string]interface{})
		if !convOk {
			return fmt.Errorf("%s: should be an object but was of type %T", name, apiUntyped)
		}

		encoded, err := json.Marshal(apiMap)
		if err != nil {
			return fmt.Errorf("%s: re-encode: %w", name, err)
		}
		var api marshaledAPI
		err = json.Unmarshal(encoded, &api)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}

		// make everyfin case-insensitive
		for k, v := range apiMap {
			delete(apiMap, k)
			apiMap[strings.ToLower(k)] = v
		}

		// delete the base attributes from the map
		delete(apiMap, "base")
		delete(apiMap, "uses")
		delete(apiMap, "enabled")

		api.others = map[string]interface{}{}
		for k, v := range apiMap {
			api.others[k] = v
		}

		mc.APIs[name] = api
	}

	return nil
}
