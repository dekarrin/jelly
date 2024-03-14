package config

import (
	"errors"
	"strings"
	"testing"

	"github.com/dekarrin/jelly"
	"github.com/stretchr/testify/assert"
)

// Exported functions to test:
//
// Dump
//
// Environment.Load - unregistered conf section, registered conf section, no special, disabledefaults on/off

func Test_Environment_Load(t *testing.T) {

}

func Test_Dump(t *testing.T) {

}

func Test_DetectFormat(t *testing.T) {
	testCases := []struct {
		name   string
		file   string
		expect jelly.Format
	}{
		{
			name:   ".yml single file",
			file:   "config.yml",
			expect: jelly.YAML,
		},
		{
			name:   ".yaml multi-dir rel path",
			file:   "path/to/config.yaml",
			expect: jelly.YAML,
		},
		{
			name:   ".YML abs path",
			file:   "/etc/path/to/config.YML",
			expect: jelly.YAML,
		},
		{
			name:   ".YAML",
			file:   "config.YML",
			expect: jelly.YAML,
		},
		{
			name:   ".YaMl",
			file:   "someConfigFile.YaMl",
			expect: jelly.YAML,
		},
		{
			name:   ".YmL",
			file:   "someConfigFile.YmL",
			expect: jelly.YAML,
		},
		{
			name:   ".jsn",
			file:   "config.jsn",
			expect: jelly.JSON,
		},
		{
			name:   ".json",
			file:   "path/to/config.json",
			expect: jelly.JSON,
		},
		{
			name:   ".JSN",
			file:   "/etc/path/to/config.JSN",
			expect: jelly.JSON,
		},
		{
			name:   ".JSON",
			file:   "config.JSON",
			expect: jelly.JSON,
		},
		{
			name:   ".jSoN",
			file:   "someConfigFile.jSoN",
			expect: jelly.JSON,
		},
		{
			name:   ".JsN",
			file:   "someConfigFile.JsN",
			expect: jelly.JSON,
		},
		{
			name:   "invalid file",
			file:   "someConfigFile.txt",
			expect: jelly.NoFormat,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			actual := DetectFormat(tc.file)

			assert.Equal(tc.expect, actual)
		})
	}
}

func Test_Environment_Register(t *testing.T) {
	dummyProvider := func() jelly.APIConfig { return nil }

	envWithJohn := &Environment{}
	envWithJohn.initDefaults()
	envWithJohn.apiConfigProviders["john"] = dummyProvider

	testCases := []struct {
		name string
		env  *Environment

		provName string
		provider func() jelly.APIConfig

		expectErrContains string
	}{
		{
			name:     "normal add",
			env:      &Environment{},
			provName: "john",
			provider: dummyProvider,
		},
		{
			name:     "add uppercase",
			env:      &Environment{},
			provName: "JOHN",
			provider: dummyProvider,
		},
		{
			name:     "add conflict",
			env:      envWithJohn,
			provName: "john",
			provider: dummyProvider,

			expectErrContains: "duplicate config section name",
		},
		{
			name:     "add conflict",
			env:      envWithJohn,
			provName: "JOHN",
			provider: dummyProvider,

			expectErrContains: "duplicate config section name",
		},
		{
			name:     "nil connector",
			env:      &Environment{},
			provName: "john",
			provider: nil,

			expectErrContains: "provider function cannot be nil",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			actual := tc.env.Register(tc.provName, tc.provider)

			if tc.expectErrContains == "" {
				assert.NoError(actual)
				assert.Contains(tc.env.apiConfigProviders, strings.ToLower(tc.provName))
			} else {
				assert.Contains(actual.Error(), tc.expectErrContains)
			}
		})
	}
}

func Test_ConnectorRegistry_List(t *testing.T) {
	regWithTestDBEntry := &ConnectorRegistry{}
	regWithTestDBEntry.initDefaults()
	regWithTestDBEntry.reg[jelly.DatabaseInMemory]["testdb"] = func(dc jelly.DatabaseConfig) (jelly.Store, error) { return nil, nil }

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
		{
			name:   "DisableDefaults has no defaults for inmem",
			cr:     &ConnectorRegistry{DisableDefaults: true},
			engine: jelly.DatabaseInMemory,
			expect: []string{},
		},
		{
			name:   "DisableDefaults has no defaults for OWDB",
			cr:     &ConnectorRegistry{DisableDefaults: true},
			engine: jelly.DatabaseOWDB,
			expect: []string{},
		},
		{
			name:   "DisableDefaults has no defaults for sqlite",
			cr:     &ConnectorRegistry{DisableDefaults: true},
			engine: jelly.DatabaseSQLite,
			expect: []string{},
		},
		{
			name:   "zero registry has no defaults for None",
			cr:     &ConnectorRegistry{},
			engine: jelly.DatabaseNone,
			expect: []string{},
		},
		{
			name:   "with extra added",
			cr:     regWithTestDBEntry,
			engine: jelly.DatabaseInMemory,
			expect: []string{defaultAuthUserDBName, "testdb"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			actual := tc.cr.List(tc.engine)

			assert.ElementsMatch(tc.expect, actual)
		})
	}
}

func Test_ConnectorRegistry_Register(t *testing.T) {
	dummyConnector := func(dc jelly.DatabaseConfig) (jelly.Store, error) { return nil, nil }

	regWithJohn := &ConnectorRegistry{}
	regWithJohn.initDefaults()
	regWithJohn.reg[jelly.DatabaseSQLite]["john"] = dummyConnector

	testCases := []struct {
		name      string
		cr        *ConnectorRegistry
		engine    jelly.DBType
		connName  string
		connector func(jelly.DatabaseConfig) (jelly.Store, error)

		expectErrContains string
	}{
		{
			name:      "normal add",
			cr:        &ConnectorRegistry{},
			engine:    jelly.DatabaseSQLite,
			connName:  "john",
			connector: dummyConnector,
		},
		{
			name:      "add uppercase",
			cr:        &ConnectorRegistry{},
			engine:    jelly.DatabaseSQLite,
			connName:  "JOHN",
			connector: dummyConnector,
		},
		{
			name:      "add conflict",
			cr:        regWithJohn,
			engine:    jelly.DatabaseSQLite,
			connName:  "john",
			connector: dummyConnector,

			expectErrContains: "already has a registered connector",
		},
		{
			name:      "add conflict uppercase",
			cr:        regWithJohn,
			engine:    jelly.DatabaseSQLite,
			connName:  "JOHN",
			connector: dummyConnector,

			expectErrContains: "already has a registered connector",
		},
		{
			name:      "unsupported DB type",
			cr:        &ConnectorRegistry{},
			engine:    jelly.DatabaseNone,
			connName:  "john",
			connector: dummyConnector,

			expectErrContains: "is not a supported DB type",
		},
		{
			name:      "nil connector",
			cr:        &ConnectorRegistry{},
			engine:    jelly.DatabaseNone,
			connName:  "john",
			connector: nil,

			expectErrContains: "connector function cannot be nil",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			actual := tc.cr.Register(tc.engine, tc.connName, tc.connector)

			if tc.expectErrContains == "" {
				assert.NoError(actual)
				assert.Contains(tc.cr.reg[tc.engine], strings.ToLower(tc.connName))
			} else {
				assert.Contains(actual.Error(), tc.expectErrContains)
			}
		})
	}
}

type fakeStore struct{}

func (fs fakeStore) Close() error { return nil }

func Test_ConnectorRegistry_Connect(t *testing.T) {
	t.Run("connector specified and exists in registry", func(t *testing.T) {
		assert := assert.New(t)

		// setup
		var connectorWasCalled bool
		registry := &ConnectorRegistry{}
		registry.initDefaults()
		registry.reg[jelly.DatabaseInMemory]["testdb"] = func(dc jelly.DatabaseConfig) (jelly.Store, error) {
			connectorWasCalled = true
			return fakeStore{}, nil
		}

		// exec
		store, err := registry.Connect(jelly.DatabaseConfig{
			Type:      jelly.DatabaseInMemory,
			Connector: "testdb",
		})

		// assert
		if !assert.NoError(err) {
			return
		}
		assert.NotNil(store)
		assert.True(connectorWasCalled, "connector function was not called")
	})

	t.Run("connector specified, does not exist in registry, fallback to default", func(t *testing.T) {
		assert := assert.New(t)

		// setup
		var connectorWasCalled bool
		registry := &ConnectorRegistry{}
		registry.initDefaults()
		registry.reg[jelly.DatabaseOWDB]["*"] = func(dc jelly.DatabaseConfig) (jelly.Store, error) {
			connectorWasCalled = true
			return fakeStore{}, nil
		}

		// exec
		store, err := registry.Connect(jelly.DatabaseConfig{
			Type:      jelly.DatabaseOWDB,
			Connector: "testdb",
		})

		// assert
		if !assert.NoError(err) {
			return
		}
		assert.NotNil(store)
		assert.True(connectorWasCalled, "connector function was not called")
	})

	t.Run("connector not specified, but fallback exists", func(t *testing.T) {
		assert := assert.New(t)

		// setup
		var connectorWasCalled bool
		registry := &ConnectorRegistry{}
		registry.initDefaults()
		registry.reg[jelly.DatabaseOWDB]["*"] = func(dc jelly.DatabaseConfig) (jelly.Store, error) {
			connectorWasCalled = true
			return fakeStore{}, nil
		}

		// exec
		store, err := registry.Connect(jelly.DatabaseConfig{
			Type:      jelly.DatabaseOWDB,
			Connector: "testdb",
		})

		// assert
		if !assert.NoError(err) {
			return
		}
		assert.NotNil(store)
		assert.True(connectorWasCalled, "connector function was not called")
	})

	t.Run("connector specified, does not exist in registry, no default - error", func(t *testing.T) {
		assert := assert.New(t)

		// setup
		registry := &ConnectorRegistry{}
		registry.initDefaults()
		delete(registry.reg[jelly.DatabaseOWDB], "*")

		// exec
		_, err := registry.Connect(jelly.DatabaseConfig{
			Type:      jelly.DatabaseOWDB,
			Connector: "testdb",
		})

		// assert
		assert.ErrorContains(err, `not a registered connector`)
		assert.ErrorContains(err, `no default "*" connector`)
	})

	t.Run("connector not specified, no default - error", func(t *testing.T) {
		assert := assert.New(t)

		// setup
		registry := &ConnectorRegistry{}
		registry.initDefaults()
		delete(registry.reg[jelly.DatabaseOWDB], "*")

		// exec
		_, err := registry.Connect(jelly.DatabaseConfig{
			Type: jelly.DatabaseOWDB,
		})

		// assert
		assert.ErrorContains(err, `does not specify connector`)
		assert.ErrorContains(err, `no default "*" connector`)
	})

	t.Run("invalid engine - error", func(t *testing.T) {
		assert := assert.New(t)

		// setup
		registry := &ConnectorRegistry{}

		// exec
		_, err := registry.Connect(jelly.DatabaseConfig{
			Type: jelly.DatabaseNone,
		})

		// assert
		assert.ErrorContains(err, `does not specify connector`)
		assert.ErrorContains(err, `no default "*" connector`)
	})

	t.Run("connector returns error - error", func(t *testing.T) {
		assert := assert.New(t)

		// setup
		registry := &ConnectorRegistry{}
		registry.initDefaults()
		registry.reg[jelly.DatabaseOWDB]["testdb"] = func(dc jelly.DatabaseConfig) (jelly.Store, error) {
			return nil, errors.New("MAJOR ISSUES")
		}

		// exec
		_, err := registry.Connect(jelly.DatabaseConfig{
			Type:      jelly.DatabaseOWDB,
			Connector: "testdb",
		})

		// assert
		assert.ErrorContains(err, "MAJOR ISSUES")
	})
}
