package jelly

import (
	"github.com/dekarrin/jelly/db"
	"github.com/dekarrin/jelly/db/sqlite"
)

var (
	DBErrConstraintViolation = db.ErrConstraintViolation
	DBErrDecodingFailure     = db.ErrDecodingFailure
	DBErrNotFound            = db.ErrNotFound
)

// TODO: put this on the DB type. Or make it at least just WrapDBError.
func WrapSqliteError(dbErr error) error {
	return sqlite.WrapDBError(dbErr)
}
