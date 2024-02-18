package jelly

import (
	"github.com/dekarrin/jelly/config"
	"github.com/dekarrin/jelly/db/sqlite"
)

// TODO: put this on the DB type. Or make it at least just WrapDBError.
func WrapSqliteError(dbErr error) error {
	return sqlite.WrapDBError(dbErr)
}

// config aliases

func TypedSlice[E any](key string, value interface{}) ([]E, error) {
	return config.TypedSlice[E](key, value)
}
