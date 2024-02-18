package jelly

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/dekarrin/jelly/types"
	"modernc.org/sqlite"
)

// WrapSQLiteError wraps an error from the SQLite engine into an error useable by
// the rest of the jelly framework. It should be called on any error returned
// from SQLite before a repo passes the error back to a caller.
func WrapSQLiteError(err error) error {
	sqliteErr := &sqlite.Error{}
	if errors.As(err, &sqliteErr) {
		primaryCode := sqliteErr.Code() & 0xff
		if primaryCode == 19 {
			return fmt.Errorf("%w: %s", types.DBErrConstraintViolation, err.Error())
		}
		if primaryCode == 1 {
			// this is a generic error and thus the string is not descriptive,
			// so preserve the original error instead
			return err
		}
		return fmt.Errorf("%s", sqlite.ErrorCodeString[sqliteErr.Code()])
	} else if errors.Is(err, sql.ErrNoRows) {
		return types.DBErrNotFound
	}
	return err
}

// TypedSlice takes a value that is passed to Set that is expected to be a slice
// of the given type and performs the required conversions. If a non-nil error
// is returned it will contain the key name automatically in its error string.
func TypedSlice[E any](key string, value interface{}) ([]E, error) {
	var typed E
	var typedValues []E

	if valueStr, ok := value.([]E); ok {
		typedValues = valueStr
		return typedValues, nil
	} else if valueSlice, ok := value.([]interface{}); ok {
		var ok bool
		for i := range valueSlice {
			if typed, ok = valueSlice[i].(E); ok {
				typedValues = append(typedValues, typed)
			} else {
				return nil, fmt.Errorf("%s[%d]: %q is not a valid string", key, i, valueSlice[i])
			}
		}
		return typedValues, nil
	} else {
		return nil, fmt.Errorf("key '%s' requires a %T but got a %T", key, typedValues, value)
	}
}
