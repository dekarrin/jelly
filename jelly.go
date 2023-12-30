// Package jelly is a simple and quick framework dekarrin/jello uses for
// learning Go servers.
package jelly

import (
	"fmt"
	"strings"

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
	auth   jelapi.API
	router chi.Router
	api    jelapi.API
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
		auth:   authAPI,
		api:    tqAPI,
		router: root,
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
			return nil, fmt.Errorf("API %q: Routes() returned API route base with doubled slash")
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
