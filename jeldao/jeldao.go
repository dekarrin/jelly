// Package jeldao provides data access objects compatible with the rest of the
// jelly framework packages.
package jeldao

import "errors"

var (
	ErrConstraintViolation = errors.New("a uniqueness constraint was violated")
	ErrNotFound            = errors.New("the requested resource was not found")
	ErrDecodingFailure     = errors.New("field could not be decoded from DB storage format to model format")
)

// Store holds all repositories.
type Store interface {
	Repo(name string) Repository[any]
	Close() error
}
