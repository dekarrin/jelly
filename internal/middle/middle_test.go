package middle

import (
	"context"
	"net/http"
	"testing"

	"github.com/dekarrin/jelly"
	"github.com/stretchr/testify/assert"
)

func reqWithContextValues(values map[ctxKey]interface{}) *http.Request {
	req := &http.Request{}
	ctx := req.Context()

	for k, v := range values {
		ctx = context.WithValue(ctx, k, v)
	}

	return req.WithContext(ctx)
}

func Test_GetLoggedInUser(t *testing.T) {
	testCases := []struct {
		name           string
		req            *http.Request
		expectUser     jelly.AuthUser
		expectLoggedIn bool
	}{
		{
			name:           "bare request",
			req:            &http.Request{},
			expectUser:     jelly.AuthUser{},
			expectLoggedIn: false,
		},
		{
			name: "user is not logged in and user value not present",
			req: reqWithContextValues(map[ctxKey]interface{}{
				ctxKeyLoggedIn: false,
			}),
			expectUser:     jelly.AuthUser{},
			expectLoggedIn: false,
		},
		{
			name: "user is not logged in and user value is present",
			req: reqWithContextValues(map[ctxKey]interface{}{
				ctxKeyLoggedIn: false,
				ctxKeyUser:     jelly.AuthUser{},
			}),
			expectUser:     jelly.AuthUser{},
			expectLoggedIn: false,
		},
		{
			name: "user is logged in",
			req: reqWithContextValues(map[ctxKey]interface{}{
				ctxKeyUser:     jelly.AuthUser{Username: "ghostlyTrickster"},
				ctxKeyLoggedIn: true,
			}),
			expectUser:     jelly.AuthUser{Username: "ghostlyTrickster"},
			expectLoggedIn: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			actualUser, actualLoggedIn := GetLoggedInUser(tc.req)

			assert.Equal(tc.expectUser, actualUser)
			assert.Equal(tc.expectLoggedIn, actualLoggedIn)
		})
	}
}
