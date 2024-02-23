package middle

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dekarrin/jelly"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	mock_jelly "github.com/dekarrin/jelly/tools/mocks/jelly"
)

func reqWithContextValues(values map[ctxKey]interface{}) *http.Request {
	req := httptest.NewRequest("", "", nil)
	ctx := req.Context()

	for k, v := range values {
		ctx = context.WithValue(ctx, k, v)
	}

	return req.WithContext(ctx)
}

func ref[E any](v E) *E {
	return &v
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

func Test_authHandler(t *testing.T) {
	type aValues struct {
		user     jelly.AuthUser
		loggedIn bool
		err      error
	}

	testCases := []struct {
		name                    string
		authRequired            bool
		authenticateReturns     aValues
		unauthDelayReturns      *time.Duration
		respUnauthorizedReturns *jelly.Result
		expectHandoff           bool
		expectLoggedIn          any
		expectUser              any
	}{
		{
			name:         "logged in user - optional auth",
			authRequired: false,
			authenticateReturns: aValues{
				user:     jelly.AuthUser{Username: "arachnidsGrip"},
				loggedIn: true,
			},
			expectLoggedIn: true,
			expectUser:     jelly.AuthUser{Username: "arachnidsGrip"},
			expectHandoff:  true,
		},
		{
			name:         "logged in user - required auth",
			authRequired: true,
			authenticateReturns: aValues{
				user:     jelly.AuthUser{Username: "arachnidsGrip"},
				loggedIn: true,
			},
			expectLoggedIn: true,
			expectUser:     jelly.AuthUser{Username: "arachnidsGrip"},
			expectHandoff:  true,
		},
		{
			name:         "no logged in user - optional auth",
			authRequired: false,
			authenticateReturns: aValues{
				loggedIn: false,
			},
			expectLoggedIn: false,
			expectUser:     jelly.AuthUser{},
			expectHandoff:  true,
		},
		{
			name:         "no logged in user - required auth",
			authRequired: true,
			authenticateReturns: aValues{
				loggedIn: false,
			},
			unauthDelayReturns:      ref(time.Millisecond),
			respUnauthorizedReturns: &jelly.Result{IsErr: true, Status: http.StatusUnauthorized},
			expectHandoff:           false,
		},
		{
			name:         "authenticate returns an error - optional auth",
			authRequired: false,
			authenticateReturns: aValues{
				err: errors.New("an error occurred"),
			},
			expectHandoff:  true,
			expectLoggedIn: false,
			expectUser:     jelly.AuthUser{},
		},
		{
			name:         "authenticate returns an error - required auth",
			authRequired: true,
			authenticateReturns: aValues{
				err: errors.New("an error occurred"),
			},
			unauthDelayReturns:      ref(time.Millisecond),
			respUnauthorizedReturns: &jelly.Result{IsErr: true, Status: http.StatusUnauthorized},
			expectHandoff:           false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			mockCtrl := gomock.NewController(t)
			mockResponseGenerator := mock_jelly.NewMockResponseGenerator(mockCtrl)
			mockAuthenticator := mock_jelly.NewMockAuthenticator(mockCtrl)
			mockLogger := mock_jelly.NewMockLogger(mockCtrl)

			var reqOnHandoff *http.Request
			recorder := httptest.NewRecorder()

			ah := authHandler{
				provider: mockAuthenticator,
				resp:     mockResponseGenerator,
				required: tc.authRequired,
				next: mwFunc(func(w http.ResponseWriter, req *http.Request) {
					reqOnHandoff = req
				}),
			}

			inputReq := httptest.NewRequest("", "/", nil)

			//Authenticate(req *http.Request) (AuthUser, bool, error)
			avs := tc.authenticateReturns
			mockAuthenticator.EXPECT().Authenticate(inputReq).Return(avs.user, avs.loggedIn, avs.err)

			if tc.authRequired {
				if tc.respUnauthorizedReturns != nil {
					msg := "authorization is required"
					if avs.err != nil {
						msg = avs.err.Error()
					}
					mockAuthenticator.EXPECT().UnauthDelay().Return(*tc.unauthDelayReturns)
					mockResponseGenerator.EXPECT().Unauthorized(gomock.Any(), msg).Return(*tc.respUnauthorizedReturns)
					mockResponseGenerator.EXPECT().LogResponse(gomock.Any(), *tc.respUnauthorizedReturns).Return()
				}
			} else if tc.authenticateReturns.err != nil {
				mockResponseGenerator.EXPECT().Logger().Return(mockLogger)
				mockLogger.EXPECT().Warnf("optional auth returned error: %v", tc.authenticateReturns.err)
			}

			// execute
			ah.ServeHTTP(recorder, inputReq)

			// assert
			if tc.expectHandoff {
				actualCtx := reqOnHandoff.Context()
				user := actualCtx.Value(ctxKeyUser)
				loggedIn := actualCtx.Value(ctxKeyLoggedIn)

				assert.Equal(tc.expectUser, user)
				assert.Equal(tc.expectLoggedIn, loggedIn)
			} else {
				resp := recorder.Result()
				assert.Equal(http.StatusUnauthorized, resp.StatusCode)
				assert.Nil(reqOnHandoff)
			}

			mockCtrl.Finish()
		})
	}
}
