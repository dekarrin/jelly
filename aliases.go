package jelly

import (
	"github.com/dekarrin/jelly/dao"
	"github.com/dekarrin/jelly/dao/sqlite"
)

var (
	DBErrConstraintViolation = dao.ErrConstraintViolation
	DBErrDecodingFailure     = dao.ErrDecodingFailure
	DBErrNotFound            = dao.ErrNotFound
)

// TODO: put this on the DB type. Or make it at least just WrapDBError.
func WrapSqliteError(dbErr error) error {
	return sqlite.WrapDBError(dbErr)
}
