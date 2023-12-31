// Package sqlite provides an interface into SQLite compatible with the rest of
// jelly packages.
package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/dekarrin/jelly/dao"
	"modernc.org/sqlite"
)

// AuthUserStore is a SQLite database that is compatible with built-in jelly
// user authentication mechanisms. It implements jeldao.AuthUserStore and it can
// be easily integrated into custom structs by embedding it.
//
// Its zero-value should not be used; call NewAuthUserStore to get an
// AuthUserStore ready for use.
type AuthUserStore struct {
	db         *sql.DB
	dbFilename string

	users *AuthUsersDB
}

func NewAuthUserStore(storageDir string) (*AuthUserStore, error) {
	st := &AuthUserStore{
		dbFilename: "data.db",
	}

	fileName := filepath.Join(storageDir, st.dbFilename)

	var err error
	st.db, err = sql.Open("sqlite", fileName)
	if err != nil {
		return nil, WrapDBError(err)
	}

	st.users = &AuthUsersDB{DB: st.db}
	st.users.init()

	return st, nil
}

func (aus *AuthUserStore) AuthUsers() dao.AuthUserRepo {
	return aus.users
}

func (aus *AuthUserStore) Close() error {
	mainDBErr := aus.db.Close()

	var err error
	if mainDBErr != nil {
		if err != nil {
			err = fmt.Errorf("%s\nadditionally: %s: %w", err.Error(), aus.dbFilename, mainDBErr)
		} else {
			err = fmt.Errorf("%s: %w", aus.dbFilename, err)
		}
	}
	return err
}

// WrapDBError wraps an error from the SQLite engine into an error useable by
// the rest of the jelly framework. It should be called on any error returned
// from SQLite before a repo passes the error back to a caller.
func WrapDBError(err error) error {
	sqliteErr := &sqlite.Error{}
	if errors.As(err, &sqliteErr) {
		if sqliteErr.Code() == 19 {
			return dao.ErrConstraintViolation
		}
		return fmt.Errorf("%s", sqlite.ErrorCodeString[sqliteErr.Code()])
	} else if errors.Is(err, sql.ErrNoRows) {
		return dao.ErrNotFound
	}
	return err
}
