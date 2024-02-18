// Package jelly is a simple and quick framework dekarrin 'jello' uses for
// learning Go servers.
//
// "Gelatin-based web servers".
package jelly

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// API holds parameters for endpoints needed to run and a service layer that
// will perform most of the actual logic. To use API, create one and then
// assign the result of its HTTP* methods as handlers to a router or some other
// kind of server mux.
type API interface {

	// Init creates the API initially and does any setup other than routing its
	// endpoints. It takes in a bundle that allows access to API config object,
	// connected DBs that the API is configured to use, a logger, and any other
	// resources available to the initializing API. Only those stores requested
	// in the API's config in the 'uses' key will be included in the bundle.
	//
	// The API should not expect that any other API has yet been initialized,
	// during a call to Init, and should not attempt to use auth middleware that
	// relies on other APIs (such as jellyauth's jwt provider). Defer actual
	// usage to another function, such as Routes.
	Init(bndl Bundle) error

	// Authenticators returns any configured authenticators that this API
	// provides. Other APIs will be able to refer to these authenticators by
	// name.
	//
	// Init must be called before Authenticators is called. It is not gauranteed
	// that all APIs in the server will have had Init called by the time a given
	// API has Authenticators called on it.
	//
	// Any Authenticator returned from this is automatically registered as an
	// Authenticator with the Auth middleware engine. Do not do so manually or
	// there may be conflicts.
	Authenticators() map[string]Authenticator

	// Routes returns a router that leads to all accessible routes in the API.
	// Additionally, returns whether the API's router contains subpaths beyond
	// just setting methods on its relative root; this affects whether
	// path-terminal slashes are redirected in the base router the API
	// router is mounted in.
	//
	// An endpoint creator passed in provides access to creation of middleware
	// configured by the server's main config file for the server to use.
	// Additionally, it also provides an Endpoint method which will wrap a
	// jelly-framework style endpoint in an http.HandlerFunc that will apply
	// standard actions such as logging, error, and panic catching.
	//
	// Init is guaranteed to have been called for all APIs in the server before
	// Routes is called, and it is safe to refer to middleware services that
	// rely on other APIs within.
	Routes(ServiceProvider) (router chi.Router, subpaths bool) // TODO: remove subpaths!!

	// Shutdown terminates any pending operations cleanly and releases any held
	// resources. It will be called after the server listener socket is shut
	// down. Implementors should examine the context's Done() channel to see if
	// they should halt during long-running operations, and do so if requested.
	Shutdown(ctx context.Context) error
}

type Component interface {
	// Name returns the name of the component, which must be unique across all
	// components that jelly is set up to use.
	Name() string

	// API returns a new, uninitialized API that the Component uses as its
	// server frontend. This instance will be initialized and passed its config
	// object at config loading time.
	API() API

	// Config returns a new APIConfig instance that the Component's config
	// section is loaded into.
	Config() APIConfig
}

type RESTServer interface {
	Config() Config
	RoutesIndex() string
	Add(name string, api API) error
	ServeForever() error
	Shutdown(ctx context.Context) error
}

// TODO: combine this bundle with the primary one
type Bundle struct {
	CBundle
	logger Logger
	dbs    map[string]Store
}

func NewBundle(apiConf CBundle, log Logger, dbs map[string]Store) Bundle {
	return Bundle{
		CBundle: apiConf,
		logger:  log,
		dbs:     dbs,
	}
}

func (bndl Bundle) Logger() Logger {
	return bndl.logger
}

// DB gets the connection to the Nth DB listed in the API's uses. Panics if the API
// config does not have at least n+1 entries.
func (bndl Bundle) DB(n int) Store {
	dbName := bndl.UsesDBs()[n]
	return bndl.DBNamed(dbName)
}

// NamedDB gets the exact DB with the given name. This will only return the DB if it
// was configured as one of the used DBs for the API.
func (bndl Bundle) DBNamed(name string) Store {
	return bndl.dbs[strings.ToLower(name)]
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

type Store interface {

	// Close closes any pending operations on the DAO store and on all of its
	// Repos. It performs any clean-up operations necessary and should always be
	// called once the Store is no longer in use.
	Close() error
}

// Middleware is a function that takes a handler and returns a new handler which
// wraps the given one and provides some additional functionality.
type Middleware func(next http.Handler) http.Handler
