package dao

import (
	"context"

	"github.com/google/uuid"
)

type Template struct {
	ID      uuid.UUID
	Content string
	Creator string
}

func (m Template) ModelID() uuid.UUID {
	return m.ID
}

// TODO: this has nothing to do with jelly/dao's Repos. Should that be updated
// to be specific to jellyauth and moved out of general DAO?
type Templates interface {
	Create(context.Context, Template) (Template, error)
	Get(context.Context, uuid.UUID) (Template, error)
	GetAll(context.Context) ([]Template, error)
	Update(context.Context, uuid.UUID, Template) (Template, error)
	Delete(context.Context, uuid.UUID) (Template, error)
	GetRandom(context.Context) (Template, error)
	Close() error
}
