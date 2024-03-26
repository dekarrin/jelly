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

func (svc services) DontPanic() jelly.Middleware {
	return svc.mid.DontPanic(svc)
}

func (svc services) OptionalAuth(authenticators ...string) jelly.Middleware {
	return svc.mid.OptionalAuth(svc, authenticators...)
}

func (svc services) RequiredAuth(authenticators ...string) jelly.Middleware {
	return svc.mid.RequiredAuth(svc, authenticators...)
}

func (svc services) SelectAuthenticator(authenticators ...string) jelly.Authenticator {
	return svc.mid.SelectAuthenticator(authenticators...)
}

func (svc services) GetLoggedInUser(req *http.Request) (user jelly.AuthUser, loggedIn bool) {
	return middle.GetLoggedInUser(req)
}

func (svc services) Endpoint(ep jelly.EndpointFunc, overrides ...jelly.Override) http.HandlerFunc {
	overs := jelly.CombineOverrides(overrides)

	return func(w http.ResponseWriter, req *http.Request) {
		r := ep(req)

		if r.Status == http.StatusUnauthorized || r.Status == http.StatusForbidden || r.Status == http.StatusInternalServerError {
			// if it's one of these statuses, either the user is improperly
			// logging in or tried to access a forbidden resource, both of which
			// should force the wait time before responding.
			auth := svc.mid.SelectAuthenticator(overs.Authenticators...)
			time.Sleep(auth.UnauthDelay())
		}

		r.WriteResponse(w)
		svc.LogResponse(req, r)
	}
}
