package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/db"
	"github.com/dekarrin/jelly/internal/authuserdao"
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
		return jelly.WrapDBError(err)
	}

	return nil
}

func (repo *AuthUsersDB) Create(ctx context.Context, u jelly.AuthUser) (jelly.AuthUser, error) {
	newUUID, err := uuid.NewRandom()
	if err != nil {
		return jelly.AuthUser{}, fmt.Errorf("could not generate ID: %w", err)
	}

	stmt, err := repo.DB.Prepare(`INSERT INTO users (id, username, password, role, email, created, modified, last_logout_time, last_login_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return jelly.AuthUser{}, jelly.WrapDBError(err)
	}

	now := db.Timestamp(time.Now())
	user := authuserdao.NewUserFromAuthUser(u)
	_, err = stmt.ExecContext(
		ctx,
		newUUID,
		user.Username,
		user.Password,
		user.Role,
		user.Email,
		now,
		now,
		now,
		db.Timestamp{},
	)
	if err != nil {
		return jelly.AuthUser{}, jelly.WrapDBError(err)
	}

	return repo.Get(ctx, newUUID)
}

func (repo *AuthUsersDB) GetAll(ctx context.Context) ([]jelly.AuthUser, error) {
	rows, err := repo.DB.QueryContext(ctx, `SELECT id, username, password, role, email, created, modified, last_logout_time, last_login_time FROM users;`)
	if err != nil {
		return nil, jelly.WrapDBError(err)
	}
	defer rows.Close()

	var all []jelly.AuthUser

	for rows.Next() {
		var user authuserdao.User
		err = rows.Scan(
			&user.ID,
			&user.Username,
			&user.Password,
			&user.Role,
			&user.Email,
			&user.Created,
			&user.Modified,
			&user.LastLogout,
			&user.LastLogin,
		)

		if err != nil {
			return nil, jelly.WrapDBError(err)
		}

		all = append(all, user.AuthUser())
	}

	if err := rows.Err(); err != nil {
		return all, jelly.WrapDBError(err)
	}

	return all, nil
}

func (repo *AuthUsersDB) Update(ctx context.Context, id uuid.UUID, u jelly.AuthUser) (jelly.AuthUser, error) {
	user := authuserdao.NewUserFromAuthUser(u)

	// deliberately not updating created
	res, err := repo.DB.ExecContext(ctx, `UPDATE users SET id=?, username=?, password=?, role=?, email=?, last_logout_time=?, last_login_time=?, modified=? WHERE id=?;`,
		user.ID,
		user.Username,
		user.Password,
		user.Role,
		user.Email,
		user.LastLogout,
		user.LastLogin,
		db.Timestamp(time.Now()),
		id,
	)
	if err != nil {
		return jelly.AuthUser{}, jelly.WrapDBError(err)
	}
	rowsAff, err := res.RowsAffected()
	if err != nil {
		return jelly.AuthUser{}, jelly.WrapDBError(err)
	}
	if rowsAff < 1 {
		return jelly.AuthUser{}, jelly.ErrNotFound
	}

	return repo.Get(ctx, user.ID)
}

func (repo *AuthUsersDB) GetByUsername(ctx context.Context, username string) (jelly.AuthUser, error) {
	user := authuserdao.User{
		Username: username,
	}

	row := repo.DB.QueryRowContext(ctx, `SELECT id, password, role, email, created, modified, last_logout_time, last_login_time FROM users WHERE username = ?;`,
		username,
	)
	err := row.Scan(
		&user.ID,
		&user.Password,
		&user.Role,
		&user.Email,
		&user.Created,
		&user.Modified,
		&user.LastLogout,
		&user.LastLogin,
	)

	if err != nil {
		return user.AuthUser(), jelly.WrapDBError(err)
	}

	return user.AuthUser(), nil
}

func (repo *AuthUsersDB) Get(ctx context.Context, id uuid.UUID) (jelly.AuthUser, error) {
	user := authuserdao.User{
		ID: id,
	}

	row := repo.DB.QueryRowContext(ctx, `SELECT username, password, role, email, created, modified, last_logout_time, last_login_time FROM users WHERE id = ?;`,
		id,
	)
	err := row.Scan(
		&user.Username,
		&user.Password,
		&user.Role,
		&user.Email,
		&user.Created,
		&user.Modified,
		&user.LastLogout,
		&user.LastLogin,
	)

	if err != nil {
		return user.AuthUser(), jelly.WrapDBError(err)
	}

	return user.AuthUser(), nil
}

func (repo *AuthUsersDB) Delete(ctx context.Context, id uuid.UUID) (jelly.AuthUser, error) {
	curVal, err := repo.Get(ctx, id)
	if err != nil {
		return curVal, err
	}

	res, err := repo.DB.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return curVal, jelly.WrapDBError(err)
	}
	rowsAff, err := res.RowsAffected()
	if err != nil {
		return curVal, jelly.WrapDBError(err)
	}
	if rowsAff < 1 {
		return curVal, jelly.ErrNotFound
	}

	return curVal, nil
}

func (repo *AuthUsersDB) Close() error {
	return repo.DB.Close()
}
