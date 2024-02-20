package middle

import (
	"net/http"
	"testing"

	"github.com/dekarrin/jelly"
	"github.com/stretchr/testify/assert"
)

func reqWithContextKeys(keys map[string]interface{}) *http.Request {
	
}

func Test_GetLoggedInUser(t *testing.T) {

	testCases := []struct {
		name           string
		req            *http.Request
		expectUser     jelly.AuthUser
		expectLoggedIn bool
	}{
		{
			name: "no user present",
			req: &http.Request{}
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
