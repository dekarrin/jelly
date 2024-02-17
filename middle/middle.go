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
	authenticators    map[string]Authenticator
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
		p.authenticators = map[string]Authenticator{}
		p.mainAuthenticator = ""
	}
}

// SelectAuthenticator retrieves and selects the first authenticator that
// matches one of the names in from. If no names are provided in from, the main
// auth for the project is returned. If from is not empty, at least one name
// listed in it must exist, or this function will panic.
func (p *Provider) SelectAuthenticator(from ...string) Authenticator {
	p.initDefaults()

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

func (p *Provider) getMainAuth() Authenticator {
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

func (p *Provider) RegisterAuthenticator(name string, authen Authenticator) error {
	p.initDefaults()

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
	Authenticate(req *http.Request) (types.AuthUser, bool, error)

	// Service returns the UserLoginService that can be used to control active
	// logins and the list of users.
	Service() UserLoginService

	// UnauthDelay is the amount of time that the system should delay responding
	// to unauthenticated requests to endpoints that require auth.
	UnauthDelay() time.Duration
}

// UserLoginService provides a way to control the state of login of users and
// retrieve users from the backend store.
type UserLoginService interface {
	// Login verifies the provided username and password against the existing user
	// in persistence and returns that user if they match. Returns the user entity
	// from the persistence layer that the username and password are valid for.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If the credentials do not match
	// a user or if the password is incorrect, it will match ErrBadCredentials. If
	// the error occured due to an unexpected problem with the DB, it will match
	// serr.ErrDB.
	Login(ctx context.Context, username string, password string) (types.AuthUser, error)

	// Logout marks the user with the given ID as having logged out, invalidating
	// any login that may be active. Returns the user entity that was logged out.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If the user doesn't exist, it
	// will match serr.ErrNotFound. If the error occured due to an unexpected
	// problem with the DB, it will match serr.ErrDB.
	Logout(ctx context.Context, who uuid.UUID) (types.AuthUser, error)

	// GetAllUsers returns all auth users currently in persistence.
	GetAllUsers(ctx context.Context) ([]types.AuthUser, error)

	// GetUser returns the user with the given ID.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If no user with that ID exists,
	// it will match serr.ErrNotFound. If the error occured due to an unexpected
	// problem with the DB, it will match serr.ErrDB. Finally, if there is an issue
	// with one of the arguments, it will match serr.ErrBadArgument.
	GetUser(ctx context.Context, id string) (types.AuthUser, error)

	// GetUserByUsername returns the user with the given username.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If no user with that ID exists,
	// it will match serr.ErrNotFound. If the error occured due to an unexpected
	// problem with the DB, it will match serr.ErrDB. Finally, if there is an issue
	// with one of the arguments, it will match serr.ErrBadArgument.
	GetUserByUsername(ctx context.Context, username string) (types.AuthUser, error)

	// CreateUser creates a new user with the given username, password, and email
	// combo. Returns the newly-created user as it exists after creation.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If a user with that username is
	// already present, it will match serr.ErrAlreadyExists. If the error occured
	// due to an unexpected problem with the DB, it will match serr.ErrDB. Finally,
	// if one of the arguments is invalid, it will match serr.ErrBadArgument.
	CreateUser(ctx context.Context, username, password, email string, role types.Role) (types.AuthUser, error)

	// UpdateUser sets all properties except the password of the user with the
	// given ID to the properties in the provider user. All the given properties
	// of the user (except password) will overwrite the existing ones. Returns
	// the updated user.
	//
	// This function cannot be used to update the password. Use UpdatePassword for
	// that.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If a user with that username or
	// ID (if they are changing) is already present, it will match
	// serr.ErrAlreadyExists. If no user with the given ID exists, it will match
	// serr.ErrNotFound. If the error occured due to an unexpected problem with the
	// DB, it will match serr.ErrDB. Finally, if one of the arguments is invalid, it
	// will match serr.ErrBadArgument.
	UpdateUser(ctx context.Context, curID, newID, username, email string, role types.Role) (types.AuthUser, error)

	// UpdatePassword sets the password of the user with the given ID to the new
	// password. The new password cannot be empty. Returns the updated user.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If no user with the given ID
	// exists, it will match serr.ErrNotFound. If the error occured due to an
	// unexpected problem with the DB, it will match serr.ErrDB. Finally, if one of
	// the arguments is invalid, it will match serr.ErrBadArgument.
	UpdatePassword(ctx context.Context, id, password string) (types.AuthUser, error)

	// DeleteUser deletes the user with the given ID. It returns the deleted user
	// just after they were deleted.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If no user with that username
	// exists, it will match serr.ErrNotFound. If the error occured due to an
	// unexpected problem with the DB, it will match serr.ErrDB. Finally, if there
	// is an issue with one of the arguments, it will match serr.ErrBadArgument.
	DeleteUser(ctx context.Context, id string) (types.AuthUser, error)
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

func (na noopAuthenticator) Service() UserLoginService {
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
	provider Authenticator
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
// were registered as an Authenticator with this package, in priority order. If
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
// preferred providers that were registered as an Authenticator with this
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
