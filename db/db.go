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
	"context"
	"database/sql/driver"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/dekarrin/jelly/serr"
	"github.com/dekarrin/jelly/types"
	"github.com/google/uuid"
)

type Store interface {

	// Close closes any pending operations on the DAO store and on all of its
	// Repos. It performs any clean-up operations necessary and should always be
	// called once the Store is no longer in use.
	Close() error
}

type Model[ID any] interface {
	// ModelID returns a jeldao-usable ID that identifies the Model uniquely.
	// For those fields which
	ModelID() ID
}

// Repo is a data object repository that maps ID-typed identifiers to M-typed
// entity models.
type Repo[ID any, M Model[ID]] interface {

	// Create creates a new model in the DB based on the provided one. Some
	// attributes in the provided one might not be used; for instance, many
	// Repos will automatically set the ID of new entities on creation, ignoring
	// any initially set ID. It is up to implementors to decide which attributes
	// are used.
	//
	// This returns the object as it appears in the DB after creation.
	//
	// An implementor may provide an empty implementation with a function that
	// always returns an error regardless of state and input. Consult the
	// documentation of the implementor for info.
	Create(context.Context, M) (M, error)

	// Get retrieves the model with the given ID. If no entity with that ID
	// exists, an error is returned.
	//
	// An implementor may provide an empty implementation with a function that
	// always returns an error regardless of state and input. Consult the
	// documentation of the implementor for info.
	Get(context.Context, ID) (M, error)

	// GetAll retrieves all entities in the associated store. If no entities
	// exist but no error otherwise occurred, the returned list of entities will
	// have a length of zero and the returned error will be nil.
	//
	// An implementor may provide an empty implementation with a function that
	// always returns an error regardless of state and input. Consult the
	// documentation of the implementor for info.
	GetAll(context.Context) ([]M, error)

	// Update updates a particular entity in the store to match the provided
	// model. Implementors may choose which properties of the provided value are
	// actually used.
	//
	// This returns the object as it appears in the DB after updating.
	//
	// An implementor may provide an empty implementation with a function that
	// always returns an error regardless of state and input. Consult the
	// documentation of the implementor for info.
	Update(context.Context, ID, M) (M, error)

	// Delete removes the given entity from the store.
	//
	// This returns the object as it appeared in the DB immediately before
	// deletion.
	//
	// An implementor may provide an empty implementation with a function that
	// always returns an error regardless of state and input. Consult the
	// documentation of the implementor for info.
	Delete(context.Context, ID) (M, error)

	// Close performs any clean-up operations required and flushes pending
	// operations. Not all Repos will actually perform operations, but it should
	// always be called as part of tear-down operations.
	Close() error

	// TODO: one day, move owdb Criterion functionality over and use that as a
	// generic interface into searches. Then we can have a GetAllBy(Filter) and
	// GetOneBy(Filter).
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

type Role int64

const (
	Guest Role = iota
	Unverified
	Normal

	Admin Role = 100
)

func (r Role) String() string {
	switch r {
	case Guest:
		return "guest"
	case Unverified:
		return "unverified"
	case Normal:
		return "normal"
	case Admin:
		return "admin"
	default:
		return fmt.Sprintf("Role(%d)", r)
	}
}

func (r Role) Value() (driver.Value, error) {
	return int64(r), nil
}

func (r *Role) Scan(value interface{}) error {
	iVal, ok := value.(int64)
	if !ok {
		return fmt.Errorf("not an integer value: %v", value)
	}

	*r = Role(iVal)

	return nil
}

func ParseRole(s string) (Role, error) {
	check := strings.ToLower(s)
	switch check {
	case "guest":
		return Guest, nil
	case "unverified":
		return Unverified, nil
	case "normal":
		return Normal, nil
	case "admin":
		return Admin, nil
	default:
		return Guest, fmt.Errorf("must be one of 'guest', 'unverified', 'normal', or 'admin'")
	}
}

// User is an auth model for use in the pre-rolled auth mechanism of user-in-db
// and login identified via JWT.
type User struct {
	ID         uuid.UUID // PK, NOT NULL
	Username   string    // UNIQUE, NOT NULL
	Password   string    // NOT NULL
	Email      Email     // NOT NULL
	Role       Role      // NOT NULL
	Created    Timestamp // NOT NULL
	Modified   Timestamp // NOT NULL
	LastLogout Timestamp // NOT NULL DEFAULT NOW()
	LastLogin  Timestamp // NOT NULL
}

func (u User) ModelID() uuid.UUID {
	return u.ID
}

type AuthUserRepo interface {
	Repo[uuid.UUID, User]

	// GetByUsername retrieves the User with the given username. If no entity
	// with that username exists, an error is returned.
	GetByUsername(ctx context.Context, username string) (User, error)
}

// AuthUserStore is an interface that defines methods for building a DAO store
// to be used as part of user auth via the jelly framework packages.
//
// TODO: should this be its own "sub-package"? example implementations. Or
// something. feels like it should live closer to auth-y type things.
type AuthUserStore interface {
	Store

	// AuthUsers returns a repository that holds users used as part of
	// authentication and login.
	AuthUsers() AuthUserRepo
}
