package jelly

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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

type EndpointFunc func(req *http.Request) Result

func UnPathParam(s string) string {
	for name, pat := range paramTypePats {
		s = strings.ReplaceAll(s, ":"+pat+"}", ":"+name+"}")
	}
	return s
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
func RedirectNoTrailingSlash(sp ServiceProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		redirPath := strings.TrimRight(req.URL.Path, "/")
		r := sp.Redirection(redirPath)
		r.WriteResponse(w)
		r.Log(req)
	}
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
		return NewError("malformed JSON in request", err, ErrBodyUnmarshal)
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
		return val, NewError("", ErrBadArgument)
	}
	return val, nil
}

// Override is a per-endpoint optional overriding of a global configuration in
// order to, for instance, use a specific Authenticator. Multiple Overrides can
// be given in a single Endpoint; if given, they will be evaluated in order with
// later ones taking precedence over others in cases of conflict and later ones
// being added to lists at lower prority in cases of lists.
type Override struct {
	Authenticators []string
}

func CombineOverrides(overs []Override) Override {
	newOver := Override{}
	for i := range overs {
		newOver.Authenticators = append(newOver.Authenticators, overs[i].Authenticators...)
	}
	return newOver
}

// ServiceProvider is passed to an API's Routes method and is used to access
// jelly middleware and standardized endpoint function wrapping to produce an
// http.HandlerFunc from an EndpointFunc.
type ServiceProvider interface {
	ResponseGenerator
	DontPanic() Middleware
	OptionalAuth(authenticators ...string) Middleware
	RequiredAuth(authenticators ...string) Middleware
	SelectAuthenticator(authenticators ...string) Authenticator
	Endpoint(ep EndpointFunc, overrides ...Override) http.HandlerFunc
	GetLoggedInUser(req *http.Request) (user AuthUser, loggedIn bool)
}
