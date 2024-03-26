package server

import (
	"net/http"
	"time"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/internal/middle"
)

type services struct {
	mid *middle.Provider
	log jelly.Logger
}

func (em services) DontPanic() jelly.Middleware {
	return em.mid.DontPanic(em)
}

func (em services) OptionalAuth(authenticators ...string) jelly.Middleware {
	return em.mid.OptionalAuth(em, authenticators...)
}

func (em services) RequiredAuth(authenticators ...string) jelly.Middleware {
	return em.mid.RequiredAuth(em, authenticators...)
}

func (em services) SelectAuthenticator(authenticators ...string) jelly.Authenticator {
	return em.mid.SelectAuthenticator(authenticators...)
}

func (em services) GetLoggedInUser(req *http.Request) (user jelly.AuthUser, loggedIn bool) {
	return middle.GetLoggedInUser(req)
}

func (em services) Endpoint(ep jelly.EndpointFunc, overrides ...jelly.Override) http.HandlerFunc {
	overs := jelly.CombineOverrides(overrides)

	return func(w http.ResponseWriter, req *http.Request) {
		r := ep(req)

		if r.Status == http.StatusUnauthorized || r.Status == http.StatusForbidden || r.Status == http.StatusInternalServerError {
			// if it's one of these statuses, either the user is improperly
			// logging in or tried to access a forbidden resource, both of which
			// should force the wait time before responding.
			auth := em.mid.SelectAuthenticator(overs.Authenticators...)
			time.Sleep(auth.UnauthDelay())
		}

		r.WriteResponse(w)
		em.LogResponse(req, r)
	}
}
