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
	"github.com/dekarrin/jelly/token"
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
	db            dao.AuthUserRepo
	secret        []byte
	required      bool
	defaultUser   dao.User
	unauthedDelay time.Duration
	next          http.Handler
}

func (ah *AuthHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var loggedIn bool
	user := ah.defaultUser

	tok, err := token.Get(req)
	if err != nil {
		// deliberately leaving as embedded if instead of &&
		if ah.required {
			// error here means token isn't present (or at least isn't in the
			// expected format, which for all intents and purposes is non-existent).
			// This is not okay if auth is required.

			r := response.Unauthorized("", err.Error())
			time.Sleep(ah.unauthedDelay)
			r.WriteResponse(w)
			r.Log(req)
			return
		}
	} else {
		// validate the token
		lookupUser, err := token.Validate(req.Context(), tok, ah.secret, ah.db)
		if err != nil {
			// deliberately leaving as embedded if instead of &&
			if ah.required {
				// there was a validation error. the user does not count as logged in.
				// if logging in is required, that's not okay.

				r := response.Unauthorized("", err.Error())
				time.Sleep(ah.unauthedDelay)
				r.WriteResponse(w)
				r.Log(req)
				return
			}
		} else {
			user = lookupUser
			loggedIn = true
		}
	}

	ctx := req.Context()
	ctx = context.WithValue(ctx, AuthLoggedIn, loggedIn)
	ctx = context.WithValue(ctx, AuthUser, user)
	req = req.WithContext(ctx)
	ah.next.ServeHTTP(w, req)
}

func RequireAuth(db dao.AuthUserRepo, secret []byte, unauthDelay time.Duration, defaultUser dao.User) Middleware {
	return func(next http.Handler) http.Handler {
		return &AuthHandler{
			db:            db,
			secret:        secret,
			unauthedDelay: unauthDelay,
			defaultUser:   dao.User{},
			required:      true,
			next:          next,
		}
	}
}

func OptionalAuth(db dao.AuthUserRepo, secret []byte, unauthDelay time.Duration, defaultUser dao.User) Middleware {
	return func(next http.Handler) http.Handler {
		return &AuthHandler{
			db:            db,
			secret:        secret,
			unauthedDelay: unauthDelay,
			defaultUser:   dao.User{},
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
