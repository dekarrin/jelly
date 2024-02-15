package server

import (
	"net/http"
	"time"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/middle"
)

type endpointCreator struct {
	mid *middle.Provider
}

func (em endpointCreator) DontPanic() middle.Middleware {
	return em.mid.DontPanic(em)
}

func (em endpointCreator) OptionalAuth(authenticators ...string) middle.Middleware {
	return em.mid.OptionalAuth(em, authenticators...)
}

func (em endpointCreator) RequiredAuth(authenticators ...string) middle.Middleware {
	return em.mid.RequiredAuth(em, authenticators...)
}

func (em endpointCreator) SelectAuthenticator(authenticators ...string) middle.Authenticator {
	return em.mid.SelectAuthenticator(authenticators...)
}

func (em endpointCreator) Endpoint(ep jelly.EndpointFunc, overrides ...jelly.Override) http.HandlerFunc {
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
		r.Log(req)
	}
}