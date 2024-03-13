// Package db provides data access objects compatible with the rest of the
// jelly framework packages.
//
// It includes basics as well as a sample implementation of Store that is
// compatible with jelly auth middleware.
//
// TODO: call this package db or somefin and move auth-specific to middleware.
// For sure by GHI-016 if not before.
package db

import (
	"database/sql/driver"
	"fmt"
	"net/mail"
	"time"

	"github.com/dekarrin/jelly"
)

func NowTimestamp() Timestamp {
	return Timestamp(time.Now().UTC())
}

// Timestamp is a time.Time variation that stores itself in the DB as the number
// of seconds since the Unix epoch. All timestamps returned from Scan will have
// timezone set to UTC.
type Timestamp time.Time

func (ts Timestamp) Format(layout string) string {
	return ts.Time().Format(layout)
}

func (ts Timestamp) Value() (driver.Value, error) {
	return time.Time(ts).Unix(), nil
}

func (ts *Timestamp) Scan(value interface{}) error {
	var stamp int64

	if iVal, ok := value.(int); ok {
		stamp = int64(iVal)
	} else if i8Val, ok := value.(int8); ok {
		stamp = int64(i8Val)
	} else if i16Val, ok := value.(int16); ok {
		stamp = int64(i16Val)
	} else if i32Val, ok := value.(int32); ok {
		stamp = int64(i32Val)
	} else if i64Val, ok := value.(int64); ok {
		stamp = i64Val
	} else {
		return jelly.NewError(fmt.Sprintf("not a signed integer value: %v", value), jelly.ErrDecodingFailure)
	}

	tVal := time.Unix(stamp, 0).In(time.UTC)
	*ts = Timestamp(tVal)
	return nil
}

func (ts Timestamp) Time() time.Time {
	return time.Time(ts)
}

// Email is a mail.Addresss that stores itself as a string.
type Email struct {
	V *mail.Address
}

func (em Email) String() string {
	if em.V == nil {
		return ""
	}
	return em.V.Address
}

func (em Email) Value() (driver.Value, error) {
	return em.String(), nil
}

func (em *Email) Scan(value interface{}) error {
	s, ok := value.(string)
	if !ok {
		return jelly.NewError(fmt.Sprintf("not a string value: %v", value), jelly.ErrDecodingFailure)
	}
	if s == "" {
		em.V = nil
		return nil
	}

	email, err := mail.ParseAddress(s)
	if err != nil {
		return jelly.NewError("", err, jelly.ErrDecodingFailure)
	}

	em.V = email
	return nil
}
