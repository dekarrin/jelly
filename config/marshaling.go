package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dekarrin/jelly/types"
	"gopkg.in/yaml.v3"
)

// Environment holds all options such as config providers that would normally be
// globally set. Users of Jelly are generally better off using the
// [jelly.Environment] type, as that contains a complete environment spanning
// the config package and any others that contain the concept of registration of
// certain key procedures and types prior to actual use.
type Environment struct {
	apiConfigProviders map[string]func() types.APIConfig

	DisableDefaults bool
}

func (env *Environment) initDefaults() {
	if env.apiConfigProviders == nil {
		env.apiConfigProviders = map[string]func() types.APIConfig{}
	}
}

type marshaledDatabase struct {
	Type      string `yaml:"type" json:"type"`
	Connector string `yaml:"connector" json:"connector"`
	Dir       string `yaml:"dir,omitempty" json:"dir,omitempty"`
	File      string `yaml:"file,omitempty" json:"file,omitempty"`
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
	Listen  string                       `yaml:"listen" json:"listen"`
	Auth    string                       `yaml:"authenticator" json:"authenticator"`
	Base    string                       `yaml:"base" json:"base"`
	DBs     map[string]marshaledDatabase `yaml:"dbs" json:"dbs"`
	APIs    map[string]marshaledAPI      `yaml:"apis" json:"apis"`
	Logging marshaledLog                 `yaml:"logging" json:"logging"`
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

func (f Format) Decode(env *Environment, data []byte) (Config, error) {
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
	err = cfg.unmarshal(env, mc)
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
// Ensure Register is called on the Environment (or an owning jelly.Environment)
// with all config sections that will be present in the loaded file.
func (env *Environment) Load(file string) (Config, error) {
	env.initDefaults()

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

	return f.Decode(env, data)
}

func (env *Environment) Register(name string, provider func() types.APIConfig) error {
	env.initDefaults()

	normName := strings.ToLower(name)
	if _, ok := env.apiConfigProviders[normName]; ok {
		return fmt.Errorf("duplicate config section name: %q is already registered", name)
	}
	if provider == nil {
		return fmt.Errorf("APIConfig provider function cannot be nil")
	}
	env.apiConfigProviders[normName] = provider
	return nil
}

func marshalAPI(api types.APIConfig) marshaledAPI {
	ma := marshaledAPI{
		Enabled: Get[bool](api, types.ConfigKeyAPIEnabled),
		Base:    Get[string](api, types.ConfigKeyAPIBase),
		Uses:    Get[[]string](api, types.ConfigKeyAPIUsesDBs),
		others:  map[string]interface{}{},
	}

	commonKeys := map[string]struct{}{}
	for _, ck := range (&types.CommonConfig{}).Keys() {
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

func unmarshalAPI(env *Environment, ma marshaledAPI, name string) (types.APIConfig, error) {
	env.initDefaults()

	nameNorm := strings.ToLower(name)

	var api types.APIConfig
	prov, ok := env.apiConfigProviders[nameNorm]
	if ok {
		api = prov()
	} else {
		// fallback - if it fails to provide one, it just gets a common config
		api = &types.CommonConfig{}
	}

	if err := api.Set(types.ConfigKeyAPIName, nameNorm); err != nil {
		return nil, fmt.Errorf(types.ConfigKeyAPIName+": %w", err)
	}
	if err := api.Set(types.ConfigKeyAPIEnabled, ma.Enabled); err != nil {
		return nil, fmt.Errorf(types.ConfigKeyAPIEnabled+": %w", err)
	}
	if err := api.Set(types.ConfigKeyAPIBase, ma.Base); err != nil {
		return nil, fmt.Errorf(types.ConfigKeyAPIBase+": %w", err)
	}
	if err := api.Set(types.ConfigKeyAPIUsesDBs, ma.Uses); err != nil {
		return nil, fmt.Errorf(types.ConfigKeyAPIUsesDBs+": %w", err)
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
func unmarshalLog(log *types.LogConfig, m marshaledLog) error {
	var err error

	log.Enabled = m.Enabled
	log.Provider, err = types.ParseLogProvider(m.Provider)
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}
	log.File = m.File

	return nil
}

// marshal returns the marshaledLog that would re-create Log if passed to
// unmarshal.
func marshalLog(log types.LogConfig) marshaledLog {
	return marshaledLog{
		Enabled:  log.Enabled,
		Provider: log.Provider.String(),
		File:     log.File,
	}
}

// unmarshal completely replaces all attributes.
//
// does no validation except that which is required for parsing.
func unmarshalGlobals(cfg *types.Globals, m marshaledConfig) error {
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
	cfg.MainAuthProvider = m.Auth

	return nil
}

// marshalToConfig modifies the given marshaledConfig such that it would
// re-create cfg when it is passed to unmarshal.
func marshalGlobalsToConfig(cfg types.Globals, mc *marshaledConfig) {
	mc.Listen = fmt.Sprintf("%s:%d", cfg.Address, cfg.Port)
	mc.Base = cfg.URIBase
	mc.Auth = cfg.MainAuthProvider
}

// unmarshal completely replaces all attributes except DBConnector with the
// values or missing values in the marshaledConfig.
//
// does no validation except that which is required for parsing.
func (cfg *Config) unmarshal(env *Environment, m marshaledConfig) error {
	if env == nil {
		env = &Environment{}
	}

	if err := unmarshalGlobals(&cfg.Globals, m); err != nil {
		return err
	}
	cfg.DBs = map[string]types.DatabaseConfig{}
	for n, marshaledDB := range m.DBs {
		var db types.DatabaseConfig
		err := unmarshalDatabase(&db, marshaledDB)
		if err != nil {
			return fmt.Errorf("dbs: %s: %w", n, err)
		}
		cfg.DBs[n] = db
	}
	cfg.APIs = map[string]types.APIConfig{}
	for n, mAPI := range m.APIs {
		api, err := unmarshalAPI(env, mAPI, n)
		if err != nil {
			return fmt.Errorf("%s: %w", n, err)
		}
		cfg.APIs[n] = api
	}
	if err := unmarshalLog(&cfg.Log, m.Logging); err != nil {
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
		Logging: marshalLog(cfg.Log),
	}

	marshalGlobalsToConfig(cfg.Globals, &mc)
	for n, db := range cfg.DBs {
		mDB := marshalDatabase(db)
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
func unmarshalDatabase(db *types.DatabaseConfig, m marshaledDatabase) error {
	var err error

	db.Type, err = types.ParseDBType(m.Type)
	if err != nil {
		return fmt.Errorf("type: %w", err)
	}

	db.DataDir = m.Dir
	db.DataFile = m.File
	db.Connector = m.Connector

	return nil
}

// marshal converts db to the marshaledDatabase that would recreate it if
// passed to unmarshal.
func marshalDatabase(db types.DatabaseConfig) marshaledDatabase {
	return marshaledDatabase{
		Type:      db.Type.String(),
		Dir:       db.DataDir,
		File:      db.DataFile,
		Connector: db.Connector,
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
	if authProv, ok := m["authenticator"]; ok {
		authProvStr, convOk := authProv.(string)
		if !convOk {
			return fmt.Errorf("authenticator: should be a string but was of type %T", authProv)
		}
		splitted := strings.Split(authProvStr, ".")
		if len(splitted) != 2 {
			return fmt.Errorf("authenticator: not in COMPONENT.PROVIDER format: %q", authProvStr)
		}
		mc.Auth = authProvStr
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
	m["authenticator"] = mc.Auth

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
