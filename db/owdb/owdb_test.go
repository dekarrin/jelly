package owdb

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// april09Date is a shorthand test function for building a date. Returns a date
// in april of 2009 with the given params in timezone UTC and nsec of 0.
func april09(day, hour, min, sec int) time.Time {
	return time.Date(2009, time.April, day, hour, min, sec, 0, time.UTC)
}

func Test_Store_Select(t *testing.T) {
	testCases := []struct {
		name   string
		store  *Store
		filter Filter
		expect []Hit
	}{
		{
			name: "select with nil Where (all of them)",
			store: &Store{hits: []Hit{
				{Time: april09(13, 0, 0, 0), Resource: "/aradia.html"},
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html"},
				{Time: april09(13, 2, 0, 0), Resource: "/tavros.html"},
			}},
			filter: nil,
			expect: []Hit{
				{Time: april09(13, 0, 0, 0), Resource: "/aradia.html"},
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html"},
				{Time: april09(13, 2, 0, 0), Resource: "/tavros.html"},
			},
		},
		{
			name: "select with empty Where (all of them)",
			store: &Store{hits: []Hit{
				{Time: april09(13, 0, 0, 0), Resource: "/aradia.html"},
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html"},
				{Time: april09(13, 2, 0, 0), Resource: "/tavros.html"},
			}},
			filter: Where{},
			expect: []Hit{
				{Time: april09(13, 0, 0, 0), Resource: "/aradia.html"},
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html"},
				{Time: april09(13, 2, 0, 0), Resource: "/tavros.html"},
			},
		},
		{
			name: "select by indexed property Time",
			store: &Store{hits: []Hit{
				{Time: april09(13, 0, 0, 0), Resource: "/aradia.html"},
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html"},
				{Time: april09(13, 2, 0, 0), Resource: "/tavros.html"},
			}},
			filter: Where{Time: IsAfter(april09(13, 0, 30, 0))},
			expect: []Hit{
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html"},
				{Time: april09(13, 2, 0, 0), Resource: "/tavros.html"},
			},
		},
		{
			name: "select by non-indexed prop Host",
			store: &Store{hits: []Hit{
				{Time: april09(13, 0, 0, 0), Resource: "/aradia.html", Host: "server1"},
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html", Host: "server2"},
				{Time: april09(13, 2, 0, 0), Resource: "/tavros.html", Host: "server1"},
			}},
			filter: Where{Host: EqualsString("server1")},
			expect: []Hit{
				{Time: april09(13, 0, 0, 0), Resource: "/aradia.html", Host: "server1"},
				{Time: april09(13, 2, 0, 0), Resource: "/tavros.html", Host: "server1"},
			},
		},
		{
			name: "select by non-indexed prop Resource",
			store: &Store{hits: []Hit{
				{Time: april09(13, 0, 0, 0), Resource: "/aradia.html"},
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html"},
				{Time: april09(13, 2, 0, 0), Resource: "/vriska.html"},
			}},
			filter: Where{Resource: EqualsString("/vriska.html")},
			expect: []Hit{
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html"},
				{Time: april09(13, 2, 0, 0), Resource: "/vriska.html"},
			},
		},
		{
			name: "select by non-indexed prop Client.Address",
			store: &Store{hits: []Hit{
				{Time: april09(13, 0, 0, 0), Resource: "/aradia.html", Client: Requester{Address: net.IPv4(10, 1, 10, 1)}},
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html", Client: Requester{Address: net.IPv4(10, 1, 10, 1)}},
				{Time: april09(13, 2, 0, 0), Resource: "/tavros.html", Client: Requester{Address: net.IPv4(192, 168, 0, 11)}},
			}},
			filter: Where{ClientAddress: EqualsIP("10.1.10.1")},
			expect: []Hit{
				{Time: april09(13, 0, 0, 0), Resource: "/aradia.html", Client: Requester{Address: net.IPv4(10, 1, 10, 1)}},
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html", Client: Requester{Address: net.IPv4(10, 1, 10, 1)}},
			},
		},
		{
			name: "select by non-indexed prop Client.Country",
			store: &Store{hits: []Hit{
				{Time: april09(13, 0, 0, 0), Resource: "/aradia.html", Client: Requester{Country: "Alternia"}},
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html", Client: Requester{Country: "Alternia"}},
				{Time: april09(13, 2, 0, 0), Resource: "/rose.html", Client: Requester{Country: "America"}},
				{Time: april09(13, 3, 0, 0), Resource: "/tavros.html", Client: Requester{Country: "Alternia"}},
			}},
			filter: Where{ClientCountry: EqualsString("America")},
			expect: []Hit{
				{Time: april09(13, 2, 0, 0), Resource: "/rose.html", Client: Requester{Country: "America"}},
			},
		},
		{
			name: "select by non-indexed prop Client.City",
			store: &Store{hits: []Hit{
				{Time: april09(13, 0, 0, 0), Resource: "/aradia.html", Client: Requester{City: "Alternia"}},
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html", Client: Requester{City: "Alternia"}},
				{Time: april09(13, 2, 0, 0), Resource: "/rose.html", Client: Requester{City: "South Colton"}},
				{Time: april09(13, 3, 0, 0), Resource: "/tavros.html", Client: Requester{City: "Alternia"}},
			}},
			filter: Where{ClientCity: EqualsString("Alternia")},
			expect: []Hit{
				{Time: april09(13, 0, 0, 0), Resource: "/aradia.html", Client: Requester{City: "Alternia"}},
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html", Client: Requester{City: "Alternia"}},
				{Time: april09(13, 3, 0, 0), Resource: "/tavros.html", Client: Requester{City: "Alternia"}},
			},
		},
		{
			name: "select by two OR'd properties",
			store: &Store{hits: []Hit{
				{Time: april09(13, 0, 0, 0), Resource: "/aradia.html", Host: "server1", Client: Requester{City: "Alternia"}},
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html", Host: "server2", Client: Requester{City: "Alternia"}},
				{Time: april09(13, 2, 0, 0), Resource: "/rose.html", Host: "server1", Client: Requester{City: "South Colton"}},
				{Time: april09(13, 3, 0, 0), Resource: "/tavros.html", Host: "server2", Client: Requester{City: "Alternia"}},
				{Time: april09(13, 4, 0, 0), Resource: "/john.html", Host: "server1", Client: Requester{City: "Maple Valley"}},
			}},
			filter: Where{ClientCity: EqualsString("Maple Valley")}.
				Or(Where{Host: EqualsString("server2")}),
			expect: []Hit{
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html", Host: "server2", Client: Requester{City: "Alternia"}},
				{Time: april09(13, 3, 0, 0), Resource: "/tavros.html", Host: "server2", Client: Requester{City: "Alternia"}},
				{Time: april09(13, 4, 0, 0), Resource: "/john.html", Host: "server1", Client: Requester{City: "Maple Valley"}},
			},
		},
		{
			name: "select by two AND'd properties",
			store: &Store{hits: []Hit{
				{Time: april09(13, 0, 0, 0), Resource: "/aradia.html", Host: "server1", Client: Requester{City: "Alternia"}},
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html", Host: "server2", Client: Requester{City: "Alternia"}},
				{Time: april09(13, 2, 0, 0), Resource: "/rose.html", Host: "server1", Client: Requester{City: "South Colton"}},
				{Time: april09(13, 3, 0, 0), Resource: "/tavros.html", Host: "server2", Client: Requester{City: "Alternia"}},
				{Time: april09(13, 4, 0, 0), Resource: "/john.html", Host: "server1", Client: Requester{City: "Maple Valley"}},
			}},
			filter: Where{ClientCity: EqualsString("Alternia")}.
				And(Where{Host: EqualsString("server2")}),
			expect: []Hit{
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html", Host: "server2", Client: Requester{City: "Alternia"}},
				{Time: april09(13, 3, 0, 0), Resource: "/tavros.html", Host: "server2", Client: Requester{City: "Alternia"}},
			},
		},
		{
			name: "select by NOT'd property",
			store: &Store{hits: []Hit{
				{Time: april09(13, 0, 0, 0), Resource: "/aradia.html", Host: "server1", Client: Requester{City: "Alternia"}},
				{Time: april09(13, 1, 0, 0), Resource: "/vriska.html", Host: "server2", Client: Requester{City: "Alternia"}},
				{Time: april09(13, 2, 0, 0), Resource: "/rose.html", Host: "server1", Client: Requester{City: "South Colton"}},
				{Time: april09(13, 3, 0, 0), Resource: "/tavros.html", Host: "server2", Client: Requester{City: "Alternia"}},
				{Time: april09(13, 4, 0, 0), Resource: "/john.html", Host: "server1", Client: Requester{City: "Maple Valley"}},
			}},
			filter: Not(Where{ClientCity: EqualsString("Alternia")}),
			expect: []Hit{
				{Time: april09(13, 2, 0, 0), Resource: "/rose.html", Host: "server1", Client: Requester{City: "South Colton"}},
				{Time: april09(13, 4, 0, 0), Resource: "/john.html", Host: "server1", Client: Requester{City: "Maple Valley"}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			actual, err := tc.store.Select(tc.filter)
			if !assert.NoError(err) {
				return
			}

			assert.Equal(tc.expect, actual)
		})
	}
}

func Test_Store_Update(t *testing.T) {
	testCases := []struct {
		name          string
		store         *Store
		filter        Filter
		upFunc        func(Hit) Hit
		expect        *Store
		expectMatched int
		expectUpdated int
	}{
		{
			name: "update single non-indexed prop, 1 match",
			store: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
			}},
			filter: Where{Time: EqualsTime(time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC))},
			upFunc: func(h Hit) Hit {
				h.Resource = "/8888.html"
				return h
			},
			expect: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/8888.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
			}},
			expectMatched: 1,
			expectUpdated: 1,
		},
		{
			name: "update multiple non-indexed props, 1 match",
			store: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
			}},
			filter: Where{Time: EqualsTime(time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC))},
			upFunc: func(h Hit) Hit {
				h.Resource = "/8888.html"
				h.Client.Address = net.IPv4(10, 0, 8, 8)
				return h
			},
			expect: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/8888.html", Client: Requester{Address: net.IPv4(10, 0, 8, 8)}},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
			}},
			expectMatched: 1,
			expectUpdated: 1,
		},
		{
			name: "update multiple non-indexed props, multiple matches",
			store: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
			}},
			filter: Where{Time: IsBetweenTimes(
				time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC),
				time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC),
			)},
			upFunc: func(h Hit) Hit {
				h.Host = "homestuck.com"
				h.Client.Country = "Alternia"
				return h
			},
			expect: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html", Host: "homestuck.com",
					Client: Requester{Country: "Alternia"}},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html", Host: "homestuck.com",
					Client: Requester{Country: "Alternia"}},
			}},
			expectMatched: 2,
			expectUpdated: 2,
		},
		{
			name: "update multiple non-indexed props, multiple matches, 1 update",
			store: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html", Host: "homestuck.com"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html", Host: "homestuck.com"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html", Host: "homestuck.com"},
			}},
			filter: Where{Time: IsBetweenTimes(
				time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC),
				time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC),
			)},
			upFunc: func(h Hit) Hit {
				h.Host = "homestuck.com"
				h.Resource = "/vriska.html"
				return h
			},
			expect: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html", Host: "homestuck.com"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html", Host: "homestuck.com"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/vriska.html", Host: "homestuck.com"},
			}},
			expectMatched: 2,
			expectUpdated: 1,
		},
		{
			name: "update multiple non-indexed props, multiple matches, no updates",
			store: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/vriska.html", Host: "homestuck.com"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html", Host: "homestuck.com"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html", Host: "homestuck.com"},
			}},
			filter: Where{Time: IsBetweenTimes(
				time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC),
				time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC),
			)},
			upFunc: func(h Hit) Hit {
				h.Host = "homestuck.com"
				h.Resource = "/vriska.html"
				return h
			},
			expect: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/vriska.html", Host: "homestuck.com"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html", Host: "homestuck.com"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html", Host: "homestuck.com"},
			}},
			expectMatched: 2,
			expectUpdated: 0,
		},
		{
			name: "update multiple non-indexed props, 0 matches",
			store: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
			}},
			filter: Where{Time: IsBetweenTimes(
				time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC),
				time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC),
			)},
			upFunc: func(h Hit) Hit {
				h.Host = "homestuck.com"
				h.Resource = "/vriska.html"
				return h
			},
			expect: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
			}},
			expectMatched: 0,
			expectUpdated: 0,
		},
		{
			name: "update indexed property, 1 match",
			store: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
				{Time: time.Date(2009, time.April, 13, 11, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
			}},
			filter: Where{Time: EqualsTime(time.Date(2009, time.April, 13, 11, 0, 0, 0, time.UTC))},
			upFunc: func(h Hit) Hit {
				h.Time = time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC)
				return h
			},
			expect: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
			}},
			expectMatched: 1,
			expectUpdated: 1,
		},
		{
			name: "update indexed property, multiple matches",
			store: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
				{Time: time.Date(2009, time.April, 13, 11, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 20, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
			}},
			filter: Where{Time: IsAfter(time.Date(2009, time.April, 13, 5, 0, 0, 0, time.UTC))},
			upFunc: func() func(h Hit) Hit {
				t := 0
				return func(h Hit) Hit {
					h.Time = time.Date(2009, time.April, 13, t, 0, 0, 0, time.UTC)
					t++
					return h
				}
			}(),
			expect: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
			}},
			expectMatched: 2,
			expectUpdated: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			actualMatched, actualUpdated, err := tc.store.Update(tc.filter, tc.upFunc)
			if !assert.NoError(err) {
				return
			}

			assert.Equal(tc.expectMatched, actualMatched, "returned matched count was not expected value")
			assert.Equal(tc.expectUpdated, actualUpdated, "returned updated count was not expected value")
			assert.Equal(tc.expect.String(), tc.store.String())
		})
	}
}

func Test_Store_Delete(t *testing.T) {
	testCases := []struct {
		name           string
		store          *Store
		filter         Filter
		expect         *Store
		expectDelCount int
	}{
		{
			name:           "delete from empty",
			store:          &Store{},
			filter:         Where{Time: EqualsTime(time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC))},
			expect:         &Store{},
			expectDelCount: 0,
		},
		{
			name: "delete non-existing",
			store: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
				{Time: time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
				{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
				{Time: time.Date(2009, time.April, 13, 5, 0, 0, 0, time.UTC), Resource: "/terezi.html"},
			}},
			filter: Where{Time: EqualsTime(time.Date(2009, time.January, 1, 0, 0, 0, 0, time.UTC))},
			expect: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
				{Time: time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
				{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
				{Time: time.Date(2009, time.April, 13, 5, 0, 0, 0, time.UTC), Resource: "/terezi.html"},
			}},
			expectDelCount: 0,
		},
		{
			name: "delete single from start",
			store: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
				{Time: time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
				{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
				{Time: time.Date(2009, time.April, 13, 5, 0, 0, 0, time.UTC), Resource: "/terezi.html"},
			}},
			filter: Where{Time: EqualsTime(time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC))},
			expect: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
				{Time: time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
				{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
				{Time: time.Date(2009, time.April, 13, 5, 0, 0, 0, time.UTC), Resource: "/terezi.html"},
			}},
			expectDelCount: 1,
		},
		{
			name: "delete single from middle",
			store: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
				{Time: time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
				{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
				{Time: time.Date(2009, time.April, 13, 5, 0, 0, 0, time.UTC), Resource: "/terezi.html"},
			}},
			filter: Where{Time: EqualsTime(time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC))},
			expect: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
				{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
				{Time: time.Date(2009, time.April, 13, 5, 0, 0, 0, time.UTC), Resource: "/terezi.html"},
			}},
			expectDelCount: 1,
		},
		{
			name: "delete single from end",
			store: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
				{Time: time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
				{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
				{Time: time.Date(2009, time.April, 13, 5, 0, 0, 0, time.UTC), Resource: "/terezi.html"},
			}},
			filter: Where{Time: EqualsTime(time.Date(2009, time.April, 13, 5, 0, 0, 0, time.UTC))},
			expect: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
				{Time: time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
				{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
			}},
			expectDelCount: 1,
		},
		{
			name: "delete multiple consecutive from start",
			store: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
				{Time: time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
				{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
				{Time: time.Date(2009, time.April, 13, 5, 0, 0, 0, time.UTC), Resource: "/terezi.html"},
			}},
			filter: Where{Time: EqualsTime(time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC))},
			expect: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
				{Time: time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
				{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
				{Time: time.Date(2009, time.April, 13, 5, 0, 0, 0, time.UTC), Resource: "/terezi.html"},
			}},
			expectDelCount: 2,
		},
		{
			name: "delete multiple consecutive from middle",
			store: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
				{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
				{Time: time.Date(2009, time.April, 13, 5, 0, 0, 0, time.UTC), Resource: "/terezi.html"},
			}},
			filter: Where{Time: EqualsTime(time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC))},
			expect: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
				{Time: time.Date(2009, time.April, 13, 5, 0, 0, 0, time.UTC), Resource: "/terezi.html"},
			}},
			expectDelCount: 2,
		},
		{
			name: "delete multiple consecutive from end",
			store: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
				{Time: time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
				{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
				{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/terezi.html"},
			}},
			filter: Where{Time: EqualsTime(time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC))},
			expect: &Store{hits: []Hit{
				{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
				{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
				{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
				{Time: time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
			}},
			expectDelCount: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			actualDelCount, err := tc.store.Delete(tc.filter)
			if !assert.NoError(err) {
				return
			}

			assert.Equal(tc.expect.String(), tc.store.String())
			assert.Equal(tc.expectDelCount, actualDelCount)
		})
	}
}

func Test_Store_applyFilter(t *testing.T) {
	testCases := []struct {
		name   string
		store  *Store
		filter Filter
		expect []int
	}{
		{
			name: "nil matches all",
			store: &Store{
				hits: []Hit{
					{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
					{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
					{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
					{Time: time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
					{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
				},
			},
			filter: nil,
			expect: []int{0, 1, 2, 3, 4},
		},
		{
			name: "single eq condition, multi-item match",
			store: &Store{
				hits: []Hit{
					{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
					{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
					{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
					{Time: time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
					{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
				},
			},
			filter: Where{Time: EqualsTime(time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC))},
			expect: []int{1, 2},
		},
		{
			name: "range condition, multi-item match",
			store: &Store{
				hits: []Hit{
					{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
					{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
					{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
					{Time: time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
					{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
				},
			},
			filter: Where{Time: IsBetweenTimes(time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC))},
			expect: []int{1, 2, 3},
		},
		{
			name: "range condition matches from start",
			store: &Store{
				hits: []Hit{
					{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
					{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
					{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
					{Time: time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
					{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
				},
			},
			filter: Where{Time: IsBetweenTimes(time.Date(2009, time.April, 12, 0, 0, 0, 0, time.UTC), time.Date(2009, time.April, 13, 1, 30, 0, 0, time.UTC))},
			expect: []int{0, 1},
		},
		{
			name: "range condition matches through end",
			store: &Store{
				hits: []Hit{
					{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
					{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
					{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
					{Time: time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
					{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
				},
			},
			filter: Where{Time: IsBetweenTimes(time.Date(2009, time.April, 13, 1, 30, 0, 0, time.UTC), time.Date(2009, time.April, 14, 0, 0, 0, 0, time.UTC))},
			expect: []int{2, 3, 4},
		},
		{
			name: "range condition matches all",
			store: &Store{
				hits: []Hit{
					{Time: time.Date(2009, time.April, 13, 0, 0, 0, 0, time.UTC), Resource: "/aradia.html"},
					{Time: time.Date(2009, time.April, 13, 1, 0, 0, 0, time.UTC), Resource: "/vriska.html"},
					{Time: time.Date(2009, time.April, 13, 2, 0, 0, 0, time.UTC), Resource: "/tavros.html"},
					{Time: time.Date(2009, time.April, 13, 3, 0, 0, 0, time.UTC), Resource: "/karkat.html"},
					{Time: time.Date(2009, time.April, 13, 4, 0, 0, 0, time.UTC), Resource: "/nepeta.html"},
				},
			},
			filter: Where{Time: IsBetweenTimes(time.Date(2009, time.April, 12, 0, 0, 0, 0, time.UTC), time.Date(2009, time.April, 14, 0, 0, 0, 0, time.UTC))},
			expect: []int{0, 1, 2, 3, 4},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			actual := tc.store.applyFilter(tc.filter)

			assert.Equal(tc.expect, actual)
		})
	}
}
