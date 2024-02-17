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

var (
	DatabaseNone     = config.DatabaseNone
	DatabaseOWDB     = config.DatabaseOWDB
	DatabaseInMemory = config.DatabaseInMemory
	DatabaseSQLite   = config.DatabaseSQLite

	ConfigKeyAPIBase    = config.KeyAPIBase
	ConfigKeyAPIName    = config.KeyAPIName
	ConfigKeyAPIEnabled = config.KeyAPIEnabled
	ConfigKeyAPIUsesDBs = config.KeyAPIUsesDBs
)

type (
	DBType         = config.DBType
	APIConfig      = config.APIConfig
	CommonConfig   = config.Common
	DatabaseConfig = config.Database
)

func TypedSlice[E any](key string, value interface{}) ([]E, error) {
	return config.TypedSlice[E](key, value)
}
