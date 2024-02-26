package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/dekarrin/jelly"
	jeldb "github.com/dekarrin/jelly/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func Test_Create(t *testing.T) {
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

	insertedTime := time.Date(2024, 2, 2, 2, 3, 12, 0, time.UTC)

	mockSelectRows := sqlmock.NewRows([]string{
		"username", "password", "role", "email", "created", "modified", "last_logout_time", "last_login_time",
	}).AddRow(
		input.Username, input.Password, input.Role.String(), input.Email, insertedTime, insertedTime, insertedTime, 0,
	)

	dbMock.
		ExpectPrepare("INSERT INTO users").
		ExpectExec().
		WithArgs(
			jeldb.AnyTime{},
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
		WillReturnRows(mockSelectRows)

	actual, err := db.Create(ctx, input)

	if !assert.NoError(err) {
		return
	}
	assert.NotEqual(input.ID, actual.ID, "ID was not automatically generated")
	assert.Equal(input.Username, actual.Username, "usernames do not match")
	assert.Equal(input.Password, actual.Password, "passwords do not match") // DAO does NOT currently handle encryption
	assert.Equal(input.Email, actual.Email, "emails do not match")
	assert.Equal(input.Role, actual.Role, "roles do not match")
	assert.NotEqual(input.Created, actual.Created, "created time was not automatically updated")
	assert.NotEqual(input.Modified, actual.Modified, "modified time was not automatically updated")
	assert.Equal(input.LastLogin, actual.LastLogin, "last login times do not match")
	assert.Equal(input.LastLogout, actual.LastLogout, "last logout times do not match")

	assert.NoError(dbMock.ExpectationsWereMet())
}
