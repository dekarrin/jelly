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
	"github.com/go-chi/chi/v5"
)

// RESTServer is an HTTP REST server that provides resources. The zero-value of
// a RESTServer should not be used directly; call New() to get one ready for
// use.
type RESTServer struct {
	auth   jelapi.API
	router chi.Router
	api    jelapi.API
}

// New creates a new RESTServer with auth endpoints pre-configured.
func New(cfg *config.Config) (RESTServer, error) {
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

	// Init API with config. TODO: add method to config to allow retrieval of
	// custom blocks.
	var authAPI jelauth.LoginAPI
	authAPI.Init(dbs, *cfg)

	// TODO: init the main one too lol.

	router := newRouter(tqAPI)

	return RESTServer{
		auth:   authAPI,
		api:    tqAPI,
		router: router,
	}, nil
}
