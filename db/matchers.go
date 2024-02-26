package db

import (
	"database/sql/driver"
	"time"

	"github.com/google/uuid"
)

// This file contains matchers to be used with DATA-DOG/go-sqlmock.

// AnyUUID is a DATA-DOG/go-sqlmock compatible matcher used for matching against
// any UUID that is encoded as a string, byte array, or directly as a uuid.UUID.
type AnyUUID struct{}

func (m AnyUUID) Match(v driver.Value) bool {
	strUUID, ok := v.(string)
	if ok {
		_, err := uuid.Parse(strUUID)
		return err == nil
	}

	bUUID, ok := v.([]byte)
	if ok {
		_, err := uuid.FromBytes(bUUID)
		return err == nil
	}

	_, ok = v.(uuid.UUID)
	return ok
}

// AnyTime is a DATA-DOG/go-sqlmock compatible matcher used for matching against
// any time.Time that is encoded as an RFC-3339 string, a unix epoch timestamp,
// directly as a time.Time, or as a db.Timestamp.
//
// If Except is set, then it will match any time besides the given one. If
// After is set, it will match any time that comes after the given one. If
// Before is set, it will match any time that comes before the given one. These
// may be combined; if multiple are given, their conditions are AND'd together.
type AnyTime struct {
	Except *time.Time
	After  *time.Time
	Before *time.Time
}

func (m AnyTime) Match(v driver.Value) bool {
	var t time.Time
	var err error

	switch typedV := v.(type) {
	case string:
		t, err = time.Parse(time.RFC3339, typedV)
		if err != nil {
			return false
		}
	case int:
		t = time.Unix(int64(typedV), 0)
	case int64:
		t = time.Unix(typedV, 0)
	case int32:
		t = time.Unix(int64(typedV), 0)
	case int16:
		t = time.Unix(int64(typedV), 0)
	case int8:
		t = time.Unix(int64(typedV), 0)
	case Timestamp:
		t = typedV.Time()
	case time.Time:
		t = typedV
	default:
		return false
	}

	if m.Except != nil {
		if t.Equal(*m.Except) {
			return false
		}
	}
	if m.After != nil {
		if !t.After(*m.After) {
			return false
		}
	}
	if m.Before != nil {
		if !t.Before(*m.Before) {
			return false
		}
	}

	return true
}
