package server

import (
	"fmt"
	"net/http"

	"github.com/dekarrin/jelly"
)

func (svc services) Logger() jelly.Logger {
	return svc.log
}

func (svc services) LogResponse(req *http.Request, r jelly.Result) {
	svc.log.LogResult(req, r)
}

// if status is http.StatusNoContent, respObj will not be read and may be nil.
// Otherwise, respObj MUST NOT be nil. If additional values are provided they
// are given to internalMsg as a format string.
func (svc services) Response(status int, respObj interface{}, internalMsg string, v ...interface{}) jelly.Result {
	msg := fmt.Sprintf(internalMsg, v...)
	return jelly.Result{
		IsJSON:      true,
		IsErr:       false,
		Status:      status,
		InternalMsg: msg,
		Resp:        respObj,
	}
}

// If additional values are provided they are given to internalMsg as a format
// string.
func (svc services) Err(status int, userMsg, internalMsg string, v ...interface{}) jelly.Result {
	msg := fmt.Sprintf(internalMsg, v...)
	return jelly.Result{
		IsJSON:      true,
		IsErr:       true,
		Status:      status,
		InternalMsg: msg,
		Resp: jelly.ErrorResponse{
			Error:  userMsg,
			Status: status,
		},
	}
}

func (svc services) Redirection(uri string) jelly.Result {
	msg := fmt.Sprintf("redirect -> %s", uri)
	return jelly.Result{
		Status:      http.StatusPermanentRedirect,
		InternalMsg: msg,
		Redir:       uri,
	}
}

// TextErr is like jsonErr but it avoids JSON encoding of any kind and writes
// the output as plain text. If additional values are provided they are given to
// internalMsg as a format string.
func (svc services) TextErr(status int, userMsg, internalMsg string, v ...interface{}) jelly.Result {
	msg := fmt.Sprintf(internalMsg, v...)
	return jelly.Result{
		IsJSON:      false,
		IsErr:       true,
		Status:      status,
		InternalMsg: msg,
		Resp:        userMsg,
	}
}

// OK returns an endpointResult containing an HTTP-200 along with a more
// detailed message (if desired; if none is provided it defaults to a generic
// one) that is not displayed to the user.
func (svc services) OK(respObj interface{}, internalMsg ...interface{}) jelly.Result {
	internalMsgFmt := "OK"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	return svc.Response(http.StatusOK, respObj, internalMsgFmt, msgArgs...)
}

// NoContent returns an endpointResult containing an HTTP-204 along
// with a more detailed message (if desired; if none is provided it defaults to
// a generic one) that is not displayed to the user.
func (svc services) NoContent(internalMsg ...interface{}) jelly.Result {
	internalMsgFmt := "no content"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	return svc.Response(http.StatusNoContent, nil, internalMsgFmt, msgArgs...)
}

// Created returns an endpointResult containing an HTTP-201 along
// with a more detailed message (if desired; if none is provided it defaults to
// a generic one) that is not displayed to the user.
func (svc services) Created(respObj interface{}, internalMsg ...interface{}) jelly.Result {
	internalMsgFmt := "created"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	return svc.Response(http.StatusCreated, respObj, internalMsgFmt, msgArgs...)
}

// Conflict returns an endpointResult containing an HTTP-409 along
// with a more detailed message (if desired; if none is provided it defaults to
// a generic one) that is not displayed to the user.
func (svc services) Conflict(userMsg string, internalMsg ...interface{}) jelly.Result {
	internalMsgFmt := "conflict"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	return svc.Err(http.StatusConflict, userMsg, internalMsgFmt, msgArgs...)
}

// BadRequest returns an endpointResult containing an HTTP-400 along
// with a more detailed message (if desired; if none is provided it defaults to
// a generic one) that is not displayed to the user.
func (svc services) BadRequest(userMsg string, internalMsg ...interface{}) jelly.Result {
	internalMsgFmt := "bad request"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	return svc.Err(http.StatusBadRequest, userMsg, internalMsgFmt, msgArgs...)
}

// MethodNotAllowed returns an endpointResult containing an HTTP-405 along
// with a more detailed message (if desired; if none is provided it defaults to
// a generic one) that is not displayed to the user.
func (svc services) MethodNotAllowed(req *http.Request, internalMsg ...interface{}) jelly.Result {
	internalMsgFmt := "method not allowed"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	userMsg := fmt.Sprintf("Method %s is not allowed for %s", req.Method, req.URL.Path)

	return svc.Err(http.StatusMethodNotAllowed, userMsg, internalMsgFmt, msgArgs...)
}

// NotFound returns an endpointResult containing an HTTP-404 response along
// with a more detailed message (if desired; if none is provided it defaults to
// a generic one) that is not displayed to the user.
func (svc services) NotFound(internalMsg ...interface{}) jelly.Result {
	internalMsgFmt := "not found"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	return svc.Err(http.StatusNotFound, "The requested resource was not found", internalMsgFmt, msgArgs...)
}

// Forbidden returns an endpointResult containing an HTTP-403 response.
// internalMsg is a detailed error message  (if desired; if none is provided it
// defaults to
// a generic one) that is not displayed to the user.
func (svc services) Forbidden(internalMsg ...interface{}) jelly.Result {
	internalMsgFmt := "forbidden"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	return svc.Err(http.StatusForbidden, "You don't have permission to do that", internalMsgFmt, msgArgs...)
}

// Unauthorized returns an endpointResult containing an HTTP-401 response
// along with the proper WWW-Authenticate header. internalMsg is a detailed
// error message  (if desired; if none is provided it defaults to
// a generic one) that is not displayed to the user.
func (svc services) Unauthorized(userMsg string, internalMsg ...interface{}) jelly.Result {
	internalMsgFmt := "unauthorized"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	if userMsg == "" {
		userMsg = "You are not authorized to do that"
	}

	return svc.Err(http.StatusUnauthorized, userMsg, internalMsgFmt, msgArgs...).
		WithHeader("WWW-Authenticate", `Basic realm="TunaQuest server", charset="utf-8"`)
}

// InternalServerError returns an endpointResult containing an HTTP-500
// response along with a more detailed message that is not displayed to the
// user. If internalMsg is provided the first argument must be a string that is
// the format string and any subsequent args are passed to Sprintf with the
// first as the format string.
func (svc services) InternalServerError(internalMsg ...interface{}) jelly.Result {
	internalMsgFmt := "internal server error"
	var msgArgs []interface{}
	if len(internalMsg) >= 1 {
		internalMsgFmt = internalMsg[0].(string)
		msgArgs = internalMsg[1:]
	}

	return svc.Err(http.StatusInternalServerError, "An internal server error occurred", internalMsgFmt, msgArgs...)
}
