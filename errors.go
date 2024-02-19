package jelly

import (
	"database/sql"
	"errors"
	"fmt"

	"modernc.org/sqlite"
)

var (
	ErrBadCredentials = errors.New("the supplied username/password combination is incorrect")
	ErrPermissions    = errors.New("you don't have permission to do that")
	ErrNotFound       = errors.New("the requested entity could not be found")
	ErrAlreadyExists  = errors.New("resource with same identifying information already exists")
	ErrDB             = errors.New("an error occured with the DB")
	ErrBadArgument    = errors.New("one or more of the arguments is invalid")
	ErrBodyUnmarshal  = errors.New("malformed data in request")

	ErrDBConstraintViolation = errors.New("a uniqueness constraint was violated")
	ErrDBNotFound            = errors.New("the requested resource was not found")
	ErrDBDecodingFailure     = errors.New("field could not be decoded from DB storage format to model format")
)

// Error is a typed error returned by certain functions in the TunaScript server
// as their error value. It contains both a message explaining what happened as
// well as one or more error values it considers to be its causes. Error is
// compatible with the use of errors.Is() - calling errors.Is on some Error
// value err along with any value of error it holds as one of its causes will
// return true. This allows for easy examination and failure condition checking
// without needing to resort to manual typecasting.
//
// If Error has at least one cause defined, the result of calling Error.Error()
// will be its primary message with the result of calling Error() on its first
// cause appended to it.
//
// Error should not be used directly; call New to create one.
type Error struct {
	msg   string
	cause []error
}

// Error returns the message defined for the Error. If a message was defined for
// it when created, that message is returned, concatenated with the result of
// calling Error() on the its first cause if one is defined. If no message or an
// empty message was defined for it when created, but there is at least one
// cause defined for it, the result of calling Error() on the first cause is
// returned. If no message is defined and no causes are defined, returns the
// empty string.
func (e Error) Error() string {
	if e.msg == "" && e.cause != nil {
		return e.cause[0].Error()
	}

	if e.cause != nil {
		return e.msg + ": " + e.cause[0].Error()
	}

	return e.msg
}

// Unwrap returns the causes of Error. The return value will be nil if no causes
// were defined for it.
//
// This function is for interaction with the errors API. It will only be used in
// Go version 1.20 and later; 1.19 will default to use of Error.Is when calling
// errors.Is on the Error.
func (e Error) Unwrap() []error {
	if len(e.cause) > 0 {
		return e.cause
	}
	return nil
}

// Is returns whether Error either Is itself the given target error, or one of
// its causes is.
//
// This function is for interaction with the errors API.
func (e Error) Is(target error) bool {
	// is the target error itself?
	if errTarget, ok := target.(Error); ok {
		if e.msg == errTarget.msg {
			if len(e.cause) == len(errTarget.cause) {
				allCausesEqual := true
				for i := range e.cause {
					if e.cause[i] != errTarget.cause[i] {
						allCausesEqual = false
						break
					}
				}
				if allCausesEqual {
					return true
				}
			}
		}
	}

	// otherwise, check if any cause equals target
	// TODO: from go docs re errors: "An Is method should only shallowly compare
	// err and the target and not call Unwrap on either.". Okay. But the thing
	// is, Go 1.19 does not support wrapping multiple errors so we have opted to
	// do things this way. In future, let's use build tags and separate files to
	// split based on go version and ensure that we have unit tests for each.
	for i := range e.cause {

		// we must check if any are of type Error, because if they are, we need
		// to run the normal Is.
		if sErr, ok := e.cause[i].(Error); ok {
			if sErr.Is(target) {
				return true
			}
		} else if e.cause[i] == target {
			return true
		}
	}
	return false
}

// WrapSQLiteError wraps an error from the SQLite engine into an error useable by
// the rest of the jelly framework. It should be called on any error returned
// from SQLite before a repo passes the error back to a caller.
//
// TODO: merge with WrapDBError
func WrapSQLiteError(err error) error {
	sqliteErr := &sqlite.Error{}
	if errors.As(err, &sqliteErr) {
		primaryCode := sqliteErr.Code() & 0xff
		if primaryCode == 19 {
			return fmt.Errorf("%w: %s", ErrDBConstraintViolation, err.Error())
		}
		if primaryCode == 1 {
			// this is a generic error and thus the string is not descriptive,
			// so preserve the original error instead
			return err
		}
		return fmt.Errorf("%s", sqlite.ErrorCodeString[sqliteErr.Code()])
	} else if errors.Is(err, sql.ErrNoRows) {
		return ErrDBNotFound
	}
	return err
}

// WrapDBErr creates a new Error that wraps the given error as a cause and
// automatically adds ErrDB as another cause. A user-set message may be provided
// if desired with msg, but it may be left as "".
func WrapDBErr(msg string, err error) Error {
	return Error{
		cause: []error{err, ErrDB},
	}
}

// NewError creates a new Error with the given message, along with any errors it
// should wrap as its causes. Providing cause errors is not required, but will
// cause it to return true when it is checked against that error via a call to
// errors.Is.
func NewError(msg string, causes ...error) Error {
	err := Error{msg: msg}
	if len(causes) > 0 {
		err.cause = make([]error, len(causes))
		copy(err.cause, causes)
	}
	return err
}
