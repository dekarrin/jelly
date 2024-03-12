package db

import (
	"database/sql/driver"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func Test_AnyUUID_Matches(t *testing.T) {
	var nilByteSlice []byte

	testCases := []struct {
		name   string
		m      AnyUUID
		input  driver.Value
		expect bool
	}{
		{
			name:   "string input - valid normal input",
			m:      AnyUUID{},
			input:  "9c9ca5e9-4305-4bfa-ab0d-a9e08ceb3c7b",
			expect: true,
		},
		{
			name:   "string input - valid null uuid",
			m:      AnyUUID{},
			input:  "00000000-0000-0000-0000-000000000000",
			expect: true,
		},
		{
			name:   "string input - empty",
			m:      AnyUUID{},
			input:  "",
			expect: false,
		},
		{
			name:   "string input - invalid",
			m:      AnyUUID{},
			input:  "not a UUID",
			expect: false,
		},
		{
			name:   "[]byte input - valid normal input",
			m:      AnyUUID{},
			input:  []byte{0x67, 0x60, 0x02, 0x42, 0xd9, 0xad, 0x48, 0x1e, 0xae, 0x4b, 0xa5, 0x40, 0x12, 0x62, 0xaa, 0x5a},
			expect: true,
		},
		{
			name:   "[]byte input - valid null uuid",
			m:      AnyUUID{},
			input:  []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			expect: true,
		},
		{
			// TODO: call out this limitation in docs of AnyUUID.
			name:   "[]byte input - invalid variant but correct size parses",
			m:      AnyUUID{},
			input:  []byte{0x4e, 0x4f, 0x54, 0x20, 0x41, 0x20, 0x55, 0x55, 0x49, 0x44, 0x20, 0x48, 0x45, 0x52, 0x45, 0x21},
			expect: true,
		},
		{
			name:   "[]byte input - empty",
			m:      AnyUUID{},
			input:  []byte{},
			expect: false,
		},
		{
			name:   "[]byte input - nil",
			m:      AnyUUID{},
			input:  nilByteSlice,
			expect: false,
		},
		{
			name:   "[]byte input - invalid text",
			m:      AnyUUID{},
			input:  []byte{0x4e, 0x4f, 0x54, 0x20, 0x41, 0x20, 0x55, 0x55, 0x49, 0x44},
			expect: false,
		},
		{
			name:   "[]byte input - invalid text, 17 bytes",
			m:      AnyUUID{},
			input:  []byte{0x4e, 0x4f, 0x54, 0x20, 0x41, 0x20, 0x55, 0x55, 0x49, 0x44, 0x20, 0x48, 0x45, 0x52, 0x45, 0x21, 0x21},
			expect: false,
		},
		{
			name:   "uuid.UUID input - normal",
			m:      AnyUUID{},
			input:  uuid.MustParse("f427d0c0-60d1-4759-8a30-9de424f54ba0"),
			expect: true,
		},
		{
			name:   "uuid.UUID input - nil uuid",
			m:      AnyUUID{},
			input:  uuid.Nil,
			expect: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			actual := tc.m.Match(tc.input)

			assert.Equal(tc.expect, actual)
		})
	}
}

func Test_AnyTime_Matches(t *testing.T) {
	testCases := []struct {
		name   string
		m      AnyTime
		input  driver.Value
		expect bool
	}{
		// with no other members set
		{
			name:   "any - string - RFC-3339 with Z offset",
			input:  "2021-01-01T02:07:14Z",
			expect: true,
		},
		{
			name:   "any - string - RFC-3339 with explicit offset",
			input:  "2020-12-31T21:07:14-05:00",
			expect: true,
		},
		{
			name:   "any - string - invalid RFC-3339 (no time)",
			input:  "2020-12-31",
			expect: false,
		},
		{
			name:   "any - int - positive",
			input:  1710246273,
			expect: true,
		},
		{
			name:   "any - zero",
			input:  0,
			expect: true,
		},
		{
			name:   "any - negative",
			input:  -1710246273,
			expect: true,
		},
		{
			name:   "any - db.Timestamp - zero",
			input:  Timestamp{},
			expect: true,
		},
		{
			name:   "any - db.Timestamp - non-zero",
			input:  NowTimestamp(),
			expect: true,
		},
		{
			name:   "any - time.Time - zero",
			input:  time.Time{},
			expect: true,
		},
		{
			name:   "any - time.Time - non-zero",
			input:  time.Now(),
			expect: true,
		},

		// any except
		{
			name:   "any except with UTC zone - string = excluded",
			input:  "2021-01-01T02:07:14Z",
			m:      AnyTime{Except: ref(time.Date(2021, 1, 1, 2, 7, 14, 0, time.UTC))},
			expect: false,
		},
		{
			name:   "any except with local zone - string = excluded",
			input:  "2021-01-01T02:07:14Z",
			m:      AnyTime{Except: ref(time.Date(2021, 1, 1, 1, 7, 14, 0, time.FixedZone("hourbehind", -3600)))},
			expect: false,
		},
		{
			name:   "any except - string = included",
			input:  "2021-01-01T02:07:14Z",
			m:      AnyTime{Except: ref(time.Date(2021, 1, 1, 3, 7, 14, 0, time.UTC))},
			expect: true,
		},
		{
			name:   "any except - int = excluded",
			input:  1609466834,
			m:      AnyTime{Except: ref(time.Date(2021, 1, 1, 2, 7, 14, 0, time.UTC))},
			expect: false,
		},
		{
			name:   "any except - int = included",
			input:  1609466835,
			m:      AnyTime{Except: ref(time.Date(2021, 1, 1, 2, 7, 14, 0, time.UTC))},
			expect: true,
		},
		{
			name:   "any except - db.Timestamp = excluded",
			input:  Timestamp(time.Date(2021, 1, 1, 2, 7, 14, 0, time.UTC)),
			m:      AnyTime{Except: ref(time.Date(2021, 1, 1, 2, 7, 14, 0, time.UTC))},
			expect: false,
		},
		{
			name:   "any except - db.Timestamp = included",
			input:  Timestamp(time.Date(2021, 1, 1, 2, 7, 14, 1, time.UTC)),
			m:      AnyTime{Except: ref(time.Date(2021, 1, 1, 2, 7, 14, 0, time.UTC))},
			expect: true,
		},
		{
			name:   "any except - time.Time = excluded",
			input:  time.Date(2021, 1, 1, 2, 7, 14, 0, time.UTC),
			m:      AnyTime{Except: ref(time.Date(2021, 1, 1, 2, 7, 14, 0, time.UTC))},
			expect: false,
		},
		{
			name:   "any except - time.Time = included",
			input:  time.Date(2021, 1, 1, 2, 7, 14, 1, time.UTC),
			m:      AnyTime{Except: ref(time.Date(2021, 1, 1, 2, 7, 14, 0, time.UTC))},
			expect: true,
		},

		// any equal to
		{
			name:   "any equal - string = excluded",
			input:  "2021-01-01T02:07:15Z",
			m:      AnyTime{EqualTo: ref(time.Date(2021, 1, 1, 3, 7, 14, 0, time.UTC))},
			expect: false,
		},
		{
			name:   "any equal with UTC zone - string = included",
			input:  "2021-01-01T02:07:14Z",
			m:      AnyTime{EqualTo: ref(time.Date(2021, 1, 1, 2, 7, 14, 0, time.UTC))},
			expect: true,
		},
		{
			name:   "any equal with local zone - string = included",
			input:  "2021-01-01T02:07:14Z",
			m:      AnyTime{EqualTo: ref(time.Date(2021, 1, 1, 1, 7, 14, 0, time.FixedZone("hourbehind", -3600)))},
			expect: true,
		},
		{
			name:   "any equal - int = excluded",
			input:  1609466835,
			m:      AnyTime{EqualTo: ref(time.Date(2021, 1, 1, 2, 7, 14, 0, time.UTC))},
			expect: false,
		},
		{
			name:   "any except - int = included",
			input:  1609466834,
			m:      AnyTime{EqualTo: ref(time.Date(2021, 1, 1, 2, 7, 14, 0, time.UTC))},
			expect: true,
		},
		{
			name:   "any except - db.Timestamp = excluded",
			input:  Timestamp(time.Date(2021, 1, 1, 2, 7, 14, 1, time.UTC)),
			m:      AnyTime{EqualTo: ref(time.Date(2021, 1, 1, 2, 7, 14, 0, time.UTC))},
			expect: false,
		},
		{
			name:   "any except - db.Timestamp = included",
			input:  Timestamp(time.Date(2021, 1, 1, 2, 7, 14, 0, time.UTC)),
			m:      AnyTime{EqualTo: ref(time.Date(2021, 1, 1, 2, 7, 14, 0, time.UTC))},
			expect: true,
		},
		{
			name:   "any except - time.Time = excluded",
			input:  time.Date(2021, 1, 1, 2, 7, 14, 1, time.UTC),
			m:      AnyTime{EqualTo: ref(time.Date(2021, 1, 1, 2, 7, 14, 0, time.UTC))},
			expect: false,
		},
		{
			name:   "any except - time.Time = included",
			input:  time.Date(2021, 1, 1, 2, 7, 14, 0, time.UTC),
			m:      AnyTime{EqualTo: ref(time.Date(2021, 1, 1, 2, 7, 14, 0, time.UTC))},
			expect: true,
		},

		// any after - str inc
		// any after - str exc
		// any after - int inc
		// any after - int excluded
		// any after - db.timestamp inc
		// any after - db.timestamp exc
		// any after - time.Time inc
		// any after - time.Time exc

		// any before - str inc
		// any before - str exc
		// any before - int inc
		// any before - int excluded
		// any before - db.timestamp inc
		// any before - db.timestamp exc
		// any before - time.Time inc
		// any before - time.Time exc
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			actual := tc.m.Match(tc.input)

			assert.Equal(tc.expect, actual)
		})
	}
}

func ref[E any](v E) *E {
	return &v
}
