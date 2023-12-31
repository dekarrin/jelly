package auth

import (
	"net/http"

	"github.com/dekarrin/jelly/dao"
	"github.com/dekarrin/jelly/token"
)

type JWTAuthProvider struct {
	db     dao.AuthUserRepo
	secret []byte
}

func (ap JWTAuthProvider) Authenticate(req *http.Request) (dao.User, bool, error) {
	tok, err := token.Get(req)
	if err != nil {
		// might not actually be a problem, let the auth engine decide if so but
		// there is no user to retrieve here
		//
		// TODO: when/if logging ever added, do that instead of just losing the
		// error
		return dao.User{}, false, nil
	}

	// validate the token
	lookupUser, err := token.Validate(req.Context(), tok, ap.secret, ap.db)
	if err != nil {
		return dao.User{}, false, err
	}

	return lookupUser, true, nil
}
