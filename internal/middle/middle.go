// Package middle contains middleware for use with the jelly server framework.
package middle

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/dekarrin/jelly"
	"github.com/google/uuid"
)

type mwFunc http.HandlerFunc

func (sf mwFunc) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	sf(w, req)
}

// ctxKey is a key in the context of a request populated by an AuthHandler.
type ctxKey int64

const (
	ctxKeyLoggedIn ctxKey = iota
	ctxKeyUser
)

func (ck ctxKey) String() string {
	switch ck {
	case ctxKeyLoggedIn:
		return "loggedIn"
	case ctxKeyUser:
		return "user"
	default:
		return fmt.Sprintf("ctxKey(%d)", int64(ck))
	}
}

func GetLoggedInUser(req *http.Request) (user jelly.AuthUser, loggedIn bool) {
	ctx := req.Context()
	loggedInUntyped := ctx.Value(ctxKeyLoggedIn)

	if loggedInUntyped == nil {
		return jelly.AuthUser{}, false
	}

	var ok bool
	loggedIn, ok = loggedInUntyped.(bool)
	if !ok {
		panic(fmt.Sprintf("bad type in request context; %s should have type bool but actually has type %T", ctxKeyLoggedIn, loggedInUntyped))
	}

	if loggedIn {
		userUntyped := ctx.Value(ctxKeyUser)
		if userUntyped == nil {
			panic(fmt.Sprintf("request context has value for %s but none for %s", ctxKeyLoggedIn, ctxKeyUser))
		}

		user, ok = userUntyped.(jelly.AuthUser)
		if !ok {
			panic(fmt.Sprintf("bad type in request context; %s should have type jelly.AuthUser but actually has type %T", ctxKeyUser, userUntyped))
		}
	}

	return user, loggedIn
}

// Provider is used to create middleware in a jelly framework project.
// Generally for callers, it will be accessed via delegated methods on an
// instance of [jelly.Environment].
type Provider struct {
	authenticators    map[string]jelly.Authenticator
	mainAuthenticator string
	DisableDefaults   bool
}

func (p *Provider) initDefaults() {
	if p.authenticators == nil {
		p.authenticators = map[string]jelly.Authenticator{}
		p.mainAuthenticator = ""
	}
}

// SelectAuthenticator retrieves and selects the first authenticator that
// matches one of the names in from. If no names are provided in from, the main
// auth for the project is returned. If from is not empty, at least one name
// listed in it must exist, or this function will panic.
func (p *Provider) SelectAuthenticator(from ...string) jelly.Authenticator {
	p.initDefaults()

	var authent jelly.Authenticator
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

func (p *Provider) getMainAuth() jelly.Authenticator {
	p.initDefaults()

	if p.mainAuthenticator == "" {
		return noopAuthenticator{}
	}
	return p.authenticators[p.mainAuthenticator]
}

func (p *Provider) RegisterMainAuthenticator(name string) error {
	p.initDefaults()

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

func (p *Provider) RegisterAuthenticator(name string, authen jelly.Authenticator) error {
	p.initDefaults()

	normName := strings.ToLower(name)
	if p.authenticators == nil {
		p.authenticators = map[string]jelly.Authenticator{}
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

// RequiredAuth returns middleware that requires that auth be used. The
// authenticators, if provided, must give the names of preferred providers that
// were registered as an jelly.Authenticator with this package, in priority order. If
// none of the given authenticators exist, this function panics. If no
// authenticator is specified, the one set as main for the project is used.
func (p Provider) RequiredAuth(resp jelly.ResponseGenerator, authenticators ...string) jelly.Middleware {
	prov := p.SelectAuthenticator(authenticators...)

	return func(next http.Handler) http.Handler {
		return &authHandler{
			provider: prov,
			required: true,
			next:     next,
			resp:     resp,
		}
	}
}

// OptionalAuth returns middleware that allows auth be used to retrieved the
// logged-in user. The authenticators, if provided, must give the names of
// preferred providers that were registered as an jelly.Authenticator with this
// package, in priority order. If none of the given authenticators exist, this
// function panics. If no authenticator is specified, the one set as main for
// the project is used.
func (p Provider) OptionalAuth(resp jelly.ResponseGenerator, authenticators ...string) jelly.Middleware {
	prov := p.SelectAuthenticator(authenticators...)

	return func(next http.Handler) http.Handler {
		return &authHandler{
			provider: prov,
			required: false,
			next:     next,
			resp:     resp,
		}
	}
}

// DontPanic returns a Middleware that performs a panic check as it exits. If
// the function is panicking, it will write out an HTTP response with a generic
// message to the client and add it to the log.
func (p Provider) DontPanic(resp jelly.ResponseGenerator) jelly.Middleware {
	return func(next http.Handler) http.Handler {
		return mwFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				if panicErr := recover(); panicErr != nil {
					r := resp.TextErr(
						http.StatusInternalServerError,
						"An internal server error occurred",
						fmt.Sprintf("panic: %v\nSTACK TRACE: %s", panicErr, string(debug.Stack())),
					)
					r.WriteResponse(w)
					resp.LogResponse(req, r)
				}
			}()
			next.ServeHTTP(w, req)
		})
	}
}

// noopAuthenticator is used as the active one when no others are specified.
type noopAuthenticator struct{}

func (na noopAuthenticator) Authenticate(req *http.Request) (jelly.AuthUser, bool, error) {
	return jelly.AuthUser{}, false, fmt.Errorf("no authenticator provider is specified for this project")
}

func (na noopAuthenticator) UnauthDelay() time.Duration {
	var d time.Duration
	return d
}

func (na noopAuthenticator) Service() jelly.UserLoginService {
	return noopLoginService{}
}

type noopLoginService struct{}

func (noop noopLoginService) Login(ctx context.Context, username string, password string) (jelly.AuthUser, error) {
	return jelly.AuthUser{}, fmt.Errorf("Login called on noop")
}
func (noop noopLoginService) Logout(ctx context.Context, who uuid.UUID) (jelly.AuthUser, error) {
	return jelly.AuthUser{}, fmt.Errorf("Logout called on noop")
}
func (noop noopLoginService) GetAllUsers(ctx context.Context) ([]jelly.AuthUser, error) {
	return nil, fmt.Errorf("GetAllUsers called on noop")
}
func (noop noopLoginService) GetUser(ctx context.Context, id string) (jelly.AuthUser, error) {
	return jelly.AuthUser{}, fmt.Errorf("GetUser called on noop")
}
func (noop noopLoginService) GetUserByUsername(ctx context.Context, username string) (jelly.AuthUser, error) {
	return jelly.AuthUser{}, fmt.Errorf("GetUserByUsername called on noop")
}
func (noop noopLoginService) CreateUser(ctx context.Context, username, password, email string, role jelly.Role) (jelly.AuthUser, error) {
	return jelly.AuthUser{}, fmt.Errorf("CreateUser called on noop")
}
func (noop noopLoginService) UpdateUser(ctx context.Context, curID, newID, username, email string, role jelly.Role) (jelly.AuthUser, error) {
	return jelly.AuthUser{}, fmt.Errorf("UpdateUser called on noop")
}
func (noop noopLoginService) UpdatePassword(ctx context.Context, id, password string) (jelly.AuthUser, error) {
	return jelly.AuthUser{}, fmt.Errorf("UpdatePassword called on noop")
}
func (noop noopLoginService) DeleteUser(ctx context.Context, id string) (jelly.AuthUser, error) {
	return jelly.AuthUser{}, fmt.Errorf("DeleteUser called on noop")
}

// authHandler is middleware that will accept a request, extract the token used
// for authentication, and make calls to get a User entity that represents the
// logged in user from the token.
//
// Keys are added to the request context before the request is passed to the
// next step in the chain. AuthUser will contain the logged-in user, and
// AuthLoggedIn will return whether the user is logged in (only applies for
// optional logins; for non-optional, not being logged in will result in an
// HTTP error being returned before the request is passed to the next handler).
type authHandler struct {
	provider jelly.Authenticator
	required bool
	next     http.Handler
	resp     jelly.ResponseGenerator
}

func (ah *authHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	user, loggedIn, err := ah.provider.Authenticate(req)

	if ah.required && !loggedIn {
		// there was a validation error or no error but not logged in.
		// if logging in is required, that's not okay.

		var msg string
		if err != nil {
			msg = err.Error()
		} else {
			msg = "authorization is required"
		}
		r := ah.resp.Unauthorized("", msg)
		time.Sleep(ah.provider.UnauthDelay())
		r.WriteResponse(w)
		ah.resp.LogResponse(req, r)
		return
	} else if !ah.required && err != nil {
		ah.resp.Logger().Warnf("optional auth returned error: %v", err)
	}

	ctx := req.Context()
	ctx = context.WithValue(ctx, ctxKeyLoggedIn, loggedIn)
	ctx = context.WithValue(ctx, ctxKeyUser, user)
	req = req.WithContext(ctx)
	ah.next.ServeHTTP(w, req)
}
