package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/cmd/jellytest/dao"
	"github.com/dekarrin/jelly/config"
	jellydao "github.com/dekarrin/jelly/dao"
	"github.com/dekarrin/jelly/logging"
	"github.com/dekarrin/jelly/middle"
	"github.com/dekarrin/jelly/response"
	"github.com/go-chi/chi/v5"
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

	newCFG.CommonConf = newCFG.CommonConf.FillDefaults().Common()

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

	if len(cfg.CommonConf.UsesDBs) < 1 {
		return fmt.Errorf("uses: must exist and have at least one entry")
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
		valueStr, err := config.TypedSlice[string](ConfigKeyMessages, value)
		if err == nil {
			cfg.Messages = valueStr
		}
		return err
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
	store dao.Datastore
	log   logging.Logger
}

func (echo *EchoAPI) Init(cb config.Bundle, dbs map[string]jellydao.Store, log logging.Logger) error {
	dbName := cb.UsesDBs()[0] // will exist, enforced by config.Validate
	jellyStore := dbs[dbName]
	store, ok := jellyStore.(dao.Datastore)
	if !ok {
		return fmt.Errorf("received unexpected store type %T", jellyStore)
	}

	echo.store = store
	echo.log = log
	ctx := context.Background()

	msgs := cb.GetSlice(ConfigKeyMessages)
	for _, m := range msgs {
		dbMsg := dao.Message{
			Content: m,
			Creator: "(config)",
		}
		created, err := echo.store.EchoMessages.Create(ctx, dbMsg)
		if err != nil {
			if !errors.Is(err, jelly.DBErrConstraintViolation) {
				return fmt.Errorf("create initial messages: %w", err)
			} else {
				echo.log.Tracef("Skipping adding message to DB via config; already exists: %q", m)
			}
		} else {
			echo.log.Debugf("Added new message to DB via config: %s - %q", created.ID, created.Content)
		}
	}

	log.Debug("Echo API initialized")

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

func (api *EchoAPI) Routes(mid *middle.Provider, em jelly.EndpointMaker) (router chi.Router, subpaths bool) {
	optAuth := mid.OptionalAuth()

	r := chi.NewRouter()

	r.With(optAuth).Get("/", api.HTTPGetEcho(em))

	return r, false
}

type EchoRequestBody struct {
	Message string `json:"message"`
}

// HTTPGetEcho returns a HandlerFunc that echoes the user message.
func (api EchoAPI) HTTPGetEcho(em jelly.EndpointMaker) http.HandlerFunc {
	return em.Endpoint(api.epEcho)
}

func (api EchoAPI) epEcho(req *http.Request) response.Result {
	var echoData EchoRequestBody

	err := jelly.ParseJSONRequest(req, &echoData)
	if err != nil {
		return response.BadRequest(err.Error(), err.Error())
	}

	msg, err := api.store.EchoMessages.GetRandom(req.Context())
	if err != nil {
		return response.InternalServerError("could not get echo template: %v", err)
	}

	resp := MessageResponseBody{
		Message: fmt.Sprintf(msg.Content, echoData.Message),
	}

	userStr := "unauthed client"
	loggedIn := req.Context().Value(middle.AuthLoggedIn).(bool)
	if loggedIn {
		user := req.Context().Value(middle.AuthUser).(jellydao.User)
		resp.Recipient = user.Username
		userStr = "user '" + user.Username + "'"
	}

	return response.OK(resp, "%s requested echo (msg len=%d)", userStr, len(echoData.Message))
}
