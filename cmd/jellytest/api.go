package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dekarrin/jelly/config"
)

const (
	ConfigKeyMessages = "messages"
	ConfigKeyRudeness = "rudeness"
)

type HelloConfig struct {
	CommonConf config.Common

	Rudeness float64
}

type EchoConfig struct {
	CommonConf config.Common

	Messages []string
}

// FillDefaults returns a new *EchoConfig identical to cfg but with unset values
// set to their defaults and values normalized.
func (cfg *EchoConfig) FillDefaults() config.APIConfig {
	newCFG := new(EchoConfig)
	*newCFG = *cfg

	newCFG.CommonConf = *newCFG.CommonConf.FillDefaults()

	if len(newCFG.Messages) < 1 {
		newCFG.Messages = []string{"%s"}
	}

	return newCFG
}

// FillDefaults returns a new *HelloConfig identical to cfg but with unset
// values set to their defaults and values normalized.
func (cfg *HelloConfig) FillDefaults() config.APIConfig {
	newCFG := new(HelloConfig)
	*newCFG = *cfg

	newCFG.CommonConf = *newCFG.CommonConf.FillDefaults()

	if newCFG.Rudeness <= 0.00000001 {
		newCFG.Rudeness = 1.0
	}

	return newCFG
}

// Validate returns an error if the Config has invalid field values set. Empty
// and unset values are considered invalid; if defaults are intended to be used,
// call Validate on the return value of FillDefaults.
func (cfg *EchoConfig) Validate() error {
	if err := cfg.CommonConf.Validate(); err != nil {
		return err
	}

	if len(cfg.Messages) < 1 {
		return fmt.Errorf("messages: must exist and have at least one entry")
	}

	return nil
}

// Validate returns an error if the Config has invalid field values set. Empty
// and unset values are considered invalid; if defaults are intended to be used,
// call Validate on the return value of FillDefaults.
func (cfg *HelloConfig) Validate() error {
	if err := cfg.CommonConf.Validate(); err != nil {
		return err
	}

	if cfg.Rudeness <= 0.00000001 {
		return fmt.Errorf("rudeness: must be greater than 0")
	}
	if cfg.Rudeness > 100.0 {
		return fmt.Errorf("rudeness: must be less than 100")
	}

	return nil
}

func (cfg *EchoConfig) Common() config.Common {
	return cfg.CommonConf
}

func (cfg *HelloConfig) Common() config.Common {
	return cfg.CommonConf
}

func (cfg *EchoConfig) Keys() []string {
	keys := cfg.CommonConf.Keys()
	keys = append(keys, ConfigKeyMessages)
	return keys
}

func (cfg *HelloConfig) Keys() []string {
	keys := cfg.CommonConf.Keys()
	keys = append(keys, ConfigKeyRudeness)
	return keys
}

func (cfg *EchoConfig) Get(key string) interface{} {
	switch strings.ToLower(key) {
	case ConfigKeyMessages:
		return cfg.Messages
	default:
		return cfg.CommonConf.Get(key)
	}
}

func (cfg *HelloConfig) Get(key string) interface{} {
	switch strings.ToLower(key) {
	case ConfigKeyRudeness:
		return cfg.Rudeness
	default:
		return cfg.CommonConf.Get(key)
	}
}

func (cfg *EchoConfig) Set(key string, value interface{}) error {
	switch strings.ToLower(key) {
	case ConfigKeyMessages:
		if valueStr, ok := value.([]string); ok {
			cfg.Messages = valueStr
			return nil
		} else {
			return fmt.Errorf("key '"+ConfigKeyMessages+"' requires a []string but got a %T", value)
		}
	default:
		return cfg.CommonConf.Set(key, value)
	}
}

func (cfg *HelloConfig) Set(key string, value interface{}) error {
	switch strings.ToLower(key) {
	case ConfigKeyRudeness:
		if valueStr, ok := value.(float64); ok {
			cfg.Rudeness = valueStr
			return nil
		} else {
			return fmt.Errorf("key '"+ConfigKeyRudeness+"' requires a []string but got a %T", value)
		}
	default:
		return cfg.CommonConf.Set(key, value)
	}
}

func (cfg *EchoConfig) SetFromString(key string, value string) error {
	switch strings.ToLower(key) {
	case ConfigKeyMessages:
		if value == "" {
			return cfg.Set(key, []string{})
		}
		msgsStrSlice := strings.Split(value, ",")
		return cfg.Set(key, msgsStrSlice)
	default:
		return cfg.CommonConf.SetFromString(key, value)
	}
}

func (cfg *HelloConfig) SetFromString(key string, value string) error {
	switch strings.ToLower(key) {
	case ConfigKeyRudeness:
		val, err := strconv.ParseFloat(s string, bitSize int)
		if value == "" {
			return cfg.Set(key, []string{})
		}
		msgsStrSlice := strings.Split(value, ",")
		return cfg.Set(key, msgsStrSlice)
	default:
		return cfg.CommonConf.SetFromString(key, value)
	}
}
