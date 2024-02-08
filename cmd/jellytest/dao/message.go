package dao

import (
	"context"

	"github.com/google/uuid"
)

type Message struct {
	ID      uuid.UUID
	Content string
	Creator string
}

func (m Message) ModelID() uuid.UUID {
	return m.ID
}

// TODO: this has nothing to do with jelly/dao's Repos. Should that be updated
// to be specific to jellyauth and moved out of general DAO?
type Messages interface {
	Create(context.Context, Message) (Message, error)
	Get(context.Context, uuid.UUID) (Message, error)
	GetAll(context.Context) ([]Message, error)
	Update(context.Context, uuid.UUID, Message) (Message, error)
	Delete(context.Context, uuid.UUID) (Message, error)
	GetRandom(context.Context) (Message, error)
	Close() error
}
