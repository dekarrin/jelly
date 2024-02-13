package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dekarrin/jelly/db"
	"github.com/dekarrin/jelly/db/sqlite/dbconv"
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

func (repo *AuthUsersDB) Create(ctx context.Context, user db.User) (db.User, error) {
	newUUID, err := uuid.NewRandom()
	if err != nil {
		return db.User{}, fmt.Errorf("could not generate ID: %w", err)
	}

	stmt, err := repo.DB.Prepare(`INSERT INTO users (id, username, password, role, email, created, modified, last_logout_time, last_login_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return db.User{}, WrapDBError(err)
	}

	now := db.Timestamp(time.Now())
	_, err = stmt.ExecContext(
		ctx,
		newUUID,
		user.Username,
		user.Password,
		user.Role,
		dbconv.Email.ToDB(user.Email),
		now,
		now,
		now,
		db.Timestamp{},
	)
	if err != nil {
		return db.User{}, WrapDBError(err)
	}

	return repo.Get(ctx, newUUID)
}

func (repo *AuthUsersDB) GetAll(ctx context.Context) ([]db.User, error) {
	rows, err := repo.DB.QueryContext(ctx, `SELECT id, username, password, role, email, created, modified, last_logout_time, last_login_time FROM users;`)
	if err != nil {
		return nil, WrapDBError(err)
	}
	defer rows.Close()

	var all []db.User

	for rows.Next() {
		var user db.User
		var email string
		err = rows.Scan(
			&user.ID,
			&user.Username,
			&user.Password,
			&user.Role,
			&email,
			&user.Created,
			&user.Modified,
			&user.LastLogout,
			&user.LastLogin,
		)

		if err != nil {
			return nil, WrapDBError(err)
		}

		err = dbconv.Email.FromDB(email, &user.Email)
		if err != nil {
			return all, fmt.Errorf("stored email %q is invalid: %w", email, err)
		}

		all = append(all, user)
	}

	if err := rows.Err(); err != nil {
		return all, WrapDBError(err)
	}

	return all, nil
}

func (repo *AuthUsersDB) Update(ctx context.Context, id uuid.UUID, user db.User) (db.User, error) {
	// deliberately not updating created
	res, err := repo.DB.ExecContext(ctx, `UPDATE users SET id=?, username=?, password=?, role=?, email=?, last_logout_time=?, last_login_time=?, modified=? WHERE id=?;`,
		user.ID,
		user.Username,
		user.Password,
		user.Role,
		dbconv.Email.ToDB(user.Email),
		user.LastLogout,
		user.LastLogin,
		db.Timestamp(time.Now()),
		id,
	)
	if err != nil {
		return db.User{}, WrapDBError(err)
	}
	rowsAff, err := res.RowsAffected()
	if err != nil {
		return db.User{}, WrapDBError(err)
	}
	if rowsAff < 1 {
		return db.User{}, db.ErrNotFound
	}

	return repo.Get(ctx, user.ID)
}

func (repo *AuthUsersDB) GetByUsername(ctx context.Context, username string) (db.User, error) {
	user := db.User{
		Username: username,
	}
	var email string

	row := repo.DB.QueryRowContext(ctx, `SELECT id, password, role, email, created, modified, last_logout_time, last_login_time FROM users WHERE username = ?;`,
		username,
	)
	err := row.Scan(
		&user.ID,
		&user.Password,
		&user.Role,
		&email,
		&user.Created,
		&user.Modified,
		&user.LastLogout,
		&user.LastLogin,
	)

	if err != nil {
		return user, WrapDBError(err)
	}

	err = dbconv.Email.FromDB(email, &user.Email)
	if err != nil {
		return user, fmt.Errorf("stored email %q is invalid: %w", email, err)
	}

	return user, nil
}

func (repo *AuthUsersDB) Get(ctx context.Context, id uuid.UUID) (db.User, error) {
	user := db.User{
		ID: id,
	}
	var email string

	row := repo.DB.QueryRowContext(ctx, `SELECT username, password, role, email, created, modified, last_logout_time, last_login_time FROM users WHERE id = ?;`,
		id,
	)
	err := row.Scan(
		&user.Username,
		&user.Password,
		&user.Role,
		&email,
		&user.Created,
		&user.Modified,
		&user.LastLogout,
		&user.LastLogin,
	)

	if err != nil {
		return user, WrapDBError(err)
	}

	err = dbconv.Email.FromDB(email, &user.Email)
	if err != nil {
		return user, fmt.Errorf("stored email %q is invalid: %w", email, err)
	}

	return user, nil
}

func (repo *AuthUsersDB) Delete(ctx context.Context, id uuid.UUID) (db.User, error) {
	curVal, err := repo.Get(ctx, id)
	if err != nil {
		return curVal, err
	}

	res, err := repo.DB.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return curVal, WrapDBError(err)
	}
	rowsAff, err := res.RowsAffected()
	if err != nil {
		return curVal, WrapDBError(err)
	}
	if rowsAff < 1 {
		return curVal, db.ErrNotFound
	}

	return curVal, nil
}

func (repo *AuthUsersDB) Close() error {
	return repo.DB.Close()
}
