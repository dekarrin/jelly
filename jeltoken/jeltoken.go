// Package jeltoken provides JWT functionality for use with the built-in user
// authentication methods in jelly.
package jeltoken

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dekarrin/jelly/jeldao"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func Validate(ctx context.Context, tok string, secret []byte, db jeldao.AuthUserRepo) (jeldao.User, error) {
	var user jeldao.User

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

		user, err = db.Get(ctx, id)
		if err != nil {
			if err == jeldao.ErrNotFound {
				return nil, fmt.Errorf("subject does not exist")
			} else {
				return nil, fmt.Errorf("subject could not be validated")
			}
		}

		var signKey []byte
		signKey = append(signKey, secret...)
		signKey = append(signKey, []byte(user.Password)...)
		signKey = append(signKey, []byte(fmt.Sprintf("%d", user.LastLogoutTime.Unix()))...)
		return signKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS512.Alg()}), jwt.WithIssuer("tqs"), jwt.WithLeeway(time.Minute))

	if err != nil {
		return jeldao.User{}, err
	}

	return user, nil
}

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

func Generate(secret []byte, u jeldao.User) (string, error) {
	claims := &jwt.MapClaims{
		"iss":        "tqs",
		"exp":        time.Now().Add(time.Hour).Unix(),
		"sub":        u.ID.String(),
		"authorized": true,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)

	var signKey []byte
	signKey = append(signKey, secret...)
	signKey = append(signKey, []byte(u.Password)...)
	signKey = append(signKey, []byte(fmt.Sprintf("%d", u.LastLogoutTime.Unix()))...)

	tokStr, err := tok.SignedString(signKey)
	if err != nil {
		return "", err
	}
	return tokStr, nil
}
