package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/dekarrin/jelly"
	jeldb "github.com/dekarrin/jelly/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func Test_Get(t *testing.T) {
	testCases := []struct {
		name              string
		id                uuid.UUID
		queryReturnsUser  jelly.AuthUser
		queryReturnsError error
		expectUser        jelly.AuthUser
		expectErrToMatch  []error
	}{
		{
			name: "happy path",
			id:   uuid.MustParse("82779fe7-d681-427d-a011-4954b6a7ec01"),
			queryReturnsUser: jelly.AuthUser{
				ID:       uuid.MustParse("82779fe7-d681-427d-a011-4954b6a7ec01"),
				Username: "turntechGodhead",
				Email:    "dave@morethanpuppets.com",
			},
			expectUser: jelly.AuthUser{
				ID:       uuid.MustParse("82779fe7-d681-427d-a011-4954b6a7ec01"),
				Username: "turntechGodhead",
				Email:    "dave@morethanpuppets.com",
			},
		},
		{
			name:              "not found error raised",
			id:                uuid.MustParse("82779fe7-d681-427d-a011-4954b6a7ec01"),
			queryReturnsError: sql.ErrNoRows,
			expectErrToMatch: []error{
				jelly.ErrDB,
				jelly.ErrNotFound,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			driver, dbMock, err := sqlmock.New()
			if !assert.NoError(err) {
				return
			}

			db := AuthUsersDB{DB: driver}
			ctx := context.Background()

			if tc.queryReturnsError != nil {
				dbMock.
					ExpectQuery("SELECT .* FROM users").
					WillReturnError(tc.queryReturnsError)
			} else {
				stored := tc.queryReturnsUser
				dbMock.
					ExpectQuery("SELECT .* FROM users").
					WillReturnRows(sqlmock.NewRows([]string{
						"username",
						"password",
						"role",
						"email",
						"created",
						"modified",
						"last_logout_time",
						"last_login_time",
					}).AddRow(
						stored.Username,
						stored.Password,
						int64(stored.Role),
						stored.Email,
						stored.Created.Unix(),
						stored.Modified.Unix(),
						stored.LastLogout.Unix(),
						stored.LastLogin.Unix(),
					))
			}

			// execute

			actual, err := db.Get(ctx, tc.id)

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

			assert.NoError(dbMock.ExpectationsWereMet())
		})
	}
}

func Test_Create(t *testing.T) {
	t.Run("successful creation - email set", func(t *testing.T) {
		assert := assert.New(t)

		driver, dbMock, err := sqlmock.New()
		if !assert.NoError(err) {
			return
		}

		db := AuthUsersDB{DB: driver}
		ctx := context.Background()

		createdTime := time.Date(2024, 2, 2, 2, 3, 12, 0, time.UTC)
		modifiedTime := createdTime.Add(1 * time.Hour)
		loginTime := createdTime.Add(4 * time.Hour)
		logoutTime := createdTime.Add(5 * time.Hour)

		input := jelly.AuthUser{
			ID:         uuid.MustParse("284968fa-1ec3-4d69-9a89-a6bbe60d2883"),
			Username:   "test",
			Password:   "(encrypted)",
			Email:      "test@example.com",
			Role:       jelly.Unverified,
			Created:    createdTime,
			Modified:   modifiedTime,
			LastLogin:  loginTime,
			LastLogout: logoutTime,
		}

		insertedTime := time.Date(2024, 2, 20, 2, 3, 12, 0, time.UTC)

		dbMock.
			ExpectPrepare("INSERT INTO users").
			ExpectExec().
			WithArgs(
				jeldb.AnyUUID{},
				input.Username,
				input.Password,
				input.Role,
				input.Email,
				jeldb.AnyTime{After: &createdTime},
				jeldb.AnyTime{After: &modifiedTime},
				jeldb.AnyTime{After: &logoutTime},
				jeldb.AnyTime{},
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		dbMock.
			ExpectQuery("SELECT .* FROM users").
			WillReturnRows(sqlmock.NewRows([]string{
				"username", "password", "role", "email", "created", "modified", "last_logout_time", "last_login_time",
			}).AddRow(
				input.Username,
				input.Password,
				int64(input.Role),
				input.Email,
				insertedTime.Unix(),
				insertedTime.Unix(),
				insertedTime.Unix(),
				jeldb.NowTimestamp().Time().Unix(),
			))

		actual, err := db.Create(ctx, input)

		if !assert.NoError(err) {
			return
		}

		// caller may set these properties on creation
		assert.Equal(input.Username, actual.Username, "usernames do not match")
		assert.Equal(input.Password, actual.Password, "passwords do not match") // DAO does not currently handle encryption
		assert.Equal(input.Email, actual.Email, "emails do not match")
		assert.Equal(input.Role, actual.Role, "roles do not match")

		// caller may not set any of these on creation; they are automatically set
		assert.NotEqual(input.ID, actual.ID, "ID was not automatically generated")
		assert.NotEqual(input.Created, actual.Created, "created time was not automatically updated")
		assert.NotEqual(input.Modified, actual.Modified, "modified time was not automatically updated")
		assert.NotEqual(input.LastLogout, actual.LastLogout, "last logout times do not match")
		assert.NotEqual(input.LastLogin, actual.LastLogin, "last login times do not match")

		assert.NoError(dbMock.ExpectationsWereMet())
	})

	t.Run("successful creation - email not set", func(t *testing.T) {
		assert := assert.New(t)

		driver, dbMock, err := sqlmock.New()
		if !assert.NoError(err) {
			return
		}

		db := AuthUsersDB{DB: driver}
		ctx := context.Background()

		createdTime := time.Date(2024, 2, 2, 2, 3, 12, 0, time.UTC)
		modifiedTime := createdTime.Add(1 * time.Hour)
		loginTime := createdTime.Add(4 * time.Hour)
		logoutTime := createdTime.Add(5 * time.Hour)

		input := jelly.AuthUser{
			ID:         uuid.MustParse("284968fa-1ec3-4d69-9a89-a6bbe60d2883"),
			Username:   "test",
			Password:   "(encrypted)",
			Email:      "",
			Role:       jelly.Unverified,
			Created:    createdTime,
			Modified:   modifiedTime,
			LastLogin:  loginTime,
			LastLogout: logoutTime,
		}

		insertedTime := time.Date(2024, 2, 20, 2, 3, 12, 0, time.UTC)

		dbMock.
			ExpectPrepare("INSERT INTO users").
			ExpectExec().
			WithArgs(
				jeldb.AnyUUID{},
				input.Username,
				input.Password,
				input.Role,
				input.Email,
				jeldb.AnyTime{After: &createdTime},
				jeldb.AnyTime{After: &modifiedTime},
				jeldb.AnyTime{After: &logoutTime},
				jeldb.AnyTime{},
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		dbMock.
			ExpectQuery("SELECT .* FROM users").
			WillReturnRows(sqlmock.NewRows([]string{
				"username", "password", "role", "email", "created", "modified", "last_logout_time", "last_login_time",
			}).AddRow(
				input.Username,
				input.Password,
				int64(input.Role),
				input.Email,
				insertedTime.Unix(),
				insertedTime.Unix(),
				insertedTime.Unix(),
				jeldb.NowTimestamp().Time().Unix(),
			))

		actual, err := db.Create(ctx, input)

		if !assert.NoError(err) {
			return
		}

		// caller may set these properties on creation
		assert.Equal(input.Username, actual.Username, "usernames do not match")
		assert.Equal(input.Password, actual.Password, "passwords do not match") // DAO does not currently handle encryption
		assert.Equal(input.Email, actual.Email, "emails do not match")
		assert.Equal(input.Role, actual.Role, "roles do not match")

		// caller may not set any of these on creation; they are automatically set
		assert.NotEqual(input.ID, actual.ID, "ID was not automatically generated")
		assert.NotEqual(input.Created, actual.Created, "created time was not automatically updated")
		assert.NotEqual(input.Modified, actual.Modified, "modified time was not automatically updated")
		assert.NotEqual(input.LastLogout, actual.LastLogout, "last logout times do not match")
		assert.NotEqual(input.LastLogin, actual.LastLogin, "last login times do not match")

		assert.NoError(dbMock.ExpectationsWereMet())
	})

	t.Run("error on insert query's prepare - is propagated and is ErrDB", func(t *testing.T) {
		assert := assert.New(t)

		createdTime := time.Date(2024, 2, 2, 2, 3, 12, 0, time.UTC)
		modifiedTime := createdTime.Add(1 * time.Hour)
		loginTime := createdTime.Add(4 * time.Hour)
		logoutTime := createdTime.Add(5 * time.Hour)

		input := jelly.AuthUser{
			ID:         uuid.MustParse("284968fa-1ec3-4d69-9a89-a6bbe60d2883"),
			Username:   "test",
			Password:   "(encrypted)",
			Email:      "",
			Role:       jelly.Unverified,
			Created:    createdTime,
			Modified:   modifiedTime,
			LastLogin:  loginTime,
			LastLogout: logoutTime,
		}

		driver, dbMock, err := sqlmock.New()
		if !assert.NoError(err) {
			return
		}

		db := AuthUsersDB{DB: driver}
		ctx := context.Background()

		dbMock.
			ExpectPrepare("INSERT INTO users").
			WillReturnError(errors.New("prepare failed"))

		_, err = db.Create(ctx, input)

		assert.IsType(jelly.Error{}, err, "err not of type jelly.Error{}")
		assert.ErrorIs(err, jelly.ErrDB, "err not of type DB")
		assert.ErrorContains(err, "prepare failed")
		assert.NoError(dbMock.ExpectationsWereMet())
	})

	t.Run("error on insert query - is propagated and is ErrDB", func(t *testing.T) {
		assert := assert.New(t)

		createdTime := time.Date(2024, 2, 2, 2, 3, 12, 0, time.UTC)
		modifiedTime := createdTime.Add(1 * time.Hour)
		loginTime := createdTime.Add(4 * time.Hour)
		logoutTime := createdTime.Add(5 * time.Hour)

		input := jelly.AuthUser{
			ID:         uuid.MustParse("284968fa-1ec3-4d69-9a89-a6bbe60d2883"),
			Username:   "test",
			Password:   "(encrypted)",
			Email:      "",
			Role:       jelly.Unverified,
			Created:    createdTime,
			Modified:   modifiedTime,
			LastLogin:  loginTime,
			LastLogout: logoutTime,
		}

		driver, dbMock, err := sqlmock.New()
		if !assert.NoError(err) {
			return
		}

		db := AuthUsersDB{DB: driver}
		ctx := context.Background()

		dbMock.
			ExpectPrepare("INSERT INTO users").
			ExpectExec().
			WillReturnError(errors.New("query failed"))

		_, err = db.Create(ctx, input)

		assert.IsType(jelly.Error{}, err, "err not of type jelly.Error{}")
		assert.ErrorIs(err, jelly.ErrDB, "err not of type DB")
		assert.ErrorContains(err, "query failed")
		assert.NoError(dbMock.ExpectationsWereMet())
	})

	t.Run("error on get query - is propagated and is correct types", func(t *testing.T) {
		assert := assert.New(t)

		createdTime := time.Date(2024, 2, 2, 2, 3, 12, 0, time.UTC)
		modifiedTime := createdTime.Add(1 * time.Hour)
		loginTime := createdTime.Add(4 * time.Hour)
		logoutTime := createdTime.Add(5 * time.Hour)

		input := jelly.AuthUser{
			ID:         uuid.MustParse("284968fa-1ec3-4d69-9a89-a6bbe60d2883"),
			Username:   "test",
			Password:   "(encrypted)",
			Email:      "",
			Role:       jelly.Unverified,
			Created:    createdTime,
			Modified:   modifiedTime,
			LastLogin:  loginTime,
			LastLogout: logoutTime,
		}

		driver, dbMock, err := sqlmock.New()
		if !assert.NoError(err) {
			return
		}

		db := AuthUsersDB{DB: driver}
		ctx := context.Background()

		dbMock.
			ExpectPrepare("INSERT INTO users").
			ExpectExec().
			WithArgs(
				jeldb.AnyUUID{},
				input.Username,
				input.Password,
				input.Role,
				input.Email,
				jeldb.AnyTime{After: &createdTime},
				jeldb.AnyTime{After: &modifiedTime},
				jeldb.AnyTime{After: &logoutTime},
				jeldb.AnyTime{},
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		dbMock.
			ExpectQuery("SELECT .* FROM users").
			WillReturnError(errors.New("get failed"))

		_, err = db.Create(ctx, input)

		assert.IsType(jelly.Error{}, err, "err not of type jelly.Error{}")
		assert.ErrorIs(err, jelly.ErrDB, "err not of type DB")
		assert.ErrorContains(err, "get failed")
		assert.NoError(dbMock.ExpectationsWereMet())
	})

}
