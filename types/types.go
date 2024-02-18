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
	"path/filepath"
	"strconv"
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

// DBType is the type of a Database connection.
type DBType string

func (dbt DBType) String() string {
	return string(dbt)
}

const (
	DatabaseNone     DBType = "none"
	DatabaseSQLite   DBType = "sqlite"
	DatabaseOWDB     DBType = "owdb"
	DatabaseInMemory DBType = "inmem"
)

// ParseDBType parses a string found in a connection string into a DBType.
func ParseDBType(s string) (DBType, error) {
	sLower := strings.ToLower(s)

	switch sLower {
	case DatabaseSQLite.String():
		return DatabaseSQLite, nil
	case DatabaseInMemory.String():
		return DatabaseInMemory, nil
	case DatabaseOWDB.String():
		return DatabaseOWDB, nil
	default:
		return DatabaseNone, fmt.Errorf("DB type %q is not one of 'sqlite', 'owdb', or 'inmem'", s)
	}
}

type APIConfig interface {
	// Common returns the parts of the API configuration that all APIs are
	// required to have. Its keys should be considered part of the configuration
	// held within the APIConfig and any function that accepts keys will accept
	// the Common keys; additionally, FillDefaults and Validate will both
	// perform their operations on the Common's keys.
	//
	// Performing mutation operations on the Common() returned will not
	// necessarily affect the APIConfig it came from. Affecting one of its key's
	// values should be done by calling the appropriate method on the APIConfig
	// with the key name.
	Common() CommonConfig

	// Keys returns a list of strings, each of which is a valid key that this
	// configuration contains. These keys may be passed to other methods to
	// access values in this config.
	//
	// Each key returned should be alpha-numeric, and snake-case is preferred
	// (though not required). If a key contains an illegal character for a
	// particular format of a config source, it will be replaced with an
	// underscore in that format; e.g. a key called "test!" would be retrieved
	// from an envvar called "APPNAME_TEST_" as opposed to "APPNAME_TEST!", as
	// the exclamation mark is not allowed in most environment variable names.
	//
	// The returned slice will contain the values returned by Common()'s Keys()
	// function as well as any other keys provided by the APIConfig. Each item
	// in the returned slice must be non-empty and unique when all keys are
	// converted to lowercase.
	Keys() []string

	// Get gets the current value of a config key. The parameter key should be a
	// string that is returned from Keys(). If key is not a string that was
	// returned from Keys, this function must return nil.
	//
	// The key is not case-sensitive.
	Get(key string) interface{}

	// Set sets the current value of a config key directly. The value must be of
	// the correct type; no parsing is done in Set.
	//
	// The key is not case-sensitive.
	Set(key string, value interface{}) error

	// SetFromString sets the current value of a config key by parsing the given
	// string for its value.
	//
	// The key is not case-sensitive.
	SetFromString(key string, value string) error

	// FillDefaults returns a copy of the APIConfig with any unset values set to
	// default values, if possible. It need not be a brand new copy; it is legal
	// for implementers to returns the same APIConfig that FillDefaults was
	// called on.
	//
	// Implementors must ensure that the returned APIConfig's Common() returns a
	// common config that has had its keys set to their defaults as well.
	FillDefaults() APIConfig

	// Validate checks all current values of the APIConfig and returns whether
	// there is any issues with them.
	//
	// Implementors must ensure that calling Validate() also calls validation on
	// the common keys as well as those that they provide.
	Validate() error
}

const (
	ConfigKeyAPIName    = "name"
	ConfigKeyAPIBase    = "base"
	ConfigKeyAPIEnabled = "enabled"
	ConfigKeyAPIUsesDBs = "uses"
)

// CommonConfig holds configuration options common to all APIs.
type CommonConfig struct {
	// Name is the name of the API. Must be unique.
	Name string

	// Enabled is whether the API is to be enabled. By default, this is false in
	// all cases.
	Enabled bool

	// Base is the base URI that all paths will be rooted at, relative to the
	// server base path. This can be "/" (or "", which is equivalent) to
	// indicate that the API is to be based directly at the URIBase of the
	// server config that this API is a part of.
	Base string

	// UsesDBs is a list of names of data stores and authenticators that the API
	// uses directly. When Init is called, it is passed active connections to
	// each of the DBs. There must be a corresponding entry for each DB name in
	// the root DBs listing in the Config this API is a part of. The
	// Authenticators slice should contain only authenticators that are provided
	// by other APIs; see their documentation for which they provide.
	UsesDBs []string
}

// FillDefaults returns a new *Common identical to cc but with unset values set
// to their defaults and values normalized.
func (cc *CommonConfig) FillDefaults() APIConfig {
	newCC := new(CommonConfig)
	*newCC = *cc

	if newCC.Base == "" {
		newCC.Base = "/"
	}

	return newCC
}

func validateBaseURI(base string) error {
	if strings.ContainsRune(base, '{') {
		return fmt.Errorf("contains disallowed char \"{\"")
	}
	if strings.ContainsRune(base, '}') {
		return fmt.Errorf("contains disallowed char \"}\"")
	}
	if strings.Contains(base, "//") {
		return fmt.Errorf("contains disallowed double-slash \"//\"")
	}
	return nil
}

// Validate returns an error if the Config has invalid field values set. Empty
// and unset values are considered invalid; if defaults are intended to be used,
// call Validate on the return value of FillDefaults.
func (cc *CommonConfig) Validate() error {
	if err := validateBaseURI(cc.Base); err != nil {
		return fmt.Errorf(ConfigKeyAPIBase+": %w", err)
	}

	return nil
}

func (cc *CommonConfig) Common() CommonConfig {
	return *cc
}

func (cc *CommonConfig) Keys() []string {
	return []string{ConfigKeyAPIName, ConfigKeyAPIEnabled, ConfigKeyAPIBase, ConfigKeyAPIUsesDBs}
}

func (cc *CommonConfig) Get(key string) interface{} {
	switch strings.ToLower(key) {
	case ConfigKeyAPIName:
		return cc.Name
	case ConfigKeyAPIEnabled:
		return cc.Enabled
	case ConfigKeyAPIBase:
		return cc.Base
	case ConfigKeyAPIUsesDBs:
		return cc.UsesDBs
	default:
		return nil
	}
}

func (cc *CommonConfig) Set(key string, value interface{}) error {
	switch strings.ToLower(key) {
	case ConfigKeyAPIName:
		if valueStr, ok := value.(string); ok {
			cc.Name = valueStr
			return nil
		} else {
			return fmt.Errorf("key '"+ConfigKeyAPIName+"' requires a string but got a %T", value)
		}
	case ConfigKeyAPIEnabled:
		if valueBool, ok := value.(bool); ok {
			cc.Enabled = valueBool
			return nil
		} else {
			return fmt.Errorf("key '"+ConfigKeyAPIEnabled+"' requires a bool but got a %T", value)
		}
	case ConfigKeyAPIBase:
		if valueStr, ok := value.(string); ok {
			cc.Base = valueStr
			return nil
		} else {
			return fmt.Errorf("key '"+ConfigKeyAPIBase+"' requires a string but got a %T", value)
		}
	case ConfigKeyAPIUsesDBs:
		if valueStrSlice, ok := value.([]string); ok {
			cc.UsesDBs = valueStrSlice
			return nil
		} else {
			return fmt.Errorf("key '"+ConfigKeyAPIUsesDBs+"' requires a []string but got a %T", value)
		}
	default:
		return fmt.Errorf("not a valid key: %q", key)
	}
}

func (cc *CommonConfig) SetFromString(key string, value string) error {
	switch strings.ToLower(key) {
	case ConfigKeyAPIName, ConfigKeyAPIBase:
		return cc.Set(key, value)
	case ConfigKeyAPIEnabled:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		return cc.Set(key, b)
	case ConfigKeyAPIUsesDBs:
		if value == "" {
			return cc.Set(key, []string{})
		}
		dbsStrSlice := strings.Split(value, ",")
		return cc.Set(key, dbsStrSlice)
	default:
		return fmt.Errorf("not a valid key: %q", key)
	}
}

// LogConfig contains logging options. Loggers are provided to APIs in the form of
// sub-components of the primary logger. If logging is enabled, the Jelly server
// will configure the logger of the chosen provider and use it for messages
// about the server itself, and will pass a sub-component logger to each API to
// use for its own logging.
type LogConfig struct {
	// Enabled is whether to enable built-in logging statements.
	Enabled bool

	// Provider must be the name of one of the logging providers. If set to
	// None or unset, it will default to logging.Jellog.
	Provider LogProvider

	// File to log to. If not set, all logging will be done to stderr and it
	// will display all logging statements. If set, the file will receive all
	// levels of log messages and stderr will show only those of Info level or
	// higher.
	File string
}

func (log LogConfig) FillDefaults() LogConfig {
	newLog := log

	if newLog.Provider == NoLog {
		newLog.Provider = Jellog
	}

	return newLog
}

func (g LogConfig) Validate() error {
	if g.Provider == NoLog {
		return fmt.Errorf("provider: must not be empty")
	}

	return nil
}

// Globals are the values of global configuration values from the top level
// config. These values are shared with every API.
type Globals struct {

	// Port is the port that the server will listen on. It will default to 8080
	// if none is given.
	Port int

	// Address is the internet address that the server will listen on. It will
	// default to "localhost" if none is given.
	Address string

	// URIBase is the base path that all APIs are rooted on. It will default to
	// "/", which is equivalent to being directly on root.
	URIBase string

	// The main auth provider to use for the project. Must be the
	// fully-qualified name of it, e.g. COMPONENT.PROVIDER format.
	MainAuthProvider string
}

func (g Globals) FillDefaults() Globals {
	newG := g

	if newG.Port == 0 {
		newG.Port = 8080
	}
	if newG.Address == "" {
		newG.Address = "localhost"
	}
	if newG.URIBase == "" {
		newG.URIBase = "/"
	}

	return newG
}

func (g Globals) Validate() error {
	if g.Port < 1 {
		return fmt.Errorf("port: must be greater than 0")
	}
	if g.Address == "" {
		return fmt.Errorf("address: must not be empty")
	}
	if err := validateBaseURI(g.URIBase); err != nil {
		return fmt.Errorf("base: %w", err)
	}

	return nil
}

// Database contains configuration settings for connecting to a persistence
// layer.
type DatabaseConfig struct {
	// Type is the type of database the config refers to, primarily for data
	// validation purposes. It also determines which of its other fields are
	// valid.
	Type DBType

	// Connector is the name of the registered connector function that should be
	// used. The function name must be registered for DBs of the given type.
	Connector string

	// DataDir is the path on disk to a directory to use to store data in. This
	// is only applicable for certain DB types: SQLite, OWDB.
	DataDir string

	// DataFile is the name of the DB file to use for an OrbweaverDB (OWDB)
	// persistence store. By default, it is "db.owv". This is only applicable
	// for certain DB types: OWDB.
	DataFile string
}

// FillDefaults returns a new Database identical to db but with unset values
// set to their defaults. In this case, if the type is not set, it is changed to
// types.DatabaseInMemory. If OWDB File is not set, it is changed to "db.owv".
func (db DatabaseConfig) FillDefaults() DatabaseConfig {
	newDB := db

	if newDB.Type == DatabaseNone {
		newDB = DatabaseConfig{Type: DatabaseInMemory}
	}
	if newDB.Type == DatabaseOWDB && newDB.DataFile == "" {
		newDB.DataFile = "db.owv"
	}
	if newDB.Connector == "" {
		newDB.Connector = "*"
	}

	return newDB
}

// Validate returns an error if the Database does not have the correct fields
// set. Its type will be checked to ensure that it is a valid type to use and
// any fields necessary for connecting to that type of DB are also checked.
func (db DatabaseConfig) Validate() error {
	switch db.Type {
	case DatabaseInMemory:
		// nothing else to check
		return nil
	case DatabaseSQLite:
		if db.DataDir == "" {
			return fmt.Errorf("DataDir not set to path")
		}
		return nil
	case DatabaseOWDB:
		if db.DataDir == "" {
			return fmt.Errorf("DataDir not set to path")
		}
		return nil
	case DatabaseNone:
		return fmt.Errorf("'none' DB is not valid")
	default:
		return fmt.Errorf("unknown database type: %q", db.Type.String())
	}
}

// ParseDBConnString parses a database connection string of the form
// "engine:params" (or just "engine" if no other params are required) into a
// valid Database config object.
//
// Supported database types and a sample string containing valid configurations
// for each are shown below. Placeholder values are between angle brackets,
// optional parts are between square brackets. Ordering of parameters does not
// matter.
//
// * In-memory database: "inmem"
// * SQLite3 DB file: "sqlite:</path/to/db/dir>""
// * OrbweaverDB: "owdb:dir=<path/to/db/dir>[,file=<new-db-file-name.owv>]"
func ParseDBConnString(s string) (DatabaseConfig, error) {
	var paramStr string
	dbParts := strings.SplitN(s, ":", 2)

	if len(dbParts) == 2 {
		paramStr = strings.TrimSpace(dbParts[1])
	}

	// parse the first section into a type, from there we can determine if
	// further params are required.
	dbEng, err := ParseDBType(strings.TrimSpace(dbParts[0]))
	if err != nil {
		return DatabaseConfig{}, fmt.Errorf("unsupported DB engine: %w", err)
	}

	switch dbEng {
	case DatabaseInMemory:
		// there cannot be any other options
		if paramStr != "" {
			return DatabaseConfig{}, fmt.Errorf("unsupported param(s) for in-memory DB engine: %s", paramStr)
		}

		return DatabaseConfig{Type: DatabaseInMemory}, nil
	case DatabaseSQLite:
		// there must be options
		if paramStr == "" {
			return DatabaseConfig{}, fmt.Errorf("sqlite DB engine requires path to data directory after ':'")
		}

		// the only option is the DB path, as long as the param str isn't
		// literally blank, it can be used.

		// convert slashes to correct type
		dd := filepath.FromSlash(paramStr)
		return DatabaseConfig{Type: DatabaseSQLite, DataDir: dd}, nil
	case DatabaseOWDB:
		// there must be options
		if paramStr == "" {
			return DatabaseConfig{}, fmt.Errorf("owdb DB engine requires qualified path to data directory after ':'")
		}

		// split the arguments, simply go through and ignore unescaped
		params, err := parseParamsMap(paramStr)
		if err != nil {
			return DatabaseConfig{}, err
		}

		db := DatabaseConfig{Type: DatabaseOWDB}

		if val, ok := params["dir"]; ok {
			db.DataDir = filepath.FromSlash(val)
		} else {
			return DatabaseConfig{}, fmt.Errorf("owdb DB engine params missing qualified path to data directory in key 'dir'")
		}

		if val, ok := params["file"]; ok {
			db.DataFile = val
		} else {
			db.DataFile = "db.owv"
		}
		return db, nil
	case DatabaseNone:
		// not allowed
		return DatabaseConfig{}, fmt.Errorf("cannot specify DB engine 'none' (perhaps you wanted 'inmem'?)")
	default:
		// unknown
		return DatabaseConfig{}, fmt.Errorf("unknown DB engine: %q", dbEng.String())
	}
}

func parseParamsMap(paramStr string) (map[string]string, error) {
	seqs := splitWithEscaped(paramStr, ",")
	if len(seqs) < 1 {
		return nil, fmt.Errorf("not a map format string: %q", paramStr)
	}

	params := map[string]string{}
	for idx, kv := range seqs {
		parsed := splitWithEscaped(kv, "=")
		if len(parsed) != 2 {
			return nil, fmt.Errorf("param %d: not a kv-pair: %q", idx, kv)
		}
		k := parsed[0]
		v := parsed[1]
		params[strings.ToLower(k)] = v
	}

	return params, nil
}

// if sep contains a backslash, nil is returned.
func splitWithEscaped(s, sep string) []string {
	if strings.Contains(s, "\\") {
		return nil
	}
	var split []string
	var cur strings.Builder
	sepr := []rune(sep)
	sr := []rune(s)
	var seprPos int
	for i := 0; i < len(sr); i++ {
		ch := sr[i]

		if ch == sepr[seprPos] {
			if seprPos+1 >= len(sepr) {
				split = append(split, cur.String())
				cur.Reset()
				seprPos = 0
			} else {
				seprPos++
			}
		} else {
			seprPos = 0
		}

		if ch == '\\' {
			cur.WriteRune(ch)
			cur.WriteRune(sr[i+1])
			i++
		}
	}

	var preSepStr string
	if seprPos > 0 {
		preSepStr = string(sepr[0:seprPos])
	}
	if cur.Len() > 0 {
		split = append(split, preSepStr+cur.String())
	}

	return split
}

// Config is a complete configuration for a server. It contains all parameters
// that can be used to configure its operation.
type Config struct {

	// Globals is all variables shared with initialization of all APIs.
	Globals Globals

	// DBs is the configurations to use for connecting to databases and other
	// persistence layers. If not provided, it will be set to a configuration
	// for using an in-memory persistence layer.
	DBs map[string]DatabaseConfig

	// APIs is the configuration for each API that will be included in a
	// configured jelly framework server. Each APIConfig must return a
	// CommonConfig whose Name is either set to blank or to the key that maps to
	// it.
	APIs map[string]APIConfig

	// Log is used to configure the built-in logging system. It can be left
	// blank to disable logging entirely.
	Log LogConfig

	// origFormat is the format of config, as a string, used in Dump.
	origFormat string
}

// FillDefaults returns a new Config identical to cfg but with unset values
// set to their defaults.
func (cfg Config) FillDefaults() Config {
	newCFG := cfg

	for name, db := range newCFG.DBs {
		newCFG.DBs[name] = db.FillDefaults()
	}
	newCFG.Globals = newCFG.Globals.FillDefaults()
	for name, api := range newCFG.APIs {
		if Get[string](api, types.ConfigKeyAPIName) == "" {
			if err := api.Set(types.ConfigKeyAPIName, name); err != nil {
				panic(fmt.Sprintf("setting a config global failed; should never happen: %v", err))
			}
		}
		api = api.FillDefaults()
		newCFG.APIs[name] = api
	}
	newCFG.Log = newCFG.Log.FillDefaults()

	// if the user has enabled the jellyauth API, set defaults now.
	if authConf, ok := newCFG.APIs["jellyauth"]; ok {
		// make shore the first DB exists
		if Get[bool](authConf, types.ConfigKeyAPIEnabled) {
			dbs := Get[[]string](authConf, types.ConfigKeyAPIUsesDBs)
			if len(dbs) > 0 {
				// make shore this DB exists
				if _, ok := newCFG.DBs[dbs[0]]; !ok {
					newCFG.DBs[dbs[0]] = types.DatabaseConfig{Type: types.DatabaseInMemory, Connector: "authuser"}.FillDefaults()
				}
			}
			if newCFG.Globals.MainAuthProvider == "" {
				newCFG.Globals.MainAuthProvider = "jellyauth.jwt"
			}
		}
	}

	return newCFG
}

// Validate returns an error if the Config has invalid field values set. Empty
// and unset values are considered invalid; if defaults are intended to be used,
// call Validate on the return value of FillDefaults.
func (cfg Config) Validate() error {
	if err := cfg.Globals.Validate(); err != nil {
		return err
	}
	if err := cfg.Log.Validate(); err != nil {
		return fmt.Errorf("logging: %w", err)
	}
	for name, db := range cfg.DBs {
		if err := db.Validate(); err != nil {
			return fmt.Errorf("dbs: %s: %w", name, err)
		}
	}
	for name, api := range cfg.APIs {
		com := cfg.APIs[name].Common()

		if name != com.Name && com.Name != "" {
			return fmt.Errorf("%s: name mismatch; API.Name is set to %q", name, com.Name)
		}
		if err := api.Validate(); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}

	// all possible values for UnauthDelayMS are valid, so no need to check it

	return nil
}
