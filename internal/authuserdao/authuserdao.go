package authuserdao

import (
	"net/mail"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/db"
	"github.com/google/uuid"
)

// User is a pre-rolled DB model version of a jelly.AuthUser.
type User struct {
	ID         uuid.UUID    // PK, NOT NULL
	Username   string       // UNIQUE, NOT NULL
	Password   string       // NOT NULL
	Email      db.Email     // NOT NULL
	Role       jelly.Role   // NOT NULL
	Created    db.Timestamp // NOT NULL
	Modified   db.Timestamp // NOT NULL
	LastLogout db.Timestamp // NOT NULL DEFAULT NOW()
	LastLogin  db.Timestamp // NOT NULL
}

func (u User) AuthUser() jelly.AuthUser {
	return jelly.AuthUser{
		ID:         u.ID,
		Username:   u.Username,
		Password:   u.Password,
		Role:       u.Role,
		Email:      u.Email.String(),
		Created:    u.Created.Time(),
		Modified:   u.Modified.Time(),
		LastLogout: u.LastLogout.Time(),
		LastLogin:  u.LastLogin.Time(),
	}
}

func NewUserFromAuthUser(au jelly.AuthUser) User {
	u := User{
		ID:         au.ID,
		Username:   au.Username,
		Password:   au.Password,
		Role:       au.Role,
		Created:    db.Timestamp(au.Created),
		Modified:   db.Timestamp(au.Modified),
		LastLogout: db.Timestamp(au.LastLogout),
		LastLogin:  db.Timestamp(au.LastLogin),
	}

	if au.Email != "" {
		m, err := mail.ParseAddress(au.Email)
		if err == nil {
			u.Email.V = m
		}
	}

	return u
}
