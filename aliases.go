package jelly

import (
	"net/http"

	"github.com/dekarrin/jelly/config"
	"github.com/dekarrin/jelly/db"
	"github.com/dekarrin/jelly/db/sqlite"
	"github.com/dekarrin/jelly/middle"
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

// middle aliases

type (
	Authenticator = middle.Authenticator
	User          = db.User
)

func GetLoggedInUser(req *http.Request) (user User, loggedIn bool) {
	return middle.GetLoggedInUser(req)
}
