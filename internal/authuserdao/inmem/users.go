package inmem

import (
	"context"
	"fmt"
	"time"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/db"
	"github.com/dekarrin/jelly/internal/authuserdao"
	"github.com/dekarrin/jelly/internal/jelsort"
	"github.com/google/uuid"
)

func NewAuthUserRepository() *AuthUserRepo {
	return &AuthUserRepo{
		users:           make(map[uuid.UUID]authuserdao.User),
		byUsernameIndex: make(map[string]uuid.UUID),
	}
}

type AuthUserRepo struct {
	users           map[uuid.UUID]authuserdao.User
	byUsernameIndex map[string]uuid.UUID
}

func (aur *AuthUserRepo) Close() error {
	return nil
}

func (aur *AuthUserRepo) Create(ctx context.Context, u jelly.AuthUser) (jelly.AuthUser, error) {
	newUUID, err := uuid.NewRandom()
	if err != nil {
		return jelly.AuthUser{}, fmt.Errorf("could not generate ID: %w", err)
	}

	user := authuserdao.NewUserFromAuthUser(u)
	user.ID = newUUID

	// make sure it's not already in the DB
	if _, ok := aur.byUsernameIndex[user.Username]; ok {
		return jelly.AuthUser{}, jelly.ErrConstraintViolation
	}

	now := db.Timestamp(time.Now())
	user.LastLogout = now
	user.Created = now
	user.Modified = now

	aur.users[user.ID] = user
	aur.byUsernameIndex[user.Username] = user.ID

	return user.AuthUser(), nil
}

func (aur *AuthUserRepo) GetAll(ctx context.Context) ([]jelly.AuthUser, error) {
	all := make([]jelly.AuthUser, len(aur.users))

	i := 0
	for k := range aur.users {
		all[i] = aur.users[k].AuthUser()
		i++
	}

	all = jelsort.By(all, func(l, r jelly.AuthUser) bool {
		return l.ID.String() < r.ID.String()
	})

	return all, nil
}

func (aur *AuthUserRepo) Update(ctx context.Context, id uuid.UUID, u jelly.AuthUser) (jelly.AuthUser, error) {
	existing, ok := aur.users[id]
	if !ok {
		return jelly.AuthUser{}, jelly.ErrNotFound
	}
	user := authuserdao.NewUserFromAuthUser(u)

	// check for conflicts on this table only
	// (inmem does not support enforcement of foreign keys)
	if user.Username != existing.Username {
		// that's okay but we need to check it
		if _, ok := aur.byUsernameIndex[user.Username]; ok {
			return jelly.AuthUser{}, jelly.ErrConstraintViolation
		}
	} else if user.ID != id {
		// that's okay but we need to check it
		if _, ok := aur.users[user.ID]; ok {
			return jelly.AuthUser{}, jelly.ErrConstraintViolation
		}
	}

	user.Modified = db.Timestamp(time.Now())
	aur.users[user.ID] = user
	aur.byUsernameIndex[user.Username] = user.ID
	if user.ID != id {
		delete(aur.users, id)
	}

	return user.AuthUser(), nil
}

func (aur *AuthUserRepo) Get(ctx context.Context, id uuid.UUID) (jelly.AuthUser, error) {
	user, ok := aur.users[id]
	if !ok {
		return jelly.AuthUser{}, jelly.ErrNotFound
	}

	return user.AuthUser(), nil
}

func (aur *AuthUserRepo) GetByUsername(ctx context.Context, username string) (jelly.AuthUser, error) {
	userID, ok := aur.byUsernameIndex[username]
	if !ok {
		return jelly.AuthUser{}, jelly.ErrNotFound
	}

	return aur.users[userID].AuthUser(), nil
}

func (aur *AuthUserRepo) Delete(ctx context.Context, id uuid.UUID) (jelly.AuthUser, error) {
	user, ok := aur.users[id]
	if !ok {
		return jelly.AuthUser{}, jelly.ErrNotFound
	}

	delete(aur.byUsernameIndex, user.Username)
	delete(aur.users, user.ID)

	return user.AuthUser(), nil
}
