// Package dbconv contains Converter functions for changing between native types
// and SQLite3 specific types.
//
// Currently, these are provided for convenience of collection and do not
// actually see use outside of manual calling of the members of the various
// Converters defined here.
package dbconv

import (
	"encoding/base64"
	"net/mail"
	"time"

	"github.com/dekarrin/jelly/dao"
	"github.com/dekarrin/jelly/serr"
	"github.com/google/uuid"
)

// Converter holds functions to convert a value to and from its database
// representation. The type param N is the native type and DB is the type in the
// database.
//
// TODO: sql.Value interface should eliminate this I believe. -deka
type Converter[N any, DB any] struct {
	ToDB   func(N) DB
	FromDB func(DB, *N) error // TODO: update this to just be func(DB) (N, error).
}

// Email converts email addresses to strings. When reading a string from the DB,
// an empty string will return a nil *mail.Address and a non-nil error.
var Email = Converter[*mail.Address, string]{
	ToDB: func(email *mail.Address) string {
		if email == nil {
			return ""
		}
		return email.Address
	},
	FromDB: func(s string, target **mail.Address) error {
		if s == "" {
			*target = nil
			return nil
		}

		email, err := mail.ParseAddress(s)
		if err != nil {
			return serr.New("", err, dao.ErrDecodingFailure)
		}

		*target = email
		return nil
	},
}

// UUID converts UUIDs to strings.
var UUID = Converter[uuid.UUID, string]{
	ToDB: uuid.UUID.String,
	FromDB: func(s string, target *uuid.UUID) error {
		u, err := uuid.Parse(s)
		if err != nil {
			return serr.New("", err, dao.ErrDecodingFailure)
		}
		*target = u
		return nil
	},
}

// Timestamp converts times into 64-bit unix timestamps.
var Timestamp = Converter[time.Time, int64]{
	ToDB: time.Time.Unix,
	FromDB: func(i int64, target *time.Time) error {
		t := time.Unix(i, 0)
		*target = t
		return nil
	},
}

// Base64EncodedBytes converts a slice of bytes to a base-64 encoded string.
var Base64EncodedBytes = Converter[[]byte, string]{
	ToDB: func(b []byte) string {
		if len(b) < 1 {
			return ""
		}
		return base64.StdEncoding.EncodeToString(b)
	},
	FromDB: func(s string, target *[]byte) error {
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return serr.New("", err, dao.ErrDecodingFailure)
		}
		*target = decoded
		return nil
	},
}
