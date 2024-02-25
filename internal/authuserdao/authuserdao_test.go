package authuserdao

import (
	"net/mail"
	"testing"
	"time"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func Test_User_AuthUser(t *testing.T) {
	assert := assert.New(t)

	uEmail, _ := mail.ParseAddress("test@example.com")
	createdTime := time.Date(2024, 2, 2, 2, 3, 12, 0, time.UTC)
	modifiedTime := createdTime.Add(1 * time.Hour)
	loginTime := createdTime.Add(4 * time.Hour)
	logoutTime := createdTime.Add(5 * time.Hour)

	user := User{
		ID:         uuid.MustParse("284968fa-1ec3-4d69-9a89-a6bbe60d2883"),
		Username:   "test",
		Password:   "(encrypted)",
		Email:      db.Email{V: uEmail},
		Role:       jelly.Guest,
		Created:    db.Timestamp(createdTime),
		Modified:   db.Timestamp(modifiedTime),
		LastLogin:  db.Timestamp(loginTime),
		LastLogout: db.Timestamp(logoutTime),
	}

	authUser := user.AuthUser()

	assert.Equal(user.ID, authUser.ID, "IDs do not match")
	assert.Equal(user.Username, authUser.Username, "Usernames do not match")
	assert.Equal(user.Password, authUser.Password, "Passwords do not match")
	assert.Equal(uEmail.Address, authUser.Email, "Emails do not match")
	assert.Equal(user.Role, authUser.Role, "Roles do not match")
	assert.Equal(createdTime, authUser.Created, "Created times do not match")
	assert.Equal(modifiedTime, authUser.Modified, "Modified times do not match")
	assert.Equal(loginTime, authUser.LastLogin, "Last login times do not match")
	assert.Equal(logoutTime, authUser.LastLogout, "Last logout times do not match")
}

func Test_NewUserFromAuthUser(t *testing.T) {
	assert := assert.New(t)

	createdTime := time.Date(2024, 2, 2, 2, 3, 12, 0, time.UTC)
	modifiedTime := createdTime.Add(1 * time.Hour)
	loginTime := createdTime.Add(4 * time.Hour)
	logoutTime := createdTime.Add(5 * time.Hour)

	authUser := jelly.AuthUser{
		ID:         uuid.MustParse("284968fa-1ec3-4d69-9a89-a6bbe60d2883"),
		Username:   "test",
		Password:   "(encrypted)",
		Email:      "test@example.com",
		Role:       jelly.Guest,
		Created:    createdTime,
		Modified:   modifiedTime,
		LastLogin:  loginTime,
		LastLogout: logoutTime,
	}

	user := NewUserFromAuthUser(authUser)

	assert.Equal(authUser.ID, user.ID, "IDs do not match")
	assert.Equal(authUser.Username, user.Username, "Usernames do not match")
	assert.Equal(authUser.Password, user.Password, "Passwords do not match")
	assert.Equal(authUser.Email, user.Email.String(), "Emails do not match")
	assert.Equal(authUser.Role, user.Role, "Roles do not match")
	assert.Equal(createdTime, user.Created.Time(), "Created times do not match")
	assert.Equal(modifiedTime, user.Modified.Time(), "Modified times do not match")
	assert.Equal(loginTime, user.LastLogin.Time(), "Last login times do not match")
	assert.Equal(logoutTime, user.LastLogout.Time(), "Last logout times do not match")
}
