package db

import (
	"database/sql/driver"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func Test_SQLMock_Matcher(t *testing.T) {
	var nilByteSlice []byte

	testCases := []struct {
		name   string
		m      sqlmock.Argument
		input  driver.Value
		expect bool
	}{
		{
			name:   "AnyUUID - string input - valid normal input",
			m:      AnyUUID{},
			input:  "9c9ca5e9-4305-4bfa-ab0d-a9e08ceb3c7b",
			expect: true,
		},
		{
			name:   "AnyUUID - string input - valid null uuid",
			m:      AnyUUID{},
			input:  "00000000-0000-0000-0000-000000000000",
			expect: true,
		},
		{
			name:   "AnyUUID - string input - empty",
			m:      AnyUUID{},
			input:  "",
			expect: false,
		},
		{
			name:   "AnyUUID - string input - invalid",
			m:      AnyUUID{},
			input:  "not a UUID",
			expect: false,
		},
		{
			name:   "AnyUUID - []byte input - valid normal input",
			m:      AnyUUID{},
			input:  []byte{0x67, 0x60, 0x02, 0x42, 0xd9, 0xad, 0x48, 0x1e, 0xae, 0x4b, 0xa5, 0x40, 0x12, 0x62, 0xaa, 0x5a},
			expect: true,
		},
		{
			name:   "AnyUUID - []byte input - valid null uuid",
			m:      AnyUUID{},
			input:  []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			expect: true,
		},
		{
			// TODO: call out this limitation in docs of AnyUUID.
			name:   "AnyUUID - []byte input - invalid variant but correct size parses",
			m:      AnyUUID{},
			input:  []byte{0x4e, 0x4f, 0x54, 0x20, 0x41, 0x20, 0x55, 0x55, 0x49, 0x44, 0x20, 0x48, 0x45, 0x52, 0x45, 0x21},
			expect: true,
		},
		{
			name:   "AnyUUID - []byte input - empty",
			m:      AnyUUID{},
			input:  []byte{},
			expect: false,
		},
		{
			name:   "AnyUUID - []byte input - nil",
			m:      AnyUUID{},
			input:  nilByteSlice,
			expect: false,
		},
		{
			name:   "AnyUUID - []byte input - invalid text",
			m:      AnyUUID{},
			input:  []byte{0x4e, 0x4f, 0x54, 0x20, 0x41, 0x20, 0x55, 0x55, 0x49, 0x44},
			expect: false,
		},
		{
			name:   "AnyUUID - []byte input - invalid text, 17 bytes",
			m:      AnyUUID{},
			input:  []byte{0x4e, 0x4f, 0x54, 0x20, 0x41, 0x20, 0x55, 0x55, 0x49, 0x44, 0x20, 0x48, 0x45, 0x52, 0x45, 0x21, 0x21},
			expect: false,
		},
		{
			name:   "AnyUUID - uuid.UUID input - normal",
			m:      AnyUUID{},
			input:  uuid.MustParse("f427d0c0-60d1-4759-8a30-9de424f54ba0"),
			expect: true,
		},
		{
			name:   "AnyUUID - uuid.UUID input - nil uuid",
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
