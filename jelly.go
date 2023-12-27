// Package jelly is a simple and quick framework dekarrin/jello uses for
// learning Go servers.
package jelly

import "github.com/go-chi/chi/v5"

// RESTServer is an HTTP REST server that provides resources. The zero-value of
// a RESTServer should not be used directly; call New() to get one ready for
// use.
type TunaQuestRESTServer struct {
	router chi.Router
	api    api.API
}
