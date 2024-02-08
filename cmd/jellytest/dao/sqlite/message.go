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

type messageStore struct {
	db    *sql.DB
	table string
}

func NewMessageStore(table string, db *sql.DB) (dao.Messages, error) {
	ms := &messageStore{
		db:    db,
		table: table,
	}

	_, err := ms.db.Exec(`
		CREATE TABLE IF NOT EXISTS ` + table + ` (
			id TEXT NOT NULL PRIMARY KEY,
			content TEXT NOT NULL,
			creator TEXT NOT NULL,
		);`)
	if err != nil {
		return nil, jelly.WrapSqliteError(err)
	}

	return ms, nil
}

func (store *messageStore) Create(ctx context.Context, msg dao.Message) (dao.Message, error) {
	newUUID, err := uuid.NewRandom()
	if err != nil {
		return dao.Message{}, fmt.Errorf("could not generate ID: %w", err)
	}

	stmt, err := store.db.Prepare(`
		INSERT INTO ` + store.table + ` (
			id, content, creator
		)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return dao.Message{}, jelly.WrapSqliteError(err)
	}

	_, err = stmt.ExecContext(
		ctx,
		dbconv.UUID.ToDB(newUUID),
		msg.Content,
		msg.Creator,
	)
	if err != nil {
		return dao.Message{}, jelly.WrapSqliteError(err)
	}

	return store.Get(ctx, newUUID)
}

func (store *messageStore) Get(ctx context.Context, id uuid.UUID) (dao.Message, error) {
	msg := dao.Message{
		ID: id,
	}

	row := store.db.QueryRowContext(ctx, `
		SELECT content, creator FROM `+store.table+` 
		WHERE id = ?;
	`, dbconv.UUID.ToDB(id))
	err := row.Scan(
		&msg.Content,
		&msg.Creator,
	)

	if err != nil {
		return msg, jelly.WrapSqliteError(err)
	}

	return msg, nil
}

func (store *messageStore) GetAll(ctx context.Context) ([]dao.Message, error) {
	rows, err := store.db.QueryContext(ctx, `
		SELECT id, content, creator
		FROM `+store.table+`;
	`)
	if err != nil {
		return nil, jelly.WrapSqliteError(err)
	}
	defer rows.Close()

	var all []dao.Message

	for rows.Next() {
		var msg dao.Message
		var id string
		err = rows.Scan(
			&id,
			&msg.Content,
			&msg.Creator,
		)

		if err != nil {
			return nil, jelly.WrapSqliteError(err)
		}

		err = dbconv.UUID.FromDB(id, &msg.ID)
		if err != nil {
			return all, fmt.Errorf("stored UUID %q is invalid: %w", id, err)
		}

		all = append(all, msg)
	}

	if err := rows.Err(); err != nil {
		return all, jelly.WrapSqliteError(err)
	}

	return all, nil
}

func (store *messageStore) Update(ctx context.Context, id uuid.UUID, msg dao.Message) (dao.Message, error) {
	res, err := store.db.ExecContext(ctx, `
		UPDATE `+store.table+`
		SET id=?, content=?, creator=?
		WHERE id=?;`,
		dbconv.UUID.ToDB(msg.ID),
		msg.Content,
		msg.Creator,
		dbconv.UUID.ToDB(id),
	)
	if err != nil {
		return dao.Message{}, jelly.WrapSqliteError(err)
	}
	rowsAff, err := res.RowsAffected()
	if err != nil {
		return dao.Message{}, jelly.WrapSqliteError(err)
	}
	if rowsAff < 1 {
		return dao.Message{}, jelly.DBErrNotFound
	}

	return store.Get(ctx, msg.ID)
}

func (store *messageStore) Delete(ctx context.Context, id uuid.UUID) (dao.Message, error) {
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

func (store *messageStore) GetRandom(ctx context.Context) (dao.Message, error) {
	tx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return dao.Message{}, jelly.WrapSqliteError(err)
	}
	defer tx.Rollback() // read-only, don't prop changes

	// need a count first
	var count int
	row := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+store.table)
	err = row.Scan(
		&count,
	)
	if err != nil {
		return dao.Message{}, jelly.WrapSqliteError(err)
	}

	if count == 0 {
		return dao.Message{}, jelly.DBErrNotFound
	}

	// select one
	selected := rand.Intn(count)

	// get that one
	var msg dao.Message
	stmt, err := tx.PrepareContext(ctx, `SELECT id, content, creator FROM `+store.table+` ORDER BY id LIMIT 1 OFFSET ?`)
	if err != nil {
		return dao.Message{}, jelly.WrapSqliteError(err)
	}
	row = stmt.QueryRowContext(ctx, selected)
	var id string
	err = row.Scan(
		&id,
		&msg.Content,
		&msg.Creator,
	)
	if err != nil {
		return dao.Message{}, jelly.WrapSqliteError(err)
	}
	err = dbconv.UUID.FromDB(id, &msg.ID)
	if err != nil {
		return msg, fmt.Errorf("stored UUID %q is invalid: %w", id, err)
	}

	return msg, nil
}

func (store *messageStore) Close() error {
	return store.db.Close()
}
