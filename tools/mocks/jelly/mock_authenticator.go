// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/dekarrin/jelly (interfaces: Authenticator)
//
// Generated by this command:
//
//	mockgen -destination tools/mocks/jelly/mock_authenticator.go github.com/dekarrin/jelly Authenticator
//

// Package mock_jelly is a generated GoMock package.
package mock_jelly

import (
	http "net/http"
	reflect "reflect"
	time "time"

	jelly "github.com/dekarrin/jelly"
	gomock "go.uber.org/mock/gomock"
)

// MockAuthenticator is a mock of Authenticator interface.
type MockAuthenticator struct {
	ctrl     *gomock.Controller
	recorder *MockAuthenticatorMockRecorder
}

// MockAuthenticatorMockRecorder is the mock recorder for MockAuthenticator.
type MockAuthenticatorMockRecorder struct {
	mock *MockAuthenticator
}

// NewMockAuthenticator creates a new mock instance.
func NewMockAuthenticator(ctrl *gomock.Controller) *MockAuthenticator {
	mock := &MockAuthenticator{ctrl: ctrl}
	mock.recorder = &MockAuthenticatorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAuthenticator) EXPECT() *MockAuthenticatorMockRecorder {
	return m.recorder
}

// Authenticate mocks base method.
func (m *MockAuthenticator) Authenticate(arg0 *http.Request) (jelly.AuthUser, bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Authenticate", arg0)
	ret0, _ := ret[0].(jelly.AuthUser)
	ret1, _ := ret[1].(bool)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Authenticate indicates an expected call of Authenticate.
func (mr *MockAuthenticatorMockRecorder) Authenticate(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Authenticate", reflect.TypeOf((*MockAuthenticator)(nil).Authenticate), arg0)
}

// Service mocks base method.
func (m *MockAuthenticator) Service() jelly.UserLoginService {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Service")
	ret0, _ := ret[0].(jelly.UserLoginService)
	return ret0
}

// Service indicates an expected call of Service.
func (mr *MockAuthenticatorMockRecorder) Service() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Service", reflect.TypeOf((*MockAuthenticator)(nil).Service))
}

// UnauthDelay mocks base method.
func (m *MockAuthenticator) UnauthDelay() time.Duration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnauthDelay")
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// UnauthDelay indicates an expected call of UnauthDelay.
func (mr *MockAuthenticatorMockRecorder) UnauthDelay() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnauthDelay", reflect.TypeOf((*MockAuthenticator)(nil).UnauthDelay))
}
