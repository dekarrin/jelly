package jelinmem

import (
	"context"
	"fmt"
	"time"

	dao "github.com/dekarrin/jelly/dao"
	"github.com/dekarrin/jelly/internal/jelsort"
	"github.com/google/uuid"
)

func NewAuthUserRepository() *AuthUserRepo {
	return &AuthUserRepo{
		users:           make(map[uuid.UUID]dao.User),
		byUsernameIndex: make(map[string]uuid.UUID),
	}
}

type AuthUserRepo struct {
	users           map[uuid.UUID]dao.User
	byUsernameIndex map[string]uuid.UUID
}

func (aur *AuthUserRepo) Close() error {
	return nil
}

func (aur *AuthUserRepo) Create(ctx context.Context, user dao.User) (dao.User, error) {
	newUUID, err := uuid.NewRandom()
	if err != nil {
		return dao.User{}, fmt.Errorf("could not generate ID: %w", err)
	}

	user.ID = newUUID

	// make sure it's not already in the DB
	if _, ok := aur.byUsernameIndex[user.Username]; ok {
		return dao.User{}, dao.ErrConstraintViolation
	}

	now := time.Now()
	user.LastLogoutTime = now
	user.Created = now
	user.Modified = now

	aur.users[user.ID] = user
	aur.byUsernameIndex[user.Username] = user.ID

	return user, nil
}

func (aur *AuthUserRepo) GetAll(ctx context.Context) ([]dao.User, error) {
	all := make([]dao.User, len(aur.users))

	i := 0
	for k := range aur.users {
		all[i] = aur.users[k]
		i++
	}

	all = jelsort.By(all, func(l, r dao.User) bool {
		return l.ID.String() < r.ID.String()
	})

	return all, nil
}

func (aur *AuthUserRepo) Update(ctx context.Context, id uuid.UUID, user dao.User) (dao.User, error) {
	existing, ok := aur.users[id]
	if !ok {
		return dao.User{}, dao.ErrNotFound
	}

	// check for conflicts on this table only
	// (inmem does not support enforcement of foreign keys)
	if user.Username != existing.Username {
		// that's okay but we need to check it
		if _, ok := aur.byUsernameIndex[user.Username]; ok {
			return dao.User{}, dao.ErrConstraintViolation
		}
	} else if user.ID != id {
		// that's okay but we need to check it
		if _, ok := aur.users[user.ID]; ok {
			return dao.User{}, dao.ErrConstraintViolation
		}
	}

	user.Modified = time.Now()
	aur.users[user.ID] = user
	aur.byUsernameIndex[user.Username] = user.ID
	if user.ID != id {
		delete(aur.users, id)
	}

	return user, nil
}

func (aur *AuthUserRepo) Get(ctx context.Context, id uuid.UUID) (dao.User, error) {
	user, ok := aur.users[id]
	if !ok {
		return dao.User{}, dao.ErrNotFound
	}

	return user, nil
}

func (aur *AuthUserRepo) GetByUsername(ctx context.Context, username string) (dao.User, error) {
	userID, ok := aur.byUsernameIndex[username]
	if !ok {
		return dao.User{}, dao.ErrNotFound
	}

	return aur.users[userID], nil
}

func (aur *AuthUserRepo) Delete(ctx context.Context, id uuid.UUID) (dao.User, error) {
	user, ok := aur.users[id]
	if !ok {
		return dao.User{}, dao.ErrNotFound
	}

	delete(aur.byUsernameIndex, user.Username)
	delete(aur.users, user.ID)

	return user, nil
}
