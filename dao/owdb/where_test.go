package owdb

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Where_Matches(t *testing.T) {
	testCases := []struct {
		name   string
		where  Where
		input  Hit
		expect bool
	}{
		{
			name:   "empty Where matches empty Hit",
			where:  Where{},
			input:  Hit{},
			expect: true,
		},
		{
			name:  "empty Where matches filled Hit",
			where: Where{},
			input: Hit{
				Time:     time.Date(1989, time.June, 12, 0, 0, 0, 0, time.UTC),
				Host:     "www.example.com",
				Resource: "/nepeta",
				Client: Requester{
					Address: net.IPv4(10, 0, 4, 13),
					Country: "Alternia",
					City:    "Also Alternia",
				},
			},
			expect: true,
		},
		{
			name:   "time criterion that never matches, against empty Hit == false",
			where:  Where{Time: Meets(func(v time.Time) bool { return false })},
			input:  Hit{},
			expect: false,
		},
		{
			name:   "time criterion that never matches, against Hit with Time of Now() == false",
			where:  Where{Time: Meets(func(v time.Time) bool { return false })},
			input:  Hit{Time: time.Now()},
			expect: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			actual := tc.where.Matches(tc.input)

			assert.Equal(tc.expect, actual)
		})
	}
}

func Test_Where_Node(t *testing.T) {
	assert := assert.New(t)

	w := Where{Time: Meets(func(v time.Time) bool { return true })}

	actual := w.Node()

	// both Where and FilterNode have uncomparable members, so we can't actually
	// compare actual with expected directly, but we *can* check whether the
	// result at least is a FilterNode in condition mode

	assert.False(actual.IsOperation())
	if !assert.NotNil(actual.Cond) {
		return
	}
	assert.NotNil(actual.Cond.Time)
}

func Test_Where_Negate(t *testing.T) {
	assert := assert.New(t)

	w := Where{Time: Meets(func(v time.Time) bool { return true })}

	actual := w.Negate()

	// both Where and FilterNode have uncomparable members, so we can't actually
	// compare actual with expected directly, but we *can* check whether the
	// result at least is a FilterNode in operation mode with a NOT operator.

	assert.True(actual.IsOperation())
	assert.Equal(NOT, actual.Op)
}

func Test_Where_And(t *testing.T) {
	isBefore2200 := Meets(func(v time.Time) bool { return v.Before(time.Date(2200, 0, 0, 0, 0, 0, 0, time.UTC)) }, "before")
	isAfter2100 := Meets(func(v time.Time) bool { return v.After(time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)) }, "after")

	testCases := []struct {
		name   string
		base   Where
		other  Filter
		more   []Filter
		expect FilterNode
	}{
		{
			name:  "normal test",
			base:  Where{Time: isBefore2200},
			other: Where{Time: isAfter2100},
			expect: FilterNode{
				Op: AND,
				Group: []FilterNode{
					{Cond: &Where{Time: isBefore2200}},
					{Cond: &Where{Time: isAfter2100}},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			actual := tc.base.And(tc.other, tc.more...)

			assert.Equal(tc.expect.String(), actual.String())
		})
	}
}

func Test_Where_Or(t *testing.T) {
	isBefore2200 := Meets(func(v time.Time) bool { return v.Before(time.Date(2200, 0, 0, 0, 0, 0, 0, time.UTC)) }, "before2200")
	isAfter2100 := Meets(func(v time.Time) bool { return v.After(time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)) }, "after2100")

	testCases := []struct {
		name   string
		base   Where
		other  Filter
		more   []Filter
		expect FilterNode
	}{
		{
			name:  "normal test",
			base:  Where{Time: isBefore2200},
			other: Where{Time: isAfter2100},
			expect: FilterNode{
				Op: OR,
				Group: []FilterNode{
					{Cond: &Where{Time: isBefore2200}},
					{Cond: &Where{Time: isAfter2100}},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			actual := tc.base.Or(tc.other, tc.more...)

			assert.Equal(tc.expect.String(), actual.String())
		})
	}
}
