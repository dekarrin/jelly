package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/config"
	"github.com/dekarrin/jelly/dao"
	"github.com/dekarrin/jelly/middle"
	"github.com/dekarrin/jelly/response"
	"github.com/dekarrin/jelly/serr"
	"github.com/dekarrin/jelly/token"
)

// LoginAPI holds endpoint frontend for the login service.
type LoginAPI struct {
	// Service is the service that the API calls to perform the requested
	// actions.
	Service LoginService

	// UnauthDelay is the amount of time that a request will pause before
	// responding with an HTTP-403, HTTP-401, or HTTP-500 to deprioritize such
	// requests from processing and I/O.
	UnauthDelay time.Duration

	// Secret is the secret used to sign JWT tokens.
	Secret []byte

	pathPrefix string

	// the name this API is configured under, used to find the name of own
	// auth provider
	name string
}

func (api *LoginAPI) Init(cb config.Bundle, dbs map[string]dao.Store) error {
	api.name = cb.Name()
	api.Secret = cb.GetByteSlice(ConfigKeySecret)
	api.UnauthDelay = cb.ServerUnauthDelay()
	authRaw := dbs["auth"]
	authStore, ok := authRaw.(dao.AuthUserStore)
	if !ok {
		return fmt.Errorf("DB provided under 'auth' does not implement jeldao.AuthUserStore")
	}
	api.Service = LoginService{
		Provider: authStore,
	}
	api.pathPrefix = cb.Base()

	return nil
}

func (api *LoginAPI) Authenticators() map[string]middle.Authenticator {
	// this provides one and only one authenticator, the jwt one.

	// we will have had Init called, ergo secret and the service db will exist
	return map[string]middle.Authenticator{
		"jwt": JWTAuthProvider{secret: api.Secret, db: api.Service.Provider.AuthUsers()},
	}
}

// Shutdown shuts down the login API. This is added to implement jelapi.API, and
// has no effect on the login API but to return the error of the context.
func (api *LoginAPI) Shutdown(ctx context.Context) error {
	return ctx.Err()
}

// HTTPGetInfo returns a HandlerFunc that retrieves information on the API and
// server.
//
// The handler has requirements for the request context it receives, and if the
// requirements are not met it may return an HTTP-500. The context must contain
// a value denoting whether the client making the request is logged-in.
func (api LoginAPI) HTTPGetInfo() http.HandlerFunc {
	return jelly.Endpoint(api.UnauthDelay, api.epGetInfo)
}

func (api LoginAPI) epGetInfo(req *http.Request) response.Result {
	loggedIn := req.Context().Value(middle.AuthLoggedIn).(bool)

	var resp InfoModel
	resp.Version.Auth = Version

	userStr := "unauthed client"
	if loggedIn {
		user := req.Context().Value(middle.AuthUser).(dao.User)
		userStr = "user '" + user.Username + "'"
	}
	return response.OK(resp, "%s got API info", userStr)
}

// HTTPCreateLogin returns a HandlerFunc that uses the API to log in a user with
// a username and password and return the auth token for that user.
func (api LoginAPI) HTTPCreateLogin() http.HandlerFunc {
	return jelly.Endpoint(api.UnauthDelay, api.epCreateLogin)
}

func (api LoginAPI) epCreateLogin(req *http.Request) response.Result {
	loginData := LoginRequest{}
	err := jelly.ParseJSONRequest(req, &loginData)
	if err != nil {
		return response.BadRequest(err.Error(), err.Error())
	}

	if loginData.Username == "" {
		return response.BadRequest("username: property is empty or missing from request", "empty username")
	}
	if loginData.Password == "" {
		return response.BadRequest("password: property is empty or missing from request", "empty password")
	}

	user, err := api.Service.Login(req.Context(), loginData.Username, loginData.Password)
	if err != nil {
		if errors.Is(err, serr.ErrBadCredentials) {
			return response.Unauthorized(serr.ErrBadCredentials.Error(), "user '%s': %s", loginData.Username, err.Error())
		} else {
			return response.InternalServerError(err.Error())
		}
	}

	// build the token
	// password is valid, generate token for user and return it.
	tok, err := token.Generate(api.Secret, user)
	if err != nil {
		return response.InternalServerError("could not generate JWT: " + err.Error())
	}

	resp := LoginResponse{
		Token:  tok,
		UserID: user.ID.String(),
	}
	return response.Created(resp, "user '"+user.Username+"' successfully logged in")
}

// HTTPDeleteLogin returns a HandlerFunc that deletes active login for some
// user. Only admin users can delete logins for users other themselves.
//
// The handler has requirements for the request context it receives, and if the
// requirements are not met it may return an HTTP-500. The context must contain
// the ID of the user to log out and the logged-in user of the client making the
// request.
func (api LoginAPI) HTTPDeleteLogin() http.HandlerFunc {
	return jelly.Endpoint(api.UnauthDelay, api.epDeleteLogin)
}

func (api LoginAPI) epDeleteLogin(req *http.Request) response.Result {
	id := jelly.RequireIDParam(req)
	user := req.Context().Value(middle.AuthUser).(dao.User)

	// is the user trying to delete someone else's login? they'd betta be the
	// admin if so!
	if id != user.ID && user.Role != dao.Admin {
		var otherUserStr string
		otherUser, err := api.Service.GetUser(req.Context(), id.String())
		// if there was another user, find out now
		if err != nil {
			if !errors.Is(err, serr.ErrNotFound) {
				return response.InternalServerError("retrieve user for perm checking: %s", err.Error())
			}
			otherUserStr = fmt.Sprintf("%d", id)
		} else {
			otherUserStr = "'" + otherUser.Username + "'"
		}

		return response.Forbidden("user '%s' (role %s) logout of user %s: forbidden", user.Username, user.Role, otherUserStr)
	}

	loggedOutUser, err := api.Service.Logout(req.Context(), id)
	if err != nil {
		if errors.Is(err, serr.ErrNotFound) {
			return response.NotFound()
		}
		return response.InternalServerError("could not log out user: " + err.Error())
	}

	var otherStr string
	if id != user.ID {
		otherStr = "user '" + loggedOutUser.Username + "'"
	} else {
		otherStr = "self"
	}

	return response.NoContent("user '%s' successfully logged out %s", user.Username, otherStr)
}

// HTTPCreateToken returns a HandlerFunc that creates a new token for the user
// the client is logged in as.
//
// The handler has requirements for the request context it receives, and if the
// requirements are not met it may return an HTTP-500. The context must contain
// the logged-in user of the client making the request.
func (api LoginAPI) HTTPCreateToken() http.HandlerFunc {
	return jelly.Endpoint(api.UnauthDelay, api.epCreateToken)
}

func (api LoginAPI) epCreateToken(req *http.Request) response.Result {
	user := req.Context().Value(middle.AuthUser).(dao.User)

	tok, err := token.Generate(api.Secret, user)
	if err != nil {
		return response.InternalServerError("could not generate JWT: " + err.Error())
	}

	resp := LoginResponse{
		Token:  tok,
		UserID: user.ID.String(),
	}
	return response.Created(resp, "user '"+user.Username+"' successfully created new token")
}

// HTTPGetAllUsers returns a HandlerFunc that retrieves all existing users. Only
// an admin user can call this endpoint.
//
// The handler has requirements for the request context it receives, and if the
// requirements are not met it may return an HTTP-500. The context must contain
// the logged-in user of the client making the request.
func (api LoginAPI) HTTPGetAllUsers() http.HandlerFunc {
	return jelly.Endpoint(api.UnauthDelay, api.epGetAllUsers)
}

// GET /users: get all users (admin auth required).
func (api LoginAPI) epGetAllUsers(req *http.Request) response.Result {
	user := req.Context().Value(middle.AuthUser).(dao.User)

	if user.Role != dao.Admin {
		return response.Forbidden("user '%s' (role %s): forbidden", user.Username, user.Role)
	}

	users, err := api.Service.GetAllUsers(req.Context())
	if err != nil {
		return response.InternalServerError(err.Error())
	}

	resp := make([]UserModel, len(users))

	for i := range users {
		resp[i] = UserModel{
			URI:            api.pathPrefix + "/users/" + users[i].ID.String(),
			ID:             users[i].ID.String(),
			Username:       users[i].Username,
			Role:           users[i].Role.String(),
			Created:        users[i].Created.Format(time.RFC3339),
			Modified:       users[i].Modified.Format(time.RFC3339),
			LastLogoutTime: users[i].LastLogoutTime.Format(time.RFC3339),
			LastLoginTime:  users[i].LastLoginTime.Format(time.RFC3339),
		}
		if users[i].Email != nil {
			resp[i].Email = users[i].Email.Address
		}
	}

	return response.OK(resp, "user '%s' got all users", user.Username)
}

// HTTPCreateUser returns a HandlerFunc that creates a new user entity. Only an
// admin user can directly create new users.
//
// The handler has requirements for the request context it receives, and if the
// requirements are not met it may return an HTTP-500. The context must contain
// the logged-in user of the client making the request.
func (api LoginAPI) HTTPCreateUser() http.HandlerFunc {
	return jelly.Endpoint(api.UnauthDelay, api.epCreateUser)
}

func (api LoginAPI) epCreateUser(req *http.Request) response.Result {
	user := req.Context().Value(middle.AuthUser).(dao.User)

	if user.Role != dao.Admin {
		return response.Forbidden("user '%s' (role %s) creation of new user: forbidden", user.Username, user.Role)
	}

	var createUser UserModel
	err := jelly.ParseJSONRequest(req, &createUser)
	if err != nil {
		return response.BadRequest(err.Error(), err.Error())
	}
	if createUser.Username == "" {
		return response.BadRequest("username: property is empty or missing from request", "empty username")
	}
	if createUser.Password == "" {
		return response.BadRequest("password: property is empty or missing from request", "empty password")
	}

	role := dao.Unverified
	if createUser.Role != "" {
		role, err = dao.ParseRole(createUser.Role)
		if err != nil {
			return response.BadRequest("role: "+err.Error(), "role: %s", err.Error())
		}
	}

	newUser, err := api.Service.CreateUser(req.Context(), createUser.Username, createUser.Password, createUser.Email, role)
	if err != nil {
		if errors.Is(err, serr.ErrAlreadyExists) {
			return response.Conflict("User with that username already exists", "user '%s' already exists", createUser.Username)
		} else if errors.Is(err, serr.ErrBadArgument) {
			return response.BadRequest(err.Error(), err.Error())
		} else {
			return response.InternalServerError(err.Error())
		}
	}

	resp := UserModel{
		URI:            api.pathPrefix + "/users/" + newUser.ID.String(),
		ID:             newUser.ID.String(),
		Username:       newUser.Username,
		Role:           newUser.Role.String(),
		Created:        newUser.Created.Format(time.RFC3339),
		Modified:       newUser.Modified.Format(time.RFC3339),
		LastLogoutTime: newUser.LastLogoutTime.Format(time.RFC3339),
		LastLoginTime:  newUser.LastLoginTime.Format(time.RFC3339),
	}

	if newUser.Email != nil {
		resp.Email = newUser.Email.Address
	}

	return response.Created(resp, "user '%s' (%s) created", resp.Username, resp.ID)
}

// HTTPGetUser returns a HandlerFunc that gets an existing user. All users may
// retrieve themselves, but only an admin user can retrieve details on other
// users.
//
// The handler has requirements for the request context it receives, and if the
// requirements are not met it may return an HTTP-500. The context must contain
// the ID of the user being operated on and the logged-in user of the client
// making the request.
func (api LoginAPI) HTTPGetUser() http.HandlerFunc {
	return jelly.Endpoint(api.UnauthDelay, api.epGetUser)
}

func (api LoginAPI) epGetUser(req *http.Request) response.Result {
	id := jelly.RequireIDParam(req)
	user := req.Context().Value(middle.AuthUser).(dao.User)

	// is the user trying to delete someone else? they'd betta be the admin if so!
	if id != user.ID && user.Role != dao.Admin {

		var otherUserStr string
		otherUser, err := api.Service.GetUser(req.Context(), id.String())
		// if there was another user, find out now
		if err != nil {
			otherUserStr = fmt.Sprintf("%d", id)
		} else {
			otherUserStr = "'" + otherUser.Username + "'"
		}

		return response.Forbidden("user '%s' (role %s) get user %s: forbidden", user.Username, user.Role, otherUserStr)
	}

	userInfo, err := api.Service.GetUser(req.Context(), id.String())
	if err != nil {
		if errors.Is(err, serr.ErrBadArgument) {
			return response.BadRequest(err.Error(), err.Error())
		} else if errors.Is(err, serr.ErrNotFound) {
			return response.NotFound()
		}
		return response.InternalServerError("could not get user: " + err.Error())
	}

	// put it into a model to return
	resp := UserModel{
		URI:            api.pathPrefix + "/users/" + userInfo.ID.String(),
		ID:             userInfo.ID.String(),
		Username:       userInfo.Username,
		Role:           userInfo.Role.String(),
		Created:        userInfo.Created.Format(time.RFC3339),
		Modified:       userInfo.Modified.Format(time.RFC3339),
		LastLogoutTime: userInfo.LastLogoutTime.Format(time.RFC3339),
		LastLoginTime:  userInfo.LastLoginTime.Format(time.RFC3339),
	}
	if userInfo.Email != nil {
		resp.Email = userInfo.Email.Address
	}

	var otherStr string
	if id != user.ID {
		if userInfo.Username != "" {
			otherStr = "user '" + userInfo.Username + "'"
		} else {
			otherStr = "user " + id.String() + " (no-op)"
		}
	} else {
		otherStr = "self"
	}

	return response.OK(resp, "user '%s' successfully got %s", user.Username, otherStr)
}

// HTTPUpdateUser returns a HandlerFunc that updates an existing user. Only
// updates to properties that are not auto-calculated are respected (e.g. trying
// to update the created time will have no effect). All users may update
// themselves, but only the admin user may update other users.
//
// The handler has requirements for the request context it receives, and if the
// requirements are not met it may return an HTTP-500. The context must contain
// the ID of the user being operated on and the logged-in user of the client
// making the request.
func (api LoginAPI) HTTPUpdateUser() http.HandlerFunc {
	return jelly.Endpoint(api.UnauthDelay, api.epUpdateUser)
}

func (api LoginAPI) epUpdateUser(req *http.Request) response.Result {
	id := jelly.RequireIDParam(req)
	user := req.Context().Value(middle.AuthUser).(dao.User)

	if id != user.ID && user.Role != dao.Admin {
		var otherUserStr string
		otherUser, err := api.Service.GetUser(req.Context(), id.String())
		// if there was another user, find out now
		if err != nil {
			otherUserStr = fmt.Sprintf("%d", id)
		} else {
			otherUserStr = "'" + otherUser.Username + "'"
		}

		return response.Forbidden("user '%s' (role %s) update user %s: forbidden", user.Username, user.Role, otherUserStr)
	}

	var updateReq UserUpdateRequest
	err := jelly.ParseJSONRequest(req, &updateReq)
	if err != nil {
		if errors.Is(err, serr.ErrBodyUnmarshal) {
			// did they send a normal user?
			var normalUser UserModel
			err2 := jelly.ParseJSONRequest(req, &normalUser)
			if err2 == nil {
				return response.BadRequest("updated fields must be objects with keys {'u': true, 'v': NEW_VALUE}", "request is UserModel, not UserUpdateRequest")
			}
		}

		return response.BadRequest(err.Error(), err.Error())
	}

	// pre-parse updateRole if needed so we return bad request before hitting
	// DB
	var updateRole dao.Role
	if updateReq.Role.Update {
		updateRole, err = dao.ParseRole(updateReq.Role.Value)
		if err != nil {
			return response.BadRequest(err.Error(), err.Error())
		}
	}

	existing, err := api.Service.GetUser(req.Context(), id.String())
	if err != nil {
		if errors.Is(err, serr.ErrNotFound) {
			return response.NotFound()
		}
		return response.InternalServerError(err.Error())
	}

	var newEmail string
	if existing.Email != nil {
		newEmail = existing.Email.Address
	}
	if updateReq.Email.Update {
		newEmail = updateReq.Email.Value
	}
	newID := existing.ID.String()
	if updateReq.ID.Update {
		newID = updateReq.ID.Value
	}
	newUsername := existing.Username
	if updateReq.Username.Update {
		newUsername = updateReq.Username.Value
	}
	newRole := existing.Role
	if updateReq.Role.Update {
		newRole = updateRole
	}

	// TODO: this is sequential modification. we need to update this when we get
	// transactions on jeldao.
	updated, err := api.Service.UpdateUser(req.Context(), id.String(), newID, newUsername, newEmail, newRole)
	if err != nil {
		if errors.Is(err, serr.ErrAlreadyExists) {
			return response.Conflict(err.Error(), err.Error())
		} else if errors.Is(err, serr.ErrNotFound) {
			return response.NotFound()
		}
		return response.InternalServerError(err.Error())
	}
	if updateReq.Password.Update {
		updated, err = api.Service.UpdatePassword(req.Context(), updated.ID.String(), updateReq.Password.Value)
		if errors.Is(err, serr.ErrNotFound) {
			return response.NotFound()
		}
		return response.InternalServerError(err.Error())
	}

	resp := UserModel{
		URI:            api.pathPrefix + "/users/" + updated.ID.String(),
		ID:             updated.ID.String(),
		Username:       updated.Username,
		Role:           updated.Role.String(),
		Created:        updated.Created.Format(time.RFC3339),
		Modified:       updated.Modified.Format(time.RFC3339),
		LastLogoutTime: updated.LastLogoutTime.Format(time.RFC3339),
		LastLoginTime:  updated.LastLoginTime.Format(time.RFC3339),
	}

	if updated.Email != nil {
		resp.Email = updated.Email.Address
	}

	return response.Created(resp, "user '%s' (%s) updated", resp.Username, resp.ID)
}

// HTTPReplaceUser returns a HandlerFunc that replaces a user entity with a
// completely new one with the same ID. Only an admin user may replace a user.
// If the user with the given ID does not exist, it will be created.
//
// The handler has requirements for the request context it receives, and if the
// requirements are not met it may return an HTTP-500. The context must contain
// the ID of the user being replaced and the logged-in user of the client making
// the request.
func (api LoginAPI) HTTPReplaceUser() http.HandlerFunc {
	return jelly.Endpoint(api.UnauthDelay, api.epReplaceUser)
}

func (api LoginAPI) epReplaceUser(req *http.Request) response.Result {
	id := jelly.RequireIDParam(req)
	user := req.Context().Value(middle.AuthUser).(dao.User)

	if user.Role != dao.Admin {
		return response.Forbidden("user '%s' (role %s) creation of new user: forbidden", user.Username, user.Role)
	}

	var createUser UserModel
	err := jelly.ParseJSONRequest(req, &createUser)
	if err != nil {
		return response.BadRequest(err.Error(), err.Error())
	}
	if createUser.Username == "" {
		return response.BadRequest("username: property is empty or missing from request", "empty username")
	}
	if createUser.Password == "" {
		return response.BadRequest("password: property is empty or missing from request", "empty password")
	}
	if createUser.ID == "" {
		createUser.ID = id.String()
	}
	if createUser.ID != id.String() {
		return response.BadRequest("id: must be same as ID in URI", "body ID different from URI ID")
	}

	role := dao.Unverified
	if createUser.Role != "" {
		role, err = dao.ParseRole(createUser.Role)
		if err != nil {
			return response.BadRequest("role: "+err.Error(), "role: %s", err.Error())
		}
	}

	newUser, err := api.Service.CreateUser(req.Context(), createUser.Username, createUser.Password, createUser.Email, role)
	if err != nil {
		if errors.Is(err, serr.ErrAlreadyExists) {
			return response.Conflict("User with that username already exists", "user '%s' already exists", createUser.Username)
		} else if errors.Is(err, serr.ErrBadArgument) {
			return response.BadRequest(err.Error(), err.Error())
		}
		return response.InternalServerError(err.Error())
	}

	// but also update it immediately to set its user ID
	newUser, err = api.Service.UpdateUser(req.Context(), newUser.ID.String(), createUser.ID, newUser.Username, newUser.Email.Address, newUser.Role)
	if err != nil {
		if errors.Is(err, serr.ErrAlreadyExists) {
			return response.Conflict("User with that username already exists", "user '%s' already exists", createUser.Username)
		} else if errors.Is(err, serr.ErrBadArgument) {
			return response.BadRequest(err.Error(), err.Error())
		}
		return response.InternalServerError(err.Error())
	}

	resp := UserModel{
		URI:            api.pathPrefix + "/users/" + newUser.ID.String(),
		ID:             newUser.ID.String(),
		Username:       newUser.Username,
		Role:           newUser.Role.String(),
		Created:        newUser.Created.Format(time.RFC3339),
		Modified:       newUser.Modified.Format(time.RFC3339),
		LastLogoutTime: newUser.LastLogoutTime.Format(time.RFC3339),
		LastLoginTime:  newUser.LastLoginTime.Format(time.RFC3339),
	}

	if newUser.Email != nil {
		resp.Email = newUser.Email.Address
	}

	return response.Created(resp, "user '%s' (%s) created", resp.Username, resp.ID)
}

// HTTPDeleteUser returns a HandlerFunc that deletes a user entity. All users
// may delete themselves, but only an admin user may delete another user.
//
// The handler has requirements for the request context it receives, and if the
// requirements are not met it may return an HTTP-500. The context must contain
// the ID of the user being deleted and the logged-in user of the client making
// the request.
func (api LoginAPI) HTTPDeleteUser() http.HandlerFunc {
	return jelly.Endpoint(api.UnauthDelay, api.epDeleteUser)
}

func (api LoginAPI) epDeleteUser(req *http.Request) response.Result {
	id := jelly.RequireIDParam(req)
	user := req.Context().Value(middle.AuthUser).(dao.User)

	// is the user trying to delete someone else? they'd betta be the admin if so!
	if id != user.ID && user.Role != dao.Admin {
		var otherUserStr string
		otherUser, err := api.Service.GetUser(req.Context(), id.String())
		// if there was another user, find out now
		if err != nil {
			otherUserStr = fmt.Sprintf("%d", id)
		} else {
			otherUserStr = "'" + otherUser.Username + "'"
		}

		return response.Forbidden("user '%s' (role %s) delete user %s: forbidden", user.Username, user.Role, otherUserStr)
	}

	deletedUser, err := api.Service.DeleteUser(req.Context(), id.String())
	if err != nil && !errors.Is(err, serr.ErrNotFound) {
		if errors.Is(err, serr.ErrBadArgument) {
			return response.BadRequest(err.Error(), err.Error())
		}
		return response.InternalServerError("could not delete user: " + err.Error())
	}

	var otherStr string
	if id != user.ID {
		if deletedUser.Username != "" {
			otherStr = "user '" + deletedUser.Username + "'"
		} else {
			otherStr = "user " + id.String() + " (no-op)"
		}
	} else {
		otherStr = "self"
	}

	return response.NoContent("user '%s' successfully deleted %s", user.Username, otherStr)
}
