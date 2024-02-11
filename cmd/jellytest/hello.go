package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/cmd/jellytest/dao"
	"github.com/dekarrin/jelly/config"
	jellydao "github.com/dekarrin/jelly/dao"
	"github.com/dekarrin/jelly/logging"
	"github.com/dekarrin/jelly/middle"
	"github.com/dekarrin/jelly/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	ConfigKeyRudes    = "rudes"
	ConfigKeyPolites  = "polites"
	ConfigKeySecrets  = "secrets"
	ConfigKeyRudeness = "rudeness"
)

type HelloConfig struct {
	CommonConf config.Common

	Rudeness       float64
	RudeMessages   []string
	PoliteMessages []string
	SecretMessages []string
}

// FillDefaults returns a new *HelloConfig identical to cfg but with unset
// values set to their defaults and values normalized.
func (cfg *HelloConfig) FillDefaults() config.APIConfig {
	newCFG := new(HelloConfig)
	*newCFG = *cfg

	newCFG.CommonConf = newCFG.CommonConf.FillDefaults().Common()

	if newCFG.Rudeness <= 0.00000001 {
		newCFG.Rudeness = 0.3
	}
	if len(newCFG.RudeMessages) <= 1 {
		newCFG.RudeMessages = []string{"Have a TERRIBLE day!"}
	}
	if len(newCFG.PoliteMessages) <= 1 {
		newCFG.PoliteMessages = []string{"Have a nice day :)"}
	}
	if len(newCFG.SecretMessages) <= 1 {
		newCFG.SecretMessages = []string{"Good morning, %s. I see you know the password for gaining access. Welcome to the crew, secret agent."}
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
		return fmt.Errorf(ConfigKeyRudeness + ": must be greater than 0")
	}
	if cfg.Rudeness > 100.0 {
		return fmt.Errorf(ConfigKeyRudeness + ": must be less than 100")
	}
	if len(cfg.RudeMessages) < 1 {
		return fmt.Errorf(ConfigKeyRudes + ": must exist and have at least one item")
	}
	if len(cfg.PoliteMessages) < 1 {
		return fmt.Errorf(ConfigKeyPolites + ": must exist and have at least one item")
	}
	if len(cfg.SecretMessages) < 1 {
		return fmt.Errorf(ConfigKeySecrets + ": must exist and have at least one item")
	}
	if len(cfg.CommonConf.UsesDBs) < 1 {
		return fmt.Errorf(config.KeyAPIUsesDBs + ": must exist and have at least one item")
	}

	return nil
}

func (cfg *HelloConfig) Common() config.Common {
	return cfg.CommonConf
}

func (cfg *HelloConfig) Keys() []string {
	keys := cfg.CommonConf.Keys()
	keys = append(keys, ConfigKeyRudeness, ConfigKeyPolites, ConfigKeyRudes, ConfigKeySecrets)
	return keys
}

func (cfg *HelloConfig) Get(key string) interface{} {
	switch strings.ToLower(key) {
	case ConfigKeyRudeness:
		return cfg.Rudeness
	case ConfigKeyPolites:
		return cfg.PoliteMessages
	case ConfigKeySecrets:
		return cfg.SecretMessages
	case ConfigKeyRudes:
		return cfg.RudeMessages
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
	case ConfigKeyPolites:
		valueStr, err := config.TypedSlice[string](ConfigKeyPolites, value)
		if err == nil {
			cfg.PoliteMessages = valueStr
		}
		return err
	case ConfigKeyRudes:
		valueStr, err := config.TypedSlice[string](ConfigKeyRudes, value)
		if err == nil {
			cfg.RudeMessages = valueStr
		}
		return err
	case ConfigKeySecrets:
		valueStr, err := config.TypedSlice[string](ConfigKeySecrets, value)
		if err == nil {
			cfg.SecretMessages = valueStr
		}
		return err
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
	case ConfigKeyRudes, ConfigKeyPolites, ConfigKeySecrets:
		if value == "" {
			return cfg.Set(key, []string{})
		}
		msgsStrSlice := strings.Split(value, ",")
		return cfg.Set(key, msgsStrSlice)
	default:
		return cfg.CommonConf.SetFromString(key, value)
	}
}

type HelloAPI struct {
	// SecretMessages is the list of secret messages returned only to
	// authenticated users.
	SecretMessages dao.Templates

	// NiceMessages is a list of polite messages. This is randomly selected from
	// when a nice greeting is requested.
	NiceMessages dao.Templates

	// RudeMessages is a list of not-nice messages. This is randomly selected
	// from when a rude greeting is requested.
	RudeMessages dao.Templates

	// RudeChance is the liklihood of getting a Rude reply when asking for a
	// random greeting. Float between 0 and 1 for percentage.
	RudeChance float64

	log logging.Logger
}

func (api *HelloAPI) Init(cb config.Bundle, dbs map[string]jellydao.Store, log logging.Logger) error {
	api.log = log

	dbName := cb.UsesDBs()[0] // will exist, enforced by config.Validate
	jellyStore := dbs[dbName]
	store, ok := jellyStore.(dao.Datastore)
	if !ok {
		return fmt.Errorf("received unexpected store type %T", jellyStore)
	}

	api.RudeChance = cb.GetFloat(ConfigKeyRudeness)
	api.NiceMessages = store.NiceTemplates
	api.RudeMessages = store.RudeTemplates
	api.SecretMessages = store.SecretTemplates

	ctx := context.Background()
	var zeroUUID uuid.UUID

	secretMsgs := cb.GetSlice(ConfigKeySecrets)
	if err := initDBWithMessages(ctx, log, api.SecretMessages, zeroUUID, secretMsgs); err != nil {
		return err
	}

	niceMsgs := cb.GetSlice(ConfigKeyPolites)
	if err := initDBWithMessages(ctx, log, api.NiceMessages, zeroUUID, niceMsgs); err != nil {
		return err
	}

	rudeMsgs := cb.GetSlice(ConfigKeyRudes)
	if err := initDBWithMessages(ctx, log, api.RudeMessages, zeroUUID, rudeMsgs); err != nil {
		return err
	}

	return nil
}

func (api *HelloAPI) Authenticators() map[string]middle.Authenticator {
	return nil
}

// Shutdown shuts down the API. This is added to implement jelly.API, and
// has no effect on the API but to return the error of the context.
func (api *HelloAPI) Shutdown(ctx context.Context) error {
	return ctx.Err()
}

func (api *HelloAPI) Routes(mid *middle.Provider, em jelly.EndpointMaker) (router chi.Router, subpaths bool) {
	optAuth := mid.OptionalAuth()
	reqAuth := mid.RequireAuth()

	r := chi.NewRouter()

	r.With(optAuth).Get("/nice", api.HTTPGetNice(em))
	r.With(optAuth).Get("/rude", api.HTTPGetRude(em))
	r.With(optAuth).Get("/random", api.HTTPGetRandom(em))
	r.With(reqAuth).Get("/secret", api.HTTPGetSecret(em))

	return r, false
}

// HTTPGetNice returns a HandlerFunc that returns a polite greeting message.
func (api HelloAPI) HTTPGetNice(em jelly.EndpointMaker) http.HandlerFunc {
	return em.Endpoint(api.epNice)
}

func (api HelloAPI) epNice(req *http.Request) response.Result {
	msg, err := api.NiceMessages.GetRandom(req.Context())
	if err != nil {
		return response.InternalServerError("could not get random nice message: %v", err)
	}

	resp := MessageResponseBody{
		Message: msg.Content,
	}

	userStr := "unauthed client"
	loggedIn := req.Context().Value(middle.AuthLoggedIn).(bool)
	if loggedIn {
		user := req.Context().Value(middle.AuthUser).(jellydao.User)
		resp.Recipient = user.Username
		userStr = "user '" + user.Username + "'"
	}

	return response.OK(resp, "%s requested a nice hello and got %s", userStr, msg.ID)
}

// HTTPGetRude returns a HandlerFunc that returns a rude greeting message.
func (api HelloAPI) HTTPGetRude(em jelly.EndpointMaker) http.HandlerFunc {
	return em.Endpoint(api.epRude)
}

func (api HelloAPI) epRude(req *http.Request) response.Result {
	msg, err := api.RudeMessages.GetRandom(req.Context())
	if err != nil {
		return response.InternalServerError("could not get random rude message: %v", err)
	}

	resp := MessageResponseBody{
		Message: msg.Content,
	}

	userStr := "unauthed client"
	loggedIn := req.Context().Value(middle.AuthLoggedIn).(bool)
	if loggedIn {
		user := req.Context().Value(middle.AuthUser).(jellydao.User)
		resp.Recipient = user.Username
		userStr = "user '" + user.Username + "'"
	}

	return response.OK(resp, "%s requested a rude hello and got %s", userStr, msg.ID)
}

// HTTPGetRandom returns a HandlerFunc that returns a random greeting message.
func (api HelloAPI) HTTPGetRandom(em jelly.EndpointMaker) http.HandlerFunc {
	return em.Endpoint(api.epRandom)
}

func (api HelloAPI) epRandom(req *http.Request) response.Result {
	var resp MessageResponseBody
	var msg dao.Template
	var selected string
	var err error

	if rand.Float64() < api.RudeChance {
		selected = "rude"

		msg, err = api.RudeMessages.GetRandom(req.Context())
		if err != nil {
			return response.InternalServerError("could not get random rude message: %v", err)
		}

		resp = MessageResponseBody{
			Message: msg.Content,
		}
	} else {
		selected = "nice"

		msg, err = api.NiceMessages.GetRandom(req.Context())
		if err != nil {
			return response.InternalServerError("could not get random nice message: %v", err)
		}

		resp = MessageResponseBody{
			Message: msg.Content,
		}
	}

	userStr := "unauthed client"
	loggedIn := req.Context().Value(middle.AuthLoggedIn).(bool)
	if loggedIn {
		user := req.Context().Value(middle.AuthUser).(jellydao.User)
		resp.Recipient = user.Username
		userStr = "user '" + user.Username + "'"
	}

	return response.OK(resp, "%s requested a random hello and got (%s) %s", userStr, selected, msg.ID)
}

// HTTPGetSecret returns a HandlerFunc that returns a secret greeting message
// available only for logged-in users.
func (api HelloAPI) HTTPGetSecret(em jelly.EndpointMaker) http.HandlerFunc {
	return em.Endpoint(api.epSecret)
}

func (api HelloAPI) epSecret(req *http.Request) response.Result {
	user := req.Context().Value(middle.AuthUser).(jellydao.User)
	userStr := "user '" + user.Username + "'"

	msg, err := api.SecretMessages.GetRandom(req.Context())
	if err != nil {
		return response.InternalServerError("could not get random secret message: %v", err)
	}

	resp := MessageResponseBody{
		Message:   fmt.Sprintf(msg.Content, user.Username),
		Recipient: user.Username,
	}

	return response.OK(resp, "%s requested a secret hello and got %s", userStr, msg.ID)
}
