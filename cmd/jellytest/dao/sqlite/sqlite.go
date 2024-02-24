package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/cmd/jellytest/dao"
)

func New(cfg jelly.DatabaseConfig) (jelly.Store, error) {
	err := os.MkdirAll(cfg.DataDir, 0770)
	if err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	filename := "templates.db"
	if cfg.DataFile != "" {
		filename = cfg.DataFile
	}

	dbPath := filepath.Join(cfg.DataDir, filename)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, jelly.WrapDBError(err)
	}

	ds := dao.Datastore{
		DB: db,
	}

	ds.EchoTemplates, err = NewTemplates(ds.DB, "echo_templates")
	if err != nil {
		return nil, fmt.Errorf("open echo_templates table: %w", err)
	}
	ds.NiceTemplates, err = NewTemplates(ds.DB, "hello_nice_templates")
	if err != nil {
		return nil, fmt.Errorf("open hello_nice_templates table: %w", err)
	}
	ds.RudeTemplates, err = NewTemplates(ds.DB, "hello_rude_templates")
	if err != nil {
		return nil, fmt.Errorf("open hello_rude_templates table: %w", err)
	}
	ds.SecretTemplates, err = NewTemplates(ds.DB, "hello_secret_templates")
	if err != nil {
		return nil, fmt.Errorf("open hello_secret_templates table: %w", err)
	}

	return ds, nil
}
