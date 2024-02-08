// Package middle contains middleware for use with the jelly server framework.
package middle

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/dekarrin/jelly/dao"
	"github.com/dekarrin/jelly/response"
)

type mwFunc http.HandlerFunc

func (sf mwFunc) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	sf(w, req)
}

// Middleware is a function that takes a handler and returns a new handler which
// wraps the given one and provides some additional functionality.
type Middleware func(next http.Handler) http.Handler

// AuthKey is a key in the context of a request populated by an AuthHandler.
type AuthKey int64

const (
	AuthLoggedIn AuthKey = iota
	AuthUser
)

// Provider is used to create middleware in a jelly framework project.
// Generally for callers, it will be accessed via delegated methods on an
// instance of [jelly.Environment].
type Provider struct {
	authenticators    map[string]Authenticator
	mainAuthenticator string
}

func DefaultProvider() Provider {
	return Provider{
		authenticators:    map[string]Authenticator{},
		mainAuthenticator: "",
	}
}

// SelectAuthenticator retrieves and selects the first authenticator that
// matches one of the names in from. If no names are provided in from, the main
// auth for the project is returned. If from is not empty, at least one name
// listed in it must exist, or this function will panic.
func (p Provider) SelectAuthenticator(from []string) Authenticator {
	var authent Authenticator
	if len(from) > 0 {
		if len(p.authenticators) < 1 {
			panic(fmt.Sprintf("no valid auth provider given in list: %q", from))
		}

		var ok bool
		for _, authName := range from {
			normName := strings.ToLower(authName)
			authent, ok = p.authenticators[normName]
			if ok {
				break
			}
		}
		if !ok {
			panic(fmt.Sprintf("no valid auth provider given in list: %q", from))
		}
	} else {
		authent = p.getMainAuth()
	}
	return authent
}

func (p Provider) getMainAuth() Authenticator {
	if p.mainAuthenticator == "" {
		return noopAuthenticator{}
	}
	return p.authenticators[p.mainAuthenticator]
}

func (p *Provider) RegisterMainAuthenticator(name string) error {
	normName := strings.ToLower(name)

	if len(p.authenticators) < 1 {
		return fmt.Errorf("no authenticator called %q has been registered; register one before trying to set it as main", normName)
	}
	if _, ok := p.authenticators[name]; !ok {
		return fmt.Errorf("no authenticator called %q has been registered; register one before trying to set it as main", normName)
	}

	p.mainAuthenticator = normName
	return nil
}

func (p *Provider) RegisterAuthenticator(name string, authen Authenticator) error {
	normName := strings.ToLower(name)
	if p.authenticators == nil {
		p.authenticators = map[string]Authenticator{}
	}

	if _, ok := p.authenticators[normName]; ok {
		return fmt.Errorf("authenticator called %q already exists", normName)
	}

	if authen == nil {
		return fmt.Errorf("authenticator cannot be nil")
	}

	p.authenticators[normName] = authen
	return nil
}

// Authenticator is middleware for an endpoint that will accept a request,
// extract the token used for authentication, and make calls to get a User
// entity that represents the logged in user from the token.
//
// Keys are added to the request context before the request is passed to the
// next step in the chain. AuthUser will contain the logged-in user, and
// AuthLoggedIn will return whether the user is logged in (only applies for
// optional logins; for non-optional, not being logged in will result in an
// HTTP error being returned before the request is passed to the next handler).
type Authenticator interface {

	// Authenticate retrieves the user details from the request using whatever
	// method is correct for the auth handler. Returns the user, whether the
	// user is currently logged in, and any error that occured. If the user is
	// not logged in but no error actually occured, a default user and logged-in
	// = false are returned with a nil error. An error should only be returned
	// if there is an issue authenticating the user, and a user not being logged
	// in does not count as an issue.
	//
	// If the user is logged-in, returns the logged-in user, true, and a nil
	// error.
	Authenticate(req *http.Request) (dao.User, bool, error)

	// UnauthDelay is the amount of time that the system should delay responding
	// to unauthenticated requests to endpoints that require auth.
	UnauthDelay() time.Duration
}

// noopAuthenticator is used as the active one when no others are specified.
type noopAuthenticator struct{}

func (na noopAuthenticator) Authenticate(req *http.Request) (dao.User, bool, error) {
	return dao.User{}, false, fmt.Errorf("no authenticator provider is specified for this project")
}

func (na noopAuthenticator) UnauthDelay() time.Duration {
	var d time.Duration
	return d
}

// AuthHandler is middleware that will accept a request, extract the token used
// for authentication, and make calls to get a User entity that represents the
// logged in user from the token.
//
// Keys are added to the request context before the request is passed to the
// next step in the chain. AuthUser will contain the logged-in user, and
// AuthLoggedIn will return whether the user is logged in (only applies for
// optional logins; for non-optional, not being logged in will result in an
// HTTP error being returned before the request is passed to the next handler).
type AuthHandler struct {
	provider Authenticator
	required bool
	next     http.Handler
}

func (ah *AuthHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	user, loggedIn, err := ah.provider.Authenticate(req)

	if ah.required {
		if err != nil || (err == nil && !loggedIn) {
			// there was a validation error or no error but not logged in.
			// if logging in is required, that's not okay.

			var msg string
			if err != nil {
				msg = err.Error()
			} else {
				msg = "authorization is required"
			}
			r := response.Unauthorized("", msg)
			time.Sleep(ah.provider.UnauthDelay())
			r.WriteResponse(w)
			r.Log(req)
			return
		}
	}

	ctx := req.Context()
	ctx = context.WithValue(ctx, AuthLoggedIn, loggedIn)
	ctx = context.WithValue(ctx, AuthUser, user)
	req = req.WithContext(ctx)
	ah.next.ServeHTTP(w, req)
}

// RequireAuth returns middleware that requires that auth be used. The
// authenticators, if provided, must give the names of preferred providers that
// were registered as an Authenticator with this package, in priority order. If
// none of the given authenticators exist, this function panics. If no
// authenticator is specified, the one set as main for the project is used.
func (p Provider) RequireAuth(authenticators ...string) Middleware {
	prov := p.SelectAuthenticator(authenticators)

	return func(next http.Handler) http.Handler {
		return &AuthHandler{
			provider: prov,
			required: true,
			next:     next,
		}
	}
}

// OptionalAuth returns middleware that allows auth be used to retrieved the
// logged-in user. The authenticators, if provided, must give the names of
// preferred providers that were registered as an Authenticator with this
// package, in priority order. If none of the given authenticators exist, this
// function panics. If no authenticator is specified, the one set as main for
// the project is used.
func (p Provider) OptionalAuth(authenticators ...string) Middleware {
	prov := p.SelectAuthenticator(authenticators)

	return func(next http.Handler) http.Handler {
		return &AuthHandler{
			provider: prov,
			required: false,
			next:     next,
		}
	}
}

// DontPanic returns a Middleware that performs a panic check as it exits. If
// the function is panicking, it will write out an HTTP response with a generic
// message to the client and add it to the log.
func (p Provider) DontPanic() Middleware {
	return func(next http.Handler) http.Handler {
		return mwFunc(func(w http.ResponseWriter, r *http.Request) {
			defer panicTo500(w, r)
			next.ServeHTTP(w, r)
		})
	}
}

func panicTo500(w http.ResponseWriter, req *http.Request) (panicVal interface{}) {
	if panicErr := recover(); panicErr != nil {
		r := response.TextErr(
			http.StatusInternalServerError,
			"An internal server error occurred",
			fmt.Sprintf("panic: %v\nSTACK TRACE: %s", panicErr, string(debug.Stack())),
		)
		r.WriteResponse(w)
		r.Log(req)
		return true
	}
	return false
}
