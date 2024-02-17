// Package token provides JWT functionality for use with the built-in user
// authentication methods in jelly.
package token

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dekarrin/jelly/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	Issuer = "jelly"
)

func Validate(ctx context.Context, tok string, secret []byte, userDB types.AuthUserRepo) (types.AuthUser, error) {
	var user types.AuthUser

	_, err := jwt.Parse(tok, func(t *jwt.Token) (interface{}, error) {
		// who is the user? we need this for further verification
		subj, err := t.Claims.GetSubject()
		if err != nil {
			return nil, fmt.Errorf("cannot get subject: %w", err)
		}

		id, err := uuid.Parse(subj)
		if err != nil {
			return nil, fmt.Errorf("cannot parse subject UUID: %w", err)
		}

		user, err = userDB.Get(ctx, id)
		if err != nil {
			if errors.Is(err, types.DBErrNotFound) {
				return nil, fmt.Errorf("subject does not exist")
			} else {
				return nil, fmt.Errorf("subject could not be validated")
			}
		}

		var signKey []byte
		signKey = append(signKey, secret...)
		signKey = append(signKey, []byte(user.Password)...)
		signKey = append(signKey, []byte(fmt.Sprintf("%d", user.LastLogout.Unix()))...)
		return signKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS512.Alg()}), jwt.WithIssuer(Issuer), jwt.WithLeeway(time.Minute))

	if err != nil {
		return types.AuthUser{}, err
	}

	return user, nil
}

// Get gets the token from the Authorization header as a bearer token.
func Get(req *http.Request) (string, error) {
	authHeader := strings.TrimSpace(req.Header.Get("Authorization"))

	if authHeader == "" {
		return "", fmt.Errorf("no authorization header present")
	}

	authParts := strings.SplitN(authHeader, " ", 2)
	if len(authParts) != 2 {
		return "", fmt.Errorf("authorization header not in Bearer format")
	}

	scheme := strings.TrimSpace(strings.ToLower(authParts[0]))
	token := strings.TrimSpace(authParts[1])

	if scheme != "bearer" {
		return "", fmt.Errorf("authorization header not in Bearer format")
	}

	return token, nil
}

func Generate(secret []byte, u types.AuthUser) (string, error) {
	claims := &jwt.MapClaims{
		"iss":        Issuer,
		"exp":        time.Now().Add(time.Hour).Unix(),
		"sub":        u.ID.String(),
		"authorized": true,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)

	var signKey []byte
	signKey = append(signKey, secret...)
	signKey = append(signKey, []byte(u.Password)...)
	signKey = append(signKey, []byte(fmt.Sprintf("%d", u.LastLogout.Unix()))...)

	tokStr, err := tok.SignedString(signKey)
	if err != nil {
		return "", err
	}
	return tokStr, nil
}
