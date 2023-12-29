// Package jelapi has API interfaces for compatibility with the rest of the
// jelly framework.
package jelapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dekarrin/jelly/config"
	"github.com/dekarrin/jelly/jeldao"
	"github.com/dekarrin/jelly/jelerr"
	"github.com/dekarrin/jelly/jelresult"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

type EndpointFunc func(req *http.Request) jelresult.Result

// API holds parameters for endpoints needed to run and a service layer that
// will perform most of the actual logic. To use API, create one and then
// assign the result of its HTTP* methods as handlers to a router or some other
// kind of server mux.
type API interface {

	// Init creates the API initially and does any setup other than routing its
	// endpoints. It takes in a complete config object and a map of dbs to
	// connected stores.
	//
	// After Init returns, the API is prepared to set up its routes with
	// RouteEndpoints.
	Init(dbs map[string]jeldao.Store, cfg config.Config) error
}

// v must be a pointer to a type. Will return error such that
// errors.Is(err, ErrMalformedBody) returns true if it is problem decoding the
// JSON itself.
func ParseJSONRequest(req *http.Request, v interface{}) error {
	contentType := req.Header.Get("Content-Type")

	if strings.ToLower(contentType) != "application/json" {
		return fmt.Errorf("request content-type is not application/json")
	}

	bodyData, err := io.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("could not read request body: %w", err)
	}
	defer func() {
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewBuffer(bodyData))
	}()

	err = json.Unmarshal(bodyData, v)
	if err != nil {
		return jelerr.New("malformed JSON in request", err, jelerr.ErrBodyUnmarshal)
	}

	return nil
}

// RequireIDParam gets the ID of the main entity being referenced in the URI and
// returns it. It panics if the key is not there or is not parsable.
func RequireIDParam(r *http.Request) uuid.UUID {
	id, err := GetURLParam(r, "id", uuid.Parse)
	if err != nil {
		panic(err.Error())
	}
	return id
}

func GetURLParam[E any](r *http.Request, key string, parse func(string) (E, error)) (val E, err error) {
	valStr := chi.URLParam(r, key)
	if valStr == "" {
		// either it does not exist or it is nil; treat both as the same and
		// return an error
		return val, fmt.Errorf("parameter does not exist")
	}

	val, err = parse(valStr)
	if err != nil {
		return val, jelerr.New("", jelerr.ErrBadArgument)
	}
	return val, nil
}

func HttpEndpoint(unauthDelay time.Duration, ep EndpointFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		r := ep(req)

		if r.Status == http.StatusUnauthorized || r.Status == http.StatusForbidden || r.Status == http.StatusInternalServerError {
			// if it's one of these statusus, either the user is improperly
			// logging in or tried to access a forbidden resource, both of which
			// should force the wait time before responding.
			time.Sleep(unauthDelay)
		}

		r.WriteResponse(w)
		r.Log(req)
	}
}
