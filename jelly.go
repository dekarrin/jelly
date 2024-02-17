// Package jelly is a simple and quick framework dekarrin 'jello' uses for
// learning Go servers.
//
// "Gelatin-based web servers".
package jelly

import (
	"context"
	"strings"

	"github.com/dekarrin/jelly/config"
	"github.com/dekarrin/jelly/middle"
	"github.com/dekarrin/jelly/types"
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
	Authenticators() map[string]middle.Authenticator

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
	Config() config.APIConfig
}

type RESTServer interface {
	Config() config.Config
	RoutesIndex() string
	Add(name string, api API) error
	ServeForever() error
	Shutdown(ctx context.Context) error
}

type Bundle struct {
	config.Bundle
	logger types.Logger
	dbs    map[string]types.Store
}

func NewBundle(apiConf config.Bundle, log types.Logger, dbs map[string]types.Store) Bundle {
	return Bundle{
		Bundle: apiConf,
		logger: log,
		dbs:    dbs,
	}
}

func (bndl Bundle) Logger() types.Logger {
	return bndl.logger
}

// DB gets the connection to the Nth DB listed in the API's uses. Panics if the API
// config does not have at least n+1 entries.
func (bndl Bundle) DB(n int) types.Store {
	dbName := bndl.UsesDBs()[n]
	return bndl.DBNamed(dbName)
}

// NamedDB gets the exact DB with the given name. This will only return the DB if it
// was configured as one of the used DBs for the API.
func (bndl Bundle) DBNamed(name string) types.Store {
	return bndl.dbs[strings.ToLower(name)]
}
