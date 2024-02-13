// Package dbconv contains Converter functions for changing between native types
// and SQLite3 specific types.
//
// Currently, these are provided for convenience of collection and do not
// actually see use outside of manual calling of the members of the various
// Converters defined here.
package dbconv

// Converter holds functions to convert a value to and from its database
// representation. The type param N is the native type and DB is the type in the
// database.
//
// TODO: sql.Value interface should eliminate this I believe. -deka
type Converter[N any, DB any] struct {
	ToDB   func(N) DB
	FromDB func(DB, *N) error // TODO: update this to just be func(DB) (N, error).
}
