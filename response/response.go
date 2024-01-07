// Pacakge response provides standardized responses for APIs in the jelly server
// framework.
package response

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

// OK returns an endpointResult containing an HTTP-200 along with a more
// detailed message (if desired; if none is provided it defaults to a generic
// one) that is not displayed to the user.
func OK(respObj interface{}, internalMsg ...interface{}) Result {
	internalMsgFmt := "OK"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	return Response(http.StatusOK, respObj, internalMsgFmt, msgArgs...)
}

// NoContent returns an endpointResult containing an HTTP-204 along
// with a more detailed message (if desired; if none is provided it defaults to
// a generic one) that is not displayed to the user.
func NoContent(internalMsg ...interface{}) Result {
	internalMsgFmt := "no content"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	return Response(http.StatusNoContent, nil, internalMsgFmt, msgArgs...)
}

// Created returns an endpointResult containing an HTTP-201 along
// with a more detailed message (if desired; if none is provided it defaults to
// a generic one) that is not displayed to the user.
func Created(respObj interface{}, internalMsg ...interface{}) Result {
	internalMsgFmt := "created"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	return Response(http.StatusCreated, respObj, internalMsgFmt, msgArgs...)
}

// Conflict returns an endpointResult containing an HTTP-409 along
// with a more detailed message (if desired; if none is provided it defaults to
// a generic one) that is not displayed to the user.
func Conflict(userMsg string, internalMsg ...interface{}) Result {
	internalMsgFmt := "conflict"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	return Err(http.StatusConflict, userMsg, internalMsgFmt, msgArgs...)
}

// BadRequest returns an endpointResult containing an HTTP-400 along
// with a more detailed message (if desired; if none is provided it defaults to
// a generic one) that is not displayed to the user.
func BadRequest(userMsg string, internalMsg ...interface{}) Result {
	internalMsgFmt := "bad request"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	return Err(http.StatusBadRequest, userMsg, internalMsgFmt, msgArgs...)
}

// MethodNotAllowed returns an endpointResult containing an HTTP-405 along
// with a more detailed message (if desired; if none is provided it defaults to
// a generic one) that is not displayed to the user.
func MethodNotAllowed(req *http.Request, internalMsg ...interface{}) Result {
	internalMsgFmt := "method not allowed"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	userMsg := fmt.Sprintf("Method %s is not allowed for %s", req.Method, req.URL.Path)

	return Err(http.StatusMethodNotAllowed, userMsg, internalMsgFmt, msgArgs...)
}

// NotFound returns an endpointResult containing an HTTP-404 response along
// with a more detailed message (if desired; if none is provided it defaults to
// a generic one) that is not displayed to the user.
func NotFound(internalMsg ...interface{}) Result {
	internalMsgFmt := "not found"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	return Err(http.StatusNotFound, "The requested resource was not found", internalMsgFmt, msgArgs...)
}

// Forbidden returns an endpointResult containing an HTTP-403 response.
// internalMsg is a detailed error message  (if desired; if none is provided it
// defaults to
// a generic one) that is not displayed to the user.
func Forbidden(internalMsg ...interface{}) Result {
	internalMsgFmt := "forbidden"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	return Err(http.StatusForbidden, "You don't have permission to do that", internalMsgFmt, msgArgs...)
}

// Unauthorized returns an endpointResult containing an HTTP-401 response
// along with the proper WWW-Authenticate header. internalMsg is a detailed
// error message  (if desired; if none is provided it defaults to
// a generic one) that is not displayed to the user.
func Unauthorized(userMsg string, internalMsg ...interface{}) Result {
	internalMsgFmt := "unauthorized"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	if userMsg == "" {
		userMsg = "You are not authorized to do that"
	}

	return Err(http.StatusUnauthorized, userMsg, internalMsgFmt, msgArgs...).
		WithHeader("WWW-Authenticate", `Basic realm="TunaQuest server", charset="utf-8"`)
}

// InternalServerError returns an endpointResult containing an HTTP-500
// response along with a more detailed message that is not displayed to the
// user. If internalMsg is provided the first argument must be a string that is
// the format string and any subsequent args are passed to Sprintf with the
// first as the format string.
func InternalServerError(internalMsg ...interface{}) Result {
	internalMsgFmt := "internal server error"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	return Err(http.StatusInternalServerError, "An internal server error occurred", internalMsgFmt, msgArgs...)
}

// if status is http.StatusNoContent, respObj will not be read and may be nil.
// Otherwise, respObj MUST NOT be nil. If additional values are provided they
// are given to internalMsg as a format string.
func Response(status int, respObj interface{}, internalMsg string, v ...interface{}) Result {
	msg := fmt.Sprintf(internalMsg, v...)
	return Result{
		IsJSON:      true,
		IsErr:       false,
		Status:      status,
		InternalMsg: msg,
		resp:        respObj,
	}
}

// If additional values are provided they are given to internalMsg as a format
// string.
func Err(status int, userMsg, internalMsg string, v ...interface{}) Result {
	msg := fmt.Sprintf(internalMsg, v...)
	return Result{
		IsJSON:      true,
		IsErr:       true,
		Status:      status,
		InternalMsg: msg,
		resp: ErrorResponse{
			Error:  userMsg,
			Status: status,
		},
	}
}

func Redirection(uri string) Result {
	msg := fmt.Sprintf("redirect -> %s", uri)
	return Result{
		Status:      http.StatusPermanentRedirect,
		InternalMsg: msg,
		redir:       uri,
	}
}

// TextErr is like jsonErr but it avoids JSON encoding of any kind and writes
// the output as plain text. If additional values are provided they are given to
// internalMsg as a format string.
func TextErr(status int, userMsg, internalMsg string, v ...interface{}) Result {
	msg := fmt.Sprintf(internalMsg, v...)
	return Result{
		IsJSON:      false,
		IsErr:       true,
		Status:      status,
		InternalMsg: msg,
		resp:        userMsg,
	}
}

type Result struct {
	Status      int
	IsErr       bool
	IsJSON      bool
	InternalMsg string

	resp  interface{}
	redir string // only used for redirects
	hdrs  [][2]string

	// set by calling PrepareMarshaledResponse.
	respJSONBytes []byte
}

func (r Result) WithHeader(name, val string) Result {
	erCopy := Result{
		IsErr:       r.IsErr,
		IsJSON:      r.IsJSON,
		Status:      r.Status,
		InternalMsg: r.InternalMsg,
		resp:        r.resp,
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
// and will returna  non-nil error.
func (r *Result) PrepareMarshaledResponse() error {
	if r.respJSONBytes != nil {
		return nil
	}

	if r.IsJSON && r.Status != http.StatusNoContent && r.redir == "" {
		var err error
		r.respJSONBytes, err = json.Marshal(r.resp)
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
		if r.redir == "" {
			respBytes = r.respJSONBytes
		}
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Status != http.StatusNoContent && r.redir == "" {
			respBytes = []byte(fmt.Sprintf("%v", r.resp))
		}
	}

	// if there is a redir, handle that now
	if r.redir != "" {
		w.Header().Set("Location", r.redir)
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
