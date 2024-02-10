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
	"github.com/dekarrin/jelly/serr"
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
	store   dao.Datastore
	log     logging.Logger
	uriBase string
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
	echo.uriBase = cb.Base()
	ctx := context.Background()

	msgs := cb.GetSlice(ConfigKeyMessages)
	if err := initDBWithMessages(ctx, log, echo.store.EchoTemplates, "(config)", msgs); err != nil {
		return err
	}

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

// HTTPPOSTTemplate returns a HandlerFunc that adds a new message to the list of
// possible ones.
func (api EchoAPI) HTTPPostTemplate(em jelly.EndpointMaker) http.HandlerFunc {
	return em.Endpoint(func(req *http.Request) response.Result {
		user := req.Context().Value(middle.AuthUser).(jellydao.User)
		userStr := "user '" + user.Username + "'"

		var data Template

		err := jelly.ParseJSONRequest(req, &data)
		if err != nil {
			return response.BadRequest(err.Error(), err.Error())
		}

		if err := data.Validate(true); err != nil {
			return response.BadRequest(err.Error(), err.Error())
		}

		newMsg := dao.Template{
			Content: data.Content,
			Creator: user.ID.String(),
		}
		created, err := api.store.EchoTemplates.Create(req.Context(), newMsg)
		if err != nil {
			if errors.Is(err, jelly.DBErrConstraintViolation) {
				return response.Conflict("a template with that exact content already exists", err)
			}
			return response.InternalServerError("could not create template: %v", err)
		}

		resp := daoToTemplate(created)
		resp.Path = fmt.Sprintf("%s/templates/%s", api.uriBase, created.ID)

		return response.OK(resp, "%s created new template %s - %q", userStr, created.ID, created.Content)
	})
}

// HTTPPOSTTemplate returns a HandlerFunc that adds a new message to the list of
// possible ones.
func (api EchoAPI) HTTPDeleteTemplate(mid *middle.Provider, em jelly.EndpointMaker) http.HandlerFunc {
	authService := mid.SelectAuthenticator().Service()

	return em.Endpoint(func(req *http.Request) response.Result {
		id := jelly.RequireIDParam(req)
		user := req.Context().Value(middle.AuthUser).(jellydao.User)

		// first, find the template owner
		t, err := api.store.EchoTemplates.Get(req.Context(), id)
		if err != nil {
			return
		}

		// is the user trying to delete someone else's login? they'd betta be
		// the admin if so!
		if id != user.ID && user.Role != jellydao.Admin {
			var otherUserStr string
			otherUser, err := authService.GetUser(req.Context(), id.String())
			// if there was another user, find out now
			if err != nil {
				if !errors.Is(err, serr.ErrNotFound) {
					return response.InternalServerError("retrieve user for perm checking: %s", err.Error())
				}
				otherUserStr = fmt.Sprintf("%s", id)
			} else {
				otherUserStr = "'" + otherUser.Username + "'"
			}

			return response.Forbidden("user '%s' (role %s) deletion of message %s created by %s: forbidden", user.Username, user.Role, otherUserStr)
		}

		var data Template

		err := jelly.ParseJSONRequest(req, &data)
		if err != nil {
			return response.BadRequest(err.Error(), err.Error())
		}

		if err := data.Validate(true); err != nil {
			return response.BadRequest(err.Error(), err.Error())
		}

		newMsg := dao.Template{
			Content: data.Content,
			Creator: userStr,
		}
		created, err := api.store.EchoTemplates.Create(req.Context(), newMsg)
		if err != nil {
			if errors.Is(err, jelly.DBErrConstraintViolation) {
				return response.Conflict("a template with that exact content already exists", err)
			}
			return response.InternalServerError("could not create template: %v", err)
		}

		resp := daoToTemplate(created)
		resp.Path = fmt.Sprintf("%s/templates/%s", api.uriBase, created.ID)

		return response.OK(resp, "%s created new template %s - %q", userStr, created.ID, created.Content)
	})
}

// HTTPGetEcho returns a HandlerFunc that echoes the user message.
func (api EchoAPI) HTTPGetEcho(em jelly.EndpointMaker) http.HandlerFunc {
	return em.Endpoint(func(req *http.Request) response.Result {
		var echoData EchoRequestBody

		err := jelly.ParseJSONRequest(req, &echoData)
		if err != nil {
			return response.BadRequest(err.Error(), err.Error())
		}

		msg, err := api.store.EchoTemplates.GetRandom(req.Context())
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
	})
}
