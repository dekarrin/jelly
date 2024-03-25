// Package jelly is a simple and quick framework dekarrin 'jello' uses for
// learning Go servers.
//
// "Gelatin-based web servers".
package jelly

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

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
	// If the returned router contains subpaths beyond just setting methods on
	// its relative root, path-terminal slashes are redirected in the base
	// router the API router is mounted in.
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
	Routes(ServiceProvider) chi.Router

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

type RoutesIndex struct {
	byEndpoint map[string][]string
	order      []string
	count      int
}

// String returns an informational string with the total number of unique routes
// and the endpoints in the server. To get an actual formatted printout, use
// FormattedList.
func (ri RoutesIndex) String() string {
	return fmt.Sprintf("Route(%d unique routes, %d endpoints)", ri.count, len(ri.byEndpoint))
}

// FormattedList returns all routes as a formatted list of paths and their
// endpoints. If none are present, returns "(no routes added)".
func (ri RoutesIndex) FormattedList() string {
	str := ri.formatBulletedList()
	if str == "" {
		return "(no routes added)"
	}
	return str
}

// Count returns the total number of unique routes (endpoint + method pairs) in
// the server.
func (ri RoutesIndex) Count() int {
	return ri.count
}

// EndpointCount returns the total number of endpoints in the server without
// considering the methods they are available at.
func (ri RoutesIndex) EndpointCount() int {
	return len(ri.byEndpoint)
}

func (ri RoutesIndex) forEachRoute(fn func(path string, methods []string)) {
	for _, path := range ri.order {
		func(path string, methods []string) {
			meths := ri.byEndpoint[path]
			fn(path, meths)
		}(path, ri.byEndpoint[path])
	}
}

func (ri RoutesIndex) formatBulletedList() string {
	var sb strings.Builder
	ri.forEachRoute(func(path string, methods []string) {
		sb.WriteString("* ")
		sb.WriteString(UnPathParam(path))
		sb.WriteString(" - ")

		sort.Strings(methods)
		for i, m := range methods {
			sb.WriteString(m)
			if i+1 < len(methods) {
				sb.WriteString(", ")
			}
		}
		sb.WriteRune('\n')
	})

	return strings.TrimSpace(sb.String())
}

func NewRoutesIndex(r chi.Router) RoutesIndex {
	routes := RoutesIndex{
		byEndpoint: make(map[string][]string),
		order:      []string{},
	}

	chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		meths, ok := routes.byEndpoint[route]
		if !ok {
			meths = []string{}
		}

		meths = append(meths, method)
		routes.byEndpoint[route] = meths
		routes.count++

		return nil
	})

	if len(routes.byEndpoint) < 1 {
		return routes
	}

	// alphabetize the routes
	for name := range routes.byEndpoint {
		routes.order = append(routes.order, name)
	}
	sort.Strings(routes.order)

	return routes
}

type RESTServer interface {
	Config() Config
	RoutesIndex() (RoutesIndex, error)
	Add(name string, api API) error
	ServeForever() error
	Shutdown(ctx context.Context) error
}

// TODO: combine this bundle with the primary one
type Bundle struct {
	api    APIConfig
	g      Globals
	logger Logger
	dbs    map[string]Store
}

func NewBundle(api APIConfig, g Globals, log Logger, dbs map[string]Store) Bundle {
	return Bundle{
		api:    api,
		g:      g,
		logger: log,
		dbs:    dbs,
	}
}

func (bndl Bundle) WithDBs(dbs map[string]Store) Bundle {
	return Bundle{
		api:    bndl.api,
		g:      bndl.g,
		logger: bndl.logger,
		dbs:    dbs,
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

// ServerPort returns the port that the server the API is being initialized for
// will listen on.
func (bndl Bundle) ServerPort() int {
	return bndl.g.Port
}

// ServerAddress returns the address that the server the API is being
// initialized for will listen on.
func (bndl Bundle) ServerAddress() string {
	return bndl.g.Address
}

// ServerBase returns the base path that all APIs in the server are mounted at.
// It will perform any needed normalization of the base string to ensure that it
// is non-empty, starts with a slash, and does not end with a slash except if it
// is "/". Can be useful for establishing "complete" paths to entities, although
// if a complete base path to the API is needed, [Bundle.Base] can be called.
func (bndl Bundle) ServerBase() string {
	base := bndl.g.URIBase

	for len(base) > 0 && base[len(base)-1] == '/' {
		// do not end with a slash, please
		base = base[:len(base)-1]
	}
	if len(base) == 0 || base[0] != '/' {
		base = "/" + base
	}

	return strings.ToLower(base)
}

// Base returns the complete URIBase path configured for any methods. This takes
// ServerBase() and APIBase() and appends them together, handling
// doubled-slashes.
func (bndl Bundle) Base() string {
	svBase := bndl.ServerBase()
	apiBase := bndl.APIBase()

	var base string
	if svBase != "" && svBase != "/" {
		base = svBase
	}
	if apiBase != "" && apiBase != "/" {
		if base != "" {
			base = base + apiBase
		} else {
			base = apiBase
		}
	}

	if base == "" {
		base = "/"
	}

	return base
}

// Has returns whether the given key exists in the API config.
func (bndl Bundle) Has(key string) bool {
	return apiHas(bndl.api, key)
}

// Name returns the name of the API as read from the API config.
//
// This is a convenience function equivalent to calling bnd.Get(KeyAPIName).
func (bndl Bundle) Name() string {
	return bndl.Get(ConfigKeyAPIName)
}

// APIBase returns the base path of the API that its routes are all mounted at.
// It will perform any needed normalization of the base string to ensure that it
// is non-empty, starts with a slash, and does not end with a slash except if it
// is "/". The returned base path is relative to the ServerBase; combine both
// ServerBase and APIBase to get the complete URI base path, or call Base() to
// do it for you.
//
// This is a convenience function equivalent to calling bnd.Get(KeyAPIBase).
func (bndl Bundle) APIBase() string {
	base := bndl.Get(ConfigKeyAPIBase)

	for len(base) > 0 && base[len(base)-1] == '/' {
		// do not end with a slash, please
		base = base[:len(base)-1]
	}
	if len(base) == 0 || base[0] != '/' {
		base = "/" + base
	}

	return strings.ToLower(base)
}

// UsesDBs returns the list of database names that the API is configured to
// connect to, in the order they were listed in config.
//
// This is a convenience function equivalent to calling
// bnd.GetSlice(KeyAPIUsesDBs).
func (bndl Bundle) UsesDBs() []string {
	return bndl.GetSlice(ConfigKeyAPIUsesDBs)
}

// Enabled returns whether the API was set to be enabled. Since this is required
// for an API to be initialized, this will always be true for an API receiving a
// Bundle in its Init method.
//
// This is a convenience function equivalent to calling
// bnd.GetBool(KeyAPIEnabled).
func (bndl Bundle) Enabled() bool {
	return bndl.GetBool(ConfigKeyAPIEnabled)
}

// Get retrieves the value of a string-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) Get(key string) string {
	var v string

	if !bndl.Has(key) {
		return v
	}

	return configGet[string](bndl.api, key)
}

// GetByteSlice retrieves the value of a []byte-typed API configuration key. If
// it doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) GetByteSlice(key string) []byte {
	var v []byte

	if !bndl.Has(key) {
		return v
	}

	return configGet[[]byte](bndl.api, key)
}

// GetSlice retrieves the value of a []string-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) GetSlice(key string) []string {
	var v []string

	if !bndl.Has(key) {
		return v
	}

	return configGet[[]string](bndl.api, key)
}

// GetIntSlice retrieves the value of a []int-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) GetIntSlice(key string) []int {
	var v []int

	if !bndl.Has(key) {
		return v
	}

	return configGet[[]int](bndl.api, key)
}

// GetBoolSlice retrieves the value of a []bool-typed API configuration key. If
// it doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) GetBoolSlice(key string) []bool {
	var v []bool

	if !bndl.Has(key) {
		return v
	}

	return configGet[[]bool](bndl.api, key)
}

// GetFloatSlice retrieves the value of a []float64-typed API configuration key.
// If it doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) GetFloatSlice(key string) []float64 {
	var v []float64

	if !bndl.Has(key) {
		return v
	}

	return configGet[[]float64](bndl.api, key)
}

// GetBool retrieves the value of a bool-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) GetBool(key string) bool {
	var v bool

	if !bndl.Has(key) {
		return v
	}

	return configGet[bool](bndl.api, key)
}

// GetFloat retrieves the value of a float64-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) GetFloat(key string) float64 {
	var v float64

	if !bndl.Has(key) {
		return v
	}

	return configGet[float64](bndl.api, key)
}

// GetTime retrieves the value of a time.Time-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) GetTime(key string) time.Time {
	var v time.Time

	if !bndl.Has(key) {
		return v
	}

	return configGet[time.Time](bndl.api, key)
}

// GetInt retrieves the value of an int-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) GetInt(key string) int {
	var v int

	if !bndl.Has(key) {
		return v
	}

	return configGet[int](bndl.api, key)
}

// GetInt8 retrieves the value of an int8-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) GetInt8(key string) int8 {
	var v int8

	if !bndl.Has(key) {
		return v
	}

	return configGet[int8](bndl.api, key)
}

// GetInt16 retrieves the value of an int16-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) GetInt16(key string) int16 {
	var v int16

	if !bndl.Has(key) {
		return v
	}

	return configGet[int16](bndl.api, key)
}

// GetInt32 retrieves the value of an int32-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) GetInt32(key string) int32 {
	var v int32

	if !bndl.Has(key) {
		return v
	}

	return configGet[int32](bndl.api, key)
}

// GetInt64 retrieves the value of an int64-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) GetInt64(key string) int64 {
	var v int64

	if !bndl.Has(key) {
		return v
	}

	return configGet[int64](bndl.api, key)
}

// GetUint retrieves the value of a uint-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) GetUint(key string) uint {
	var v uint

	if !bndl.Has(key) {
		return v
	}

	return configGet[uint](bndl.api, key)
}

// GetUint8 retrieves the value of a uint8-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) GetUint8(key string) uint8 {
	var v uint8

	if !bndl.Has(key) {
		return v
	}

	return configGet[uint8](bndl.api, key)
}

// GetUint16 retrieves the value of a uint16-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) GetUint16(key string) uint16 {
	var v uint16

	if !bndl.Has(key) {
		return v
	}

	return configGet[uint16](bndl.api, key)
}

// GetUint32 retrieves the value of a uint32-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) GetUint32(key string) uint32 {
	var v uint32

	if !bndl.Has(key) {
		return v
	}

	return configGet[uint32](bndl.api, key)
}

// GetUint64 retrieves the value of a uint64-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bndl Bundle) GetUint64(key string) uint64 {
	var v uint64

	if !bndl.Has(key) {
		return v
	}

	return configGet[uint64](bndl.api, key)
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
	StdLog
)

func (p LogProvider) String() string {
	switch p {
	case NoLog:
		return "none"
	case Jellog:
		return "jellog"
	case StdLog:
		return "std"
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
	case StdLog.String():
		return StdLog, nil
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
