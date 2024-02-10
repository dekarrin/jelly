package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/cmd/jellytest/dao"
	"github.com/dekarrin/jelly/dao/sqlite/dbconv"
	"github.com/google/uuid"
)

type templateStore struct {
	db    *sql.DB
	table string
}

func NewTemplates(db *sql.DB, table string) (dao.Templates, error) {
	ts := &templateStore{
		db:    db,
		table: table,
	}

	_, err := ts.db.Exec(`
		CREATE TABLE IF NOT EXISTS ` + table + ` (
			id TEXT NOT NULL PRIMARY KEY,
			content TEXT NOT NULL UNIQUE,
			creator TEXT NOT NULL
		);`)
	if err != nil {
		return nil, jelly.WrapSqliteError(err)
	}

	return ts, nil
}

func (store *templateStore) Create(ctx context.Context, t dao.Template) (dao.Template, error) {
	newUUID, err := uuid.NewRandom()
	if err != nil {
		return dao.Template{}, fmt.Errorf("could not generate ID: %w", err)
	}

	stmt, err := store.db.Prepare(`
		INSERT INTO ` + store.table + ` (
			id, content, creator
		)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return dao.Template{}, jelly.WrapSqliteError(err)
	}

	_, err = stmt.ExecContext(
		ctx,
		dbconv.UUID.ToDB(newUUID),
		t.Content,
		t.Creator,
	)
	if err != nil {
		return dao.Template{}, jelly.WrapSqliteError(err)
	}

	return store.Get(ctx, newUUID)
}

func (store *templateStore) Get(ctx context.Context, id uuid.UUID) (dao.Template, error) {
	t := dao.Template{
		ID: id,
	}

	row := store.db.QueryRowContext(ctx, `
		SELECT content, creator FROM `+store.table+` 
		WHERE id = ?;
	`, dbconv.UUID.ToDB(id))
	err := row.Scan(
		&t.Content,
		&t.Creator,
	)

	if err != nil {
		return t, jelly.WrapSqliteError(err)
	}

	return t, nil
}

func (store *templateStore) GetAll(ctx context.Context) ([]dao.Template, error) {
	rows, err := store.db.QueryContext(ctx, `
		SELECT id, content, creator
		FROM `+store.table+`;
	`)
	if err != nil {
		return nil, jelly.WrapSqliteError(err)
	}
	defer rows.Close()

	var all []dao.Template

	for rows.Next() {
		var t dao.Template
		var id string
		err = rows.Scan(
			&id,
			&t.Content,
			&t.Creator,
		)

		if err != nil {
			return nil, jelly.WrapSqliteError(err)
		}

		err = dbconv.UUID.FromDB(id, &t.ID)
		if err != nil {
			return all, fmt.Errorf("stored UUID %q is invalid: %w", id, err)
		}

		all = append(all, t)
	}

	if err := rows.Err(); err != nil {
		return all, jelly.WrapSqliteError(err)
	}

	return all, nil
}

func (store *templateStore) Update(ctx context.Context, id uuid.UUID, t dao.Template) (dao.Template, error) {
	res, err := store.db.ExecContext(ctx, `
		UPDATE `+store.table+`
		SET id=?, content=?, creator=?
		WHERE id=?;`,
		dbconv.UUID.ToDB(t.ID),
		t.Content,
		t.Creator,
		dbconv.UUID.ToDB(id),
	)
	if err != nil {
		return dao.Template{}, jelly.WrapSqliteError(err)
	}
	rowsAff, err := res.RowsAffected()
	if err != nil {
		return dao.Template{}, jelly.WrapSqliteError(err)
	}
	if rowsAff < 1 {
		return dao.Template{}, jelly.DBErrNotFound
	}

	return store.Get(ctx, t.ID)
}

func (store *templateStore) Delete(ctx context.Context, id uuid.UUID) (dao.Template, error) {
	curVal, err := store.Get(ctx, id)
	if err != nil {
		return curVal, err
	}

	res, err := store.db.ExecContext(ctx, `
		DELETE FROM `+store.table+`
		WHERE id = ?
	`, dbconv.UUID.ToDB(id))
	if err != nil {
		return curVal, jelly.WrapSqliteError(err)
	}
	rowsAff, err := res.RowsAffected()
	if err != nil {
		return curVal, jelly.WrapSqliteError(err)
	}
	if rowsAff < 1 {
		return curVal, jelly.DBErrNotFound
	}

	return curVal, nil
}

func (store *templateStore) GetRandom(ctx context.Context) (dao.Template, error) {
	tx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return dao.Template{}, jelly.WrapSqliteError(err)
	}
	defer tx.Rollback() // read-only, don't prop changes

	// need a count first
	var count int
	row := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+store.table)
	err = row.Scan(
		&count,
	)
	if err != nil {
		return dao.Template{}, jelly.WrapSqliteError(err)
	}

	if count == 0 {
		return dao.Template{}, jelly.DBErrNotFound
	}

	// select one
	selected := rand.Intn(count)

	// get that one
	var t dao.Template
	stmt, err := tx.PrepareContext(ctx, `SELECT id, content, creator FROM `+store.table+` ORDER BY id LIMIT 1 OFFSET ?`)
	if err != nil {
		return dao.Template{}, jelly.WrapSqliteError(err)
	}
	row = stmt.QueryRowContext(ctx, selected)
	var id string
	err = row.Scan(
		&id,
		&t.Content,
		&t.Creator,
	)
	if err != nil {
		return dao.Template{}, jelly.WrapSqliteError(err)
	}
	err = dbconv.UUID.FromDB(id, &t.ID)
	if err != nil {
		return t, fmt.Errorf("stored UUID %q is invalid: %w", id, err)
	}

	return t, nil
}

func (store *templateStore) Close() error {
	return store.db.Close()
}
