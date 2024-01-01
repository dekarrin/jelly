package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type marshaledDatabase struct {
	Type string `yaml:"type" json:"type"`
	Dir  string `yaml:"dir" json:"dir"`
	File string `yaml:"file" json:"file"`
}

type marshaledAPI struct {
	Base    string   `yaml:"base" json:"base"`
	Enabled bool     `yaml:"enabled" json:"enabled"`
	Uses    []string `yaml:"uses" json:"uses"`
}

type marshaledConfig struct {
	Listen      string                       `yaml:"listen" json:"listen"`
	Base        string                       `yaml:"base" json:"base"`
	Secret      []byte                       `yaml:"secret" json:"secret"`
	DBs         map[string]marshaledDatabase `yaml:"dbs" json:"dbs"`
	UnauthDelay int                          `yaml:"unauth_delay" json:"unauth_delay"`
	APIs        map[string]marshaledAPI      `yaml:"apis`
}

// Load loads a configuration from a JSON or YAML file. The format of the file
// is determined by examining its extension; files ending in .json are parsed as
// JSON files, and files ending in .yaml or .yml are parsed as YAML files. Other
// extensions are not supported. The extension is not case-sensitive.
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

	// TODO: insert the default loaded stuff (like an auto jellyauth) here.

	err := cfg.unmarshal(mc)
	return cfg, err
}
