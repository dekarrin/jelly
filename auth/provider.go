package auth

import (
	"net/http"
	"time"

	"github.com/dekarrin/jelly/db"
	"github.com/dekarrin/jelly/middle"
	"github.com/dekarrin/jelly/token"
)

type JWTAuthProvider struct {
	db          db.AuthUserRepo
	secret      []byte
	unauthDelay time.Duration
	srv         LoginService
}

func (ap JWTAuthProvider) Authenticate(req *http.Request) (db.User, bool, error) {
	tok, err := token.Get(req)
	if err != nil {
		// might not actually be a problem, let the auth engine decide if so but
		// there is no user to retrieve here
		//
		// TODO: when/if logging ever added, do that instead of just losing the
		// error
		return db.User{}, false, nil
	}

	// validate the token
	lookupUser, err := token.Validate(req.Context(), tok, ap.secret, ap.db)
	if err != nil {
		return db.User{}, false, err
	}

	return lookupUser, true, nil
}

func (ap JWTAuthProvider) UnauthDelay() time.Duration {
	return ap.unauthDelay
}

func (ap JWTAuthProvider) Service() middle.UserLoginService {
	return ap.srv
}
