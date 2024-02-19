// Package auth provides user authentication and login services and APIs. It
// supplies the "jellyauth" component.
//
// To use the jellyauth component, simply add a "jellyauth" section to your
// config and call jelly.Use(auth.Component) before loading config. This config
// section overrides the typical API defaults and will create a fully functional
// system simply by being enabled, albeit with an unpersisted, in-memory
// database.
//
// TODO: carry over config instructions from the example.
package auth

import (
	"github.com/dekarrin/jelly"
)

const (
	Version = "0.0.1"
)

type ComponentInfo struct{}

func (ci ComponentInfo) Name() string {
	return "jellyauth"
}

func (ci ComponentInfo) API() jelly.API {
	return &LoginAPI{}
}

func (ci ComponentInfo) Config() jelly.APIConfig {
	return &Config{}
}

var (
	// Component holds the component information for jellyauth. This is passed
	// to jelly.Use to enable the use of jellyauth in a server.
	Component jelly.Component = ComponentInfo{}
)

type LoginResponse struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserModel struct {
	URI            string `json:"uri"`
	ID             string `json:"id,omitempty"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	Email          string `json:"email,"`
	Role           string `json:"role,omitempty"`
	Created        string `json:"created,omitempty"`
	Modified       string `json:"modified,omitempty"`
	LastLogoutTime string `json:"last_logout,omitempty"`
	LastLoginTime  string `json:"last_login,omitempty"`
}

type UserUpdateRequest struct {
	ID       UpdateString `json:"id,omitempty"`
	Username UpdateString `json:"username,omitempty"`
	Password UpdateString `json:"password,omitempty"`
	Email    UpdateString `json:"email,"`
	Role     UpdateString `json:"role,omitempty"`
}

type UpdateString struct {
	Update bool   `json:"u,omitempty"`
	Value  string `json:"v,omitempty"`
}

type InfoModel struct {
	Version struct {
		Auth string `json:"auth"`
	} `json:"version"`
}
