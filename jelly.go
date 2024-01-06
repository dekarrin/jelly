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

// todo: this should probs be called component providers and functionality of
// RegisterAuto be merged with the Use function.
var componentProviders = map[string]func() API{}

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
	// The API should not expect that any other API has yet been initialized,
	// during a call to Init, and should not attempt to use auth middleware that
	// relies on other APIs (such as jellyauth's jwt provider). Defer actual
	// usage to another function, such as Routes.
	Init(cb config.Bundle, dbs map[string]dao.Store) error

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
	Authenticators() map[string]middle.Authenticator

	// Routes returns a router that leads to all accessible routes in the API.
	// Additionally, returns whether the API's router contains subpaths beyond
	// just setting methods on its relative root; this affects whether
	// path-terminal slashes are redirected in the base router the API
	// router is mounted in.
	//
	// Init is guaranteed to have been called for all APIs in the server before
	// Routes is called, and it is safe to refer to middleware services that
	// rely on other APIs within.
	Routes() (router chi.Router, subpaths bool)

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
	Config() config.APIConfig
}

// UseComponent enables the given component and its section in config. Required
// to be called at least once for every pre-rolled component in use (such as
// jelly/auth) prior to loading config that contains its section. Calling
// UseComponent twice with a component with the same name will cause a panic.
func UseComponent(c Component) {
	normName := strings.ToLower(c.Name())
	if _, ok := componentProviders[normName]; ok {
		panic(fmt.Sprintf("duplicate component: %q is already in-use", c.Name()))
	}

	if err := config.Register(normName, c.Config); err != nil {
		panic(fmt.Sprintf("register component config section: %v", err))
	}

	componentProviders[normName] = c.API
}

// RESTServer is an HTTP REST server that provides resources. The zero-value of
// a RESTServer should not be used directly; call New() to get one ready for
// use.
type RESTServer struct {
	mtx         *sync.Mutex
	rtr         chi.Router
	closing     bool
	serving     bool
	http        *http.Server
	apis        map[string]API
	apiBases    map[string]string
	basesToAPIs map[string]string // used for tracking that APIs do not eat each other
	dbs         map[string]dao.Store
	cfg         config.Config // config that it was started with.
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
		copy := new(config.Config)
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

	rs := RESTServer{
		apis:        map[string]API{},
		apiBases:    map[string]string{},
		mtx:         &sync.Mutex{},
		basesToAPIs: map[string]string{},
		dbs:         dbs,
		cfg:         *cfg,
	}

	// check on pre-rolled components
	for name, prov := range componentProviders {
		if _, ok := cfg.APIs[name]; ok {
			preRolled := prov()
			if err := rs.Add(name, preRolled); err != nil {
				return RESTServer{}, fmt.Errorf("auto-add enabled API %s: create API: %w", name, err)
			}
		}
	}

	return rs, nil
}

// RoutesIndex returns a human-readable formatted string that lists all routes
// and methods currently available in the server.
func (rs *RESTServer) RoutesIndex() string {
	var sb strings.Builder
	r := rs.routeAllAPIs()
	chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		sb.WriteString(fmt.Sprintf("* %s %s\n", strings.ToUpper(method), route))
		return nil
	})
	return strings.TrimSpace(sb.String())
}

// routeAllAPIs is called just before serving. it gets all enabled routes and
// mounts them in the base router.
func (rs *RESTServer) routeAllAPIs() chi.Router {
	rs.mtx.Lock()
	defer rs.mtx.Unlock()

	if rs.rtr != nil {
		return rs.rtr
	}

	// Create root router
	root := chi.NewRouter()
	root.Use(middle.DontPanic())

	// make server base router
	r := root
	if rs.cfg.Globals.URIBase != "/" {
		r = chi.NewRouter()
		root.Mount(rs.cfg.Globals.URIBase, r)
	}

	for name, api := range rs.apis {
		apiConf, ok := rs.cfg.APIs[name]
		if !ok {
			apiConf = (&config.Common{Name: name}).FillDefaults()
		}
		if config.Get[bool](apiConf, config.KeyAPIEnabled) {
			base := rs.apiBases[name]
			apiRouter, subpaths := api.Routes()

			if apiRouter != nil {
				r.Mount(base, apiRouter)
				if !subpaths {
					r.HandleFunc(base+"/", RedirectNoTrailingSlash)
				}
			}
		}
	}

	rs.rtr = root

	return root
}

// Add adds the given API to the server. If it is enabled in its config, it will
// be initialized with the configuration section that matches its name. The name
// is case-insensitive and will be normalized to lowercase. It is an error to
// use the same normalized name in two calls to Add on the same RESTServer.
//
// Returns an error if there is any issue initializing the API.
func (rs *RESTServer) Add(name string, api API) error {
	normName := strings.ToLower(name)

	if _, ok := rs.apis[name]; ok {
		return fmt.Errorf("API named %q has already been added", normName)
	}

	apiConf, ok := rs.cfg.APIs[normName]
	if !ok {
		return fmt.Errorf("missing config for API %q", name)
	}

	// aquire mtx to modify the stored router
	rs.mtx.Lock()
	defer func() {
		rs.mtx.Unlock()
	}()
	// make shore to reset the router so we don't re-use it
	rs.rtr = nil

	rs.apis[normName] = api
	if config.Get[bool](apiConf, config.KeyAPIEnabled) {
		base, err := rs.initAPI(normName, api)
		if err != nil {
			return err
		}
		rs.apiBases[normName] = base

		auths := api.Authenticators()
		for aName, a := range auths {
			fullName := name + "." + aName

			// TODO: probs shouldn't have a struct type call a global-affecting func.
			middle.RegisterAuthenticator(fullName, a)
		}
	}

	return nil
}

func (rs *RESTServer) initAPI(name string, api API) (string, error) {
	apiConf, ok := rs.cfg.APIs[strings.ToLower(name)]
	if !ok {
		return "", fmt.Errorf("missing config for API %q", name)
	}

	// find the actual dbs it uses
	usedDBs := map[string]dao.Store{}
	usedDBNames := config.Get[[]string](apiConf, config.KeyAPIUsesDBs)

	for _, dbName := range usedDBNames {
		connectedDB, ok := rs.dbs[strings.ToLower(dbName)]
		if !ok {
			return "", fmt.Errorf("API refers to missing DB %q", strings.ToLower(dbName))
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
		if curUser, ok := rs.basesToAPIs[checkBase]; ok {
			return "", fmt.Errorf("API %q and %q specify effectively identical API route bases of %q", name, curUser, base)
		}
		rs.basesToAPIs[checkBase] = name
	}
	apiConf.Set(config.KeyAPIBase, base)

	// make config bundle for them
	if err := api.Init(config.NewBundle(apiConf, rs.cfg.Globals), usedDBs); err != nil {
		return "", fmt.Errorf("init API %q: Init(): %w", name, err)
	}

	return base, nil
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

	addr := fmt.Sprintf("%s:%d", rs.cfg.Globals.Address, rs.cfg.Globals.Port)
	rtr := rs.routeAllAPIs()
	rs.http = &http.Server{Addr: addr, Handler: rtr}

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

	if rs.http != nil {
		err := rs.http.Shutdown(ctx)
		if err != nil {
			fullError = fmt.Errorf("stop HTTP server: %w", err)
		}
		rs.http = nil
		if err != nil && err == ctx.Err() {
			// if its due to the context expiring or timing out, we should
			// immediately exit without waiting for clean shutdown of the APIs.
			return fullError
		}
	}

	// call life-cycle shutdown on each API
	for name, api := range rs.apis {
		apiConf, ok := rs.cfg.APIs[name]
		if !ok {
			apiConf = (&config.Common{Name: name}).FillDefaults()
		}
		if !config.Get[bool](apiConf, config.KeyAPIEnabled) {
			continue
		}

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
