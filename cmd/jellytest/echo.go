package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/config"
	"github.com/dekarrin/jelly/dao"
	"github.com/dekarrin/jelly/middle"
	"github.com/dekarrin/jelly/response"
)

const (
	ConfigKeyMessages = "messages"
)

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

func (cfg *EchoConfig) Common() config.Common {
	return cfg.CommonConf
}

func (cfg *EchoConfig) Keys() []string {
	keys := cfg.CommonConf.Keys()
	keys = append(keys, ConfigKeyMessages)
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

type EchoAPI struct {
	// Messages is a list of messages that an echo can reply with. Each should
	// be a format string that expects to receive the message sent by the user
	// as its first argument.
	Messages []string

	// UnauthDelay should eventually be mitigated by referring to an
	// authenticator.
	UnauthDelay time.Duration
}

func (echo *EchoAPI) Init(cfg config.APIConfig, g config.Globals, dbs map[string]dao.Store) error {
	echoCfg, ok := cfg.(*EchoConfig)
	if !ok {
		return fmt.Errorf("wrong config type, expected *EchoConfig but got %T", cfg)
	}

	echo.Messages = make([]string, len(echoCfg.Messages))
	copy(echo.Messages, echoCfg.Messages)
	echo.UnauthDelay = g.UnauthDelay()

	return nil
}

func (echo *EchoAPI) Authenticators() map[string]middle.Authenticator {
	return nil
}

// Shutdown shuts down the login API. This is added to implement jelly.API, and
// has no effect on the API but to return the error of the context.
func (echo *EchoAPI) Shutdown(ctx context.Context) error {
	return ctx.Err()
}

// HTTPGetEcho returns a HandlerFunc that echoes the user message.
func (api EchoAPI) HTTPGetEcho() http.HandlerFunc {
	return jelly.Endpoint(0, api.epEcho)
}

type EchoResponse struct {
}

func (api EchoAPI) epEcho(req *http.Request) response.Result {
	loggedIn := req.Context().Value(middle.AuthLoggedIn).(bool)

	var resp InfoModel
	resp.Version.Auth = Version

	userStr := "unauthed client"
	if loggedIn {
		user := req.Context().Value(middle.AuthUser).(dao.User)
		userStr = "user '" + user.Username + "'"
	}
	return response.OK(resp, "%s got API info", userStr)
}
