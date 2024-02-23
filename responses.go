package jelly

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type ErrorResponse struct {
	Error  string `json:"error"`
	Status int    `json:"status"`
}

// should not be directly init'd probs because log will not be set
type Result struct {
	Status      int
	IsErr       bool
	IsJSON      bool
	InternalMsg string

	Resp  interface{}
	Redir string // only used for redirects

	hdrs [][2]string

	log Logger

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
		log:         r.log,
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
	LogResponse(req *http.Request, r Result)
}
