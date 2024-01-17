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
	Dir  string `yaml:"dir,omitempty" json:"dir,omitempty"`
	File string `yaml:"file,omitempty" json:"file,omitempty"`
}

type marshaledAPI struct {
	Base    string   `yaml:"base" json:"base"`
	Enabled bool     `yaml:"enabled" json:"enabled"`
	Uses    []string `yaml:"uses" json:"uses"`

	others map[string]interface{}
}

func (mc marshaledAPI) marshalMap() map[string]interface{} {
	m := map[string]interface{}{}

	for name, other := range mc.others {
		m[name] = other
	}

	m["base"] = mc.Base
	m["enabled"] = mc.Enabled
	m["uses"] = mc.Uses

	return m
}

func (mc marshaledAPI) MarshalYAML() (interface{}, error) {
	return mc.marshalMap(), nil
}

func (mc marshaledAPI) MarshalJSON() ([]byte, error) {
	return json.Marshal(mc.marshalMap())
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
	File     string `yaml:"file,omitempty" json:"file,omitempty"`
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

	cfg.origFormat = f
	err = cfg.unmarshal(mc)
	return cfg, err
}

func (f Format) Encode(c Config) ([]byte, error) {
	mc := c.marshal()
	var err error
	var data []byte

	switch f {
	case JSON:
		data, err = json.Marshal(mc)
	case YAML:
		data, err = yaml.Marshal(mc)
	default:
		return nil, fmt.Errorf("cannot marshal data in format %q", f.String())
	}

	return data, err
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
	ext = strings.TrimPrefix(ext, ".")

	for _, f := range SupportedFormats() {
		for _, checkedExt := range f.Extensions() {
			checkedExt = strings.ToLower(checkedExt)
			checkedExt = strings.TrimPrefix(checkedExt, ".")
			if ext == strings.ToLower(checkedExt) {
				return f
			}
		}
	}

	return NoFormat
}

// Dump dumps the configuration into the bytes in a formatted file. This is the
// complete representation of the current state of the Config, and if parsed by
// Load, would result in an equivalent config.
//
// The config will be dumped in the same format it was loaded with, or will
// default to YAML if the cfg was created without loading from a data stream. To
// encode a Config in a specific format, call Encode(cfg) on the desired Format.
//
// This function will cause a panic if there is a problem marshaling the config
// data in its format.
func (cfg Config) Dump() []byte {
	f := cfg.origFormat
	if f == NoFormat {
		f = YAML
	}
	b, err := f.Encode(cfg)
	if err != nil {
		panic(fmt.Sprintf("format encoding failed: %v", err))
	}
	return b
}

// Load loads a configuration from a JSON or YAML file. The format of the file
// is determined by examining its extension; files ending in .json are parsed as
// JSON files, and files ending in .yaml or .yml are parsed as YAML files. Other
// extensions are not supported. The extension is not case-sensitive.
//
// Ensure Register is called with all config sections that will be present
// before calling Load.
func Load(file string) (Config, error) {
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

		return Config{}, fmt.Errorf("%s: incompatible format; must be a %s file", file, msg.String())
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", file, err)
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

func marshalAPI(api APIConfig) marshaledAPI {
	ma := marshaledAPI{
		Enabled: Get[bool](api, KeyAPIEnabled),
		Base:    Get[string](api, KeyAPIBase),
		Uses:    Get[[]string](api, KeyAPIUsesDBs),
		others:  map[string]interface{}{},
	}

	commonKeys := map[string]struct{}{}
	for _, ck := range (&Common{}).Keys() {
		commonKeys[ck] = struct{}{}
	}

	for _, key := range api.Keys() {
		// skip common keys; they are already covered above
		if _, isCommonKey := commonKeys[key]; isCommonKey {
			continue
		}

		value := api.Get(key)

		if slValue, ok := value.([]byte); ok {
			value = string(slValue)
		}
		ma.others[key] = value
	}

	return ma
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

// marshal returns the marshaledLog that would re-create Log if passed to
// unmarshal.
func (log Log) marshal() marshaledLog {
	return marshaledLog{
		Enabled:  log.Enabled,
		Provider: log.Provider.String(),
		File:     log.File,
	}
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

// marshalToConfig modifies the given marshaledConfig such that it would
// re-create cfg when it is passed to unmarshal.
func (cfg Globals) marshalToConfig(mc *marshaledConfig) {
	mc.Listen = fmt.Sprintf("%s:%d", cfg.Address, cfg.Port)
	mc.Base = cfg.URIBase
	mc.UnauthDelay = cfg.UnauthDelayMillis
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

// marshal converts a config to the marshaledConfig that would recreate it if
// passed to unmarshal.
func (cfg Config) marshal() marshaledConfig {
	mc := marshaledConfig{
		DBs:     map[string]marshaledDatabase{},
		APIs:    map[string]marshaledAPI{},
		Logging: cfg.Log.marshal(),
	}

	cfg.Globals.marshalToConfig(&mc)
	for n, db := range cfg.DBs {
		mDB := db.marshal()
		mc.DBs[n] = mDB
	}
	for n, api := range cfg.APIs {
		mAPI := marshalAPI(api)
		mc.APIs[n] = mAPI
	}

	return mc
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

// marshal converts db to the marshaledDatabase that would recreate it if
// passed to unmarshal.
func (db Database) marshal() marshaledDatabase {
	return marshaledDatabase{
		Type: db.Type.String(),
		Dir:  db.DataDir,
		File: db.DataFile,
	}
}

func (mc *marshaledConfig) unmarshalMap(m map[string]interface{}, unmarshalFn func([]byte, interface{}) error, marshalFn func(interface{}) ([]byte, error)) error {
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
	if loggingUntyped, ok := m["logging"]; ok {
		loggingObj, convOk := loggingUntyped.(map[string]interface{})
		if !convOk {
			return fmt.Errorf("logging: should be an object but was of type %T", loggingUntyped)
		}
		encoded, err := marshalFn(loggingObj)
		if err != nil {
			return fmt.Errorf("logging: re-encode: %w", err)
		}
		err = unmarshalFn(encoded, &mc.Logging)
		if err != nil {
			return fmt.Errorf("logging: %w", err)
		}
		delete(m, "logging")
	}

	mc.DBs = map[string]marshaledDatabase{}
	if dbs, ok := m["dbs"]; ok {
		dbsObj, convOk := dbs.(map[string]interface{})
		if !convOk {
			return fmt.Errorf("dbs: should be an object but was of type %T", dbs)
		}
		for name, dbUntyped := range dbsObj {
			encoded, err := marshalFn(dbUntyped)
			if err != nil {
				return fmt.Errorf("dbs: %s: re-encode: %w", name, err)
			}
			var db marshaledDatabase
			err = unmarshalFn(encoded, &db)
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

		encoded, err := marshalFn(apiMap)
		if err != nil {
			return fmt.Errorf("%s: re-encode: %w", name, err)
		}

		var api marshaledAPI
		err = unmarshalFn(encoded, &api)
		if err != nil {
			// okay we need to convert lines to "nth property of"
			//
			// rn we only have error msg lineno correction for yaml; JSON isn't
			// currently tested
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

func (mc *marshaledConfig) UnmarshalYAML(n *yaml.Node) error {
	var m map[string]interface{}
	if err := n.Decode(&m); err != nil {
		return err
	}

	return mc.unmarshalMap(m, yaml.Unmarshal, yaml.Marshal)
}

func (mc marshaledConfig) marshalMap() interface{} {
	m := map[string]interface{}{}

	for n, api := range mc.APIs {
		m[n] = api
	}

	m["logging"] = mc.Logging
	m["base"] = mc.Base
	m["dbs"] = mc.DBs
	m["listen"] = mc.Listen
	m["unauth_delay"] = mc.UnauthDelay

	return m
}

func (mc marshaledConfig) MarshalYAML() (interface{}, error) {
	return mc.marshalMap(), nil
}

func (mc *marshaledConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(mc.marshalMap())
}

func (mc *marshaledConfig) UnmarshalJSON(b []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	return mc.unmarshalMap(m, json.Unmarshal, json.Marshal)
}
