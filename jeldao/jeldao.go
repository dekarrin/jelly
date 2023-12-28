// Package jeldao provides data access objects compatible with the rest of the
// jelly framework packages.
package jeldao

import (
	"context"
	"errors"
)

var (
	ErrConstraintViolation = errors.New("a uniqueness constraint was violated")
	ErrNotFound            = errors.New("the requested resource was not found")
	ErrDecodingFailure     = errors.New("field could not be decoded from DB storage format to model format")
)

type ID interface {
	Value() interface{}
	Fields() map[string]interface{}
}

type Model interface {
	ID() ID
	Indexed() [][]string
	Fields() map[string]interface{}
}

// Store holds all repositories.
type Store interface {

	// Repo must return a Repo. Type parameters make declaring that here tricky;
	// we don't go with Repo[Model] because that explicitly requires a Model,
	// not an implementor of.
	Repo(name string) any

	Close() error
}

type Repo[E Model] interface {

	// Create creates a new model in the DB based on the provided one. Some
	// attributes in the provided one might not be used; for instance, many
	// Repos will automatically set the ID of new entities on creation, ignoring
	// any initially set ID. It is up to implementors to decide which attributes
	// are used.
	//
	// This returns the object as it appears in the DB after creation.
	Create(context.Context, E) (E, error)

	Get(context.Context, ID) (E, error)

	GetAll(context.Context)
}
