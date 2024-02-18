package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/serr"
	"github.com/dekarrin/jelly/token"
	"github.com/dekarrin/jelly/types"
)

var useJellyauthJWT = jelly.Override{Authenticators: []string{"jellyauth.jwt"}}

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

	log types.Logger
}

func (api *LoginAPI) Init(cb jelly.Bundle) error {
	api.name = cb.Name()
	api.log = cb.Logger()
	api.Secret = cb.GetByteSlice(ConfigKeySecret)

	unauth := cb.GetInt(ConfigKeyUnauthDelay)
	var d time.Duration
	if unauth >= 0 {
		d = time.Duration(unauth) * time.Millisecond
	}
	api.UnauthDelay = d

	authRaw := cb.DB(0)
	authStore, ok := authRaw.(types.AuthUserStore)
	if !ok {
		return fmt.Errorf("DB provided under 'auth' does not implement db.AuthUserStore")
	}
	api.Service = LoginService{
		Provider: authStore,
	}
	api.pathPrefix = cb.Base()

	ctx := context.Background()
	setAdmin := cb.Get(ConfigKeySetAdmin)
	if setAdmin != "" {
		username, pass, err := parseSetAdmin(setAdmin)
		if err != nil {
			return fmt.Errorf(ConfigKeySetAdmin+": %w", err)
		}

		existing, err := api.Service.GetUserByUsername(ctx, username)
		if err == nil {
			// a user exists. we need to update it.
			user, err := api.Service.UpdatePassword(ctx, existing.ID.String(), pass)
			if err != nil {
				return fmt.Errorf("update password for user %q: %w", username, err)
			}
			api.log.Debugf("updated user %s's password due to set-admin config", username)
			// make shore their role is set to admin as well
			if user.Role != types.Admin {
				_, err = api.Service.UpdateUser(ctx, user.ID.String(), user.ID.String(), user.Username, user.Email, types.Admin)
				if err != nil {
					return fmt.Errorf("update role to admin for user %q: %w", username, err)
				}
				api.log.Debugf("updated user %s's role to admin due to set-admin config", username)
			}
		} else {
			if !errors.Is(err, serr.ErrNotFound) {
				return fmt.Errorf("retrieve user for admin promotion: %w", err)
			}

			_, err = api.Service.CreateUser(ctx, username, pass, "", types.Admin)
			if err != nil {
				return fmt.Errorf("creating admin user: %w", err)
			}

			api.log.Debugf("created user %s as admin due to set-admin config", username)
		}
	}

	return nil
}

func (api *LoginAPI) Authenticators() map[string]types.Authenticator {
	// this provides one and only one authenticator, the jwt one.

	// we will have had Init called, ergo secret and the service db will exist
	return map[string]types.Authenticator{
		"jwt": JWTAuthProvider{
			secret:      api.Secret,
			db:          api.Service.Provider.AuthUsers(),
			unauthDelay: api.UnauthDelay,
			srv:         api.Service,
		},
	}
}

// Shutdown shuts down the login API. This is added to implement jelapi.API, and
// has no effect on the login API but to return the error of the context.
func (api *LoginAPI) Shutdown(ctx context.Context) error {
	return ctx.Err()
}

// httpGetInfo returns a HandlerFunc that retrieves information on the API and
// server.
//
// The handler has requirements for the request context it receives, and if the
// requirements are not met it may return an HTTP-500. The context must contain
// a value denoting whether the client making the request is logged-in.
func (api LoginAPI) httpGetInfo(em jelly.ServiceProvider) http.HandlerFunc {
	return em.Endpoint(func(req *http.Request) types.Result {
		user, loggedIn := em.GetLoggedInUser(req)

		var resp InfoModel
		resp.Version.Auth = Version

		userStr := "unauthed client"
		if loggedIn {
			userStr = "user '" + user.Username + "'"
		}
		return em.OK(resp, "%s got API info", userStr)
	}, useJellyauthJWT)
}

// httpCreateLogin returns a HandlerFunc that uses the API to log in a user with
// a username and password and return the auth token for that user.
func (api LoginAPI) httpCreateLogin(em jelly.ServiceProvider) http.HandlerFunc {
	return em.Endpoint(func(req *http.Request) types.Result {
		loginData := LoginRequest{}
		err := jelly.ParseJSONRequest(req, &loginData)
		if err != nil {
			return em.BadRequest(err.Error(), err.Error())
		}

		if loginData.Username == "" {
			return em.BadRequest("username: property is empty or missing from request", "empty username")
		}
		if loginData.Password == "" {
			return em.BadRequest("password: property is empty or missing from request", "empty password")
		}

		user, err := api.Service.Login(req.Context(), loginData.Username, loginData.Password)
		if err != nil {
			if errors.Is(err, serr.ErrBadCredentials) {
				return em.Unauthorized(serr.ErrBadCredentials.Error(), "user '%s': %s", loginData.Username, err.Error())
			} else {
				return em.InternalServerError(err.Error())
			}
		}

		// build the token
		// password is valid, generate token for user and return it.
		tok, err := token.Generate(api.Secret, user)
		if err != nil {
			return em.InternalServerError("could not generate JWT: " + err.Error())
		}

		resp := LoginResponse{
			Token:  tok,
			UserID: user.ID.String(),
		}
		return em.Created(resp, "user '"+user.Username+"' successfully logged in")
	}, useJellyauthJWT)
}

// httpDeleteLogin returns a HandlerFunc that deletes active login for some
// user. Only admin users can delete logins for users other themselves.
//
// The handler has requirements for the request context it receives, and if the
// requirements are not met it may return an HTTP-500. The context must contain
// the ID of the user to log out and the logged-in user of the client making the
// request.
func (api LoginAPI) httpDeleteLogin(em jelly.ServiceProvider) http.HandlerFunc {
	return em.Endpoint(func(req *http.Request) types.Result {
		id := jelly.RequireIDParam(req)
		user, _ := em.GetLoggedInUser(req)

		// is the user trying to delete someone else's login? they'd betta be the
		// admin if so!
		if id != user.ID && user.Role != types.Admin {
			var otherUserStr string
			otherUser, err := api.Service.GetUser(req.Context(), id.String())
			// if there was another user, find out now
			if err != nil {
				if !errors.Is(err, serr.ErrNotFound) {
					return em.InternalServerError("retrieve user for perm checking: %s", err.Error())
				}
				otherUserStr = id.String()
			} else {
				otherUserStr = "'" + otherUser.Username + "'"
			}

			return em.Forbidden("user '%s' (role %s) logout of user %s: forbidden", user.Username, user.Role, otherUserStr)
		}

		loggedOutUser, err := api.Service.Logout(req.Context(), id)
		if err != nil {
			if errors.Is(err, serr.ErrNotFound) {
				return em.NotFound()
			}
			return em.InternalServerError("could not log out user: " + err.Error())
		}

		var otherStr string
		if id != user.ID {
			otherStr = "user '" + loggedOutUser.Username + "'"
		} else {
			otherStr = "self"
		}

		return em.NoContent("user '%s' successfully logged out %s", user.Username, otherStr)
	}, useJellyauthJWT)
}

// httpCreateToken returns a HandlerFunc that creates a new token for the user
// the client is logged in as.
//
// The handler has requirements for the request context it receives, and if the
// requirements are not met it may return an HTTP-500. The context must contain
// the logged-in user of the client making the request.
func (api LoginAPI) httpCreateToken(em jelly.ServiceProvider) http.HandlerFunc {
	return em.Endpoint(func(req *http.Request) types.Result {
		user, _ := em.GetLoggedInUser(req)

		tok, err := token.Generate(api.Secret, user)
		if err != nil {
			return em.InternalServerError("could not generate JWT: " + err.Error())
		}

		resp := LoginResponse{
			Token:  tok,
			UserID: user.ID.String(),
		}
		return em.Created(resp, "user '"+user.Username+"' successfully created new token")
	}, useJellyauthJWT)
}

// httpGetAllUsers returns a HandlerFunc that retrieves all existing users. Only
// an admin user can call this endpoint.
//
// The handler has requirements for the request context it receives, and if the
// requirements are not met it may return an HTTP-500. The context must contain
// the logged-in user of the client making the request.
func (api LoginAPI) httpGetAllUsers(em jelly.ServiceProvider) http.HandlerFunc {
	return em.Endpoint(func(req *http.Request) types.Result {
		user, _ := em.GetLoggedInUser(req)

		if user.Role != types.Admin {
			return em.Forbidden("user '%s' (role %s): forbidden", user.Username, user.Role)
		}

		users, err := api.Service.GetAllUsers(req.Context())
		if err != nil {
			return em.InternalServerError(err.Error())
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
				LastLogoutTime: users[i].LastLogout.Format(time.RFC3339),
				LastLoginTime:  users[i].LastLogin.Format(time.RFC3339),
				Email:          users[i].Email,
			}
		}

		return em.OK(resp, "user '%s' got all users", user.Username)
	}, useJellyauthJWT)
}

// httpCreateUser returns a HandlerFunc that creates a new user entity. Only an
// admin user can directly create new users.
//
// The handler has requirements for the request context it receives, and if the
// requirements are not met it may return an HTTP-500. The context must contain
// the logged-in user of the client making the request.
func (api LoginAPI) httpCreateUser(em jelly.ServiceProvider) http.HandlerFunc {
	return em.Endpoint(func(req *http.Request) types.Result {
		user, _ := em.GetLoggedInUser(req)

		if user.Role != types.Admin {
			return em.Forbidden("user '%s' (role %s) creation of new user: forbidden", user.Username, user.Role)
		}

		var createUser UserModel
		err := jelly.ParseJSONRequest(req, &createUser)
		if err != nil {
			return em.BadRequest(err.Error(), err.Error())
		}
		if createUser.Username == "" {
			return em.BadRequest("username: property is empty or missing from request", "empty username")
		}
		if createUser.Password == "" {
			return em.BadRequest("password: property is empty or missing from request", "empty password")
		}

		role := types.Unverified
		if createUser.Role != "" {
			role, err = types.ParseRole(createUser.Role)
			if err != nil {
				return em.BadRequest("role: "+err.Error(), "role: %s", err.Error())
			}
		}

		newUser, err := api.Service.CreateUser(req.Context(), createUser.Username, createUser.Password, createUser.Email, role)
		if err != nil {
			if errors.Is(err, serr.ErrAlreadyExists) {
				return em.Conflict("User with that username already exists", "user '%s' already exists", createUser.Username)
			} else if errors.Is(err, serr.ErrBadArgument) {
				return em.BadRequest(err.Error(), err.Error())
			} else {
				return em.InternalServerError(err.Error())
			}
		}

		resp := UserModel{
			URI:            api.pathPrefix + "/users/" + newUser.ID.String(),
			ID:             newUser.ID.String(),
			Username:       newUser.Username,
			Role:           newUser.Role.String(),
			Created:        newUser.Created.Format(time.RFC3339),
			Modified:       newUser.Modified.Format(time.RFC3339),
			LastLogoutTime: newUser.LastLogout.Format(time.RFC3339),
			LastLoginTime:  newUser.LastLogin.Format(time.RFC3339),
			Email:          newUser.Email,
		}

		return em.Created(resp, "user '%s' (%s) created", resp.Username, resp.ID)
	}, useJellyauthJWT)
}

// httpGetUser returns a HandlerFunc that gets an existing user. All users may
// retrieve themselves, but only an admin user can retrieve details on other
// users.
//
// The handler has requirements for the request context it receives, and if the
// requirements are not met it may return an HTTP-500. The context must contain
// the ID of the user being operated on and the logged-in user of the client
// making the request.
func (api LoginAPI) httpGetUser(em jelly.ServiceProvider) http.HandlerFunc {
	return em.Endpoint(func(req *http.Request) types.Result {
		id := jelly.RequireIDParam(req)
		user, _ := em.GetLoggedInUser(req)

		// is the user trying to delete someone else? they'd betta be the admin if so!
		if id != user.ID && user.Role != types.Admin {

			var otherUserStr string
			otherUser, err := api.Service.GetUser(req.Context(), id.String())
			// if there was another user, find out now
			if err != nil {
				otherUserStr = id.String()
			} else {
				otherUserStr = "'" + otherUser.Username + "'"
			}

			return em.Forbidden("user '%s' (role %s) get user %s: forbidden", user.Username, user.Role, otherUserStr)
		}

		userInfo, err := api.Service.GetUser(req.Context(), id.String())
		if err != nil {
			if errors.Is(err, serr.ErrBadArgument) {
				return em.BadRequest(err.Error(), err.Error())
			} else if errors.Is(err, serr.ErrNotFound) {
				return em.NotFound()
			}
			return em.InternalServerError("could not get user: " + err.Error())
		}

		// put it into a model to return
		resp := UserModel{
			URI:            api.pathPrefix + "/users/" + userInfo.ID.String(),
			ID:             userInfo.ID.String(),
			Username:       userInfo.Username,
			Role:           userInfo.Role.String(),
			Created:        userInfo.Created.Format(time.RFC3339),
			Modified:       userInfo.Modified.Format(time.RFC3339),
			LastLogoutTime: userInfo.LastLogout.Format(time.RFC3339),
			LastLoginTime:  userInfo.LastLogin.Format(time.RFC3339),
			Email:          userInfo.Email,
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

		return em.OK(resp, "user '%s' successfully got %s", user.Username, otherStr)
	}, useJellyauthJWT)
}

// httpUpdateUser returns a HandlerFunc that updates an existing user. Only
// updates to properties that are not auto-calculated are respected (e.g. trying
// to update the created time will have no effect). All users may update
// themselves, but only the admin user may update other users.
//
// The handler has requirements for the request context it receives, and if the
// requirements are not met it may return an HTTP-500. The context must contain
// the ID of the user being operated on and the logged-in user of the client
// making the request.
func (api LoginAPI) httpUpdateUser(em jelly.ServiceProvider) http.HandlerFunc {
	return em.Endpoint(func(req *http.Request) types.Result {
		id := jelly.RequireIDParam(req)
		user, _ := em.GetLoggedInUser(req)

		if id != user.ID && user.Role != types.Admin {
			var otherUserStr string
			otherUser, err := api.Service.GetUser(req.Context(), id.String())
			// if there was another user, find out now
			if err != nil {
				otherUserStr = id.String()
			} else {
				otherUserStr = "'" + otherUser.Username + "'"
			}

			return em.Forbidden("user '%s' (role %s) update user %s: forbidden", user.Username, user.Role, otherUserStr)
		}

		var updateReq UserUpdateRequest
		err := jelly.ParseJSONRequest(req, &updateReq)
		if err != nil {
			if errors.Is(err, serr.ErrBodyUnmarshal) {
				// did they send a normal user?
				var normalUser UserModel
				err2 := jelly.ParseJSONRequest(req, &normalUser)
				if err2 == nil {
					return em.BadRequest("updated fields must be objects with keys {'u': true, 'v': NEW_VALUE}", "request is UserModel, not UserUpdateRequest")
				}
			}

			return em.BadRequest(err.Error(), err.Error())
		}

		// pre-parse updateRole if needed so we return bad request before hitting
		// DB
		var updateRole types.Role
		if updateReq.Role.Update {
			updateRole, err = types.ParseRole(updateReq.Role.Value)
			if err != nil {
				return em.BadRequest(err.Error(), err.Error())
			}
		}

		existing, err := api.Service.GetUser(req.Context(), id.String())
		if err != nil {
			if errors.Is(err, serr.ErrNotFound) {
				return em.NotFound()
			}
			return em.InternalServerError(err.Error())
		}

		var newEmail string
		if existing.Email != "" {
			newEmail = existing.Email
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
		// transactions on jeldb.
		updated, err := api.Service.UpdateUser(req.Context(), id.String(), newID, newUsername, newEmail, newRole)
		if err != nil {
			if errors.Is(err, serr.ErrAlreadyExists) {
				return em.Conflict(err.Error(), err.Error())
			} else if errors.Is(err, serr.ErrNotFound) {
				return em.NotFound()
			}
			return em.InternalServerError(err.Error())
		}
		if updateReq.Password.Update {
			updated, err = api.Service.UpdatePassword(req.Context(), updated.ID.String(), updateReq.Password.Value)
			if errors.Is(err, serr.ErrNotFound) {
				return em.NotFound()
			}
			return em.InternalServerError(err.Error())
		}

		resp := UserModel{
			URI:            api.pathPrefix + "/users/" + updated.ID.String(),
			ID:             updated.ID.String(),
			Username:       updated.Username,
			Role:           updated.Role.String(),
			Created:        updated.Created.Format(time.RFC3339),
			Modified:       updated.Modified.Format(time.RFC3339),
			LastLogoutTime: updated.LastLogout.Format(time.RFC3339),
			LastLoginTime:  updated.LastLogin.Format(time.RFC3339),
			Email:          updated.Email,
		}

		return em.Created(resp, "user '%s' (%s) updated", resp.Username, resp.ID)
	}, useJellyauthJWT)
}

// httpReplaceUser returns a HandlerFunc that replaces a user entity with a
// completely new one with the same ID. Only an admin user may replace a user.
// If the user with the given ID does not exist, it will be created.
//
// The handler has requirements for the request context it receives, and if the
// requirements are not met it may return an HTTP-500. The context must contain
// the ID of the user being replaced and the logged-in user of the client making
// the request.
func (api LoginAPI) httpReplaceUser(em jelly.ServiceProvider) http.HandlerFunc {
	return em.Endpoint(func(req *http.Request) types.Result {
		id := jelly.RequireIDParam(req)
		user, _ := em.GetLoggedInUser(req)

		if user.Role != types.Admin {
			return em.Forbidden("user '%s' (role %s) creation of new user: forbidden", user.Username, user.Role)
		}

		var createUser UserModel
		err := jelly.ParseJSONRequest(req, &createUser)
		if err != nil {
			return em.BadRequest(err.Error(), err.Error())
		}
		if createUser.Username == "" {
			return em.BadRequest("username: property is empty or missing from request", "empty username")
		}
		if createUser.Password == "" {
			return em.BadRequest("password: property is empty or missing from request", "empty password")
		}
		if createUser.ID == "" {
			createUser.ID = id.String()
		}
		if createUser.ID != id.String() {
			return em.BadRequest("id: must be same as ID in URI", "body ID different from URI ID")
		}

		role := types.Unverified
		if createUser.Role != "" {
			role, err = types.ParseRole(createUser.Role)
			if err != nil {
				return em.BadRequest("role: "+err.Error(), "role: %s", err.Error())
			}
		}

		newUser, err := api.Service.CreateUser(req.Context(), createUser.Username, createUser.Password, createUser.Email, role)
		if err != nil {
			if errors.Is(err, serr.ErrAlreadyExists) {
				return em.Conflict("User with that username already exists", "user '%s' already exists", createUser.Username)
			} else if errors.Is(err, serr.ErrBadArgument) {
				return em.BadRequest(err.Error(), err.Error())
			}
			return em.InternalServerError(err.Error())
		}

		// but also update it immediately to set its user ID
		newUser, err = api.Service.UpdateUser(req.Context(), newUser.ID.String(), createUser.ID, newUser.Username, newUser.Email, newUser.Role)
		if err != nil {
			if errors.Is(err, serr.ErrAlreadyExists) {
				return em.Conflict("User with that username already exists", "user '%s' already exists", createUser.Username)
			} else if errors.Is(err, serr.ErrBadArgument) {
				return em.BadRequest(err.Error(), err.Error())
			}
			return em.InternalServerError(err.Error())
		}

		resp := UserModel{
			URI:            api.pathPrefix + "/users/" + newUser.ID.String(),
			ID:             newUser.ID.String(),
			Username:       newUser.Username,
			Role:           newUser.Role.String(),
			Created:        newUser.Created.Format(time.RFC3339),
			Modified:       newUser.Modified.Format(time.RFC3339),
			LastLogoutTime: newUser.LastLogout.Format(time.RFC3339),
			LastLoginTime:  newUser.LastLogin.Format(time.RFC3339),
			Email:          newUser.Email,
		}

		return em.Created(resp, "user '%s' (%s) created", resp.Username, resp.ID)
	}, useJellyauthJWT)
}

// httpDeleteUser returns a HandlerFunc that deletes a user entity. All users
// may delete themselves, but only an admin user may delete another user.
//
// The handler has requirements for the request context it receives, and if the
// requirements are not met it may return an HTTP-500. The context must contain
// the ID of the user being deleted and the logged-in user of the client making
// the request.
func (api LoginAPI) httpDeleteUser(em jelly.ServiceProvider) http.HandlerFunc {
	return em.Endpoint(func(req *http.Request) types.Result {
		id := jelly.RequireIDParam(req)
		user, _ := em.GetLoggedInUser(req)

		// is the user trying to delete someone else? they'd betta be the admin if so!
		if id != user.ID && user.Role != types.Admin {
			var otherUserStr string
			otherUser, err := api.Service.GetUser(req.Context(), id.String())
			// if there was another user, find out now
			if err != nil {
				otherUserStr = id.String()
			} else {
				otherUserStr = "'" + otherUser.Username + "'"
			}

			return em.Forbidden("user '%s' (role %s) delete user %s: forbidden", user.Username, user.Role, otherUserStr)
		}

		deletedUser, err := api.Service.DeleteUser(req.Context(), id.String())
		if err != nil && !errors.Is(err, serr.ErrNotFound) {
			if errors.Is(err, serr.ErrBadArgument) {
				return em.BadRequest(err.Error(), err.Error())
			}
			return em.InternalServerError("could not delete user: " + err.Error())
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

		return em.NoContent("user '%s' successfully deleted %s", user.Username, otherStr)
	}, useJellyauthJWT)
}
