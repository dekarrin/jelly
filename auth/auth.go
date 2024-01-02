// Package auth provides user authentication and login services and APIs.
package auth

import (
	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/config"
)

const (
	Version = "0.0.1"
)

func init() {
	jelly.RegisterAuto("jellyauth",
		func() jelly.API { return &LoginAPI{} },
		func() config.APIConfig { return &Config{} },
	)
}

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
