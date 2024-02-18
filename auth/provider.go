package auth

import (
	"net/http"
	"time"

	"github.com/dekarrin/jelly/types"
)

type JWTAuthProvider struct {
	db          types.AuthUserRepo
	secret      []byte
	unauthDelay time.Duration
	srv         LoginService
}

func (ap JWTAuthProvider) Authenticate(req *http.Request) (types.AuthUser, bool, error) {
	tok, err := getToken(req)
	if err != nil {
		// might not actually be a problem, let the auth engine decide if so but
		// there is no user to retrieve here
		//
		// TODO: when/if logging ever added, do that instead of just losing the
		// error
		return types.AuthUser{}, false, nil
	}

	// validate the token
	lookupUser, err := validateToken(req.Context(), tok, ap.secret, ap.db)
	if err != nil {
		return types.AuthUser{}, false, err
	}

	return lookupUser, true, nil
}

func (ap JWTAuthProvider) UnauthDelay() time.Duration {
	return ap.unauthDelay
}

func (ap JWTAuthProvider) Service() types.UserLoginService {
	return ap.srv
}
