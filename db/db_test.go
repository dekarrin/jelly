package db

import (
	"net/mail"
	"testing"
	"time"

	"github.com/dekarrin/jelly"
	"github.com/stretchr/testify/assert"
)

func Test_NowTimestamp(t *testing.T) {
	startTime := time.Now()
	time.Sleep(5 * time.Millisecond)

	actual := NowTimestamp()

	time.Sleep(5 * time.Millisecond)
	endTime := time.Now()

	assert := assert.New(t)
	assert.Less(startTime, actual.Time(), "actual timestamp not after start")
	assert.Less(actual.Time(), endTime, "actual timestamp not before end")
}

func Test_Timestamp_Scan(t *testing.T) {
	testCases := []struct {
		name             string
		value            interface{}
		expect           Timestamp
		expectErrToMatch []error
	}{
		{
			name:   "normal number (int)",
			value:  1239639181,
			expect: Timestamp(time.Date(2009, 4, 13, 16, 13, 1, 0, time.UTC)),
		},
		{
			name:   "normal number (int8)",
			value:  int8(120),
			expect: Timestamp(time.Date(1970, 1, 1, 0, 2, 0, 0, time.UTC)),
		},
		{
			name:   "normal number (int16)",
			value:  int16(32413),
			expect: Timestamp(time.Date(1970, 1, 1, 9, 0, 13, 0, time.UTC)),
		},
		{
			name:   "normal number (int32)",
			value:  int32(69413),
			expect: Timestamp(time.Date(1970, 1, 1, 19, 16, 53, 0, time.UTC)),
		},
		{
			name:   "normal number (int64)",
			value:  int64(1713024781),
			expect: Timestamp(time.Date(2024, 4, 13, 16, 13, 1, 0, time.UTC)),
		},
		{
			name:             "bad input",
			value:            "sup",
			expectErrToMatch: []error{jelly.ErrDecodingFailure},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			var actual Timestamp
			err := actual.Scan(tc.value)

			if tc.expectErrToMatch == nil {
				if !assert.NoError(err) {
					return
				}
				assert.Equal(tc.expect.Format(time.RFC3339), actual.Format(time.RFC3339))
			} else {
				if !assert.Error(err) {
					return
				}
				if !assert.IsType(jelly.Error{}, err, "wrong type error") {
					return
				}

				for _, expectMatch := range tc.expectErrToMatch {
					assert.ErrorIs(err, expectMatch)
				}
			}
		})
	}
}

func Test_Timestamp_Value(t *testing.T) {
	testCases := []struct {
		name   string
		input  Timestamp
		expect int64
	}{
		{
			name:   "normal",
			input:  Timestamp(time.Date(2009, 4, 13, 16, 13, 1, 0, time.UTC)),
			expect: 1239639181,
		},
		{
			name:   "negative timestamp",
			input:  Timestamp(time.Date(1969, 12, 25, 4, 13, 0, 0, time.UTC)),
			expect: -589620,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			actual, _ := tc.input.Value()

			assert.Equal(tc.expect, actual)
		})
	}
}

func Test_Email_Scan(t *testing.T) {
	testCases := []struct {
		name             string
		value            interface{}
		expect           Email
		expectErrToMatch []error
	}{
		{
			name:   "an email",
			value:  "bob@example.com",
			expect: Email{V: &mail.Address{Address: "bob@example.com"}},
		},
		{
			name:   "empty",
			value:  "",
			expect: Email{},
		},
		{
			name:             "not a string",
			value:            7,
			expectErrToMatch: []error{jelly.ErrDecodingFailure},
		},
		{
			name:             "invalid email",
			value:            "88888888",
			expectErrToMatch: []error{jelly.ErrDecodingFailure},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			var actual Email
			err := actual.Scan(tc.value)

			if tc.expectErrToMatch == nil {
				if !assert.NoError(err) {
					return
				}
				assert.Equal(tc.expect, actual)
			} else {
				if !assert.Error(err) {
					return
				}
				if !assert.IsType(jelly.Error{}, err, "wrong type error") {
					return
				}

				for _, expectMatch := range tc.expectErrToMatch {
					assert.ErrorIs(err, expectMatch)
				}
			}
		})
	}
}

func Test_Email_Value(t *testing.T) {
	testCases := []struct {
		name   string
		input  Email
		expect string
	}{
		{
			name:   "normal",
			input:  Email{V: &mail.Address{Address: "jude@iwantobelieve.com"}},
			expect: "jude@iwantobelieve.com",
		},
		{
			name:   "empty",
			input:  Email{},
			expect: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			actual, _ := tc.input.Value()

			assert.Equal(tc.expect, actual)
		})
	}
}
