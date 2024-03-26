package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/cmd/jellytest/dao"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func initDBWithTemplates(ctx context.Context, log jelly.Logger, repo dao.Templates, creator uuid.UUID, contents []string) error {
	for _, m := range contents {
		dbMsg := dao.Template{
			Content: m,
			Creator: creator,
		}
		created, err := repo.Create(ctx, dbMsg)
		if err != nil {
			if !errors.Is(err, jelly.ErrConstraintViolation) {
				return fmt.Errorf("create initial messages: %w", err)
			} else {
				log.Tracef("Skipping adding message to DB via config; already exists: %q", m)
			}
		} else {
			log.Debugf("Added new message to DB via config: %s - %q", created.ID, created.Content)
		}
	}
	return nil
}

// Template is the representation of a message template resource.
type Template struct {
	ID      string `json:"id,omitempty"`
	Content string `json:"content"`
	Creator string `json:"creator,omitempty"`
	Path    string `json:"path,omitempty"`
}

// DAO creates a data abstraction object that represents this model. Conversion
// of values is performed; while empty values are allowed for ID and Creator
// (and will simply result in a zero-value ID in the returned object), non-empty
// invalid values will cause an error.
func (m Template) DAO() (dao.Template, error) {
	var err error

	t := dao.Template{
		Content: m.Content,
	}

	if m.ID != "" {
		t.ID, err = uuid.Parse(m.ID)
		if err != nil {
			return t, err
		}
	}
	if m.Creator != "" {
		t.Creator, err = uuid.Parse(m.Creator)
		if err != nil {
			return t, err
		}
	}

	return t, nil
}

func (t Template) Validate(requireFormatVerb bool) error {
	if t.Content == "" {
		return errors.New("'content' field must exist and be set to a non-empty value")
	}

	if requireFormatVerb {
		if !strings.Contains(t.Content, "%s") && !strings.Contains(t.Content, "%v") && !strings.Contains(t.Content, "%q") {
			return errors.New("template must contain at least one %v, %s, or %q")
		}
	}

	return nil
}

func daoToTemplates(ts []dao.Template, uriBase string) []Template {
	output := make([]Template, len(ts))
	for i := range ts {
		output[i] = daoToTemplate(ts[i], uriBase)
	}
	return output
}

func daoToTemplate(t dao.Template, uriBase string) Template {
	if !strings.HasSuffix("/", uriBase) {
		uriBase += "/"
	}

	m := Template{
		Content: t.Content,
		ID:      t.ID.String(),
		Creator: t.Creator.String(),
		Path:    fmt.Sprintf("%stemplates/%s", uriBase, t.ID),
	}

	return m
}

type templateEndpoints struct {
	templates         dao.Templates
	em                jelly.EndpointServices
	uriBase           string
	name              string
	requireFormatVerb bool
}

func (ep templateEndpoints) routes() (router chi.Router) {
	r := chi.NewRouter()

	r.Use(ep.em.RequiredAuth())

	r.Get("/", ep.httpGetAllTemplates())
	r.Post("/", ep.httpCreateTemplate())

	r.Route("/"+jelly.PathParam("id:uuid"), func(r chi.Router) {
		r.Get("/", ep.httpGetTemplate())
		r.Put("/", ep.httpUpdateTemplate())
		r.Delete("/", ep.httpDeleteTemplate())
	})

	return r
}

func (ep templateEndpoints) httpGetAllTemplates() http.HandlerFunc {
	return ep.em.Endpoint(func(req *http.Request) jelly.Result {
		user, _ := ep.em.GetLoggedInUser(req)

		all, err := ep.templates.GetAll(req.Context())
		if err != nil {
			if !errors.Is(err, jelly.ErrNotFound) {
				return ep.em.InternalServerError("retrieve all %s templates: %v", ep.name, err)
			}
		}

		if len(all) == 0 {
			all = []dao.Template{}
		}

		// logged-in users can retrieve any template, no need for a role check

		resp := daoToTemplates(all, ep.uriBase)
		return ep.em.OK(resp, "user '%s' retrieved all %s templates", user.Username, ep.name)
	})
}

func (ep templateEndpoints) httpGetTemplate() http.HandlerFunc {
	return ep.em.Endpoint(func(req *http.Request) jelly.Result {
		id := jelly.RequireIDParam(req)
		user, _ := ep.em.GetLoggedInUser(req)

		retrieved, err := ep.templates.Get(req.Context(), id)
		if err != nil {
			if errors.Is(err, jelly.ErrNotFound) {
				return ep.em.NotFound()
			}
			return ep.em.InternalServerError("retrieve %s template: %v", ep.name, err)
		}

		// logged-in users can retrieve any template, no need for a role check

		resp := daoToTemplate(retrieved, ep.uriBase)
		return ep.em.OK(resp, "user '%s' retrieved %s template %s", user.Username, ep.name, retrieved.ID)
	})
}

func (ep templateEndpoints) httpCreateTemplate() http.HandlerFunc {
	return ep.em.Endpoint(func(req *http.Request) jelly.Result {
		user, _ := ep.em.GetLoggedInUser(req)
		userStr := "user '" + user.Username + "'"

		var data Template

		err := jelly.ParseJSONRequest(req, &data)
		if err != nil {
			return ep.em.BadRequest(err.Error(), err.Error())
		}

		if err := data.Validate(ep.requireFormatVerb); err != nil {
			return ep.em.BadRequest(err.Error(), err.Error())
		}

		newMsg, err := data.DAO()
		if err != nil {
			return ep.em.BadRequest(err.Error(), err.Error())
		}
		newMsg.Creator = user.ID

		created, err := ep.templates.Create(req.Context(), newMsg)
		if err != nil {
			if errors.Is(err, jelly.ErrConstraintViolation) {
				return ep.em.Conflict("a template with that exact content already exists", err.Error())
			}
			return ep.em.InternalServerError("could not create %s template: %v", ep.name, err)
		}

		resp := daoToTemplate(created, ep.uriBase)

		return ep.em.Created(resp, "%s created new %s template %s - %q", userStr, ep.name, created.ID, created.Content)
	})
}

func (ep templateEndpoints) httpDeleteTemplate() http.HandlerFunc {
	authService := ep.em.SelectAuthenticator().Service()

	return ep.em.Endpoint(func(req *http.Request) jelly.Result {
		id := jelly.RequireIDParam(req)
		user, _ := ep.em.GetLoggedInUser(req)

		// first, find the template owner
		t, err := ep.templates.Get(req.Context(), id)
		if err != nil {
			if errors.Is(err, jelly.ErrNotFound) {
				return ep.em.NotFound()
			}
			return ep.em.InternalServerError("retrieve %s template to delete: %v", ep.name, err)
		}

		// is the user trying to delete someone else's template (or one added
		// via config)? they'd betta be the admin if so!
		if t.Creator != user.ID && user.Role != jelly.Admin {
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
					if !errors.Is(err, jelly.ErrNotFound) {
						return ep.em.InternalServerError("retrieve other %s template's user: %s", ep.name, err.Error())
					}
					creatorStr = id.String()
				} else {
					creatorStr = "user '" + otherUser.Username + "'"
				}
			}

			return ep.em.Forbidden("user '%s' (role %s) deletion of %s template %s created by %s: forbidden", user.Username, user.Role, ep.name, t.ID, creatorStr)
		}

		deleted, err := ep.templates.Delete(req.Context(), id)
		if err != nil {
			if errors.Is(err, jelly.ErrNotFound) {
				return ep.em.NotFound()
			} else {
				return ep.em.InternalServerError("delete %s template from DB: %v", ep.name, err)
			}
		}

		resp := daoToTemplate(deleted, ep.uriBase)

		return ep.em.OK(resp, "user '%s' deleted %s template %s", user.Username, ep.name, deleted.ID)
	})
}

func (ep templateEndpoints) httpUpdateTemplate() http.HandlerFunc {
	authService := ep.em.SelectAuthenticator().Service()

	return ep.em.Endpoint(func(req *http.Request) jelly.Result {
		id := jelly.RequireIDParam(req)
		user, _ := ep.em.GetLoggedInUser(req)

		var submitted Template

		err := jelly.ParseJSONRequest(req, &submitted)
		if err != nil {
			return ep.em.BadRequest(err.Error(), err.Error())
		}

		if err := submitted.Validate(ep.requireFormatVerb); err != nil {
			return ep.em.BadRequest(err.Error(), err.Error())
		}

		updateCreator := true
		if submitted.Creator == "" {
			updateCreator = false
		}

		daoSubmitted, err := submitted.DAO()
		if err != nil {
			return ep.em.BadRequest(err.Error(), err.Error())
		}
		daoSubmitted.ID = id

		// first, find the original to check perms
		t, err := ep.templates.Get(req.Context(), id)
		if err != nil {
			if errors.Is(err, jelly.ErrNotFound) {
				return ep.em.NotFound()
			}
			return ep.em.InternalServerError("retrieve %s template to update: %v", ep.name, err)
		}

		if !updateCreator {
			daoSubmitted.Creator = t.Creator
		}

		// is the user trying to update someone else's template (or one added
		// via config)? they'd betta be the admin if so!
		//
		// also applies if updating the user.Creator; only admin can do that!
		if (t.Creator != user.ID || t.Creator != daoSubmitted.Creator) && user.Role != jelly.Admin {
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
					if !errors.Is(err, jelly.ErrNotFound) {
						return ep.em.InternalServerError("retrieve other %s template's user: %s", ep.name, err.Error())
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

			return ep.em.Forbidden("user '%s' (role %s) update of %s %s: forbidden", user.Username, user.Role, ep.name, problem)
		}

		// ensure the actual user it is being changed to exists
		// TODO: when using own authuser impl with own users, remove this check
		// and replace with constraint violation on upsert check
		if daoSubmitted.Creator != t.Creator {
			_, err := authService.GetUser(req.Context(), submitted.Creator)
			if err != nil {
				if errors.Is(err, jelly.ErrNotFound) {
					return ep.em.BadRequest("no user with ID %s exists", submitted.Creator)
				}
				return ep.em.InternalServerError("get %s updated creator to confirm user exists: ", ep.name, err.Error())
			}
		}

		updated, err := ep.templates.Update(req.Context(), id, daoSubmitted)
		if err != nil {
			if errors.Is(err, jelly.ErrNotFound) {
				return ep.em.NotFound()
			} else if errors.Is(err, jelly.ErrConstraintViolation) {
				return ep.em.Conflict("a template with that exact content already exists", err.Error())
			} else {
				return ep.em.InternalServerError("update %s template: %v", ep.name, err)
			}
		}

		resp := daoToTemplate(updated, ep.uriBase)

		return ep.em.OK(resp, "user '%s' updated %s template %s", user.Username, ep.name, updated.ID)
	})
}
