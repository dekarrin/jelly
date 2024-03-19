package server

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/internal/logging"
	"github.com/go-chi/chi/v5"
)

// restServer is an HTTP REST server that provides resources. The zero-value of
// a restServer should not be used directly; call New() to get one ready for
// use.
type restServer struct {
	mtx         *sync.Mutex
	rtr         chi.Router
	closing     bool
	serving     bool
	http        *http.Server
	apis        map[string]jelly.API
	apiBases    map[string]string
	basesToAPIs map[string]string // used for tracking that APIs do not eat each other
	dbs         map[string]jelly.Store
	cfg         jelly.Config // config that it was started with.

	log jelly.Logger // used for logging. if logging disabled, this will be set to a no-op logger

	env *Environment // ptr back to the environment that this server was created in.
}

// NewServer creates a new RESTServer ready to have new APIs added to it. All
// configured DBs are connected to before this function returns, and the config
// is retained for future operations. Any registered auto-APIs are automatically
// added via Add as per the configuration; this includes both built-in and
// user-supplied APIs.
func (env *Environment) NewServer(cfg *jelly.Config) (jelly.RESTServer, error) {
	env.initDefaults()

	// check config
	if cfg == nil {
		cfg = &jelly.Config{}
	} else {
		copy := new(jelly.Config)
		*copy = *cfg
		cfg = copy
	}
	*cfg = cfg.FillDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	var logger jelly.Logger = logging.NoOpLogger{}
	// config is loaded, make the first thing we start be our logger
	if cfg.Log.Enabled {
		var err error

		logger, err = logging.New(cfg.Log.Provider, cfg.Log.File)
		if err != nil {
			return nil, fmt.Errorf("create logger: %w", err)
		}
	}

	// connect DBs
	dbs := map[string]jelly.Store{}
	for name, db := range cfg.DBs {
		db, err := env.connectors.Connect(db)
		if err != nil {
			return nil, fmt.Errorf("connect DB %q: %w", name, err)
		}
		dbs[strings.ToLower(name)] = db
	}

	rs := &restServer{
		apis:        map[string]jelly.API{},
		apiBases:    map[string]string{},
		mtx:         &sync.Mutex{},
		basesToAPIs: map[string]string{},
		dbs:         dbs,
		cfg:         *cfg,
		log:         logger,

		env: env,
	}

	// check on pre-rolled components, they need to be inited first.
	for _, name := range env.componentProvidersOrder {
		prov := env.componentProviders[name]
		if _, ok := cfg.APIs[name]; ok {
			preRolled := prov()
			if err := rs.Add(name, preRolled); err != nil {
				return nil, fmt.Errorf("component API %s: create API: %w", name, err)
			}
			logger.Debugf("Added pre-rolled component %q", name)
		}
	}

	// okay, after the pre-rolls are initialized and authenticators added, it
	// should be safe to set the main authenticator
	if cfg.Globals.MainAuthProvider != "" {
		env.SetMainAuthenticator(cfg.Globals.MainAuthProvider)
	}

	return rs, nil
}

// Config returns the conifguration that the server used during creation.
// Modifying the returned config will have no effect on the server.
func (rs restServer) Config() jelly.Config {
	return rs.cfg.FillDefaults()
}

// RoutesIndex returns a human-readable formatted string that lists all routes
// and methods currently available in the server.
func (rs *restServer) RoutesIndex() string {
	routeMethods := map[string][]string{}

	r := rs.routeAllAPIs()
	chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		meths, ok := routeMethods[route]
		if !ok {
			meths = []string{}
		}

		meths = append(meths, method)
		routeMethods[route] = meths

		return nil
	})

	// alphabetize the routes
	allRoutes := []string{}
	for name := range routeMethods {
		allRoutes = append(allRoutes, name)
	}
	sort.Strings(allRoutes)

	// write the sorted routes
	var sb strings.Builder
	for _, r := range allRoutes {
		sb.WriteString("* ")
		sb.WriteString(r)
		sb.WriteString(" - ")

		meths := routeMethods[r]
		sort.Strings(meths)
		for i, m := range meths {
			sb.WriteString(m)
			if i+1 < len(meths) {
				sb.WriteString(", ")
			}
		}
		sb.WriteRune('\n')
	}

	return jelly.UnPathParam(strings.TrimSpace(sb.String()))
}

// routeAllAPIs is called just before serving. it gets all enabled routes and
// mounts them in the base router.
func (rs *restServer) routeAllAPIs() chi.Router {
	rs.mtx.Lock()
	defer rs.mtx.Unlock()

	if rs.rtr != nil {
		return rs.rtr
	}

	env := rs.env
	if env == nil {
		env = &Environment{}
		env.initDefaults()
	}

	sp := endpointCreator{mid: env.middleProv, log: rs.log}

	// Create root router
	root := chi.NewRouter()
	root.Use(env.middleProv.DontPanic(sp))

	// make server base router
	r := root
	if rs.cfg.Globals.URIBase != "/" {
		r = chi.NewRouter()
		root.Mount(rs.cfg.Globals.URIBase, r)
	}

	for name, api := range rs.apis {
		apiConf := rs.getAPIConfigBundle(name)
		if apiConf.Enabled() {
			base := rs.apiBases[name]
			apiRouter := api.Routes(sp)

			if apiRouter != nil {
				r.Mount(base, apiRouter)
				if base != "/" {

					// check if there are subpaths
					hasSubpaths := false

					chi.Walk(apiRouter, func(_, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
						trimmedRoute := strings.TrimLeft(route, "/")
						if trimmedRoute != "" {
							hasSubpaths = true
						}
						return nil
					})

					if !hasSubpaths {
						r.HandleFunc(base+"/", jelly.RedirectNoTrailingSlash(sp))
					}
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
func (rs *restServer) Add(name string, api jelly.API) error {
	name = strings.ToLower(name)

	if _, ok := rs.apis[name]; ok {
		return fmt.Errorf("API named %q has already been added", name)
	}

	apiConf := rs.getAPIConfigBundle(name)

	// aquire mtx to modify the stored router
	rs.mtx.Lock()
	defer func() {
		rs.mtx.Unlock()
	}()
	// make shore to reset the router so we don't re-use it
	rs.rtr = nil

	env := rs.env
	if env == nil {
		env = &Environment{}
	}

	rs.apis[name] = api
	if apiConf.Enabled() {
		rs.log.Debugf("Added API %q; initializing...", name)
		base, err := rs.initAPI(name, api)
		if err != nil {
			return err
		}
		rs.apiBases[name] = base

		auths := api.Authenticators()
		for aName, a := range auths {
			fullName := name + "." + aName
			env.RegisterAuthenticator(fullName, a)
		}
	} else {
		rs.log.Debugf("Added API %q; skipping initialization due to enabled=false", name)
	}

	return nil
}

// will return default "common bundle" with only the name set if the named API
// is not in the configured APIs. dbs will not be set.
func (rs *restServer) getAPIConfigBundle(name string) jelly.Bundle {
	conf, ok := rs.cfg.APIs[strings.ToLower(name)]
	if !ok {
		return jelly.NewBundle((&jelly.CommonConfig{Name: name}).FillDefaults(), rs.cfg.Globals, rs.log, nil)
	}
	return jelly.NewBundle(conf, rs.cfg.Globals, rs.log, nil)
}

func (rs *restServer) initAPI(name string, api jelly.API) (string, error) {
	// using strings.ToLower is getting old. probs should just do that once on
	// input and then assume all controlled code is good to go
	if _, ok := rs.cfg.APIs[strings.ToLower(name)]; !ok {
		rs.log.Warnf("config section %q is not present", name)
	}
	apiConf := rs.getAPIConfigBundle(name)

	// find the actual dbs it uses
	usedDBs := map[string]jelly.Store{}
	usedDBNames := apiConf.UsesDBs()

	for _, dbName := range usedDBNames {
		connectedDB, ok := rs.dbs[strings.ToLower(dbName)]
		if !ok {
			return "", fmt.Errorf("API refers to missing DB %q", strings.ToLower(dbName))
		}
		usedDBs[strings.ToLower(dbName)] = connectedDB
	}

	base := apiConf.APIBase()
	// routing must be unique on case-insensitive basis (unless it's root, in
	// which case we make zero assumptions)
	if base != "/" {
		if curUser, ok := rs.basesToAPIs[base]; ok {
			return "", fmt.Errorf("API %q and %q specify effectively identical API route bases of %q", name, curUser, base)
		}
		rs.basesToAPIs[base] = name
	}

	// TODO: after jellog is patched, add in use of api's name to logger via use of sublogger

	initBundle := apiConf.WithDBs(usedDBs)

	if err := api.Init(initBundle); err != nil {
		return "", fmt.Errorf("init API %q: Init(): %w", name, err)
	}
	rs.log.Debugf("Successfully initialized API %q", name)

	return base, nil
}

func (rs *restServer) checkCreatedViaNew() {
	if rs.mtx == nil {
		panic("server mutex is in invalid state; was this RESTServer created with New()?")
	}
}

// ServeForever begins listening on the server's configured address and port for
// HTTP REST client requests.
//
// This function will block until the server is stopped. If it returns as a
// result of rs.Close() being called elsewhere, it will return
// http.ErrServerClosed.
func (rs *restServer) ServeForever() (err error) {
	rs.checkCreatedViaNew()
	rs.mtx.Lock()
	if rs.serving {
		rs.mtx.Unlock()
		return fmt.Errorf("server is already running")
	}
	rs.serving = true
	rs.mtx.Unlock()

	addr := fmt.Sprintf("%s:%d", rs.cfg.Globals.Address, rs.cfg.Globals.Port)

	// calling into user code, do a panic check
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occurred while running server: %v", r)
		}
	}()
	rtr := rs.routeAllAPIs()
	rs.http = &http.Server{Addr: addr, Handler: rtr}

	defer func() {
		rs.mtx.Lock()
		rs.closing = false
		rs.serving = false
		rs.mtx.Unlock()
	}()

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
func (rs *restServer) Shutdown(ctx context.Context) error {
	rs.checkCreatedViaNew()
	rs.mtx.Lock()
	defer rs.mtx.Unlock()
	if rs.closing {
		return fmt.Errorf("close already in-progress in another goroutine")
	}
	if !rs.serving {
		return fmt.Errorf("server is not running")
	}
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
		apiConf := rs.getAPIConfigBundle(name)
		if !apiConf.Enabled() {
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
