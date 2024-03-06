package jelly

import (
	"context"
	"database/sql/driver"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

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

// AuthUser is an auth model for use in the pre-rolled auth mechanism of user-in-db
// and login identified via JWT.
type AuthUser struct {
	ID         uuid.UUID // PK, NOT NULL
	Username   string    // UNIQUE, NOT NULL
	Password   string    // NOT NULL
	Email      string    // NOT NULL
	Role       Role      // NOT NULL
	Created    time.Time // NOT NULL
	Modified   time.Time // NOT NULL
	LastLogout time.Time // NOT NULL DEFAULT NOW()
	LastLogin  time.Time // NOT NULL
}

// WithID returns a copy of user but with ID set to the given one. The original
// is not modified.
func (user AuthUser) WithID(id uuid.UUID) AuthUser {
	newUser := user
	newUser.ID = id
	return newUser
}

// WithUsername returns a copy of user but with Username set to the given one.
// The original is not modified.
func (user AuthUser) WithUsername(username string) AuthUser {
	newUser := user
	newUser.Username = username
	return newUser
}

// WithPassword returns a copy of user but with Password set to the given one.
// The original is not modified.
func (user AuthUser) WithPassword(password string) AuthUser {
	newUser := user
	newUser.Password = password
	return newUser
}

// WithEmail returns a copy of user but with Email set to the given one. The
// original is not modified.
func (user AuthUser) WithEmail(email string) AuthUser {
	newUser := user
	newUser.Email = email
	return newUser
}

// WithRole returns a copy of user but with Role set to the given one. The
// original is not modified.
func (user AuthUser) WithRole(role Role) AuthUser {
	newUser := user
	newUser.Role = role
	return newUser
}

// WithCreated returns a copy of user but with Created set to the given time.
// The original is not modified.
//
// Note that in pre-rolled AuthUser stores, the DAO will automatically set the
// created time and ignores manual modification to that value after creation.
func (user AuthUser) WithCreated(created time.Time) AuthUser {
	newUser := user
	newUser.Created = created
	return newUser
}

// WithModified returns a copy of user but with Modified set to the given time.
// The original is not modified.
//
// Note that in pre-rolled AuthUser stores, the DAO will automatically set the
// modified time and ignores manual modifications to that value.
func (user AuthUser) WithModified(modified time.Time) AuthUser {
	newUser := user
	newUser.Modified = modified
	return newUser
}

// WithLastLogin returns a copy of user but with LastLogin set to the given
// time. The original is not modified.
func (user AuthUser) WithLastLogin(lastLogin time.Time) AuthUser {
	newUser := user
	newUser.LastLogin = lastLogin
	return newUser
}

// WithLastLogout returns a copy of user but with LastLogout set to the given
// time. The original is not modified.
func (user AuthUser) WithLastLogout(lastLogout time.Time) AuthUser {
	newUser := user
	newUser.LastLogout = lastLogout
	return newUser
}

type AuthUserRepo interface {
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
	//
	// LastLogin, LastLogout, Created, and Modified are all automatically set to
	// the current timestamp regardless of their values in the given AuthUser.
	Create(context.Context, AuthUser) (AuthUser, error)

	// Get retrieves the model with the given ID. If no entity with that ID
	// exists, an error is returned.
	//
	// An implementor may provide an empty implementation with a function that
	// always returns an error regardless of state and input. Consult the
	// documentation of the implementor for info.
	Get(context.Context, uuid.UUID) (AuthUser, error)

	// GetAll retrieves all entities in the associated store. If no entities
	// exist but no error otherwise occurred, the returned list of entities will
	// have a length of zero and the returned error will be nil.
	//
	// An implementor may provide an empty implementation with a function that
	// always returns an error regardless of state and input. Consult the
	// documentation of the implementor for info.
	//
	// If there are no entries, the implementation should return an empty list
	// and a nil error, even if the underlying driver returns a not-found error.
	//
	// TODO: audao/sqlite/users.go returns error and should not on GetAll.
	GetAll(context.Context) ([]AuthUser, error)

	// Update updates a particular entity in the store to match the provided
	// model. Implementors may choose which properties of the provided value are
	// actually used.
	//
	// This returns the object as it appears in the DB after updating.
	//
	// An implementor may provide an empty implementation with a function that
	// always returns an error regardless of state and input. Consult the
	// documentation of the implementor for info.
	//
	// Modified is updated automatically; Created is ignored entirely.
	Update(context.Context, uuid.UUID, AuthUser) (AuthUser, error)

	// Delete removes the given entity from the store.
	//
	// This returns the object as it appeared in the DB immediately before
	// deletion.
	//
	// An implementor may provide an empty implementation with a function that
	// always returns an error regardless of state and input. Consult the
	// documentation of the implementor for info.
	Delete(context.Context, uuid.UUID) (AuthUser, error)

	// Close performs any clean-up operations required and flushes pending
	// operations. Not all Repos will actually perform operations, but it should
	// always be called as part of tear-down operations.
	Close() error

	// TODO: one day, move owdb Criterion functionality over and use that as a
	// generic interface into searches. Then we can have a GetAllBy(Filter) and
	// GetOneBy(Filter).

	// GetByUsername retrieves the User with the given username. If no entity
	// with that username exists, an error is returned.
	GetByUsername(ctx context.Context, username string) (AuthUser, error)
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

// UserLoginService provides a way to control the state of login of users and
// retrieve users from the backend store.
type UserLoginService interface {
	// Login verifies the provided username and password against the existing user
	// in persistence and returns that user if they match. Returns the user entity
	// from the persistence layer that the username and password are valid for.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If the credentials do not match
	// a user or if the password is incorrect, it will match ErrBadCredentials. If
	// the error occured due to an unexpected problem with the DB, it will match
	// serr.ErrDB.
	Login(ctx context.Context, username string, password string) (AuthUser, error)

	// Logout marks the user with the given ID as having logged out, invalidating
	// any login that may be active. Returns the user entity that was logged out.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If the user doesn't exist, it
	// will match serr.ErrNotFound. If the error occured due to an unexpected
	// problem with the DB, it will match serr.ErrDB.
	Logout(ctx context.Context, who uuid.UUID) (AuthUser, error)

	// GetAllUsers returns all auth users currently in persistence.
	GetAllUsers(ctx context.Context) ([]AuthUser, error)

	// GetUser returns the user with the given ID.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If no user with that ID exists,
	// it will match serr.ErrNotFound. If the error occured due to an unexpected
	// problem with the DB, it will match serr.ErrDB. Finally, if there is an issue
	// with one of the arguments, it will match serr.ErrBadArgument.
	GetUser(ctx context.Context, id string) (AuthUser, error)

	// GetUserByUsername returns the user with the given username.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If no user with that ID exists,
	// it will match serr.ErrNotFound. If the error occured due to an unexpected
	// problem with the DB, it will match serr.ErrDB. Finally, if there is an issue
	// with one of the arguments, it will match serr.ErrBadArgument.
	GetUserByUsername(ctx context.Context, username string) (AuthUser, error)

	// CreateUser creates a new user with the given username, password, and email
	// combo. Returns the newly-created user as it exists after creation.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If a user with that username is
	// already present, it will match serr.ErrAlreadyExists. If the error occured
	// due to an unexpected problem with the DB, it will match serr.ErrDB. Finally,
	// if one of the arguments is invalid, it will match serr.ErrBadArgument.
	CreateUser(ctx context.Context, username, password, email string, role Role) (AuthUser, error)

	// UpdateUser sets all properties except the password of the user with the
	// given ID to the properties in the provider user. All the given properties
	// of the user (except password) will overwrite the existing ones. Returns
	// the updated user.
	//
	// This function cannot be used to update the password. Use UpdatePassword for
	// that.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If a user with that username or
	// ID (if they are changing) is already present, it will match
	// serr.ErrAlreadyExists. If no user with the given ID exists, it will match
	// serr.ErrNotFound. If the error occured due to an unexpected problem with the
	// DB, it will match serr.ErrDB. Finally, if one of the arguments is invalid, it
	// will match serr.ErrBadArgument.
	UpdateUser(ctx context.Context, curID, newID, username, email string, role Role) (AuthUser, error)

	// UpdatePassword sets the password of the user with the given ID to the new
	// password. The new password cannot be empty. Returns the updated user.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If no user with the given ID
	// exists, it will match serr.ErrNotFound. If the error occured due to an
	// unexpected problem with the DB, it will match serr.ErrDB. Finally, if one of
	// the arguments is invalid, it will match serr.ErrBadArgument.
	UpdatePassword(ctx context.Context, id, password string) (AuthUser, error)

	// DeleteUser deletes the user with the given ID. It returns the deleted user
	// just after they were deleted.
	//
	// The returned error, if non-nil, will return true for various calls to
	// errors.Is depending on what caused the error. If no user with that username
	// exists, it will match serr.ErrNotFound. If the error occured due to an
	// unexpected problem with the DB, it will match serr.ErrDB. Finally, if there
	// is an issue with one of the arguments, it will match serr.ErrBadArgument.
	DeleteUser(ctx context.Context, id string) (AuthUser, error)
}

// Authenticator is middleware for an endpoint that will accept a request,
// extract the token used for authentication, and make calls to get a User
// entity that represents the logged in user from the token.
//
// Keys are added to the request context before the request is passed to the
// next step in the chain. AuthUser will contain the logged-in user, and
// AuthLoggedIn will return whether the user is logged in (only applies for
// optional logins; for non-optional, not being logged in will result in an
// HTTP error being returned before the request is passed to the next handler).
type Authenticator interface {

	// Authenticate retrieves the user details from the request using whatever
	// method is correct for the auth handler. Returns the user, whether the
	// user is currently logged in, and any error that occured. If the user is
	// not logged in but no error actually occured, a default user and logged-in
	// = false are returned with a nil error. An error should only be returned
	// if there is an issue authenticating the user, and a user not being logged
	// in does not count as an issue. If the user fails to validate due to bad
	// credentials, that does count and should be returned as an error.
	//
	// If the user is logged-in, returns the logged-in user, true, and a nil
	// error.
	Authenticate(req *http.Request) (AuthUser, bool, error)

	// Service returns the UserLoginService that can be used to control active
	// logins and the list of users.
	Service() UserLoginService

	// UnauthDelay is the amount of time that the system should delay responding
	// to unauthenticated requests to endpoints that require auth.
	UnauthDelay() time.Duration
}
