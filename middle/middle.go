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

var (
	authProviders = map[string]Authenticator{}
)

func RegisterAuthenticator(name string, prov Authenticator) error {
	normName := strings.ToLower(name)
	if _, ok := authProviders[normName]; ok {
		return fmt.Errorf("authenticator called %q already exists", normName)
	}

	if prov == nil {
		return fmt.Errorf("authenticator cannot be nil")
	}

	authProviders[normName] = prov
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
	provider      Authenticator
	required      bool
	unauthedDelay time.Duration
	next          http.Handler
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
			time.Sleep(ah.unauthedDelay)
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
// authenticator must be the name of one that was registered as an Authenticator
// with this package.
//
// If the given authenticator does not exist, panics.
func RequireAuth(authenticator string, unauthDelay time.Duration) Middleware {
	prov, ok := authProviders[strings.ToLower(authenticator)]
	if !ok {
		panic(fmt.Sprintf("not a valid auth provider: %q", strings.ToLower(authenticator)))
	}

	return func(next http.Handler) http.Handler {
		return &AuthHandler{
			provider:      prov,
			unauthedDelay: unauthDelay,
			required:      true,
			next:          next,
		}
	}
}

// RequireAuth returns middleware that allows auth be used to retrieved the
// logged-in user. The authenticator must be the name of one that was registered
// as an Authenticator with this package.
//
// If the given authenticator does not exist, panics.
func OptionalAuth(authenticator string, unauthDelay time.Duration) Middleware {
	prov, ok := authProviders[strings.ToLower(authenticator)]
	if !ok {
		panic(fmt.Sprintf("not a valid auth provider: %q", strings.ToLower(authenticator)))
	}

	return func(next http.Handler) http.Handler {
		return &AuthHandler{
			provider:      prov,
			unauthedDelay: unauthDelay,
			required:      false,
			next:          next,
		}
	}
}

// DontPanic returns a Middleware that performs a panic check as it exits. If
// the function is panicking, it will write out an HTTP response with a generic
// message to the client and add it to the log.
func DontPanic() Middleware {
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
