package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dekarrin/jelly/dao"
	"github.com/dekarrin/jelly/dao/sqlite/dbconv"
	"github.com/google/uuid"
)

type AuthUsersDB struct {
	DB *sql.DB
}

func (repo *AuthUsersDB) init() error {
	_, err := repo.DB.Exec(`CREATE TABLE IF NOT EXISTS users (
		id TEXT NOT NULL PRIMARY KEY,
		username TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		role INTEGER NOT NULL,
		email TEXT NOT NULL,
		created INTEGER NOT NULL,
		modified INTEGER NOT NULL,
		last_logout_time INTEGER NOT NULL,
		last_login_time INTEGER NOT NULL
	);`)
	if err != nil {
		return WrapDBError(err)
	}

	return nil
}

func (repo *AuthUsersDB) Create(ctx context.Context, user dao.User) (dao.User, error) {
	newUUID, err := uuid.NewRandom()
	if err != nil {
		return dao.User{}, fmt.Errorf("could not generate ID: %w", err)
	}

	stmt, err := repo.DB.Prepare(`INSERT INTO users (id, username, password, role, email, created, modified, last_logout_time, last_login_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return dao.User{}, WrapDBError(err)
	}

	now := time.Now()
	_, err = stmt.ExecContext(
		ctx,
		dbconv.UUID.ToDB(newUUID),
		user.Username,
		user.Password,
		user.Role,
		dbconv.Email.ToDB(user.Email),
		dbconv.Timestamp.ToDB(now),
		dbconv.Timestamp.ToDB(now),
		dbconv.Timestamp.ToDB(now),
		dbconv.Timestamp.ToDB(time.Time{}),
	)
	if err != nil {
		return dao.User{}, WrapDBError(err)
	}

	return repo.Get(ctx, newUUID)
}

func (repo *AuthUsersDB) GetAll(ctx context.Context) ([]dao.User, error) {
	rows, err := repo.DB.QueryContext(ctx, `SELECT id, username, password, role, email, created, modified, last_logout_time, last_login_time FROM users;`)
	if err != nil {
		return nil, WrapDBError(err)
	}
	defer rows.Close()

	var all []dao.User

	for rows.Next() {
		var user dao.User
		var email string
		var logoutTime int64
		var loginTime int64
		var created int64
		var modified int64
		var id string
		err = rows.Scan(
			&id,
			&user.Username,
			&user.Password,
			&user.Role,
			&email,
			&created,
			&modified,
			&logoutTime,
			&loginTime,
		)

		if err != nil {
			return nil, WrapDBError(err)
		}

		err = dbconv.UUID.FromDB(id, &user.ID)
		if err != nil {
			return all, fmt.Errorf("stored UUID %q is invalid: %w", id, err)
		}
		err = dbconv.Email.FromDB(email, &user.Email)
		if err != nil {
			return all, fmt.Errorf("stored email %q is invalid: %w", email, err)
		}
		err = dbconv.Timestamp.FromDB(logoutTime, &user.LastLogoutTime)
		if err != nil {
			return all, fmt.Errorf("stored last_logout_time %d is invalid: %w", logoutTime, err)
		}
		err = dbconv.Timestamp.FromDB(loginTime, &user.LastLoginTime)
		if err != nil {
			return all, fmt.Errorf("stored last_login_time %d is invalid: %w", loginTime, err)
		}
		err = dbconv.Timestamp.FromDB(created, &user.Created)
		if err != nil {
			return all, fmt.Errorf("stored created time %d is invalid: %w", created, err)
		}
		err = dbconv.Timestamp.FromDB(modified, &user.Modified)
		if err != nil {
			return all, fmt.Errorf("stored modified time %d is invalid: %w", modified, err)
		}

		all = append(all, user)
	}

	if err := rows.Err(); err != nil {
		return all, WrapDBError(err)
	}

	return all, nil
}

func (repo *AuthUsersDB) Update(ctx context.Context, id uuid.UUID, user dao.User) (dao.User, error) {
	// deliberately not updating created
	res, err := repo.DB.ExecContext(ctx, `UPDATE users SET id=?, username=?, password=?, role=?, email=?, last_logout_time=?, last_login_time=?, modified=? WHERE id=?;`,
		dbconv.UUID.ToDB(user.ID),
		user.Username,
		user.Password,
		user.Role,
		dbconv.Email.ToDB(user.Email),
		dbconv.Timestamp.ToDB(user.LastLogoutTime),
		dbconv.Timestamp.ToDB(user.LastLoginTime),
		dbconv.Timestamp.ToDB(time.Now()),
		dbconv.UUID.ToDB(id),
	)
	if err != nil {
		return dao.User{}, WrapDBError(err)
	}
	rowsAff, err := res.RowsAffected()
	if err != nil {
		return dao.User{}, WrapDBError(err)
	}
	if rowsAff < 1 {
		return dao.User{}, dao.ErrNotFound
	}

	return repo.Get(ctx, user.ID)
}

func (repo *AuthUsersDB) GetByUsername(ctx context.Context, username string) (dao.User, error) {
	user := dao.User{
		Username: username,
	}
	var id string
	var email string
	var logout int64
	var login int64
	var created int64
	var modified int64

	row := repo.DB.QueryRowContext(ctx, `SELECT id, password, role, email, created, modified, last_logout_time, last_login_time FROM users WHERE username = ?;`,
		username,
	)
	err := row.Scan(
		&id,
		&user.Password,
		&user.Role,
		&email,
		&created,
		&modified,
		&logout,
		&login,
	)

	if err != nil {
		return user, WrapDBError(err)
	}

	err = dbconv.UUID.FromDB(id, &user.ID)
	if err != nil {
		return user, fmt.Errorf("stored UUID %q is invalid: %w", id, err)
	}
	err = dbconv.Email.FromDB(email, &user.Email)
	if err != nil {
		return user, fmt.Errorf("stored email %q is invalid: %w", email, err)
	}
	err = dbconv.Timestamp.FromDB(logout, &user.LastLogoutTime)
	if err != nil {
		return user, fmt.Errorf("stored last_logout_time %d is invalid: %w", logout, err)
	}
	err = dbconv.Timestamp.FromDB(login, &user.LastLoginTime)
	if err != nil {
		return user, fmt.Errorf("stored last_login_time %d is invalid: %w", login, err)
	}
	err = dbconv.Timestamp.FromDB(created, &user.Created)
	if err != nil {
		return user, fmt.Errorf("stored created time %d is invalid: %w", created, err)
	}
	err = dbconv.Timestamp.FromDB(modified, &user.Modified)
	if err != nil {
		return user, fmt.Errorf("stored modified time %d is invalid: %w", modified, err)
	}

	return user, nil
}

func (repo *AuthUsersDB) Get(ctx context.Context, id uuid.UUID) (dao.User, error) {
	user := dao.User{
		ID: id,
	}
	var email string
	var logout int64
	var login int64
	var created int64
	var modified int64

	row := repo.DB.QueryRowContext(ctx, `SELECT username, password, role, email, created, modified, last_logout_time, last_login_time FROM users WHERE id = ?;`,
		dbconv.UUID.ToDB(id),
	)
	err := row.Scan(
		&user.Username,
		&user.Password,
		&user.Role,
		&email,
		&created,
		&modified,
		&logout,
		&login,
	)

	if err != nil {
		return user, WrapDBError(err)
	}

	err = dbconv.Email.FromDB(email, &user.Email)
	if err != nil {
		return user, fmt.Errorf("stored email %q is invalid: %w", email, err)
	}
	err = dbconv.Timestamp.FromDB(logout, &user.LastLogoutTime)
	if err != nil {
		return user, fmt.Errorf("stored last_logout_time %d is invalid: %w", logout, err)
	}
	err = dbconv.Timestamp.FromDB(login, &user.LastLoginTime)
	if err != nil {
		return user, fmt.Errorf("stored last_login_time %d is invalid: %w", login, err)
	}
	err = dbconv.Timestamp.FromDB(created, &user.Created)
	if err != nil {
		return user, fmt.Errorf("stored created time %d is invalid: %w", created, err)
	}
	err = dbconv.Timestamp.FromDB(modified, &user.Modified)
	if err != nil {
		return user, fmt.Errorf("stored modified time %d is invalid: %w", modified, err)
	}

	return user, nil
}

func (repo *AuthUsersDB) Delete(ctx context.Context, id uuid.UUID) (dao.User, error) {
	curVal, err := repo.Get(ctx, id)
	if err != nil {
		return curVal, err
	}

	res, err := repo.DB.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, dbconv.UUID.ToDB(id))
	if err != nil {
		return curVal, WrapDBError(err)
	}
	rowsAff, err := res.RowsAffected()
	if err != nil {
		return curVal, WrapDBError(err)
	}
	if rowsAff < 1 {
		return curVal, dao.ErrNotFound
	}

	return curVal, nil
}

func (repo *AuthUsersDB) Close() error {
	return repo.DB.Close()
}
