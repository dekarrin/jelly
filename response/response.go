// Pacakge response provides standardized responses for APIs in the jelly server
// framework.
package response

import (
	"fmt"
	"net/http"

	"github.com/dekarrin/jelly/types"
)

// OK returns an endpointResult containing an HTTP-200 along with a more
// detailed message (if desired; if none is provided it defaults to a generic
// one) that is not displayed to the user.
func OK(respObj interface{}, internalMsg ...interface{}) types.Result {
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
func NoContent(internalMsg ...interface{}) types.Result {
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
func Created(respObj interface{}, internalMsg ...interface{}) types.Result {
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
func Conflict(userMsg string, internalMsg ...interface{}) types.Result {
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
func BadRequest(userMsg string, internalMsg ...interface{}) types.Result {
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
func MethodNotAllowed(req *http.Request, internalMsg ...interface{}) types.Result {
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
func NotFound(internalMsg ...interface{}) types.Result {
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
func Forbidden(internalMsg ...interface{}) types.Result {
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
func Unauthorized(userMsg string, internalMsg ...interface{}) types.Result {
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
func InternalServerError(internalMsg ...interface{}) types.Result {
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
func Response(status int, respObj interface{}, internalMsg string, v ...interface{}) types.Result {
	msg := fmt.Sprintf(internalMsg, v...)
	return types.Result{
		IsJSON:      true,
		IsErr:       false,
		Status:      status,
		InternalMsg: msg,
		Resp:        respObj,
	}
}

// If additional values are provided they are given to internalMsg as a format
// string.
func Err(status int, userMsg, internalMsg string, v ...interface{}) types.Result {
	msg := fmt.Sprintf(internalMsg, v...)
	return types.Result{
		IsJSON:      true,
		IsErr:       true,
		Status:      status,
		InternalMsg: msg,
		Resp: types.ErrorResponse{
			Error:  userMsg,
			Status: status,
		},
	}
}

func Redirection(uri string) types.Result {
	msg := fmt.Sprintf("redirect -> %s", uri)
	return types.Result{
		Status:      http.StatusPermanentRedirect,
		InternalMsg: msg,
		Redir:       uri,
	}
}

// TextErr is like jsonErr but it avoids JSON encoding of any kind and writes
// the output as plain text. If additional values are provided they are given to
// internalMsg as a format string.
func TextErr(status int, userMsg, internalMsg string, v ...interface{}) types.Result {
	msg := fmt.Sprintf(internalMsg, v...)
	return types.Result{
		IsJSON:      false,
		IsErr:       true,
		Status:      status,
		InternalMsg: msg,
		Resp:        userMsg,
	}
}
