package auth

import (
	"net/http"
	"time"

	"github.com/dekarrin/jelly"
	"github.com/go-chi/chi/v5"
)

func p(s string) (pathParam string) {
	return jelly.PathParam(s)
}

func (api *loginAPI) Routes(em jelly.EndpointServices) chi.Router {
	r := chi.NewRouter()

	login := api.routesForLogin(em)
	tokens := api.routesForToken(em)
	users := api.routesForAuthUser(em)
	info := api.routesForInfo(em)

	r.Mount("/login", login)
	r.Mount("/tokens", tokens)
	r.Mount("/users", users)
	r.Mount("/info", info)
	r.HandleFunc("/info/", jelly.RedirectNoTrailingSlash(em)) // TODO: this doesn't appear to do anyfin

	// TODO: make this library properly use jelly.RedirectNoTrailingSlash

	// TODO: should these be at top level and controlled by jelly?
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		res := em.NotFound()
		res.WriteResponse(w)
		em.LogResponse(req, res)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		time.Sleep(api.UnauthDelay)
		res := em.MethodNotAllowed(req)
		res.WriteResponse(w)
		em.LogResponse(req, res)
	})

	return r
}

func (api loginAPI) routesForLogin(em jelly.EndpointServices) chi.Router {
	reqAuth := em.RequiredAuth(api.name + ".jwt")

	r := chi.NewRouter()

	r.Post("/", api.httpCreateLogin(em))
	r.With(reqAuth).Delete("/"+p("id:uuid"), api.httpDeleteLogin(em))
	r.HandleFunc("/"+p("id:uuid")+"/", jelly.RedirectNoTrailingSlash(em))

	return r
}

func (api loginAPI) routesForToken(em jelly.EndpointServices) chi.Router {
	reqAuth := em.RequiredAuth(api.name + ".jwt")

	r := chi.NewRouter()

	r.With(reqAuth).Post("/", api.httpCreateToken(em))

	return r
}

func (api loginAPI) routesForAuthUser(em jelly.EndpointServices) chi.Router {
	reqAuth := em.RequiredAuth(api.name + ".jwt")

	r := chi.NewRouter()

	r.Use(reqAuth)

	r.Get("/", api.httpGetAllUsers(em))
	r.Post("/", api.httpCreateUser(em))

	r.Route("/"+p("id:uuid"), func(r chi.Router) {
		r.Get("/", api.httpGetUser(em))
		r.Put("/", api.httpReplaceUser(em))
		r.Patch("/", api.httpUpdateUser(em))
		r.Delete("/", api.httpDeleteUser(em))
	})

	return r
}

func (api loginAPI) routesForInfo(em jelly.EndpointServices) chi.Router {
	optAuth := em.OptionalAuth(api.name + ".jwt")

	r := chi.NewRouter()

	r.With(optAuth).Get("/", api.httpGetInfo(em))

	return r
}
