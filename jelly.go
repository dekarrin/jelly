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
	"github.com/dekarrin/jelly/jelapi"
	"github.com/dekarrin/jelly/jelauth"
	"github.com/dekarrin/jelly/jeldao"
	"github.com/dekarrin/jelly/jelmid"
	"github.com/go-chi/chi/v5"
)

const (
	// TODO: move to config
	RootURIPrefix = "/api/v1"
)

// RESTServer is an HTTP REST server that provides resources. The zero-value of
// a RESTServer should not be used directly; call New() to get one ready for
// use.
type RESTServer struct {
	mtx     *sync.Mutex
	closing bool
	serving bool
	http    *http.Server
	router  chi.Router
	apis    map[string]jelapi.API
}

// New creates a new RESTServer. The built-in login/auth endpoints and API
// service are pre-configured unless disabled in config. Additionally, Init and
// Routes is called on all given APIs, and the result of Routes is used to route
// the master API router to it.
//
// This function takes ownership of apis and it should not be used after this
// function is called.
func New(cfg *config.Config, apis map[string]jelapi.API) (RESTServer, error) {
	// check config
	if cfg == nil {
		cfg = &config.Config{}
	}
	*cfg = cfg.FillDefaults()
	if err := cfg.Validate(); err != nil {
		return RESTServer{}, fmt.Errorf("config: %w", err)
	}

	// connect DBs
	dbs := map[string]jeldao.Store{}
	for name, db := range cfg.DBs {
		db, err := cfg.DBConnector.Connect(db)
		if err != nil {
			return RESTServer{}, fmt.Errorf("connect DB %q: %w", name, err)
		}
		dbs[strings.ToLower(name)] = db
	}

	// make API router
	apisRouter, err := newAPIsRouter(dbs, *cfg, apis)
	if err != nil {
		return RESTServer{}, fmt.Errorf("setup APIs: %w", err)
	}

	// Create root router
	root := chi.NewRouter()
	root.Use(jelmid.DontPanic())
	root.Mount(RootURIPrefix, apisRouter)

	// Init API with config.

	return RESTServer{
		apis:   apis,
		router: root,
		http:   &http.Server{Handler: root},
		mtx:    &sync.Mutex{},
	}, nil
}

func newAPIsRouter(dbs map[string]jeldao.Store, cfg config.Config, apis map[string]jelapi.API) (chi.Router, error) {
	// first toss a new LoginAPI in.
	var authAPI jelauth.LoginAPI
	if _, ok := apis["auth"]; ok { // TODO: call it authn or login or somefin.
		return nil, fmt.Errorf("a user-supplied API has name of built-in API 'auth'; not allowed")
	}
	apis["auth"] = &authAPI

	apisRouter := chi.NewRouter()

	// map base name to API name
	usedBases := map[string]string{}

	for name, api := range apis {
		// TODO: add method to config to allow retrieval of
		// custom blocks.
		if err := api.Init(dbs, cfg); err != nil {
			return nil, fmt.Errorf("init API %q: Init(): %w", name, err)
		}
		base, r, subpaths := api.Routes()
		if strings.ContainsAny(base, "{}") {
			return nil, fmt.Errorf("API %q: Routes() returned API route base with '{' or '}': %q", name, base)
		}
		if strings.Contains(base, "//") {
			return nil, fmt.Errorf("API %q: Routes() returned API route base with doubled slash", name)
		}
		if base == "" {
			return nil, fmt.Errorf("API %q: Routes() returned empty API route base", name)
		}
		if base == "/" {
			return nil, fmt.Errorf("API %q: Routes() returned \"/\" for API route base", name)
		}
		for base[len(base)-1] == '/' {
			// do not end with a slash, please
			base = base[:len(base)-1]
		}
		if base == "" {
			return nil, fmt.Errorf("API %q: Routes() returned API route base made of slashes", name)
		}
		if base[0] != '/' {
			base = "/" + base
		}

		// routing must be unique on case-insensitive basis
		if curUser, ok := usedBases[strings.ToLower(base)]; ok {
			return nil, fmt.Errorf("API %q and %q both request API route base of %q", name, curUser, base)
		}
		usedBases[strings.ToLower(base)] = name
		apisRouter.Mount(base, r)
		if !subpaths {
			apisRouter.HandleFunc(base+"/", jelapi.RedirectNoTrailingSlash)
		}
	}

	return apisRouter, nil
}

// ServeForever begins listening on the given address and port for HTTP REST
// client requests. If address is kept as "", it will default to "localhost". If
// port is less than 1, it will default to 8080.
//
// This function will block until the server is stopped. If it returns as a
// result of rs.Close() being called elsewhere, it will return
// http.ErrServerClosed.
//
// TODO: make this not accept addr and port. Use config instead.
func (rs *RESTServer) ServeForever(address string, port int) error {
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

	if address == "" {
		address = "localhost"
	}
	if port < 1 {
		port = 8080
	}

	rs.http.Addr = fmt.Sprintf("%s:%d", address, port)

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
		rs.http = &http.Server{Handler: rs.router}
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
