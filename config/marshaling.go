package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dekarrin/jelly/logging"
	"gopkg.in/yaml.v3"
)

var apiConfigProviders = map[string]func() APIConfig{}

type marshaledDatabase struct {
	Type string `yaml:"type" json:"type"`
	Dir  string `yaml:"dir" json:"dir"`
	File string `yaml:"file" json:"file"`
}

type marshaledAPI struct {
	Base    string   `yaml:"base" json:"base"`
	Enabled bool     `yaml:"enabled" json:"enabled"`
	Uses    []string `yaml:"uses" json:"uses"`

	others map[string]interface{}
}

type marshaledConfig struct {
	Listen      string                       `yaml:"listen" json:"listen"`
	Base        string                       `yaml:"base" json:"base"`
	DBs         map[string]marshaledDatabase `yaml:"dbs" json:"dbs"`
	UnauthDelay int                          `yaml:"unauth_delay" json:"unauth_delay"`
	APIs        map[string]marshaledAPI      `yaml:"apis" json:"apis"`
	Logging     marshaledLog                 `yaml:"logging" json:"logging"`
}

type marshaledLog struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Provider string `yaml:"provider" json:"provider"`
	File     string `yaml:"file" json:"file"`
}

type Format int

const (
	NoFormat Format = iota
	JSON
	YAML
)

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

func (f Format) Decode(data []byte) (Config, error) {
	var cfg Config
	var mc marshaledConfig
	var err error

	switch f {
	case JSON:
		err = json.Unmarshal(data, &mc)
	case YAML:
		err = yaml.Unmarshal(data, &mc)
	default:
		return cfg, fmt.Errorf("cannot unmarshal data in format %q", f.String())
	}

	if err != nil {
		return cfg, err
	}
	err = cfg.unmarshal(mc)
	return cfg, err
}

// SupportedFormats returns a list of formats that the config module supports
// decoding. Includes all but NoFormat.
func SupportedFormats() []Format {
	return []Format{JSON, YAML}
}

// DetectFormat detects the format of a given configuration file and returns the
// Format that can decode it. Returns NoFormat if the format could not be
// detected.
func DetectFormat(file string) Format {
	ext := strings.ToLower(filepath.Ext(file))

	for _, f := range SupportedFormats() {
		for _, checkedExt := range f.Extensions() {
			if ext == strings.ToLower(checkedExt) {
				return f
			}
		}
	}

	return NoFormat
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

	f := DetectFormat(file)
	if f == NoFormat {
		var msg strings.Builder

		formats := SupportedFormats()
		for i, f := range formats {
			exts := f.Extensions()
			for j, ext := range exts {
				// if on the last ext of the last format and there was at least
				// one before, add a leading "or "
				if j+1 >= len(exts) && i+1 >= len(formats) && msg.Len() > 0 {
					msg.WriteString("or ")
				}

				msg.WriteRune('.')
				msg.WriteString(ext)

				// if there is at least one more extension, add an ", "
				if j+1 < len(exts) || i+1 < len(formats) {
					msg.WriteString(", ")
				}
			}
		}

		return cfg, fmt.Errorf("%s: incompatible format; must be a %s file", file, msg.String())
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return cfg, fmt.Errorf("%s: %w", file, err)
	}

	return f.Decode(data)
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

	var api APIConfig
	prov, ok := apiConfigProviders[nameNorm]
	if ok {
		api = prov()
	} else {
		// fallback - if it fails to provide one, it just gets a common config
		api = &Common{}
	}

	if err := api.Set(KeyAPIName, nameNorm); err != nil {
		return nil, fmt.Errorf(KeyAPIName+": %w", err)
	}
	if err := api.Set(KeyAPIEnabled, ma.Enabled); err != nil {
		return nil, fmt.Errorf(KeyAPIEnabled+": %w", err)
	}
	if err := api.Set(KeyAPIBase, ma.Base); err != nil {
		return nil, fmt.Errorf(KeyAPIBase+": %w", err)
	}
	if err := api.Set(KeyAPIUsesDBs, ma.Uses); err != nil {
		return nil, fmt.Errorf(KeyAPIUsesDBs+": %w", err)
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
func (log *Log) unmarshal(m marshaledLog) error {
	var err error

	log.Enabled = m.Enabled
	log.Provider, err = logging.ParseProvider(m.Provider)
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}
	log.File = m.File

	return nil
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
	if err := cfg.Log.unmarshal(m.Logging); err != nil {
		return fmt.Errorf("logging: %w", err)
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
	if err := n.Decode(&m); err != nil {
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
			// okay we need to convert lines to "nth property of"
			// first, find the line part
			if typeErr, ok := err.(*yaml.TypeError); ok {
				errStr := ""
				for i := range typeErr.Errors {
					if i != 0 {
						errStr += "\n"
					}
					errStr += "key #" + typeErr.Errors[i][len("line "):]
				}
				err = fmt.Errorf("%s", errStr)
			}
			return fmt.Errorf("API %q: %w", name, err)
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
