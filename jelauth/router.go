package jelauth

import (
	"net/http"
	"time"

	"github.com/dekarrin/jelly/jelapi"
	"github.com/dekarrin/jelly/jeldao"
	"github.com/dekarrin/jelly/jelmid"
	"github.com/dekarrin/jelly/jelresult"
	"github.com/go-chi/chi/v5"
)

func p(s string) (pathParam string) {
	return jelapi.PathParam(s)
}

func (api *LoginAPI) Routes() (router chi.Router, subpaths bool) {
	r := chi.NewRouter()

	login := api.routesForLogin()
	tokens := api.routesForToken()
	users := api.routesForAuthUser()
	info := api.routesForInfo()

	r.Mount("/login", login)
	r.Mount("/tokens", tokens)
	r.Mount("/users", users)
	r.Mount("/info", info)
	r.HandleFunc("/info/", jelapi.RedirectNoTrailingSlash)

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		res := jelresult.NotFound()
		res.WriteResponse(w)
		res.Log(req)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		time.Sleep(api.UnauthDelay)
		res := jelresult.MethodNotAllowed(req)
		res.WriteResponse(w)
		res.Log(req)
	})

	return r, true
}

func (api LoginAPI) routesForLogin() chi.Router {
	reqAuth := jelmid.RequireAuth(api.Service.Provider.AuthUsers(), api.Secret, api.UnauthDelay, jeldao.User{})

	r := chi.NewRouter()

	r.Post("/", api.HTTPCreateLogin())
	r.With(reqAuth).Delete("/"+p("id:uuid"), api.HTTPDeleteLogin())
	r.HandleFunc("/"+p("id:uuid")+"/", jelapi.RedirectNoTrailingSlash)

	return r
}

func (api LoginAPI) routesForToken() chi.Router {
	reqAuth := jelmid.RequireAuth(api.Service.Provider.AuthUsers(), api.Secret, api.UnauthDelay, jeldao.User{})

	r := chi.NewRouter()

	r.With(reqAuth).Post("/", api.HTTPCreateToken())

	return r
}

func (api LoginAPI) routesForAuthUser() chi.Router {
	reqAuth := jelmid.RequireAuth(api.Service.Provider.AuthUsers(), api.Secret, api.UnauthDelay, jeldao.User{})

	r := chi.NewRouter()

	r.Use(reqAuth)

	r.Get("/", api.HTTPGetAllUsers())
	r.Post("/", api.HTTPCreateUser())

	r.Route("/"+p("id:uuid"), func(r chi.Router) {
		r.Get("/", api.HTTPGetUser())
		r.Put("/", api.HTTPReplaceUser())
		r.Patch("/", api.HTTPUpdateUser())
		r.Delete("/", api.HTTPDeleteUser())
	})

	return r
}

func (api LoginAPI) routesForInfo() chi.Router {
	optAuth := jelmid.OptionalAuth(api.Service.Provider.AuthUsers(), api.Secret, api.UnauthDelay, jeldao.User{})

	r := chi.NewRouter()

	r.With(optAuth).Get("/", api.HTTPGetInfo())

	return r
}
