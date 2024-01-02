package owdb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_EqualsTime(t *testing.T) {
	type fnParams struct {
		v time.Time
	}

	testCases := []struct {
		name   string
		params fnParams
		input  time.Time
		expect bool
	}{
		{
			name:   "eq",
			params: fnParams{v: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC)},
			input:  time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC),
			expect: true,
		},
		{
			name:   "neq",
			params: fnParams{v: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC)},
			input:  time.Date(2009, time.April, 1, 0, 0, 0, 0, time.UTC),
			expect: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			crit := EqualsTime(tc.params.v)

			actual := crit.Meets(tc.input)

			assert.Equal(tc.expect, actual)
		})
	}
}

func Test_DoesNot(t *testing.T) {
	type fnParams struct {
		crit Criterion[int]
	}

	testCases := []struct {
		name   string
		params fnParams
		input  int
		expect bool
	}{
		{
			name:   "!eq",
			params: fnParams{crit: Meets(func(v int) bool { return v == 1 })},
			input:  1,
			expect: false,
		},
		{
			name:   "!neq",
			params: fnParams{crit: Meets(func(v int) bool { return v == 1 })},
			input:  0,
			expect: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			crit := DoesNot(tc.params.crit)

			actual := crit.Meets(tc.input)

			assert.Equal(tc.expect, actual)
		})
	}
}
