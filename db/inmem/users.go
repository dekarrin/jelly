package inmem

import (
	"context"
	"fmt"
	"time"

	"github.com/dekarrin/jelly/db"
	"github.com/dekarrin/jelly/internal/jelsort"
	"github.com/dekarrin/jelly/types"
	"github.com/google/uuid"
)

func NewAuthUserRepository() *AuthUserRepo {
	return &AuthUserRepo{
		users:           make(map[uuid.UUID]db.User),
		byUsernameIndex: make(map[string]uuid.UUID),
	}
}

type AuthUserRepo struct {
	users           map[uuid.UUID]db.User
	byUsernameIndex map[string]uuid.UUID
}

func (aur *AuthUserRepo) Close() error {
	return nil
}

func (aur *AuthUserRepo) Create(ctx context.Context, user db.User) (db.User, error) {
	newUUID, err := uuid.NewRandom()
	if err != nil {
		return db.User{}, fmt.Errorf("could not generate ID: %w", err)
	}

	user.ID = newUUID

	// make sure it's not already in the DB
	if _, ok := aur.byUsernameIndex[user.Username]; ok {
		return db.User{}, types.DBErrConstraintViolation
	}

	now := db.Timestamp(time.Now())
	user.LastLogout = now
	user.Created = now
	user.Modified = now

	aur.users[user.ID] = user
	aur.byUsernameIndex[user.Username] = user.ID

	return user, nil
}

func (aur *AuthUserRepo) GetAll(ctx context.Context) ([]db.User, error) {
	all := make([]db.User, len(aur.users))

	i := 0
	for k := range aur.users {
		all[i] = aur.users[k]
		i++
	}

	all = jelsort.By(all, func(l, r db.User) bool {
		return l.ID.String() < r.ID.String()
	})

	return all, nil
}

func (aur *AuthUserRepo) Update(ctx context.Context, id uuid.UUID, user db.User) (db.User, error) {
	existing, ok := aur.users[id]
	if !ok {
		return db.User{}, types.DBErrNotFound
	}

	// check for conflicts on this table only
	// (inmem does not support enforcement of foreign keys)
	if user.Username != existing.Username {
		// that's okay but we need to check it
		if _, ok := aur.byUsernameIndex[user.Username]; ok {
			return db.User{}, types.DBErrConstraintViolation
		}
	} else if user.ID != id {
		// that's okay but we need to check it
		if _, ok := aur.users[user.ID]; ok {
			return db.User{}, types.DBErrConstraintViolation
		}
	}

	user.Modified = db.Timestamp(time.Now())
	aur.users[user.ID] = user
	aur.byUsernameIndex[user.Username] = user.ID
	if user.ID != id {
		delete(aur.users, id)
	}

	return user, nil
}

func (aur *AuthUserRepo) Get(ctx context.Context, id uuid.UUID) (db.User, error) {
	user, ok := aur.users[id]
	if !ok {
		return db.User{}, types.DBErrNotFound
	}

	return user, nil
}

func (aur *AuthUserRepo) GetByUsername(ctx context.Context, username string) (db.User, error) {
	userID, ok := aur.byUsernameIndex[username]
	if !ok {
		return db.User{}, types.DBErrNotFound
	}

	return aur.users[userID], nil
}

func (aur *AuthUserRepo) Delete(ctx context.Context, id uuid.UUID) (db.User, error) {
	user, ok := aur.users[id]
	if !ok {
		return db.User{}, types.DBErrNotFound
	}

	delete(aur.byUsernameIndex, user.Username)
	delete(aur.users, user.ID)

	return user, nil
}
