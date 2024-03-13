package config

import (
	"testing"

	"github.com/dekarrin/jelly"
	"github.com/stretchr/testify/assert"
)

// Exported functions to test:
//
// DetectFormat
// Dump
// SupportedFormats - no reason to test this one, informational
//
// Environment.Load - unregistered conf section, registered conf section, no special, disabledefaults on/off
// Environment.Register - disabledefaults on/off
//
// ConnectorRegistry.Connect
// ConnectorRegistry.List
// ConnectorRegistry.Register
//
// When done writing tests for ConnectorRegistry, move it to DB.

// TODO: when done, move to db pkg
func Test_ConnectorRegistry_List(t *testing.T) {
	testCases := []struct {
		name   string
		cr     *ConnectorRegistry
		engine jelly.DBType
		expect []string
	}{
		{
			name:   "zero registry returns default auth user DB - inmem",
			cr:     &ConnectorRegistry{},
			engine: jelly.DatabaseInMemory,
			expect: []string{defaultAuthUserDBName},
		},
		{
			name:   "zero registry returns default auth user DB - sqlite",
			cr:     &ConnectorRegistry{},
			engine: jelly.DatabaseSQLite,
			expect: []string{defaultAuthUserDBName},
		},
		{
			name:   "zero registry has any-match default for OWDB",
			cr:     &ConnectorRegistry{},
			engine: jelly.DatabaseOWDB,
			expect: []string{anyMatchDBName},
		},
		{
			name:   "zero registry has no defaults for None",
			cr:     &ConnectorRegistry{},
			engine: jelly.DatabaseNone,
			expect: []string{},
		},

		// NEED MORE
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			actual := tc.cr.List(tc.engine)

			assert.Equal(tc.expect, actual)
		})
	}
}

// TODO: when done, move to db pkg
func Test_ConnectorRegistry_Register(t *testing.T) {
}

// TODO: when done, move to db pkg
func Test_ConnectorRegistry_Connect(t *testing.T) {
}

func Test_Environment_Load(t *testing.T) {

}

func Test_Environment_Register(t *testing.T) {

}

func Test_DetectFormat(t *testing.T) {

}

func Test_Dump(t *testing.T) {

}
