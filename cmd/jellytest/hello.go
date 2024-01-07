package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dekarrin/jelly/config"
	"github.com/dekarrin/jelly/dao"
	"github.com/dekarrin/jelly/logging"
	"github.com/dekarrin/jelly/middle"
	"github.com/go-chi/chi/v5"
)

const (
	ConfigKeyRudeness = "rudeness"
)

type HelloConfig struct {
	CommonConf config.Common

	Rudeness float64
}

// FillDefaults returns a new *HelloConfig identical to cfg but with unset
// values set to their defaults and values normalized.
func (cfg *HelloConfig) FillDefaults() config.APIConfig {
	newCFG := new(HelloConfig)
	*newCFG = *cfg

	newCFG.CommonConf = newCFG.CommonConf.FillDefaults().Common()

	if newCFG.Rudeness <= 0.00000001 {
		newCFG.Rudeness = 1.0
	}

	return newCFG
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

func (cfg *HelloConfig) Common() config.Common {
	return cfg.CommonConf
}

func (cfg *HelloConfig) Keys() []string {
	keys := cfg.CommonConf.Keys()
	keys = append(keys, ConfigKeyRudeness)
	return keys
}

func (cfg *HelloConfig) Get(key string) interface{} {
	switch strings.ToLower(key) {
	case ConfigKeyRudeness:
		return cfg.Rudeness
	default:
		return cfg.CommonConf.Get(key)
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

func (cfg *HelloConfig) SetFromString(key string, value string) error {
	switch strings.ToLower(key) {
	case ConfigKeyRudeness:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		return cfg.Set(key, f)
	default:
		return cfg.CommonConf.SetFromString(key, value)
	}
}

type HelloAPI struct {
	// NiceMessages is a list of polite messages. This is randomly selected from
	// when a nice greeting is requested.
	NiceMessages []string

	// RudeMessages is a list of not-nice messages. This is randomly selected
	// from when a rude greeting is requested.
	RudeMessages []string

	// RudeChance is the liklihood of getting a Rude reply when asking for a
	// random greeting. Float between 0 and 1 for percentage.
	RudeChance float64

	// UnauthDelay is the amount of time that a request will pause before
	// responding with an HTTP-403, HTTP-401, or HTTP-500 to deprioritize such
	// requests from processing and I/O.
	UnauthDelay time.Duration
}

func (echo *HelloAPI) Init(cb config.Bundle, dbs map[string]dao.Store, log logging.Logger) error {
	return fmt.Errorf("not impelmented")
}

func (echo *HelloAPI) Authenticators() map[string]middle.Authenticator {
	return nil
}

// Shutdown shuts down the API. This is added to implement jelly.API, and
// has no effect on the API but to return the error of the context.
func (echo *HelloAPI) Shutdown(ctx context.Context) error {
	return ctx.Err()
}

func (api *HelloAPI) Routes() (router chi.Router, subpaths bool) {
	panic("not implemented")
}
