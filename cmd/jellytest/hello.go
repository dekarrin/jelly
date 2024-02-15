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
	CommonConf jelly.CommonConfig

	Rudeness       float64
	RudeMessages   []string
	PoliteMessages []string
	SecretMessages []string
}

// FillDefaults returns a new *HelloConfig identical to cfg but with unset
// values set to their defaults and values normalized.
func (cfg *HelloConfig) FillDefaults() jelly.APIConfig {
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
		return fmt.Errorf(jelly.ConfigKeyAPIUsesDBs + ": must exist and have at least one item")
	}

	return nil
}

func (cfg *HelloConfig) Common() jelly.CommonConfig {
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
		valueStr, err := jelly.TypedSlice[string](ConfigKeyPolites, value)
		if err == nil {
			cfg.PoliteMessages = valueStr
		}
		return err
	case ConfigKeyRudes:
		valueStr, err := jelly.TypedSlice[string](ConfigKeyRudes, value)
		if err == nil {
			cfg.RudeMessages = valueStr
		}
		return err
	case ConfigKeySecrets:
		valueStr, err := jelly.TypedSlice[string](ConfigKeySecrets, value)
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
	// secrets is the list of secret messages returned only to
	// authenticated users.
	secrets dao.Templates

	// nices is a list of polite messages. This is randomly selected from
	// when a nice greeting is requested.
	nices dao.Templates

	// rudes is a list of not-nice messages. This is randomly selected
	// from when a rude greeting is requested.
	rudes dao.Templates

	// rudeChance is the liklihood of getting a Rude reply when asking for a
	// random greeting. Float between 0 and 1 for percentage.
	rudeChance float64

	log jelly.Logger

	uriBase string
}

func (api *HelloAPI) Init(cb jelly.Bundle) error {
	api.log = cb.Logger()

	jellyStore := cb.DB(0) // will exist, enforced by Validate
	store, ok := jellyStore.(dao.Datastore)
	if !ok {
		return fmt.Errorf("received unexpected store type %T", jellyStore)
	}

	api.rudeChance = cb.GetFloat(ConfigKeyRudeness)
	api.nices = store.NiceTemplates
	api.rudes = store.RudeTemplates
	api.secrets = store.SecretTemplates
	api.uriBase = cb.Base()

	ctx := context.Background()
	var zeroUUID uuid.UUID

	secretMsgs := cb.GetSlice(ConfigKeySecrets)
	if err := initDBWithTemplates(ctx, api.log, api.secrets, zeroUUID, secretMsgs); err != nil {
		return err
	}

	niceMsgs := cb.GetSlice(ConfigKeyPolites)
	if err := initDBWithTemplates(ctx, api.log, api.nices, zeroUUID, niceMsgs); err != nil {
		return err
	}

	rudeMsgs := cb.GetSlice(ConfigKeyRudes)
	if err := initDBWithTemplates(ctx, api.log, api.rudes, zeroUUID, rudeMsgs); err != nil {
		return err
	}

	return nil
}

func (api *HelloAPI) Authenticators() map[string]jelly.Authenticator {
	return nil
}

// Shutdown shuts down the API. This is added to implement jelly.API, and
// has no effect on the API but to return the error of the context.
func (api *HelloAPI) Shutdown(ctx context.Context) error {
	return ctx.Err()
}

func (api *HelloAPI) Routes(em jelly.ServiceProvider) (router chi.Router, subpaths bool) {
	niceTemplates := templateEndpoints{em: em, uriBase: api.uriBase, requireFormatVerb: false}
	rudeTemplates := niceTemplates
	secretTemplates := niceTemplates

	niceTemplates.templates = api.nices
	niceTemplates.uriBase += "/nice"
	niceTemplates.name = "nice"

	rudeTemplates.templates = api.rudes
	rudeTemplates.uriBase += "/rude"
	rudeTemplates.name = "rude"

	secretTemplates.templates = api.secrets
	secretTemplates.uriBase += "/secret"
	secretTemplates.name = "secret"
	secretTemplates.requireFormatVerb = true

	optAuth := em.OptionalAuth()
	reqAuth := em.RequiredAuth()

	r := chi.NewRouter()

	r.With(optAuth).Get("/nice", api.httpGetNice(em))
	r.Mount("/nice/templates", niceTemplates.routes())
	r.With(optAuth).Get("/rude", api.httpGetRude(em))
	r.Mount("/rude/templates", rudeTemplates.routes())
	r.With(optAuth).Get("/random", api.httpGetRandom(em))
	r.With(reqAuth).Get("/secret", api.httpGetSecret(em))
	r.Mount("/secret/templates", secretTemplates.routes())

	return r, false
}

// httpGetNice returns a HandlerFunc that returns a polite greeting message.
func (api HelloAPI) httpGetNice(em jelly.ServiceProvider) http.HandlerFunc {
	return em.Endpoint(func(req *http.Request) response.Result {
		msg, err := api.nices.GetRandom(req.Context())
		if err != nil {
			return response.InternalServerError("could not get random nice message: %v", err)
		}

		resp := messageResponseBody{
			Message: msg.Content,
		}

		userStr := "unauthed client"
		user, loggedIn := jelly.GetLoggedInUser(req)
		if loggedIn {
			resp.Recipient = user.Username
			userStr = "user '" + user.Username + "'"
		}

		return response.OK(resp, "%s requested a nice hello and got %s", userStr, msg.ID)
	})
}

// httpGetRude returns a HandlerFunc that returns a rude greeting message.
func (api HelloAPI) httpGetRude(em jelly.ServiceProvider) http.HandlerFunc {
	return em.Endpoint(func(req *http.Request) response.Result {
		msg, err := api.rudes.GetRandom(req.Context())
		if err != nil {
			return response.InternalServerError("could not get random rude message: %v", err)
		}

		resp := messageResponseBody{
			Message: msg.Content,
		}

		userStr := "unauthed client"
		user, loggedIn := jelly.GetLoggedInUser(req)
		if loggedIn {
			resp.Recipient = user.Username
			userStr = "user '" + user.Username + "'"
		}

		return response.OK(resp, "%s requested a rude hello and got %s", userStr, msg.ID)
	})
}

// httpGetRandom returns a HandlerFunc that returns a random greeting message.
func (api HelloAPI) httpGetRandom(em jelly.ServiceProvider) http.HandlerFunc {
	return em.Endpoint(func(req *http.Request) response.Result {
		var resp messageResponseBody
		var msg dao.Template
		var selected string
		var err error

		if rand.Float64() < api.rudeChance {
			selected = "rude"

			msg, err = api.rudes.GetRandom(req.Context())
			if err != nil {
				return response.InternalServerError("could not get random rude message: %v", err)
			}

			resp = messageResponseBody{
				Message: msg.Content,
			}
		} else {
			selected = "nice"

			msg, err = api.nices.GetRandom(req.Context())
			if err != nil {
				return response.InternalServerError("could not get random nice message: %v", err)
			}

			resp = messageResponseBody{
				Message: msg.Content,
			}
		}

		userStr := "unauthed client"
		user, loggedIn := jelly.GetLoggedInUser(req)
		if loggedIn {
			resp.Recipient = user.Username
			userStr = "user '" + user.Username + "'"
		}

		return response.OK(resp, "%s requested a random hello and got (%s) %s", userStr, selected, msg.ID)
	})
}

// httpGetSecret returns a HandlerFunc that returns a secret greeting message
// available only for logged-in users.
func (api HelloAPI) httpGetSecret(em jelly.ServiceProvider) http.HandlerFunc {
	return em.Endpoint(func(req *http.Request) response.Result {
		user, _ := jelly.GetLoggedInUser(req)
		userStr := "user '" + user.Username + "'"

		msg, err := api.secrets.GetRandom(req.Context())
		if err != nil {
			return response.InternalServerError("could not get random secret message: %v", err)
		}

		resp := messageResponseBody{
			Message:   fmt.Sprintf(msg.Content, user.Username),
			Recipient: user.Username,
		}

		return response.OK(resp, "%s requested a secret hello and got %s", userStr, msg.ID)
	})
}
