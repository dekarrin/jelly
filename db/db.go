// Package db provides data access objects compatible with the rest of the
// jelly framework packages.
//
// It includes basics as well as a sample implementation of Store that is
// compatible with jelly auth middleware.
//
// TODO: call this package db or somefin and move auth-specific to middleware.
// For sure by GHI-016 if not before.
package db

import (
	"database/sql/driver"
	"fmt"
	"net/mail"
	"time"

	"github.com/dekarrin/jelly/serr"
	"github.com/dekarrin/jelly/types"
	"github.com/google/uuid"
)

type Model[ID any] interface {
	// ModelID returns a jeldao-usable ID that identifies the Model uniquely.
	// For those fields which
	ModelID() ID
}

func NowTimestamp() Timestamp {
	return Timestamp(time.Now())
}

// Timestamp is a time.Time variation that stores itself in the DB as the number
// of seconds since the Unix epoch.
type Timestamp time.Time

func (ts Timestamp) Format(layout string) string {
	return ts.Time().Format(layout)
}

func (ts Timestamp) Value() (driver.Value, error) {
	return time.Time(ts).Unix(), nil
}

func (ts *Timestamp) Scan(value interface{}) error {
	iVal, ok := value.(int64)
	if !ok {
		return fmt.Errorf("not an integer value: %v", value)
	}

	tVal := time.Unix(iVal, 0)
	*ts = Timestamp(tVal)
	return nil
}

func (ts Timestamp) Time() time.Time {
	return time.Time(ts)
}

// Email is a mail.Addresss that stores itself as a string.
type Email struct {
	V *mail.Address
}

func (em Email) String() string {
	if em.V == nil {
		return ""
	}
	return em.V.Address
}

func (em Email) Value() (driver.Value, error) {
	return em.String(), nil
}

func (em *Email) Scan(value interface{}) error {
	s, ok := value.(string)
	if !ok {
		return serr.New(fmt.Sprintf("not an integer value: %v", value), types.DBErrDecodingFailure)
	}
	if s == "" {
		em.V = nil
		return nil
	}

	email, err := mail.ParseAddress(s)
	if err != nil {
		return serr.New("", err, types.DBErrDecodingFailure)
	}

	em.V = email
	return nil
}

// User is a pre-rolled DB model version of a jelly.AuthUser.
type User struct {
	ID         uuid.UUID  // PK, NOT NULL
	Username   string     // UNIQUE, NOT NULL
	Password   string     // NOT NULL
	Email      Email      // NOT NULL
	Role       types.Role // NOT NULL
	Created    Timestamp  // NOT NULL
	Modified   Timestamp  // NOT NULL
	LastLogout Timestamp  // NOT NULL DEFAULT NOW()
	LastLogin  Timestamp  // NOT NULL
}

func (u User) ModelID() uuid.UUID {
	return u.ID
}

func (u User) AuthUser() types.AuthUser {
	return types.AuthUser{
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

func NewUserFromAuthUser(au types.AuthUser) User {
	u := User{
		ID:         au.ID,
		Username:   au.Username,
		Password:   au.Password,
		Role:       au.Role,
		Created:    Timestamp(au.Created),
		Modified:   Timestamp(au.Modified),
		LastLogout: Timestamp(au.LastLogout),
		LastLogin:  Timestamp(au.LastLogin),
	}

	if au.Email != "" {
		m, err := mail.ParseAddress(au.Email)
		if err == nil {
			u.Email.V = m
		}
	}

	return u
}
