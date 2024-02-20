package auth

import (
	"net/http"
	"time"

	"github.com/dekarrin/jelly"
)

type jwtAuthProvider struct {
	db          jelly.AuthUserRepo
	secret      []byte
	unauthDelay time.Duration
	srv         loginService
}

func (ap jwtAuthProvider) Authenticate(req *http.Request) (jelly.AuthUser, bool, error) {
	tok, err := getToken(req)
	if err != nil {
		// might not actually be a problem, let the auth engine decide if so but
		// there is no user to retrieve here
		//
		// TODO: when/if logging ever added, do that instead of just losing the
		// error
		return jelly.AuthUser{}, false, nil
	}

	// validate the token
	lookupUser, err := validateToken(req.Context(), tok, ap.secret, ap.db)
	if err != nil {
		return jelly.AuthUser{}, false, err
	}

	return lookupUser, true, nil
}

func (ap jwtAuthProvider) UnauthDelay() time.Duration {
	return ap.unauthDelay
}

func (ap jwtAuthProvider) Service() jelly.UserLoginService {
	return ap.srv
}
