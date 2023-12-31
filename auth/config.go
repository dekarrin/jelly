package auth

import (
	"fmt"
	"strings"

	"github.com/dekarrin/jelly/config"
)

const (
	ConfigKeySecret   = "secret"
	ConfigKeySetAdmin = "set_admin"
)

const (
	MaxSecretSize = 64
	MinSecretSize = 32
)

type Config struct {
	CommonConf config.Common

	// Secret is the secret used for signing tokens. If not provided, a default
	// key is used.
	Secret []byte

	// SetAdmin sets the initial admin user in the DB. If it doesn't exist,
	// it's created on initialization. Format must be USERNAME:PASSWORD. This
	// will not default; if none is provided, no user is created. If the user
	// already exists, it will have its password set to the given one.
	SetAdmin string
}

// FillDefaults returns a new *Config identical to cfg but with unset values set
// to their defaults and values normalized.
func (cfg *Config) FillDefaults() config.APIConfig {
	newCFG := new(Config)
	*newCFG = *cfg

	// if no other options are specified except for enable, fill with standard
	if newCFG.CommonConf.Enabled {
		if config.Get[string](newCFG, config.KeyAPIBase) == "" {
			newCFG.Set(config.KeyAPIBase, "/auth")
		}
		if len(config.Get[[]string](newCFG, config.KeyAPIUsesDBs)) < 1 {
			newCFG.Set(config.KeyAPIUsesDBs, []string{"auth"})
		}
	}

	newCFG.CommonConf = newCFG.CommonConf.FillDefaults().Common()

	if newCFG.Secret == nil {
		newCFG.Secret = []byte("DEFAULT_NONPROD_TOKEN_SECRET_DO_NOT_USE")
	}

	return newCFG
}

// Validate returns an error if the Config has invalid field values set. Empty
// and unset values are considered invalid; if defaults are intended to be used,
// call Validate on the return value of FillDefaults.
func (cfg *Config) Validate() error {
	if err := cfg.CommonConf.Validate(); err != nil {
		return err
	}

	if len(cfg.CommonConf.UsesDBs) < 1 {
		return fmt.Errorf("use of at least one database must be declared")
	}

	if len(cfg.Secret) < MinSecretSize {
		return fmt.Errorf(ConfigKeySecret+": must be at least %d bytes, but is %d", MinSecretSize, len(cfg.Secret))
	}
	if len(cfg.Secret) > MaxSecretSize {
		return fmt.Errorf(ConfigKeySecret+": must be no more than %d bytes, but is %d", MaxSecretSize, len(cfg.Secret))
	}

	if cfg.SetAdmin != "" {
		_, _, err := parseSetAdmin(cfg.SetAdmin)
		if err != nil {
			return err
		}
	}

	return nil
}

func parseSetAdmin(s string) (user, pass string, err error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf(ConfigKeySetAdmin + ": not in USERNAME:PASSWORD format")
	}
	if len(parts[0]) < 1 {
		return "", "", fmt.Errorf(ConfigKeySetAdmin + ": username cannot be blank")
	}
	if len(parts[1]) < 1 {
		return "", "", fmt.Errorf(ConfigKeySetAdmin + ": password cannot be blank")
	}

	return parts[0], parts[1], nil
}

func (cfg *Config) Common() config.Common {
	return cfg.CommonConf
}

func (cfg *Config) Keys() []string {
	keys := cfg.CommonConf.Keys()
	keys = append(keys, ConfigKeySecret, ConfigKeySetAdmin)
	return keys
}

func (cfg *Config) Get(key string) interface{} {
	switch strings.ToLower(key) {
	case ConfigKeySecret:
		return cfg.Secret
	case ConfigKeySetAdmin:
		return cfg.SetAdmin
	default:
		return cfg.CommonConf.Get(key)
	}
}

func (cfg *Config) Set(key string, value interface{}) error {
	switch strings.ToLower(key) {
	case ConfigKeySetAdmin:
		if valueStr, ok := value.(string); ok {
			cfg.SetAdmin = valueStr
			return nil
		} else {
			return fmt.Errorf("key '"+ConfigKeySetAdmin+"' requires a string but got a %T", value)
		}
	case ConfigKeySecret:
		if valueStr, ok := value.([]byte); ok {
			cfg.Secret = valueStr
			return nil
		} else {
			return fmt.Errorf("key '"+ConfigKeySecret+"' requires a []byte but got a %T", value)
		}
	default:
		return cfg.CommonConf.Set(key, value)
	}
}

func (cfg *Config) SetFromString(key string, value string) error {
	switch strings.ToLower(key) {
	case ConfigKeySecret, ConfigKeySetAdmin:
		return cfg.Set(key, value)
	default:
		return cfg.CommonConf.SetFromString(key, value)
	}
}
