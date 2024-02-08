package auth

import (
	"net/http"
	"time"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/middle"
	"github.com/dekarrin/jelly/response"
	"github.com/go-chi/chi/v5"
)

func p(s string) (pathParam string) {
	return jelly.PathParam(s)
}

func (api *LoginAPI) Routes(mid *middle.Provider, em jelly.EndpointMaker) (router chi.Router, subpaths bool) {
	r := chi.NewRouter()

	login := api.routesForLogin(mid, em)
	tokens := api.routesForToken(mid, em)
	users := api.routesForAuthUser(mid, em)
	info := api.routesForInfo(mid, em)

	r.Mount("/login", login)
	r.Mount("/tokens", tokens)
	r.Mount("/users", users)
	r.Mount("/info", info)
	r.HandleFunc("/info/", jelly.RedirectNoTrailingSlash)

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		res := response.NotFound()
		res.WriteResponse(w)
		res.Log(req)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		time.Sleep(api.UnauthDelay)
		res := response.MethodNotAllowed(req)
		res.WriteResponse(w)
		res.Log(req)
	})

	return r, true
}

func (api LoginAPI) routesForLogin(mid *middle.Provider, em jelly.EndpointMaker) chi.Router {
	reqAuth := mid.RequireAuth(api.name + ".jwt")

	r := chi.NewRouter()

	r.Post("/", api.HTTPCreateLogin(em))
	r.With(reqAuth).Delete("/"+p("id:uuid"), api.HTTPDeleteLogin(em))
	r.HandleFunc("/"+p("id:uuid")+"/", jelly.RedirectNoTrailingSlash)

	return r
}

func (api LoginAPI) routesForToken(mid *middle.Provider, em jelly.EndpointMaker) chi.Router {
	reqAuth := mid.RequireAuth(api.name + ".jwt")

	r := chi.NewRouter()

	r.With(reqAuth).Post("/", api.HTTPCreateToken(em))

	return r
}

func (api LoginAPI) routesForAuthUser(mid *middle.Provider, em jelly.EndpointMaker) chi.Router {
	reqAuth := mid.RequireAuth(api.name + ".jwt")

	r := chi.NewRouter()

	r.Use(reqAuth)

	r.Get("/", api.HTTPGetAllUsers(em))
	r.Post("/", api.HTTPCreateUser(em))

	r.Route("/"+p("id:uuid"), func(r chi.Router) {
		r.Get("/", api.HTTPGetUser(em))
		r.Put("/", api.HTTPReplaceUser(em))
		r.Patch("/", api.HTTPUpdateUser(em))
		r.Delete("/", api.HTTPDeleteUser(em))
	})

	return r
}

func (api LoginAPI) routesForInfo(mid *middle.Provider, em jelly.EndpointMaker) chi.Router {
	optAuth := mid.OptionalAuth(api.name + ".jwt")

	r := chi.NewRouter()

	r.With(optAuth).Get("/", api.HTTPGetInfo(em))

	return r
}
