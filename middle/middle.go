// Package middle contains middleware for use with the jelly server framework.
package middle

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/dekarrin/jelly/types"
	"github.com/google/uuid"
)

type mwFunc http.HandlerFunc

func (sf mwFunc) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	sf(w, req)
}

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
	authenticators    map[string]types.Authenticator
	mainAuthenticator string
	DisableDefaults   bool
}

func GetLoggedInUser(req *http.Request) (user types.AuthUser, loggedIn bool) {
	loggedIn = req.Context().Value(AuthLoggedIn).(bool)
	if loggedIn {
		user = req.Context().Value(AuthUser).(types.AuthUser)
	}

	return user, loggedIn
}

func (p *Provider) initDefaults() {
	if p.authenticators == nil {
		p.authenticators = map[string]types.Authenticator{}
		p.mainAuthenticator = ""
	}
}

// SelectAuthenticator retrieves and selects the first authenticator that
// matches one of the names in from. If no names are provided in from, the main
// auth for the project is returned. If from is not empty, at least one name
// listed in it must exist, or this function will panic.
func (p *Provider) SelectAuthenticator(from ...string) types.Authenticator {
	p.initDefaults()

	var authent types.Authenticator
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

func (p *Provider) getMainAuth() types.Authenticator {
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

func (p *Provider) RegisterAuthenticator(name string, authen types.Authenticator) error {
	p.initDefaults()

	normName := strings.ToLower(name)
	if p.authenticators == nil {
		p.authenticators = map[string]types.Authenticator{}
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

// noopAuthenticator is used as the active one when no others are specified.
type noopAuthenticator struct{}

func (na noopAuthenticator) Authenticate(req *http.Request) (types.AuthUser, bool, error) {
	return types.AuthUser{}, false, fmt.Errorf("no authenticator provider is specified for this project")
}

func (na noopAuthenticator) UnauthDelay() time.Duration {
	var d time.Duration
	return d
}

func (na noopAuthenticator) Service() types.UserLoginService {
	return noopLoginService{}
}

type noopLoginService struct{}

func (noop noopLoginService) Login(ctx context.Context, username string, password string) (types.AuthUser, error) {
	return types.AuthUser{}, fmt.Errorf("Login called on noop")
}
func (noop noopLoginService) Logout(ctx context.Context, who uuid.UUID) (types.AuthUser, error) {
	return types.AuthUser{}, fmt.Errorf("Logout called on noop")
}
func (noop noopLoginService) GetAllUsers(ctx context.Context) ([]types.AuthUser, error) {
	return nil, fmt.Errorf("GetAllUsers called on noop")
}
func (noop noopLoginService) GetUser(ctx context.Context, id string) (types.AuthUser, error) {
	return types.AuthUser{}, fmt.Errorf("GetUser called on noop")
}
func (noop noopLoginService) GetUserByUsername(ctx context.Context, username string) (types.AuthUser, error) {
	return types.AuthUser{}, fmt.Errorf("GetUserByUsername called on noop")
}
func (noop noopLoginService) CreateUser(ctx context.Context, username, password, email string, role types.Role) (types.AuthUser, error) {
	return types.AuthUser{}, fmt.Errorf("CreateUser called on noop")
}
func (noop noopLoginService) UpdateUser(ctx context.Context, curID, newID, username, email string, role types.Role) (types.AuthUser, error) {
	return types.AuthUser{}, fmt.Errorf("UpdateUser called on noop")
}
func (noop noopLoginService) UpdatePassword(ctx context.Context, id, password string) (types.AuthUser, error) {
	return types.AuthUser{}, fmt.Errorf("UpdatePassword called on noop")
}
func (noop noopLoginService) DeleteUser(ctx context.Context, id string) (types.AuthUser, error) {
	return types.AuthUser{}, fmt.Errorf("DeleteUser called on noop")
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
	provider types.Authenticator
	required bool
	next     http.Handler
	resp     types.ResponseGenerator
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
			r := ah.resp.Unauthorized("", msg)
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

// RequiredAuth returns middleware that requires that auth be used. The
// authenticators, if provided, must give the names of preferred providers that
// were registered as an types.Authenticator with this package, in priority order. If
// none of the given authenticators exist, this function panics. If no
// authenticator is specified, the one set as main for the project is used.
func (p Provider) RequiredAuth(resp types.ResponseGenerator, authenticators ...string) types.Middleware {
	prov := p.SelectAuthenticator(authenticators...)

	return func(next http.Handler) http.Handler {
		return &AuthHandler{
			provider: prov,
			required: true,
			next:     next,
			resp:     resp,
		}
	}
}

// OptionalAuth returns middleware that allows auth be used to retrieved the
// logged-in user. The authenticators, if provided, must give the names of
// preferred providers that were registered as an types.Authenticator with this
// package, in priority order. If none of the given authenticators exist, this
// function panics. If no authenticator is specified, the one set as main for
// the project is used.
func (p Provider) OptionalAuth(resp types.ResponseGenerator, authenticators ...string) types.Middleware {
	prov := p.SelectAuthenticator(authenticators...)

	return func(next http.Handler) http.Handler {
		return &AuthHandler{
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
func (p Provider) DontPanic(resp types.ResponseGenerator) types.Middleware {
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
					r.Log(req)
				}
			}()
			next.ServeHTTP(w, req)
		})
	}
}
