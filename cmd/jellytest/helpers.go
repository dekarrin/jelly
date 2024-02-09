package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/cmd/jellytest/dao"
	"github.com/dekarrin/jelly/logging"
)

func initDBWithMessages(ctx context.Context, log logging.Logger, repo dao.Messages, creator string, msgs []string) error {
	for _, m := range msgs {
		dbMsg := dao.Message{
			Content: m,
			Creator: creator,
		}
		created, err := repo.Create(ctx, dbMsg)
		if err != nil {
			if !errors.Is(err, jelly.DBErrConstraintViolation) {
				return fmt.Errorf("create initial messages: %w", err)
			} else {
				log.Tracef("Skipping adding message to DB via config; already exists: %q", m)
			}
		} else {
			log.Debugf("Added new message to DB via config: %s - %q", created.ID, created.Content)
		}
	}
	return nil
}
