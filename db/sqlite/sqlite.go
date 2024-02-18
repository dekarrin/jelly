// Package sqlite provides an interface into SQLite compatible with the rest of
// jelly packages.
package sqlite

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/dekarrin/jelly"
	"modernc.org/sqlite"
)

// WrapDBError wraps an error from the SQLite engine into an error useable by
// the rest of the jelly framework. It should be called on any error returned
// from SQLite before a repo passes the error back to a caller.
func WrapDBError(err error) error {
	sqliteErr := &sqlite.Error{}
	if errors.As(err, &sqliteErr) {
		primaryCode := sqliteErr.Code() & 0xff
		if primaryCode == 19 {
			return fmt.Errorf("%w: %s", jelly.DBErrConstraintViolation, err.Error())
		}
		if primaryCode == 1 {
			// this is a generic error and thus the string is not descriptive,
			// so preserve the original error instead
			return err
		}
		return fmt.Errorf("%s", sqlite.ErrorCodeString[sqliteErr.Code()])
	} else if errors.Is(err, sql.ErrNoRows) {
		return jelly.DBErrNotFound
	}
	return err
}
