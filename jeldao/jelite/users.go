package jelite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dekarrin/tunaq/server/dao"
	"github.com/google/uuid"
)

func NewUsersDBConn(file string) (*UsersDB, error) {
	repo := &UsersDB{}

	var err error
	repo.db, err = sql.Open("sqlite", file)
	if err != nil {
		return nil, wrapDBError(err)
	}

	return repo, repo.init()
}

type UsersDB struct {
	db *sql.DB
}

func (repo *UsersDB) init() error {
	_, err := repo.db.Exec(`CREATE TABLE IF NOT EXISTS users (
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
		return wrapDBError(err)
	}

	return nil
}

func (repo *UsersDB) Create(ctx context.Context, user dao.User) (dao.User, error) {
	newUUID, err := uuid.NewRandom()
	if err != nil {
		return dao.User{}, fmt.Errorf("could not generate ID: %w", err)
	}

	stmt, err := repo.db.Prepare(`INSERT INTO users (id, username, password, role, email, created, modified, last_logout_time, last_login_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return dao.User{}, wrapDBError(err)
	}

	now := time.Now()
	_, err = stmt.ExecContext(
		ctx,
		convertToDB_UUID(newUUID),
		user.Username,
		user.Password,
		convertToDB_Role(user.Role),
		convertToDB_Email(user.Email),
		convertToDB_Time(now),
		convertToDB_Time(now),
		convertToDB_Time(now),
		convertToDB_Time(time.Time{}),
	)
	if err != nil {
		return dao.User{}, wrapDBError(err)
	}

	return repo.GetByID(ctx, newUUID)
}

func (repo *UsersDB) GetAll(ctx context.Context) ([]dao.User, error) {
	rows, err := repo.db.QueryContext(ctx, `SELECT id, username, password, role, email, created, modified, last_logout_time, last_login_time FROM users;`)
	if err != nil {
		return nil, wrapDBError(err)
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
		var role string
		var id string
		err = rows.Scan(
			&id,
			&user.Username,
			&user.Password,
			&role,
			&email,
			&created,
			&modified,
			&logoutTime,
			&loginTime,
		)

		if err != nil {
			return nil, wrapDBError(err)
		}

		err = convertFromDB_UUID(id, &user.ID)
		if err != nil {
			return all, fmt.Errorf("stored UUID %q is invalid: %w", id, err)
		}
		err = convertFromDB_Email(email, &user.Email)
		if err != nil {
			return all, fmt.Errorf("stored email %q is invalid: %w", email, err)
		}
		err = convertFromDB_Time(logoutTime, &user.LastLogoutTime)
		if err != nil {
			return all, fmt.Errorf("stored last_logout_time %d is invalid: %w", logoutTime, err)
		}
		err = convertFromDB_Time(loginTime, &user.LastLoginTime)
		if err != nil {
			return all, fmt.Errorf("stored last_login_time %d is invalid: %w", loginTime, err)
		}
		err = convertFromDB_Time(created, &user.Created)
		if err != nil {
			return all, fmt.Errorf("stored created time %d is invalid: %w", created, err)
		}
		err = convertFromDB_Time(modified, &user.Modified)
		if err != nil {
			return all, fmt.Errorf("stored modified time %d is invalid: %w", modified, err)
		}
		err = convertFromDB_Role(role, &user.Role)
		if err != nil {
			return all, fmt.Errorf("stored role %q is invalid: %w", role, err)
		}

		all = append(all, user)
	}

	if err := rows.Err(); err != nil {
		return all, wrapDBError(err)
	}

	return all, nil
}

func (repo *UsersDB) Update(ctx context.Context, id uuid.UUID, user dao.User) (dao.User, error) {
	// deliberately not updating created
	res, err := repo.db.ExecContext(ctx, `UPDATE users SET id=?, username=?, password=?, role=?, email=?, last_logout_time=?, last_login_time=?, modified=? WHERE id=?;`,
		convertToDB_UUID(user.ID),
		user.Username,
		user.Password,
		convertToDB_Role(user.Role),
		convertToDB_Email(user.Email),
		convertToDB_Time(user.LastLogoutTime),
		convertToDB_Time(user.LastLoginTime),
		convertToDB_Time(time.Now()),
		convertToDB_UUID(id),
	)
	if err != nil {
		return dao.User{}, wrapDBError(err)
	}
	rowsAff, err := res.RowsAffected()
	if err != nil {
		return dao.User{}, wrapDBError(err)
	}
	if rowsAff < 1 {
		return dao.User{}, dao.ErrNotFound
	}

	return repo.GetByID(ctx, user.ID)
}

func (repo *UsersDB) GetByUsername(ctx context.Context, username string) (dao.User, error) {
	user := dao.User{
		Username: username,
	}
	var id string
	var role string
	var email string
	var logout int64
	var login int64
	var created int64
	var modified int64

	row := repo.db.QueryRowContext(ctx, `SELECT id, password, role, email, created, modified, last_logout_time, last_login_time FROM users WHERE username = ?;`,
		username,
	)
	err := row.Scan(
		&id,
		&user.Password,
		&role,
		&email,
		&created,
		&modified,
		&logout,
		&login,
	)

	if err != nil {
		return user, wrapDBError(err)
	}

	err = convertFromDB_UUID(id, &user.ID)
	if err != nil {
		return user, fmt.Errorf("stored UUID %q is invalid: %w", id, err)
	}
	err = convertFromDB_Email(email, &user.Email)
	if err != nil {
		return user, fmt.Errorf("stored email %q is invalid: %w", email, err)
	}
	err = convertFromDB_Time(logout, &user.LastLogoutTime)
	if err != nil {
		return user, fmt.Errorf("stored last_logout_time %d is invalid: %w", logout, err)
	}
	err = convertFromDB_Time(login, &user.LastLoginTime)
	if err != nil {
		return user, fmt.Errorf("stored last_login_time %d is invalid: %w", login, err)
	}
	err = convertFromDB_Time(created, &user.Created)
	if err != nil {
		return user, fmt.Errorf("stored created time %d is invalid: %w", created, err)
	}
	err = convertFromDB_Time(modified, &user.Modified)
	if err != nil {
		return user, fmt.Errorf("stored modified time %d is invalid: %w", modified, err)
	}
	err = convertFromDB_Role(role, &user.Role)
	if err != nil {
		return user, fmt.Errorf("stored role %q is invalid: %w", role, err)
	}

	return user, nil
}
func (repo *UsersDB) GetByID(ctx context.Context, id uuid.UUID) (dao.User, error) {
	user := dao.User{
		ID: id,
	}
	var role string
	var email string
	var logout int64
	var login int64
	var created int64
	var modified int64

	row := repo.db.QueryRowContext(ctx, `SELECT username, password, role, email, created, modified, last_logout_time, last_login_time FROM users WHERE id = ?;`,
		convertToDB_UUID(id),
	)
	err := row.Scan(
		&user.Username,
		&user.Password,
		&role,
		&email,
		&created,
		&modified,
		&logout,
		&login,
	)

	if err != nil {
		return user, wrapDBError(err)
	}

	err = convertFromDB_Email(email, &user.Email)
	if err != nil {
		return user, fmt.Errorf("stored email %q is invalid: %w", email, err)
	}
	err = convertFromDB_Time(logout, &user.LastLogoutTime)
	if err != nil {
		return user, fmt.Errorf("stored last_logout_time %d is invalid: %w", logout, err)
	}
	err = convertFromDB_Time(login, &user.LastLoginTime)
	if err != nil {
		return user, fmt.Errorf("stored last_login_time %d is invalid: %w", login, err)
	}
	err = convertFromDB_Time(created, &user.Created)
	if err != nil {
		return user, fmt.Errorf("stored created time %d is invalid: %w", created, err)
	}
	err = convertFromDB_Time(modified, &user.Modified)
	if err != nil {
		return user, fmt.Errorf("stored modified time %d is invalid: %w", modified, err)
	}
	err = convertFromDB_Role(role, &user.Role)
	if err != nil {
		return user, fmt.Errorf("stored role %q is invalid: %w", role, err)
	}

	return user, nil
}

func (repo *UsersDB) Delete(ctx context.Context, id uuid.UUID) (dao.User, error) {
	curVal, err := repo.GetByID(ctx, id)
	if err != nil {
		return curVal, err
	}

	res, err := repo.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, convertToDB_UUID(id))
	if err != nil {
		return curVal, wrapDBError(err)
	}
	rowsAff, err := res.RowsAffected()
	if err != nil {
		return curVal, wrapDBError(err)
	}
	if rowsAff < 1 {
		return curVal, dao.ErrNotFound
	}

	return curVal, nil
}

func (repo *UsersDB) Close() error {
	return repo.db.Close()
}
