package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"net/mail"
	"time"

	"github.com/dekarrin/jelly/dao"
	"github.com/dekarrin/jelly/jelerr"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// LoginService is a pre-rolled login and authentication backend service. It is
// backed by a persistence layer and will make calls to persist when needed.
//
// The zero-value of LoginService is not ready to be used until its Provider is
// set.
type LoginService struct {
	Provider dao.AuthUserStore
}

// Login verifies the provided username and password against the existing user
// in persistence and returns that user if they match. Returns the user entity
// from the persistence layer that the username and password are valid for.
//
// The returned error, if non-nil, will return true for various calls to
// errors.Is depending on what caused the error. If the credentials do not match
// a user or if the password is incorrect, it will match ErrBadCredentials. If
// the error occured due to an unexpected problem with the DB, it will match
// serr.ErrDB.
func (svc LoginService) Login(ctx context.Context, username string, password string) (dao.User, error) {
	user, err := svc.Provider.AuthUsers().GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return dao.User{}, jelerr.ErrBadCredentials
		}
		return dao.User{}, jelerr.WrapDB("", err)
	}

	// verify password
	bcryptHash, err := base64.StdEncoding.DecodeString(user.Password)
	if err != nil {
		return dao.User{}, err
	}

	err = bcrypt.CompareHashAndPassword(bcryptHash, []byte(password))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return dao.User{}, jelerr.ErrBadCredentials
		}
		return dao.User{}, jelerr.WrapDB("", err)
	}

	// successful login; update the DB
	user.LastLoginTime = time.Now()
	user, err = svc.Provider.AuthUsers().Update(ctx, user.ID, user)
	if err != nil {
		return dao.User{}, jelerr.WrapDB("cannot update user login time", err)
	}

	return user, nil
}

// Logout marks the user with the given ID as having logged out, invalidating
// any login that may be active. Returns the user entity that was logged out.
//
// The returned error, if non-nil, will return true for various calls to
// errors.Is depending on what caused the error. If the user doesn't exist, it
// will match serr.ErrNotFound. If the error occured due to an unexpected
// problem with the DB, it will match serr.ErrDB.
func (svc LoginService) Logout(ctx context.Context, who uuid.UUID) (dao.User, error) {
	existing, err := svc.Provider.AuthUsers().Get(ctx, who)
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return dao.User{}, jelerr.ErrNotFound
		}
		return dao.User{}, jelerr.WrapDB("could not retrieve user", err)
	}

	existing.LastLogoutTime = time.Now()

	updated, err := svc.Provider.AuthUsers().Update(ctx, existing.ID, existing)
	if err != nil {
		return dao.User{}, jelerr.WrapDB("could not update user", err)
	}

	return updated, nil
}

// GetAllUsers returns all auth users currently in persistence.
func (svc LoginService) GetAllUsers(ctx context.Context) ([]dao.User, error) {
	users, err := svc.Provider.AuthUsers().GetAll(ctx)
	if err != nil {
		return nil, jelerr.WrapDB("", err)
	}

	return users, nil
}

// GetUser returns the user with the given ID.
//
// The returned error, if non-nil, will return true for various calls to
// errors.Is depending on what caused the error. If no user with that ID exists,
// it will match serr.ErrNotFound. If the error occured due to an unexpected
// problem with the DB, it will match serr.ErrDB. Finally, if there is an issue
// with one of the arguments, it will match serr.ErrBadArgument.
func (svc LoginService) GetUser(ctx context.Context, id string) (dao.User, error) {
	uuidID, err := uuid.Parse(id)
	if err != nil {
		return dao.User{}, jelerr.New("ID is not valid", jelerr.ErrBadArgument)
	}

	user, err := svc.Provider.AuthUsers().Get(ctx, uuidID)
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return dao.User{}, jelerr.ErrNotFound
		}
		return dao.User{}, jelerr.WrapDB("could not get user", err)
	}

	return user, nil
}

// CreateUser creates a new user with the given username, password, and email
// combo. Returns the newly-created user as it exists after creation.
//
// The returned error, if non-nil, will return true for various calls to
// errors.Is depending on what caused the error. If a user with that username is
// already present, it will match serr.ErrAlreadyExists. If the error occured
// due to an unexpected problem with the DB, it will match serr.ErrDB. Finally,
// if one of the arguments is invalid, it will match serr.ErrBadArgument.
func (svc LoginService) CreateUser(ctx context.Context, username, password, email string, role dao.Role) (dao.User, error) {
	var err error
	if username == "" {
		return dao.User{}, jelerr.New("username cannot be blank", err, jelerr.ErrBadArgument)
	}
	if password == "" {
		return dao.User{}, jelerr.New("password cannot be blank", err, jelerr.ErrBadArgument)
	}

	var storedEmail *mail.Address
	if email != "" {
		storedEmail, err = mail.ParseAddress(email)
		if err != nil {
			return dao.User{}, jelerr.New("email is not valid", err, jelerr.ErrBadArgument)
		}
	}

	_, err = svc.Provider.AuthUsers().GetByUsername(ctx, username)
	if err == nil {
		return dao.User{}, jelerr.New("a user with that username already exists", jelerr.ErrAlreadyExists)
	} else if !errors.Is(err, dao.ErrNotFound) {
		return dao.User{}, jelerr.WrapDB("", err)
	}

	passHash, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		if err == bcrypt.ErrPasswordTooLong {
			return dao.User{}, jelerr.New("password is too long", err, jelerr.ErrBadArgument)
		} else {
			return dao.User{}, jelerr.New("password could not be encrypted", err)
		}
	}

	storedPass := base64.StdEncoding.EncodeToString(passHash)

	newUser := dao.User{
		Username: username,
		Password: storedPass,
		Email:    storedEmail,
		Role:     role,
	}

	user, err := svc.Provider.AuthUsers().Create(ctx, newUser)
	if err != nil {
		if errors.Is(err, dao.ErrConstraintViolation) {
			return dao.User{}, jelerr.ErrAlreadyExists
		}
		return dao.User{}, jelerr.WrapDB("could not create user", err)
	}

	return user, nil
}

// UpdateUser sets the properties of the user with the given ID to the
// properties in the given user. All the given properties of the user will
// overwrite the existing ones. Returns the updated user.
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
func (svc LoginService) UpdateUser(ctx context.Context, curID, newID, username, email string, role dao.Role) (dao.User, error) {
	var err error

	if username == "" {
		return dao.User{}, jelerr.New("username cannot be blank", err, jelerr.ErrBadArgument)
	}

	var storedEmail *mail.Address
	if email != "" {
		storedEmail, err = mail.ParseAddress(email)
		if err != nil {
			return dao.User{}, jelerr.New("email is not valid", err, jelerr.ErrBadArgument)
		}
	}

	uuidCurID, err := uuid.Parse(curID)
	if err != nil {
		return dao.User{}, jelerr.New("current ID is not valid", jelerr.ErrBadArgument)
	}
	uuidNewID, err := uuid.Parse(newID)
	if err != nil {
		return dao.User{}, jelerr.New("new ID is not valid", jelerr.ErrBadArgument)
	}

	daoUser, err := svc.Provider.AuthUsers().Get(ctx, uuidCurID)
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return dao.User{}, jelerr.New("user not found", jelerr.ErrNotFound)
		}
	}

	if curID != newID {
		_, err := svc.Provider.AuthUsers().Get(ctx, uuidNewID)
		if err == nil {
			return dao.User{}, jelerr.New("a user with that username already exists", jelerr.ErrAlreadyExists)
		} else if !errors.Is(err, dao.ErrNotFound) {
			return dao.User{}, jelerr.WrapDB("", err)
		}
	}
	if daoUser.Username != username {
		_, err := svc.Provider.AuthUsers().GetByUsername(ctx, username)
		if err == nil {
			return dao.User{}, jelerr.New("a user with that username already exists", jelerr.ErrAlreadyExists)
		} else if !errors.Is(err, dao.ErrNotFound) {
			return dao.User{}, jelerr.WrapDB("", err)
		}
	}

	daoUser.Email = storedEmail
	daoUser.ID = uuidNewID
	daoUser.Username = username
	daoUser.Role = role

	updatedUser, err := svc.Provider.AuthUsers().Update(ctx, uuidCurID, daoUser)
	if err != nil {
		if errors.Is(err, dao.ErrConstraintViolation) {
			return dao.User{}, jelerr.New("a user with that ID/username already exists", jelerr.ErrAlreadyExists)
		} else if errors.Is(err, dao.ErrNotFound) {
			return dao.User{}, jelerr.New("user not found", jelerr.ErrNotFound)
		}
		return dao.User{}, jelerr.WrapDB("", err)
	}

	return updatedUser, nil
}

// UpdatePassword sets the password of the user with the given ID to the new
// password. The new password cannot be empty. Returns the updated user.
//
// The returned error, if non-nil, will return true for various calls to
// errors.Is depending on what caused the error. If no user with the given ID
// exists, it will match serr.ErrNotFound. If the error occured due to an
// unexpected problem with the DB, it will match serr.ErrDB. Finally, if one of
// the arguments is invalid, it will match serr.ErrBadArgument.
func (svc LoginService) UpdatePassword(ctx context.Context, id, password string) (dao.User, error) {
	if password == "" {
		return dao.User{}, jelerr.New("password cannot be empty", jelerr.ErrBadArgument)
	}
	uuidID, err := uuid.Parse(id)
	if err != nil {
		return dao.User{}, jelerr.New("ID is not valid", jelerr.ErrBadArgument)
	}

	existing, err := svc.Provider.AuthUsers().Get(ctx, uuidID)
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return dao.User{}, jelerr.New("no user with that ID exists", jelerr.ErrNotFound)
		}
		return dao.User{}, jelerr.WrapDB("", err)
	}

	passHash, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		if err == bcrypt.ErrPasswordTooLong {
			return dao.User{}, jelerr.New("password is too long", err, jelerr.ErrBadArgument)
		} else {
			return dao.User{}, jelerr.New("password could not be encrypted", err)
		}
	}

	storedPass := base64.StdEncoding.EncodeToString(passHash)

	existing.Password = storedPass

	updated, err := svc.Provider.AuthUsers().Update(ctx, uuidID, existing)
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return dao.User{}, jelerr.New("no user with that ID exists", jelerr.ErrNotFound)
		}
		return dao.User{}, jelerr.WrapDB("could not update user", err)
	}

	return updated, nil
}

// DeleteUser deletes the user with the given ID. It returns the deleted user
// just after they were deleted.
//
// The returned error, if non-nil, will return true for various calls to
// errors.Is depending on what caused the error. If no user with that username
// exists, it will match serr.ErrNotFound. If the error occured due to an
// unexpected problem with the DB, it will match serr.ErrDB. Finally, if there
// is an issue with one of the arguments, it will match serr.ErrBadArgument.
func (svc LoginService) DeleteUser(ctx context.Context, id string) (dao.User, error) {
	uuidID, err := uuid.Parse(id)
	if err != nil {
		return dao.User{}, jelerr.New("ID is not valid", jelerr.ErrBadArgument)
	}

	user, err := svc.Provider.AuthUsers().Delete(ctx, uuidID)
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return dao.User{}, jelerr.ErrNotFound
		}
		return dao.User{}, jelerr.WrapDB("could not delete user", err)
	}

	return user, nil
}
