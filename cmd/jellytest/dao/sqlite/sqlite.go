package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/cmd/jellytest/dao"
	"github.com/dekarrin/jelly/config"
	jellydao "github.com/dekarrin/jelly/dao"
)

func New(cfg config.Database) (jellydao.Store, error) {
	err := os.MkdirAll(cfg.DataDir, 0770)
	if err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	filename := "messages.db"
	if cfg.DataFile != "" {
		filename = cfg.DataFile
	}

	dbPath := filepath.Join(cfg.DataDir, filename)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, jelly.WrapSqliteError(err)
	}

	ds := dao.Datastore{
		DB: db,
	}

	ds.EchoMessages, err = NewMessageStore(ds.DB, "echo_messages")
	if err != nil {
		return nil, fmt.Errorf("open echo_messages table: %w", err)
	}
	ds.NiceMessages, err = NewMessageStore(ds.DB, "hello_nice_messages")
	if err != nil {
		return nil, fmt.Errorf("open hello_nice_messages table: %w", err)
	}
	ds.RudeMessages, err = NewMessageStore(ds.DB, "hello_rude_messages")
	if err != nil {
		return nil, fmt.Errorf("open hello_rude_messages table: %w", err)
	}
	ds.SecretMessages, err = NewMessageStore(ds.DB, "hello_secret_messages")
	if err != nil {
		return nil, fmt.Errorf("open hello_secret_messages table: %w", err)
	}

	return ds, nil
}
