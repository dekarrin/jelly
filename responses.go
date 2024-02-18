package jelly

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

type ErrorResponse struct {
	Error  string `json:"error"`
	Status int    `json:"status"`
}

type Result struct {
	Status      int
	IsErr       bool
	IsJSON      bool
	InternalMsg string

	Resp  interface{}
	Redir string // only used for redirects

	hdrs [][2]string

	// set by calling PrepareMarshaledResponse.
	respJSONBytes []byte
}

func (r Result) WithHeader(name, val string) Result {
	erCopy := Result{
		IsErr:       r.IsErr,
		IsJSON:      r.IsJSON,
		Status:      r.Status,
		InternalMsg: r.InternalMsg,
		Resp:        r.Resp,
		hdrs:        r.hdrs,
	}

	erCopy.hdrs = append(erCopy.hdrs, [2]string{name, val})
	return erCopy
}

// PrepareMarshaledResponse sets the respJSONBytes to the marshaled version of
// the response if required. If required, and there is a problem marshaling, an
// error is returned. If not required, nil error is always returned.
//
// If PrepareMarshaledResponse has been successfully called with a non-nil
// returned error at least once for r, calling this method again has no effect
// and will return a  non-nil error.
func (r *Result) PrepareMarshaledResponse() error {
	if r.respJSONBytes != nil {
		return nil
	}

	if r.IsJSON && r.Status != http.StatusNoContent && r.Redir == "" {
		var err error
		r.respJSONBytes, err = json.Marshal(r.Resp)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r Result) WriteResponse(w http.ResponseWriter) {
	// if this hasn't been properly created, panic
	if r.Status == 0 {
		panic("result not populated")
	}

	err := r.PrepareMarshaledResponse()
	if err != nil {
		panic(fmt.Sprintf("could not marshal response: %s", err.Error()))
	}

	var respBytes []byte

	if r.IsJSON {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Redir == "" {
			respBytes = r.respJSONBytes
		}
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Status != http.StatusNoContent && r.Redir == "" {
			respBytes = []byte(fmt.Sprintf("%v", r.Resp))
		}
	}

	// if there is a redir, handle that now
	if r.Redir != "" {
		w.Header().Set("Location", r.Redir)
	}

	for i := range r.hdrs {
		w.Header().Set(r.hdrs[i][0], r.hdrs[i][1])
	}

	w.WriteHeader(r.Status)

	if r.Status != http.StatusNoContent {
		w.Write(respBytes)
	}
}

// TODO: detangle this and make it use the actual log provider.
func (r Result) Log(req *http.Request) {
	if r.IsErr {
		LogHTTPResponse("ERROR", req, r.Status, r.InternalMsg)
	} else {
		LogHTTPResponse("INFO", req, r.Status, r.InternalMsg)
	}
}

func LogHTTPResponse(level string, req *http.Request, respStatus int, msg string) {
	if len(level) > 5 {
		level = level[0:5]
	}

	for len(level) < 5 {
		level += " "
	}

	// we don't really care about the ephemeral port from the client end
	remoteAddrParts := strings.SplitN(req.RemoteAddr, ":", 2)
	remoteIP := remoteAddrParts[0]

	log.Printf("%s %s %s %s: HTTP-%d %s", level, remoteIP, req.Method, req.URL.Path, respStatus, msg)
}

type ResponseGenerator interface {
	OK(respObj interface{}, internalMsg ...interface{}) Result
	NoContent(internalMsg ...interface{}) Result
	Created(respObj interface{}, internalMsg ...interface{}) Result
	Conflict(userMsg string, internalMsg ...interface{}) Result
	BadRequest(userMsg string, internalMsg ...interface{}) Result
	MethodNotAllowed(req *http.Request, internalMsg ...interface{}) Result
	NotFound(internalMsg ...interface{}) Result
	Forbidden(internalMsg ...interface{}) Result
	Unauthorized(userMsg string, internalMsg ...interface{}) Result
	InternalServerError(internalMsg ...interface{}) Result
	Redirection(uri string) Result
	Response(status int, respObj interface{}, internalMsg string, v ...interface{}) Result
	Err(status int, userMsg, internalMsg string, v ...interface{}) Result
	TextErr(status int, userMsg, internalMsg string, v ...interface{}) Result
}
