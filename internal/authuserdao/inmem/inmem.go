package inmem

import (
	"fmt"

	"github.com/dekarrin/jelly/types"
)

// AuthUserStore is an in-memory database that is compatible with built-in jelly
// user authentication mechanisms. It implements jeldao.AuthUserStore and it can
// be easily integrated into custom structs by embedding it.
//
// Its zero-value should not be used; call NewAuthUserStore to get an
// AuthUserStore ready for use.
type AuthUserStore struct {
	users *AuthUserRepo
}

func NewAuthUserStore() *AuthUserStore {
	st := &AuthUserStore{
		users: NewAuthUserRepository(),
	}
	return st
}

func (aus *AuthUserStore) AuthUsers() types.AuthUserRepo {
	return aus.users
}

func (aus *AuthUserStore) Close() error {
	var err error
	nextErr := aus.users.Close()
	if nextErr != err {
		if err != nil {
			err = fmt.Errorf("%s\nadditionally, %w", err, nextErr)
		} else {
			err = nextErr
		}
	}

	return err
}
