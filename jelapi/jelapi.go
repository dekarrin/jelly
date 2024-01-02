// Package jelapi has API interfaces for compatibility with the rest of the
// jelly framework.
//
// TODO: the way this is shaping up, this package could probably just be merged
// with jelly.
package jelapi

import (
	"bytes"
	"context"
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
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var (
	paramTypePats = map[string]string{
		"uuid":     `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`,
		"email":    `\S+@\S+`,
		"num":      `\d+`,
		"alpha":    `[A-Za-z]+`,
		"alphanum": `[A-Za-z0-9]+`,
	}
)

type EndpointFunc func(req *http.Request) jelresult.Result

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
	Init(cfg config.APIConfig, g config.Globals, dbs map[string]jeldao.Store) error

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

// PathParam translates strings of the form "name:type" to a URI path parameter
// string of the form "{name:regex}" compatible with the routers used in the
// jelly framework. Only request URIs whose path parameters match their
// respective regexes (if any) will match that route.
//
// Note that this only does basic matching for path routing. API endpoint logic
// will still need to decode the received string. Do not rely on, for example,
// the "email" type preventing malicious or invalid email; it only checks the
// string.
//
// Currently, PathParam supports the following parameter type names:
//
//   - "uuid" - UUID strings.
//   - "email" - Two strings separated by an @ sign.
//   - "num" - One or more digits 0-9.
//   - "alpha" - One or more Latin letters A-Z or a-z.
//   - "alphanum" - One or more Latin letters A-Z, a-z, or digits 0-9.
//
// If a different regex is needed for a path parameter, give it manually in the
// path using "{name:regex}" syntax instead of using PathParam; this is simply to use
// the above listed shortcuts.
//
// If only name is given in the string (with no colon), then the string
// "{" + name + "}" is returned.
func PathParam(nameType string) string {
	var name string
	var pat string

	parts := strings.SplitN(nameType, ":", 2)
	name = parts[0]
	if len(parts) == 2 {
		// we have a type, if it's a name in the paramTypePats map use that else
		// treat it as a normal pattern
		pat = parts[1]

		if translatedPat, ok := paramTypePats[parts[1]]; ok {
			pat = translatedPat
		}
	}

	if pat == "" {
		return "{" + name + "}"
	}
	return "{" + name + ":" + pat + "}"
}

// RedirectNoTrailingSlash is an http.HandlerFunc that redirects to the same URL as the
// request but with no trailing slash.
func RedirectNoTrailingSlash(w http.ResponseWriter, req *http.Request) {
	redirPath := strings.TrimRight(req.URL.Path, "/")
	r := jelresult.Redirection(redirPath)
	r.WriteResponse(w)
	r.Log(req)
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
