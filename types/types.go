// Package types is a top-level types collection used to move in types before
// eventually relocating them to jelly root package.
//
// TODO: this package must be eliminated by 0.1.0.
package types

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ErrorResponse struct {
	Error  string `json:"error"`
	Status int    `json:"status"`
}

type Result struct {
	Status      int
	IsErr       bool
	IsJSON      bool
	InternalMsg string

	Resp  interface{}
	Redir string // only used for redirects

	hdrs [][2]string

	// set by calling PrepareMarshaledResponse.
	respJSONBytes []byte
}

func (r Result) WithHeader(name, val string) Result {
	erCopy := Result{
		IsErr:       r.IsErr,
		IsJSON:      r.IsJSON,
		Status:      r.Status,
		InternalMsg: r.InternalMsg,
		Resp:        r.Resp,
		hdrs:        r.hdrs,
	}

	erCopy.hdrs = append(erCopy.hdrs, [2]string{name, val})
	return erCopy
}

// PrepareMarshaledResponse sets the respJSONBytes to the marshaled version of
// the response if required. If required, and there is a problem marshaling, an
// error is returned. If not required, nil error is always returned.
//
// If PrepareMarshaledResponse has been successfully called with a non-nil
// returned error at least once for r, calling this method again has no effect
// and will return a  non-nil error.
func (r *Result) PrepareMarshaledResponse() error {
	if r.respJSONBytes != nil {
		return nil
	}

	if r.IsJSON && r.Status != http.StatusNoContent && r.Redir == "" {
		var err error
		r.respJSONBytes, err = json.Marshal(r.Resp)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r Result) WriteResponse(w http.ResponseWriter) {
	// if this hasn't been properly created, panic
	if r.Status == 0 {
		panic("result not populated")
	}

	err := r.PrepareMarshaledResponse()
	if err != nil {
		panic(fmt.Sprintf("could not marshal response: %s", err.Error()))
	}

	var respBytes []byte

	if r.IsJSON {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Redir == "" {
			respBytes = r.respJSONBytes
		}
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Status != http.StatusNoContent && r.Redir == "" {
			respBytes = []byte(fmt.Sprintf("%v", r.Resp))
		}
	}

	// if there is a redir, handle that now
	if r.Redir != "" {
		w.Header().Set("Location", r.Redir)
	}

	for i := range r.hdrs {
		w.Header().Set(r.hdrs[i][0], r.hdrs[i][1])
	}

	w.WriteHeader(r.Status)

	if r.Status != http.StatusNoContent {
		w.Write(respBytes)
	}
}

// TODO: detangle this and make it use the actual log provider.
func (r Result) Log(req *http.Request) {
	if r.IsErr {
		LogHTTPResponse("ERROR", req, r.Status, r.InternalMsg)
	} else {
		LogHTTPResponse("INFO", req, r.Status, r.InternalMsg)
	}
}

func LogHTTPResponse(level string, req *http.Request, respStatus int, msg string) {
	if len(level) > 5 {
		level = level[0:5]
	}

	for len(level) < 5 {
		level += " "
	}

	// we don't really care about the ephemeral port from the client end
	remoteAddrParts := strings.SplitN(req.RemoteAddr, ":", 2)
	remoteIP := remoteAddrParts[0]

	log.Printf("%s %s %s %s: HTTP-%d %s", level, remoteIP, req.Method, req.URL.Path, respStatus, msg)
}

type ResponseGenerator interface {
	OK(respObj interface{}, internalMsg ...interface{}) Result
	NoContent(internalMsg ...interface{}) Result
	Created(respObj interface{}, internalMsg ...interface{}) Result
	Conflict(userMsg string, internalMsg ...interface{}) Result
	BadRequest(userMsg string, internalMsg ...interface{}) Result
	MethodNotAllowed(req *http.Request, internalMsg ...interface{}) Result
	NotFound(internalMsg ...interface{}) Result
	Forbidden(internalMsg ...interface{}) Result
	Unauthorized(userMsg string, internalMsg ...interface{}) Result
	InternalServerError(internalMsg ...interface{}) Result
	Redirection(uri string) Result
	Response(status int, respObj interface{}, internalMsg string, v ...interface{}) Result
	Err(status int, userMsg, internalMsg string, v ...interface{}) Result
	TextErr(status int, userMsg, internalMsg string, v ...interface{}) Result
}

// Logger is an object that is used to log messages. Use the New functions in
// the logging sub-package to create one.
type Logger interface {
	// Debug writes a message to the log at Debug level.
	Debug(string)

	// Debugf writes a formatted message to the log at Debug level.
	Debugf(string, ...interface{})

	// Error writes a message to the log at Error level.
	Error(string)

	// Errorf writes a formatted message to the log at Error level.
	Errorf(string, ...interface{})

	// Info writes a message to the log at Info level.
	Info(string)

	// Infof writes a formatted message to the log at Info level.
	Infof(string, ...interface{})

	// Trace writes a message to the log at Trace level.
	Trace(string)

	// Tracef writes a formatted message to the log at Trace level.
	Tracef(string, ...interface{})

	// Warn writes a message to the log at Warn level.
	Warn(string)

	// Warnf writes a formatted message to the log at Warn level.
	Warnf(string, ...interface{})

	// DebugBreak adds a 'break' between events in the log at Debug level. The
	// meaning of a break varies based on the underlying log; for text-based
	// logs, it is generally a newline character.
	DebugBreak()

	// ErrorBreak adds a 'break' between events in the log at Error level. The
	// meaning of a break varies based on the underlying log; for text-based
	// logs, it is generally a newline character.
	ErrorBreak()

	// InfoBreak adds a 'break' between events in the log at Info level. The
	// meaning of a break varies based on the underlying log; for text-based
	// logs, it is generally a newline character.
	InfoBreak()

	// TraceBreak adds a 'break' between events in the log at Trace level. The
	// meaning of a break varies based on the underlying log; for text-based
	// logs, it is generally a newline character.
	TraceBreak()

	// WarnBreak adds a 'break' between events in the log at Warn level. The
	// meaning of a break varies based on the underlying log; for text-based
	// logs, it is generally a newline character.
	WarnBreak()

	// LogResult logs a request and the response to that request.
	LogResult(req *http.Request, r Result)
}

type LogProvider int

const (
	NoLog LogProvider = iota
	Jellog
)

func (p LogProvider) String() string {
	switch p {
	case NoLog:
		return "none"
	case Jellog:
		return "jellog"
	default:
		return fmt.Sprintf("LogProvider(%d)", int(p))
	}
}

func ParseLogProvider(s string) (LogProvider, error) {
	switch strings.ToLower(s) {
	case NoLog.String(), "":
		return NoLog, nil
	case Jellog.String():
		return Jellog, nil
	default:
		return NoLog, fmt.Errorf("unknown LogProvider %q", s)
	}
}

var (
	DBErrConstraintViolation = errors.New("a uniqueness constraint was violated")
	DBErrNotFound            = errors.New("the requested resource was not found")
	DBErrDecodingFailure     = errors.New("field could not be decoded from DB storage format to model format")
)

type Role int64

const (
	Guest Role = iota
	Unverified
	Normal

	Admin Role = 100
)

func (r Role) String() string {
	switch r {
	case Guest:
		return "guest"
	case Unverified:
		return "unverified"
	case Normal:
		return "normal"
	case Admin:
		return "admin"
	default:
		return fmt.Sprintf("Role(%d)", r)
	}
}

func (r Role) Value() (driver.Value, error) {
	return int64(r), nil
}

func (r *Role) Scan(value interface{}) error {
	iVal, ok := value.(int64)
	if !ok {
		return fmt.Errorf("not an integer value: %v", value)
	}

	*r = Role(iVal)

	return nil
}

func ParseRole(s string) (Role, error) {
	check := strings.ToLower(s)
	switch check {
	case "guest":
		return Guest, nil
	case "unverified":
		return Unverified, nil
	case "normal":
		return Normal, nil
	case "admin":
		return Admin, nil
	default:
		return Guest, fmt.Errorf("must be one of 'guest', 'unverified', 'normal', or 'admin'")
	}
}

// AuthUser is an auth model for use in the pre-rolled auth mechanism of user-in-db
// and login identified via JWT.
type AuthUser struct {
	ID         uuid.UUID // PK, NOT NULL
	Username   string    // UNIQUE, NOT NULL
	Password   string    // NOT NULL
	Email      string    // NOT NULL
	Role       Role      // NOT NULL
	Created    time.Time // NOT NULL
	Modified   time.Time // NOT NULL
	LastLogout time.Time // NOT NULL DEFAULT NOW()
	LastLogin  time.Time // NOT NULL
}

type Store interface {

	// Close closes any pending operations on the DAO store and on all of its
	// Repos. It performs any clean-up operations necessary and should always be
	// called once the Store is no longer in use.
	Close() error
}

type AuthUserRepo interface {
	// Create creates a new model in the DB based on the provided one. Some
	// attributes in the provided one might not be used; for instance, many
	// Repos will automatically set the ID of new entities on creation, ignoring
	// any initially set ID. It is up to implementors to decide which attributes
	// are used.
	//
	// This returns the object as it appears in the DB after creation.
	//
	// An implementor may provide an empty implementation with a function that
	// always returns an error regardless of state and input. Consult the
	// documentation of the implementor for info.
	Create(context.Context, AuthUser) (AuthUser, error)

	// Get retrieves the model with the given ID. If no entity with that ID
	// exists, an error is returned.
	//
	// An implementor may provide an empty implementation with a function that
	// always returns an error regardless of state and input. Consult the
	// documentation of the implementor for info.
	Get(context.Context, uuid.UUID) (AuthUser, error)

	// GetAll retrieves all entities in the associated store. If no entities
	// exist but no error otherwise occurred, the returned list of entities will
	// have a length of zero and the returned error will be nil.
	//
	// An implementor may provide an empty implementation with a function that
	// always returns an error regardless of state and input. Consult the
	// documentation of the implementor for info.
	GetAll(context.Context) ([]AuthUser, error)

	// Update updates a particular entity in the store to match the provided
	// model. Implementors may choose which properties of the provided value are
	// actually used.
	//
	// This returns the object as it appears in the DB after updating.
	//
	// An implementor may provide an empty implementation with a function that
	// always returns an error regardless of state and input. Consult the
	// documentation of the implementor for info.
	Update(context.Context, uuid.UUID, AuthUser) (AuthUser, error)

	// Delete removes the given entity from the store.
	//
	// This returns the object as it appeared in the DB immediately before
	// deletion.
	//
	// An implementor may provide an empty implementation with a function that
	// always returns an error regardless of state and input. Consult the
	// documentation of the implementor for info.
	Delete(context.Context, uuid.UUID) (AuthUser, error)

	// Close performs any clean-up operations required and flushes pending
	// operations. Not all Repos will actually perform operations, but it should
	// always be called as part of tear-down operations.
	Close() error

	// TODO: one day, move owdb Criterion functionality over and use that as a
	// generic interface into searches. Then we can have a GetAllBy(Filter) and
	// GetOneBy(Filter).

	// GetByUsername retrieves the User with the given username. If no entity
	// with that username exists, an error is returned.
	GetByUsername(ctx context.Context, username string) (AuthUser, error)
}

// AuthUserStore is an interface that defines methods for building a DAO store
// to be used as part of user auth via the jelly framework packages.
//
// TODO: should this be its own "sub-package"? example implementations. Or
// something. feels like it should live closer to auth-y type things.
type AuthUserStore interface {
	Store

	// AuthUsers returns a repository that holds users used as part of
	// authentication and login.
	AuthUsers() AuthUserRepo
}

// Middleware is a function that takes a handler and returns a new handler which
// wraps the given one and provides some additional functionality.
type Middleware func(next http.Handler) http.Handler

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
	Login(ctx context.Context, username string, password string) (AuthUser, error)

	// Logout marks the user with the given ID as having logged out, invalidating
	// any login that may be active. Returns the user entity that was logged out.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If the user doesn't exist, it
	// will match serr.ErrNotFound. If the error occured due to an unexpected
	// problem with the DB, it will match serr.ErrDB.
	Logout(ctx context.Context, who uuid.UUID) (AuthUser, error)

	// GetAllUsers returns all auth users currently in persistence.
	GetAllUsers(ctx context.Context) ([]AuthUser, error)

	// GetUser returns the user with the given ID.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If no user with that ID exists,
	// it will match serr.ErrNotFound. If the error occured due to an unexpected
	// problem with the DB, it will match serr.ErrDB. Finally, if there is an issue
	// with one of the arguments, it will match serr.ErrBadArgument.
	GetUser(ctx context.Context, id string) (AuthUser, error)

	// GetUserByUsername returns the user with the given username.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If no user with that ID exists,
	// it will match serr.ErrNotFound. If the error occured due to an unexpected
	// problem with the DB, it will match serr.ErrDB. Finally, if there is an issue
	// with one of the arguments, it will match serr.ErrBadArgument.
	GetUserByUsername(ctx context.Context, username string) (AuthUser, error)

	// CreateUser creates a new user with the given username, password, and email
	// combo. Returns the newly-created user as it exists after creation.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If a user with that username is
	// already present, it will match serr.ErrAlreadyExists. If the error occured
	// due to an unexpected problem with the DB, it will match serr.ErrDB. Finally,
	// if one of the arguments is invalid, it will match serr.ErrBadArgument.
	CreateUser(ctx context.Context, username, password, email string, role Role) (AuthUser, error)

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
	UpdateUser(ctx context.Context, curID, newID, username, email string, role Role) (AuthUser, error)

	// UpdatePassword sets the password of the user with the given ID to the new
	// password. The new password cannot be empty. Returns the updated user.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If no user with the given ID
	// exists, it will match serr.ErrNotFound. If the error occured due to an
	// unexpected problem with the DB, it will match serr.ErrDB. Finally, if one of
	// the arguments is invalid, it will match serr.ErrBadArgument.
	UpdatePassword(ctx context.Context, id, password string) (AuthUser, error)

	// DeleteUser deletes the user with the given ID. It returns the deleted user
	// just after they were deleted.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If no user with that username
	// exists, it will match serr.ErrNotFound. If the error occured due to an
	// unexpected problem with the DB, it will match serr.ErrDB. Finally, if there
	// is an issue with one of the arguments, it will match serr.ErrBadArgument.
	DeleteUser(ctx context.Context, id string) (AuthUser, error)
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
	Authenticate(req *http.Request) (AuthUser, bool, error)

	// Service returns the UserLoginService that can be used to control active
	// logins and the list of users.
	Service() UserLoginService

	// UnauthDelay is the amount of time that the system should delay responding
	// to unauthenticated requests to endpoints that require auth.
	UnauthDelay() time.Duration
}
