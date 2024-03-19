// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/dekarrin/jelly (interfaces: API)
//
// Generated by this command:
//
//	mockgen -destination tools/mocks/jelly/mock_api.go github.com/dekarrin/jelly API
//

// Package mock_jelly is a generated GoMock package.
package mock_jelly

import (
	context "context"
	reflect "reflect"

	jelly "github.com/dekarrin/jelly"
	chi "github.com/go-chi/chi/v5"
	gomock "go.uber.org/mock/gomock"
)

// MockAPI is a mock of API interface.
type MockAPI struct {
	ctrl     *gomock.Controller
	recorder *MockAPIMockRecorder
}

// MockAPIMockRecorder is the mock recorder for MockAPI.
type MockAPIMockRecorder struct {
	mock *MockAPI
}

// NewMockAPI creates a new mock instance.
func NewMockAPI(ctrl *gomock.Controller) *MockAPI {
	mock := &MockAPI{ctrl: ctrl}
	mock.recorder = &MockAPIMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAPI) EXPECT() *MockAPIMockRecorder {
	return m.recorder
}

// Authenticators mocks base method.
func (m *MockAPI) Authenticators() map[string]jelly.Authenticator {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Authenticators")
	ret0, _ := ret[0].(map[string]jelly.Authenticator)
	return ret0
}

// Authenticators indicates an expected call of Authenticators.
func (mr *MockAPIMockRecorder) Authenticators() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Authenticators", reflect.TypeOf((*MockAPI)(nil).Authenticators))
}

// Init mocks base method.
func (m *MockAPI) Init(arg0 jelly.Bundle) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockAPIMockRecorder) Init(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockAPI)(nil).Init), arg0)
}

// Routes mocks base method.
func (m *MockAPI) Routes(arg0 jelly.ServiceProvider) chi.Router {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Routes", arg0)
	ret0, _ := ret[0].(chi.Router)
	return ret0
}

// Routes indicates an expected call of Routes.
func (mr *MockAPIMockRecorder) Routes(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Routes", reflect.TypeOf((*MockAPI)(nil).Routes), arg0)
}

// Shutdown mocks base method.
func (m *MockAPI) Shutdown(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Shutdown", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Shutdown indicates an expected call of Shutdown.
func (mr *MockAPIMockRecorder) Shutdown(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Shutdown", reflect.TypeOf((*MockAPI)(nil).Shutdown), arg0)
}
