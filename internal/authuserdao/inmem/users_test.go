package inmem

import (
	"context"
	"testing"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/internal/authuserdao"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

var (
	testUser_dave = jelly.AuthUser{
		ID:       uuid.MustParse("82779fe7-d681-427d-a011-4954b6a7ec01"),
		Username: "turntechGodhead",
		Email:    "dave@morethanpuppets.com",
	}

	testUser_rose = jelly.AuthUser{
		ID:       uuid.MustParse("82779fe7-d681-427d-a011-4954b6a7ec02"),
		Username: "tentacleTherapist",
		Email:    "rose@skaialabs.net",
	}

	testUser_jade = jelly.AuthUser{
		ID:       uuid.MustParse("82779fe7-d681-427d-a011-4954b6a7ec03"),
		Username: "gardenGnostic",
		Email:    "jade@ohnothanksiusepesterchum.com",
	}

	testUser_john = jelly.AuthUser{
		ID:       uuid.MustParse("82779fe7-d681-427d-a011-4954b6a7ec04"),
		Username: "ectoBiologist",
		Email:    "john@ghostbusters2.online",
	}

	testUser_dave_badEmail = jelly.AuthUser{
		ID:       uuid.MustParse("82779fe7-d681-427d-a011-4954b6a7ec01"),
		Username: "turntechGodhead",
		Email:    "invalid email",
	}

	testDAOUser_dave = authuserdao.NewUserFromAuthUser(testUser_dave)
	testDAOUser_jade = authuserdao.NewUserFromAuthUser(testUser_jade)
	testDAOUser_rose = authuserdao.NewUserFromAuthUser(testUser_rose)
	testDAOUser_john = authuserdao.NewUserFromAuthUser(testUser_john)
)

func copyRepo(repo *AuthUserRepo) *AuthUserRepo {
	copy := &AuthUserRepo{}
	if repo.users != nil {
		copy.users = make(map[uuid.UUID]authuserdao.User, len(repo.users))
		for k := range repo.users {
			copy.users[k] = repo.users[k]
		}
	}
	if repo.byUsernameIndex != nil {
		copy.byUsernameIndex = make(map[string]uuid.UUID, len(repo.byUsernameIndex))
		for k := range repo.byUsernameIndex {
			copy.byUsernameIndex[k] = repo.byUsernameIndex[k]
		}
	}

	return repo
}

func repoWithIndexedUsers(users ...authuserdao.User) *AuthUserRepo {
	db := &AuthUserRepo{
		users:           map[uuid.UUID]authuserdao.User{},
		byUsernameIndex: map[string]uuid.UUID{},
	}

	for _, u := range users {
		db.users[u.ID] = u
		db.byUsernameIndex[u.Username] = u.ID
	}

	return db
}

func Test_Get(t *testing.T) {
	testCases := []struct {
		name string
		db   *AuthUserRepo

		id               uuid.UUID
		expectUser       jelly.AuthUser
		expectErrToMatch []error
	}{
		{
			name: "happy path",
			db:   repoWithIndexedUsers(testDAOUser_dave),

			id:         testUser_dave.ID,
			expectUser: testUser_dave,
		},
		{
			name: "zero-valued DB",
			db:   &AuthUserRepo{},

			id: testUser_dave.ID,
			expectErrToMatch: []error{
				jelly.ErrDB,
				jelly.ErrNotFound,
			},
		},
		{
			name: "user is not in DB",
			db:   repoWithIndexedUsers(testDAOUser_jade),

			id: testUser_dave.ID,
			expectErrToMatch: []error{
				jelly.ErrDB,
				jelly.ErrNotFound,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			ctx := context.Background()

			// execute
			actual, err := tc.db.Get(ctx, tc.id)

			// assert

			if tc.expectErrToMatch == nil {
				if !assert.NoError(err) {
					return
				}
				assert.Equal(tc.expectUser, actual)
			} else {
				if !assert.Error(err) {
					return
				}
				if !assert.IsType(jelly.Error{}, err, "wrong type error") {
					return
				}

				for _, expectMatch := range tc.expectErrToMatch {
					assert.ErrorIs(err, expectMatch)
				}
			}
		})
	}
}

func Test_GetByUsername(t *testing.T) {
	testCases := []struct {
		name string
		db   *AuthUserRepo

		username         string
		expectUser       jelly.AuthUser
		expectErrToMatch []error
	}{
		{
			name: "happy path",
			db:   repoWithIndexedUsers(testDAOUser_dave),

			username:   testUser_dave.Username,
			expectUser: testUser_dave,
		},
		{
			name: "zero-valued DB",
			db:   &AuthUserRepo{},

			username: testUser_dave.Username,
			expectErrToMatch: []error{
				jelly.ErrDB,
				jelly.ErrNotFound,
			},
		},
		{
			name: "user is not in DB",
			db:   repoWithIndexedUsers(testDAOUser_jade),

			username: testUser_dave.Username,
			expectErrToMatch: []error{
				jelly.ErrDB,
				jelly.ErrNotFound,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			ctx := context.Background()

			// execute
			actual, err := tc.db.GetByUsername(ctx, tc.username)

			// assert

			if tc.expectErrToMatch == nil {
				if !assert.NoError(err) {
					return
				}
				assert.Equal(tc.expectUser, actual)
			} else {
				if !assert.Error(err) {
					return
				}
				if !assert.IsType(jelly.Error{}, err, "wrong type error") {
					return
				}

				for _, expectMatch := range tc.expectErrToMatch {
					assert.ErrorIs(err, expectMatch)
				}
			}
		})
	}
}

func Test_GetAll(t *testing.T) {
	testCases := []struct {
		name string
		db   *AuthUserRepo

		expectUsers      []jelly.AuthUser
		expectErrToMatch []error
	}{
		{
			name: "happy path",

			db: repoWithIndexedUsers(testDAOUser_dave, testDAOUser_jade, testDAOUser_john, testDAOUser_rose),

			expectUsers: []jelly.AuthUser{testUser_dave, testUser_jade, testUser_john, testUser_rose},
		},
		{
			name: "zero-valued DB",

			db: &AuthUserRepo{},

			expectUsers: []jelly.AuthUser{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			ctx := context.Background()

			// execute
			actual, err := tc.db.GetAll(ctx)

			// assert

			if tc.expectErrToMatch == nil {
				if !assert.NoError(err) {
					return
				}
				assert.ElementsMatch(tc.expectUsers, actual)
			} else {
				if !assert.Error(err) {
					return
				}
				if !assert.IsType(jelly.Error{}, err, "wrong type error") {
					return
				}

				for _, expectMatch := range tc.expectErrToMatch {
					assert.ErrorIs(err, expectMatch)
				}
			}
		})
	}
}

func Test_Create(t *testing.T) {

	testCases := []struct {
		name string
		db   *AuthUserRepo

		user jelly.AuthUser

		expectErrToMatch []error
	}{
		{
			name: "normal create",
			db:   repoWithIndexedUsers(testDAOUser_dave),

			user: testUser_rose,
		},
		{
			name: "empty DB",
			db:   &AuthUserRepo{},

			user: testUser_rose,
		},
		{
			name: "conflict via username is rejected",
			db:   repoWithIndexedUsers(testDAOUser_dave),

			user: testUser_rose.WithUsername(testUser_dave.Username),

			expectErrToMatch: []error{jelly.ErrDB, jelly.ErrConstraintViolation},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			ctx := context.Background()

			originalDB := copyRepo(tc.db)

			actual, err := tc.db.Create(ctx, tc.user)

			if tc.expectErrToMatch == nil {
				if !assert.NoError(err) {
					return
				}

				// some properties are under control of DAO and thus the
				// inserted user must not be what the user set

				// caller may set these properties on creation:
				assert.Equal(tc.user.Username, actual.Username, "usernames do not match")
				assert.Equal(tc.user.Password, actual.Password, "passwords do not match") // DAO does not currently handle encryption
				assert.Equal(tc.user.Email, actual.Email, "emails do not match")
				assert.Equal(tc.user.Role, actual.Role, "roles do not match")

				// caller may not set any of these on creation; they are automatically set
				assert.NotEqual(tc.user.ID, actual.ID, "ID was not automatically generated")
				assert.Less(tc.user.Created, actual.Created, "created time was not automatically updated")
				assert.Less(tc.user.Modified, actual.Modified, "modified time was not automatically updated")
				assert.Less(tc.user.LastLogout, actual.LastLogout, "last logout time was not automatically updated")
				assert.Less(tc.user.LastLogin, actual.LastLogin, "last login time was not automatically updated")

				// check that the DB now has the new one added:
				assert.Contains(tc.db.byUsernameIndex, tc.user.Username)
				assert.Contains(tc.db.users, tc.db.byUsernameIndex[tc.user.Username])
			} else {
				if !assert.Error(err) {
					return
				}
				if !assert.IsType(jelly.Error{}, err, "wrong type error") {
					return
				}

				for _, expectMatch := range tc.expectErrToMatch {
					assert.ErrorIs(err, expectMatch)
				}

				// check that the DB did not change:
				assert.Equal(originalDB, tc.db, "error returned, but DB was still mutated")
			}

		})
	}
}

func Test_Update(t *testing.T) {
	testCases := []struct {
		name string
		db   *AuthUserRepo

		id   uuid.UUID
		user jelly.AuthUser

		expectUser       jelly.AuthUser
		expectErrToMatch []error
	}{
		{
			name: "update normally - only email and role",
			db:   repoWithIndexedUsers(testDAOUser_rose),

			id: testUser_rose.ID,
			user: testUser_rose.
				WithEmail("rose@lolar.com").
				WithRole(jelly.Admin),

			expectUser: testUser_rose.
				WithEmail("rose@lolar.com").
				WithRole(jelly.Admin),
		},
		{
			name: "update normally - change ID",
			db:   repoWithIndexedUsers(testDAOUser_rose),

			id: testUser_rose.ID,
			user: testUser_rose.
				WithID(uuid.MustParse("a42f0aa9-d87c-4a08-81cf-6b2db251d09b")),

			expectUser: testUser_rose.
				WithID(uuid.MustParse("a42f0aa9-d87c-4a08-81cf-6b2db251d09b")),
		},
		{
			name: "update normally - change username",
			db:   repoWithIndexedUsers(testDAOUser_rose),

			id: testUser_rose.ID,
			user: testUser_rose.
				WithUsername("grimdarkRose"),

			expectUser: testUser_rose.
				WithUsername("grimdarkRose"),
		},
		{
			name: "update empty DB",
			db:   &AuthUserRepo{},

			id: testUser_rose.ID,
			user: testUser_rose.
				WithUsername("grimdarkRose"),

			expectErrToMatch: []error{jelly.ErrDB, jelly.ErrNotFound},
		},
		{
			name: "update user that does not exist in DB",
			db:   repoWithIndexedUsers(testDAOUser_dave),

			id: testUser_rose.ID,
			user: testUser_rose.
				WithUsername("grimdarkRose"),

			expectErrToMatch: []error{jelly.ErrDB, jelly.ErrNotFound},
		},
		{
			name: "update to conflicting username",
			db:   repoWithIndexedUsers(testDAOUser_dave, testDAOUser_rose),

			id: testUser_rose.ID,
			user: testUser_rose.
				WithUsername(testUser_dave.Username),

			expectErrToMatch: []error{jelly.ErrDB, jelly.ErrConstraintViolation},
		},
		{
			name: "update to conflicting ID",
			db:   repoWithIndexedUsers(testDAOUser_dave, testDAOUser_rose),

			id: testUser_rose.ID,
			user: testUser_rose.
				WithID(testUser_dave.ID),

			expectErrToMatch: []error{jelly.ErrDB, jelly.ErrConstraintViolation},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			ctx := context.Background()

			originalDB := copyRepo(tc.db)

			var changedCreated bool
			if origUser, ok := tc.db.users[tc.id]; ok {
				changedCreated = !origUser.Created.Time().Equal(tc.user.Created)
			}

			actual, err := tc.db.Update(ctx, tc.id, tc.user)

			if tc.expectErrToMatch == nil {
				if !assert.NoError(err) {
					return
				}

				// some properties are under control of DAO and thus the
				// inserted user must not be what the user set

				// caller may set these properties on creation:
				assert.Equal(tc.user.ID, actual.ID, "IDs do not match")
				assert.Equal(tc.user.Username, actual.Username, "usernames do not match")
				assert.Equal(tc.user.Password, actual.Password, "passwords do not match") // DAO does not currently handle encryption
				assert.Equal(tc.user.Email, actual.Email, "emails do not match")
				assert.Equal(tc.user.Role, actual.Role, "roles do not match")
				assert.Equal(tc.user.LastLogout, actual.LastLogout, "last logout times do not match")
				assert.Equal(tc.user.LastLogin, actual.LastLogin, "last login times do not match")

				// caller may not set any of these on update; they are automatically set
				assert.Less(tc.user.Modified, actual.Modified, "modified time was not automatically updated")

				// caller may never update this
				if changedCreated {
					assert.NotEqual(tc.user.Created, actual.Created, "created time was unexpectedly updated")
				}

				// make sure the returned user the stored one:
				assert.Contains(tc.db.byUsernameIndex, actual.Username)
				assert.Contains(tc.db.users, actual.ID)
				assert.Equal(authuserdao.NewUserFromAuthUser(actual), tc.db.users[actual.ID], "stored user is not returned one")
			} else {
				if !assert.Error(err) {
					return
				}
				if !assert.IsType(jelly.Error{}, err, "wrong type error") {
					return
				}

				for _, expectMatch := range tc.expectErrToMatch {
					assert.ErrorIs(err, expectMatch)
				}

				// check that the DB did not change:
				assert.Equal(originalDB, tc.db, "error returned, but DB was still mutated")
			}
		})
	}
}

// func Test_Delete(t *testing.T) {
// 	testCases := []struct {
// 		name string

// 		id uuid.UUID

// 		getQueryReturnsUser  jelly.AuthUser
// 		getQueryReturnsError error

// 		deleteQueryReturnsError        error
// 		deleteQueryReturnsRowsAffected int64
// 		deleteQueryIsSkipped           bool

// 		expectUser       jelly.AuthUser
// 		expectErrToMatch []error
// 	}{
// 		{
// 			name: "normal delete",
// 			id:   testUser_rose.ID,

// 			getQueryReturnsUser:            testUser_rose,
// 			deleteQueryReturnsRowsAffected: 1,

// 			expectUser: testUser_rose,
// 		},
// 		{
// 			name: "get query returns generic error",
// 			id:   testUser_rose.ID,

// 			getQueryReturnsError: errors.New("error"),
// 			deleteQueryIsSkipped: true,

// 			expectErrToMatch: []error{jelly.ErrDB},
// 		},
// 		{
// 			name: "get query returns not-found",
// 			id:   testUser_rose.ID,

// 			getQueryReturnsError: sql.ErrNoRows,
// 			deleteQueryIsSkipped: true,

// 			expectErrToMatch: []error{jelly.ErrDB, jelly.ErrNotFound},
// 		},
// 		{
// 			name: "delete query returns generic error",
// 			id:   testUser_rose.ID,

// 			getQueryReturnsUser:     testUser_rose,
// 			deleteQueryReturnsError: errors.New("bad error"),

// 			expectErrToMatch: []error{jelly.ErrDB},
// 		},
// 		{
// 			name: "delete query returns not-found",
// 			id:   testUser_rose.ID,

// 			getQueryReturnsUser:     testUser_rose,
// 			deleteQueryReturnsError: jelly.ErrNotFound,

// 			expectErrToMatch: []error{jelly.ErrDB, jelly.ErrNotFound},
// 		},
// 		{
// 			name: "delete query returns no rows affected",
// 			id:   testUser_rose.ID,

// 			getQueryReturnsUser:            testUser_rose,
// 			deleteQueryReturnsRowsAffected: 0,

// 			expectErrToMatch: []error{jelly.ErrDB, jelly.ErrNotFound},
// 		},
// 	}

// 	for _, tc := range testCases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			assert := assert.New(t)

// 			driver, dbMock, err := sqlmock.New()
// 			if !assert.NoError(err) {
// 				return
// 			}

// 			db := AuthUsersDB{DB: driver}
// 			ctx := context.Background()

// 			// mock setup
// 			if tc.getQueryReturnsError != nil {
// 				dbMock.
// 					ExpectQuery("SELECT .* FROM users").
// 					WillReturnError(tc.getQueryReturnsError)
// 			} else {
// 				stored := tc.getQueryReturnsUser
// 				dbMock.
// 					ExpectQuery("SELECT .* FROM users").
// 					WillReturnRows(sqlmock.NewRows([]string{
// 						"username",
// 						"password",
// 						"role",
// 						"email",
// 						"created",
// 						"modified",
// 						"last_logout_time",
// 						"last_login_time",
// 					}).AddRow(
// 						stored.Username,
// 						stored.Password,
// 						int64(stored.Role),
// 						stored.Email,
// 						stored.Created.Unix(),
// 						stored.Modified.Unix(),
// 						stored.LastLogout.Unix(),
// 						stored.LastLogin.Unix(),
// 					))

// 				if tc.deleteQueryReturnsError != nil {
// 					dbMock.
// 						ExpectExec("DELETE FROM users").
// 						WillReturnError(tc.deleteQueryReturnsError)
// 				} else if !tc.deleteQueryIsSkipped {
// 					dbMock.
// 						ExpectExec("DELETE FROM users").
// 						WithArgs(tc.id).
// 						WillReturnResult(sqlmock.NewResult(0, tc.deleteQueryReturnsRowsAffected))
// 				}

// 				// execute
// 				actual, err := db.Delete(ctx, tc.id)

// 				// assert
// 				if tc.expectErrToMatch == nil {
// 					if !assert.NoError(err) {
// 						return
// 					}
// 					assert.Equal(tc.expectUser, actual)
// 				} else {
// 					if !assert.Error(err) {
// 						return
// 					}
// 					if !assert.IsType(jelly.Error{}, err, "wrong type error") {
// 						return
// 					}

// 					for _, expectMatch := range tc.expectErrToMatch {
// 						assert.ErrorIs(err, expectMatch)
// 					}
// 				}

// 				assert.NoError(dbMock.ExpectationsWereMet())
// 			}
// 		})
// 	}
// }
