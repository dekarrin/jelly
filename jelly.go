// Package jelly is a simple and quick framework dekarrin/jello uses for
// learning Go servers.
package jelly

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/dekarrin/jelly/config"
	"github.com/dekarrin/jelly/dao"
	"github.com/dekarrin/jelly/middle"
	"github.com/go-chi/chi/v5"
)

var autoAPIProviders = map[string]func() API{}

// API holds parameters for endpoints needed to run and a service layer that
// will perform most of the actual logic. To use API, create one and then
// assign the result of its HTTP* methods as handlers to a router or some other
// kind of server mux.
type API interface {

	// Init creates the API initially and does any setup other than routing its
	// endpoints. It takes in a complete config object and a map of dbs to
	// connected stores. Only those stores requested in the API's config in the
	// 'uses' key will be included here.
	//
	// After Init returns, the API is prepared to return its routes with Routes.
	Init(cfg config.APIConfig, g config.Globals, dbs map[string]dao.Store) error

	// Routes returns a router that leads to all accessible routes in the API.
	// Additionally, returns whether the API's router contains subpaths beyond
	// just setting methods on its relative root; this affects whether
	// path-terminal slashes are redirected in the base router the API
	// router is mounted in.
	//
	// Init must be called before Routes is called.
	Routes() (router chi.Router, subpaths bool)

	// Shutdown terminates any pending operations cleanly and releases any held
	// resources. It will be called after the server listener socket is shut
	// down. Implementors should examine the context's Done() channel to see if
	// they should halt during long-running operations, and do so if requested.
	Shutdown(ctx context.Context) error
}

// RegisterAuto marks an API as being in-use and gives functions to provide a
// new empty instance of the API and a new empty instance of its associated
// config. This will make the API automatically instantiating in calls
// subsequent calls to New().
//
// Calling this function for every API to be used is not required, but it is for
// those that are to be automatically configured when the server is created with
// New(). Note that if this function is not called for an API, you will need to
// call config.Register() in order to set the config type for the API, and Add()
// on server instances in order to actually add and configure the API.
func RegisterAuto(name string, provider func() API, confProvider func() config.APIConfig) error {
	normName := strings.ToLower(name)
	if _, ok := autoAPIProviders[normName]; ok {
		return fmt.Errorf("duplicate API name: %q is already registered", name)
	}
	if err := config.Register(name, confProvider); err != nil {
		return fmt.Errorf("register API config section: %w", err)
	}
	if provider == nil {
		return fmt.Errorf("API instance provider function cannot be nil")
	}

	autoAPIProviders[normName] = provider
	return nil
}

// RESTServer is an HTTP REST server that provides resources. The zero-value of
// a RESTServer should not be used directly; call New() to get one ready for
// use.
type RESTServer struct {
	mtx        *sync.Mutex
	closing    bool
	serving    bool
	http       *http.Server
	rootRouter chi.Router
	baseRouter chi.Router
	apis       map[string]API
	usedBases  map[string]string // used for tracking that APIs do not eat each other
	dbs        map[string]dao.Store
	cfg        config.Config // config that it was started with.
}

// New creates a new RESTServer ready to have new APIs added to it. All
// configured DBs are connected to before this function returns, and the config
// is retained for future operations. Any registered auto-APIs are automatically
// added via Add as per the configuration; this includes both built-in and
// user-supplied APIs.
func New(cfg *config.Config) (RESTServer, error) {
	// check config
	if cfg == nil {
		cfg = &config.Config{}
	} else {
		var copy *config.Config
		*copy = *cfg
		cfg = copy
	}
	*cfg = cfg.FillDefaults()
	if err := cfg.Validate(); err != nil {
		return RESTServer{}, fmt.Errorf("config: %w", err)
	}

	// connect DBs
	dbs := map[string]dao.Store{}
	for name, db := range cfg.DBs {
		db, err := cfg.DBConnector.Connect(db)
		if err != nil {
			return RESTServer{}, fmt.Errorf("connect DB %q: %w", name, err)
		}
		dbs[strings.ToLower(name)] = db
	}

	// Create root router
	root := chi.NewRouter()
	root.Use(middle.DontPanic())

	// make API router
	baseRouter := root
	if cfg.Globals.URIBase != "/" {
		baseRouter = chi.NewRouter()
		root.Mount(cfg.Globals.URIBase, baseRouter)
	}

	rs := RESTServer{
		apis:       map[string]API{},
		rootRouter: root,
		baseRouter: baseRouter,
		http:       &http.Server{Handler: root},
		mtx:        &sync.Mutex{},
		usedBases:  map[string]string{},
		dbs:        dbs,
		cfg:        *cfg,
	}

	// check on pre-rolled APIs
	for name, prov := range autoAPIProviders {
		if prConf, ok := cfg.APIs[name]; ok && config.Get[bool](prConf, config.KeyAPIEnabled) {
			preRolled := prov()
			if err := rs.Add(name, preRolled); err != nil {
				return RESTServer{}, fmt.Errorf("auto-add enabled API %s: create API: %w", name, err)
			}
		}
	}

	return rs, nil
}

// Add adds the given API and initializes it with the configuration section that
// matches its name. The name is case-insensitive and will be normalized to
// lowercase. It is an error to use the same normalized name in two calls to Add
// on the same RESTServer.
//
// Returns an error if there is any issue initializing the API.
func (rs *RESTServer) Add(name string, api API) error {
	normName := strings.ToLower(name)

	if _, ok := rs.apis[name]; ok {
		return fmt.Errorf("API named %q has already been added", normName)
	}

	rs.apis[normName] = api
	return rs.initAPI(normName, rs.apis[normName])
}

func (rs *RESTServer) initAPI(name string, api API) error {
	apiConf, ok := rs.cfg.APIs[strings.ToLower(name)]
	if !ok {
		return fmt.Errorf("missing config for API %q", name)
	}

	// find the actual dbs it uses
	usedDBs := map[string]dao.Store{}
	usedDBNames := config.Get[[]string](apiConf, config.KeyAPIDBs)

	for _, dbName := range usedDBNames {
		connectedDB, ok := rs.dbs[strings.ToLower(dbName)]
		if !ok {
			return fmt.Errorf("API refers to missing DB %q", strings.ToLower(dbName))
		}
		usedDBs[strings.ToLower(dbName)] = connectedDB
	}

	base := config.Get[string](apiConf, config.KeyAPIBase)
	for base[len(base)-1] == '/' {
		// do not end with a slash, please
		base = base[:len(base)-1]
	}
	if base[0] != '/' {
		base = "/" + base
	}
	// routing must be unique on case-insensitive basis (unless it's root, in
	// which case we make zero assumptions)
	if base != "/" {
		checkBase := strings.ToLower(base)
		if curUser, ok := rs.usedBases[checkBase]; ok {
			return fmt.Errorf("API %q and %q specify effectively identical API route bases of %q", name, curUser, base)
		}
		rs.usedBases[checkBase] = name
	}
	apiConf.Set(config.KeyAPIBase, base)

	if err := api.Init(apiConf, rs.cfg.Globals, usedDBs); err != nil {
		return fmt.Errorf("init API %q: Init(): %w", name, err)
	}

	r, subpaths := api.Routes()

	rs.baseRouter.Mount(base, r)
	if !subpaths {
		rs.baseRouter.HandleFunc(base+"/", RedirectNoTrailingSlash)
	}

	return nil
}

// ServeForever begins listening on the server's configured address and port for
// HTTP REST client requests.
//
// This function will block until the server is stopped. If it returns as a
// result of rs.Close() being called elsewhere, it will return
// http.ErrServerClosed.
func (rs *RESTServer) ServeForever() error {
	rs.mtx.Lock()
	if rs.serving {
		rs.mtx.Unlock()
		return fmt.Errorf("server is already running")
	}
	rs.serving = true
	rs.mtx.Unlock()

	defer func() {
		rs.mtx.Lock()
		rs.closing = false
		rs.serving = false
		rs.mtx.Unlock()
	}()

	rs.http.Addr = fmt.Sprintf("%s:%d", rs.cfg.Globals.Address, rs.cfg.Globals.Port)

	return rs.http.ListenAndServe()
}

// Shutdown shuts down the server gracefully, first closing the HTTP server to
// new connections and then shutting down each individual API the server was
// created with. This will cause ServeForever to return in any Go thread that is
// blocking on it. If the passed-in context is canceled while shutting down, it
// will halt graceful shutdown of the HTTP server and the APIs.
//
// Returns a non-nil error if the server is not currently running due to a call
// to ServeForever or Serve.
//
// Once Shutdown returns, the RESTServer should not be used again.
func (rs *RESTServer) Shutdown(ctx context.Context) error {
	rs.mtx.Lock()
	if rs.closing {
		rs.mtx.Unlock()
		return fmt.Errorf("close already in-progress in another goroutine")
	}
	if !rs.serving {
		rs.mtx.Unlock()
		return fmt.Errorf("server is not running")
	}
	defer rs.mtx.Unlock()
	rs.closing = true

	var fullError error

	if rs.http.Addr != "" {
		err := rs.http.Shutdown(ctx)
		if err != nil {
			fullError = fmt.Errorf("stop HTTP server: %w", err)
		}
		rs.http = &http.Server{Handler: rs.rootRouter}
		if err != nil && err == ctx.Err() {
			// if its due to the context expiring or timing out, we should
			// immediately exit without waiting for clean shutdown of the APIs.
			return fullError
		}
	}

	// call life-cycle shutdown on each API
	for name, api := range rs.apis {
		select {
		case <-ctx.Done():
			apiErr := ctx.Err()
			if fullError != nil {
				fullError = fmt.Errorf("%s\nadditionally: %w", fullError, apiErr)
			} else {
				fullError = apiErr
			}

			// for context end, immediately close
			return fullError
		default:
			if err := api.Shutdown(ctx); err != nil {
				apiErr := fmt.Errorf("shutdown API %q: %w", name, err)
				if fullError != nil {
					fullError = fmt.Errorf("%s\nadditionally: %w", fullError, apiErr)
				} else {
					fullError = apiErr
				}
			}
		}
	}

	return fullError
}
