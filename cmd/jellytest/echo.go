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
	"github.com/google/uuid"
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
	var zeroUUID uuid.UUID
	if err := initDBWithMessages(ctx, log, echo.store.EchoTemplates, zeroUUID, msgs); err != nil {
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
	r.Mount("/templates", api.routesForTemplates(mid, em))

	return r, true
}

func (api *EchoAPI) routesForTemplates(mid *middle.Provider, em jelly.EndpointMaker) (router chi.Router) {
	r := chi.NewRouter()

	r.Use(mid.RequireAuth())

	r.Get("/", api.HTTPGetAllTemplates(em))
	r.Post("/", api.HTTPPostTemplate(em))

	r.Route("/"+jelly.PathParam("id:uuid"), func(r chi.Router) {
		r.Get("/", api.HTTPGetTemplate(em))
		r.Put("/", api.HTTPUpdateTemplate(mid, em))
		r.Delete("/", api.HTTPDeleteTemplate(mid, em))
	})

	return r
}

type EchoRequestBody struct {
	Message string `json:"message"`
}

// HTTPPostTemplate returns a HandlerFunc that adds a new message to the list of
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

		newMsg, err := data.DAO()
		if err != nil {
			return response.BadRequest(err.Error(), err.Error())
		}
		newMsg.Creator = user.ID

		created, err := api.store.EchoTemplates.Create(req.Context(), newMsg)
		if err != nil {
			if errors.Is(err, jelly.DBErrConstraintViolation) {
				return response.Conflict("a template with that exact content already exists", err)
			}
			return response.InternalServerError("could not create template: %v", err)
		}

		resp := daoToTemplate(created, api.uriBase)

		return response.Created(resp, "%s created new template %s - %q", userStr, created.ID, created.Content)
	})
}

// HTTPGetTemplate returns a HandlerFunc that returns the requested template.
func (api EchoAPI) HTTPGetTemplate(em jelly.EndpointMaker) http.HandlerFunc {
	return em.Endpoint(func(req *http.Request) response.Result {
		id := jelly.RequireIDParam(req)
		user := req.Context().Value(middle.AuthUser).(jellydao.User)

		retrieved, err := api.store.EchoTemplates.Get(req.Context(), id)
		if err != nil {
			if errors.Is(err, jelly.DBErrNotFound) {
				return response.NotFound()
			}
			return response.InternalServerError("retrieve template: %v", err)
		}

		// logged-in users can retrieve any template, no need for a role check

		resp := daoToTemplate(retrieved, api.uriBase)
		return response.OK(resp, "user '%s' retrieved template %s", user.Username, retrieved.ID)
	})
}

// HTTPGetAllTemplates returns a HandlerFunc that returns all requested
// templates.
func (api EchoAPI) HTTPGetAllTemplates(em jelly.EndpointMaker) http.HandlerFunc {
	return em.Endpoint(func(req *http.Request) response.Result {
		user := req.Context().Value(middle.AuthUser).(jellydao.User)

		all, err := api.store.EchoTemplates.GetAll(req.Context())
		if err != nil {
			if !errors.Is(err, jelly.DBErrNotFound) {
				return response.InternalServerError("retrieve all templates: %v", err)
			}
		}

		if len(all) == 0 {
			all = []dao.Template{}
		}

		// logged-in users can retrieve any template, no need for a role check

		resp := daoToTemplates(all, api.uriBase)
		return response.OK(resp, "user '%s' retrieved all templates", user.Username)
	})
}

// HTTPDeleteTemplate returns a HandlerFunc that deletes the requested template.
func (api EchoAPI) HTTPDeleteTemplate(mid *middle.Provider, em jelly.EndpointMaker) http.HandlerFunc {
	authService := mid.SelectAuthenticator().Service()

	return em.Endpoint(func(req *http.Request) response.Result {
		id := jelly.RequireIDParam(req)
		user := req.Context().Value(middle.AuthUser).(jellydao.User)

		// first, find the template owner
		t, err := api.store.EchoTemplates.Get(req.Context(), id)
		if err != nil {
			if errors.Is(err, jelly.DBErrNotFound) {
				return response.NotFound()
			}
			return response.InternalServerError("retrieve template to delete: %v", err)
		}

		// is the user trying to delete someone else's template (or one added
		// via config)? they'd betta be the admin if so!
		if t.Creator != user.ID && user.Role != jellydao.Admin {
			var creatorStr string
			var zeroUUID uuid.UUID
			if t.Creator == zeroUUID {
				// if it is the zero value ID, then it is created by config and no
				// other user exists
				creatorStr = "config file"
			} else {
				otherUser, err := authService.GetUser(req.Context(), id.String())
				// if there was another user, find out now
				if err != nil {
					if !errors.Is(err, serr.ErrNotFound) {
						return response.InternalServerError("retrieve other template's user: %s", err.Error())
					}
					creatorStr = id.String()
				} else {
					creatorStr = "user '" + otherUser.Username + "'"
				}
			}

			return response.Forbidden("user '%s' (role %s) deletion of template %s created by %s: forbidden", user.Username, user.Role, t.ID, creatorStr)
		}

		deleted, err := api.store.EchoTemplates.Delete(req.Context(), id)
		if err != nil {
			if errors.Is(err, jelly.DBErrNotFound) {
				return response.NotFound()
			} else {
				return response.InternalServerError("delete template from DB: %v", err)
			}
		}

		resp := daoToTemplate(deleted, api.uriBase)

		return response.OK(resp, "user '%s' deleted template %s", user.Username, deleted.ID)
	})
}

// HTTPUpdateTemplate returns a HandlerFunc that updates the requested template.
func (api EchoAPI) HTTPUpdateTemplate(mid *middle.Provider, em jelly.EndpointMaker) http.HandlerFunc {
	authService := mid.SelectAuthenticator().Service()

	return em.Endpoint(func(req *http.Request) response.Result {
		id := jelly.RequireIDParam(req)
		user := req.Context().Value(middle.AuthUser).(jellydao.User)

		var submitted Template

		err := jelly.ParseJSONRequest(req, &submitted)
		if err != nil {
			return response.BadRequest(err.Error(), err.Error())
		}

		if err := submitted.Validate(true); err != nil {
			return response.BadRequest(err.Error(), err.Error())
		}

		daoSubmitted, err := submitted.DAO()
		if err != nil {
			return response.BadRequest(err.Error(), err.Error())
		}

		// first, find the original to check perms
		t, err := api.store.EchoTemplates.Get(req.Context(), id)
		if err != nil {
			if errors.Is(err, jelly.DBErrNotFound) {
				return response.NotFound()
			}
			return response.InternalServerError("retrieve template to update: %v", err)
		}

		// is the user trying to update someone else's template (or one added
		// via config)? they'd betta be the admin if so!
		//
		// also applies if updating the user.Creator; only admin can do that!
		if (t.Creator != user.ID || t.Creator != daoSubmitted.Creator) && user.Role != jellydao.Admin {
			var creatorStr string
			var zeroUUID uuid.UUID
			if t.Creator == zeroUUID {
				// if it is the zero value ID, then it is created by config and no
				// other user exists
				creatorStr = "config file"
			} else {
				otherUser, err := authService.GetUser(req.Context(), id.String())
				// if there was another user, find out now
				if err != nil {
					if !errors.Is(err, serr.ErrNotFound) {
						return response.InternalServerError("retrieve other template's user: %s", err.Error())
					}
					creatorStr = id.String()
				} else {
					creatorStr = "user '" + otherUser.Username + "'"
				}
			}

			var problem string
			if t.Creator != user.ID {
				problem = fmt.Sprintf("template %s created by %s", t.ID, creatorStr)
			} else {
				problem = fmt.Sprintf("own template %s", t.ID)
			}

			if t.Creator != daoSubmitted.Creator {
				problem += fmt.Sprintf(" and setting new owner %s", daoSubmitted.Creator)
			}

			return response.Forbidden("user '%s' (role %s) update of %s: forbidden", user.Username, user.Role, problem)
		}

		// ensure the actual user it is being changed to exists
		// TODO: when using own authuser impl with own users, remove this check
		// and replace with constraint violation on upsert check
		if daoSubmitted.Creator != t.Creator {
			_, err := authService.GetUser(req.Context(), submitted.Creator)
			if err != nil {
				if errors.Is(err, serr.ErrNotFound) {
					return response.BadRequest("no user with ID %s exists", submitted.Creator)
				}
				return response.InternalServerError("get updated creator to confirm user exists: ", err.Error())
			}
		}

		updated, err := api.store.EchoTemplates.Update(req.Context(), id, daoSubmitted)
		if err != nil {
			if errors.Is(err, jelly.DBErrNotFound) {
				return response.NotFound()
			} else {
				return response.InternalServerError("delete template from DB: %v", err)
			}
		}

		resp := daoToTemplate(updated, api.uriBase)

		return response.OK(resp, "user '%s' updated template %s", user.Username, updated.ID)
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

		t, err := api.store.EchoTemplates.GetRandom(req.Context())
		if err != nil {
			return response.InternalServerError("could not get echo template: %v", err)
		}

		resp := MessageResponseBody{
			Message: fmt.Sprintf(t.Content, echoData.Message),
		}

		userStr := "unauthed client"
		loggedIn := req.Context().Value(middle.AuthLoggedIn).(bool)
		if loggedIn {
			user := req.Context().Value(middle.AuthUser).(jellydao.User)
			resp.Recipient = user.Username
			userStr = "user '" + user.Username + "'"
		}

		return response.OK(resp, "%s requested echo (msg len=%d), got template %s", userStr, len(echoData.Message), t.ID)
	})
}
