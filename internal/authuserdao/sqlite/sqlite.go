// Package sqlite provides a pre-rolled SQLite database backend for an
// AuthUserStore.
package sqlite

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/dekarrin/jelly"
)

// AuthUserStore is a SQLite database that is compatible with built-in jelly
// user authentication mechanisms. It implements jeldb.AuthUserStore and it can
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
		return nil, jelly.WrapDBError(err)
	}

	st.users = &AuthUsersDB{DB: st.db}
	st.users.init()

	return st, nil
}

func (aus *AuthUserStore) AuthUsers() jelly.AuthUserRepo {
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
