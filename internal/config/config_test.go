package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dekarrin/jelly"
	mock_jelly "github.com/dekarrin/jelly/tools/mocks/jelly"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// Exported functions to test:
//
// Dump
//
// Environment.Load - unregistered conf section, registered conf section, no special, disabledefaults on/off

func Test_Environment_Load(t *testing.T) {
	emptyYAMLConfig := jelly.Config{
		DBs:    make(map[string]jelly.DatabaseConfig),
		APIs:   make(map[string]jelly.APIConfig),
		Format: jelly.YAML,
	}
	emptyJSONConfig := jelly.Config{
		DBs:    make(map[string]jelly.DatabaseConfig),
		APIs:   make(map[string]jelly.APIConfig),
		Format: jelly.JSON,
	}

	testCases := []struct {
		name     string
		env      *Environment
		filename string
		content  string

		expect            jelly.Config
		expectErrContains string
	}{
		{
			name:     "yaml - empty config file",
			env:      &Environment{},
			filename: "config.yaml",
			content:  "",
			expect:   emptyYAMLConfig,
		},
		{
			name:     "yaml - listen - address:port",
			env:      &Environment{},
			filename: "config.yaml",
			content:  `listen: 127.0.0.1:8002`,

			expect: jelly.Config{
				Globals: jelly.Globals{
					Port:    8002,
					Address: "127.0.0.1",
				},
				DBs:    make(map[string]jelly.DatabaseConfig),
				APIs:   make(map[string]jelly.APIConfig),
				Format: jelly.YAML,
			},
		},
		{
			name:     "yaml - listen - address:",
			env:      &Environment{},
			filename: "config.yaml",
			content:  `listen: '127.0.0.1:'`,

			expect: jelly.Config{
				Globals: jelly.Globals{
					Address: "127.0.0.1",
				},
				DBs:    make(map[string]jelly.DatabaseConfig),
				APIs:   make(map[string]jelly.APIConfig),
				Format: jelly.YAML,
			},
		},
		{
			name:     "yaml - listen - :port",
			env:      &Environment{},
			filename: "config.yaml",
			content:  `listen: :8002`,

			expect: jelly.Config{
				Globals: jelly.Globals{
					Port: 8002,
				},
				DBs:    make(map[string]jelly.DatabaseConfig),
				APIs:   make(map[string]jelly.APIConfig),
				Format: jelly.YAML,
			},
		},
		{
			name:     "yaml - base - non-slashed",
			env:      &Environment{},
			filename: "config.yaml",
			content:  `base: hello`,

			expect: jelly.Config{
				Globals: jelly.Globals{
					URIBase: "hello",
				},
				DBs:    make(map[string]jelly.DatabaseConfig),
				APIs:   make(map[string]jelly.APIConfig),
				Format: jelly.YAML,
			},
		},
		{
			name:     "yaml - base - slashed",
			env:      &Environment{},
			filename: "config.yaml",
			content:  `base: /hello`,
			expect: jelly.Config{
				Globals: jelly.Globals{
					URIBase: "/hello",
				},
				DBs:    make(map[string]jelly.DatabaseConfig),
				APIs:   make(map[string]jelly.APIConfig),
				Format: jelly.YAML,
			},
		},
		{
			name:     "yaml - base - slashed at end",
			env:      &Environment{},
			filename: "config.yaml",
			content:  `base: hello/`,
			expect: jelly.Config{
				Globals: jelly.Globals{
					URIBase: "hello/",
				},
				DBs:    make(map[string]jelly.DatabaseConfig),
				APIs:   make(map[string]jelly.APIConfig),
				Format: jelly.YAML,
			},
		},
		{
			name:     "json - empty config file",
			env:      &Environment{},
			filename: "config.json",
			content:  "",
			expect:   emptyJSONConfig,
		},
		{
			name:     "json - empty object",
			env:      &Environment{},
			filename: "config.json",
			content:  "{}",
			expect:   emptyJSONConfig,
		},
		{
			name:     "json - listen - address:port",
			env:      &Environment{},
			filename: "config.json",
			content:  `{"listen": "127.0.0.1:8002"}`,

			expect: jelly.Config{
				Globals: jelly.Globals{
					Port:    8002,
					Address: "127.0.0.1",
				},
				DBs:    make(map[string]jelly.DatabaseConfig),
				APIs:   make(map[string]jelly.APIConfig),
				Format: jelly.JSON,
			},
		},
		{
			name:     "json - listen - address:",
			env:      &Environment{},
			filename: "config.json",
			content:  `{"listen": "127.0.0.1:"}`,

			expect: jelly.Config{
				Globals: jelly.Globals{
					Address: "127.0.0.1",
				},
				DBs:    make(map[string]jelly.DatabaseConfig),
				APIs:   make(map[string]jelly.APIConfig),
				Format: jelly.JSON,
			},
		},
		{
			name:     "json - listen - :port",
			env:      &Environment{},
			filename: "config.json",
			content:  `{"listen": ":8002"}`,

			expect: jelly.Config{
				Globals: jelly.Globals{
					Port: 8002,
				},
				DBs:    make(map[string]jelly.DatabaseConfig),
				APIs:   make(map[string]jelly.APIConfig),
				Format: jelly.JSON,
			},
		},
		{
			name:     "json - base - non-slashed",
			env:      &Environment{},
			filename: "config.json",
			content:  `{"base": "hello"}`,

			expect: jelly.Config{
				Globals: jelly.Globals{
					URIBase: "hello",
				},
				DBs:    make(map[string]jelly.DatabaseConfig),
				APIs:   make(map[string]jelly.APIConfig),
				Format: jelly.JSON,
			},
		},
		{
			name:     "json - base - slashed",
			env:      &Environment{},
			filename: "config.json",
			content:  `{"base": "/hello"}`,
			expect: jelly.Config{
				Globals: jelly.Globals{
					URIBase: "/hello",
				},
				DBs:    make(map[string]jelly.DatabaseConfig),
				APIs:   make(map[string]jelly.APIConfig),
				Format: jelly.JSON,
			},
		},
		{
			name:     "json - base - slashed at end",
			env:      &Environment{},
			filename: "config.json",
			content:  `{"base": "hello/"}`,
			expect: jelly.Config{
				Globals: jelly.Globals{
					URIBase: "hello/",
				},
				DBs:    make(map[string]jelly.DatabaseConfig),
				APIs:   make(map[string]jelly.APIConfig),
				Format: jelly.JSON,
			},
		},
		{
			name:     "yaml - all options config file, default conf reader",
			env:      &Environment{},
			filename: "config.yaml",
			content: `
listen: 10.0.28.16:80
base: api/
authenticator: john.egbert

logging:
  enabled: true
  provider: std
  file: /var/log/jelly.log

dbs:
  testdb:
    type: sqlite
    file: /var/lib/jelly/testdb.sqlite
  userdb:
    type: inmem
    connector: john
  hitsdb:
    type: owdb
    dir: /var/lib/jelly/hitsdb
    file: hitsdb.owdb

users:
  enabled: true
  base: /users
  uses: [testdb, userdb]
  vriska: 88888888
`,
			expect: jelly.Config{
				Globals: jelly.Globals{
					Port:             80,
					Address:          "10.0.28.16",
					URIBase:          "api/",
					MainAuthProvider: "john.egbert",
				},
				DBs: map[string]jelly.DatabaseConfig{
					"testdb": {
						Type:     jelly.DatabaseSQLite,
						DataFile: "/var/lib/jelly/testdb.sqlite",
					},
					"userdb": {
						Type:      jelly.DatabaseInMemory,
						Connector: "john",
					},
					"hitsdb": {
						Type:     jelly.DatabaseOWDB,
						DataDir:  "/var/lib/jelly/hitsdb",
						DataFile: "hitsdb.owdb",
					},
				},
				Log: jelly.LogConfig{
					Enabled:  true,
					Provider: jelly.StdLog,
					File:     "/var/log/jelly.log",
				},
				APIs: map[string]jelly.APIConfig{
					"users": &jelly.CommonConfig{
						Name:    "users",
						Enabled: true,
						Base:    "/users",
						UsesDBs: []string{"testdb", "userdb"},
					},
				},
				Format: jelly.YAML,
			},
		},
		{
			name:     "json - all options config file, default conf reader",
			env:      &Environment{},
			filename: "config.json",
			content: `
		{
			"listen": "10.0.28.16:80",
			"base": "api/",
			"authenticator": "john.egbert",
			"logging": {
				"enabled": true,
				"provider": "std",
				"file": "/var/log/jelly.log"
			},
			"dbs": {
				"testdb": {
					"type": "sqlite",
					"file": "/var/lib/jelly/testdb.sqlite"
				},
				"userdb": {
					"type": "inmem",
					"connector": "john"
				},
				"hitsdb": {
					"type": "owdb",
					"dir": "/var/lib/jelly/hitsdb",
					"file": "hitsdb.owdb"
				}
			},
			"users": {
				"enabled": true,
				"base": "/users",
				"uses": ["testdb", "userdb"],
				"vriska": 88888888
			}
		}
		`,
			expect: jelly.Config{
				Globals: jelly.Globals{
					Port:             80,
					Address:          "10.0.28.16",
					URIBase:          "api/",
					MainAuthProvider: "john.egbert",
				},
				DBs: map[string]jelly.DatabaseConfig{
					"testdb": {
						Type:     jelly.DatabaseSQLite,
						DataFile: "/var/lib/jelly/testdb.sqlite",
					},
					"userdb": {
						Type:      jelly.DatabaseInMemory,
						Connector: "john",
					},
					"hitsdb": {
						Type:     jelly.DatabaseOWDB,
						DataDir:  "/var/lib/jelly/hitsdb",
						DataFile: "hitsdb.owdb",
					},
				},
				Log: jelly.LogConfig{
					Enabled:  true,
					Provider: jelly.StdLog,
					File:     "/var/log/jelly.log",
				},
				APIs: map[string]jelly.APIConfig{
					"users": &jelly.CommonConfig{
						Name:    "users",
						Enabled: true,
						Base:    "/users",
						UsesDBs: []string{"testdb", "userdb"},
					},
				},
				Format: jelly.JSON,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			// dump contents of config to a temp file
			tmpdir := t.TempDir()
			confPath := filepath.Join(tmpdir, tc.filename)
			writeFileErr := os.WriteFile(confPath, []byte(tc.content), 0666)
			if writeFileErr != nil {
				panic(fmt.Sprintf("failed to write file to load from: %v", writeFileErr))
			}

			// config file now exists, time to load it
			actual, err := tc.env.Load(confPath)
			if tc.expectErrContains == "" {
				if !assert.NoError(err) {
					return
				}
				assert.Equal(tc.expect, actual)
			} else {
				assert.ErrorContains(err, tc.expectErrContains)
			}
		})
	}

	// slightly more complicated test cases:
	t.Run("yaml - all options config file, registered conf reader", func(t *testing.T) {
		filename := "config.yaml"
		content := `
listen: 10.0.28.16:80
base: api/
authenticator: john.egbert

logging:
  enabled: true
  provider: std
  file: /var/log/jelly.log

dbs:
  testdb:
    type: sqlite
    file: /var/lib/jelly/testdb.sqlite
  userdb:
    type: inmem
    connector: john
  hitsdb:
    type: owdb
    dir: /var/lib/jelly/hitsdb
    file: hitsdb.owdb

users:
  enabled: true
  base: /users
  uses: [testdb, userdb]
  vriska: 88888888
`
		expect := jelly.Config{
			Globals: jelly.Globals{
				Port:             80,
				Address:          "10.0.28.16",
				URIBase:          "api/",
				MainAuthProvider: "john.egbert",
			},
			DBs: map[string]jelly.DatabaseConfig{
				"testdb": {
					Type:     jelly.DatabaseSQLite,
					DataFile: "/var/lib/jelly/testdb.sqlite",
				},
				"userdb": {
					Type:      jelly.DatabaseInMemory,
					Connector: "john",
				},
				"hitsdb": {
					Type:     jelly.DatabaseOWDB,
					DataDir:  "/var/lib/jelly/hitsdb",
					DataFile: "hitsdb.owdb",
				},
			},
			Log: jelly.LogConfig{
				Enabled:  true,
				Provider: jelly.StdLog,
				File:     "/var/log/jelly.log",
			},
			APIs: map[string]jelly.APIConfig{
				"users": &jelly.CommonConfig{
					Name:    "users",
					Enabled: true,
					Base:    "/users",
					UsesDBs: []string{"testdb", "userdb"},
				},
			},
			Format: jelly.YAML,
		}

		assert := assert.New(t)

		// dump contents of config to a temp file
		tmpdir := t.TempDir()
		confPath := filepath.Join(tmpdir, filename)
		writeFileErr := os.WriteFile(confPath, []byte(content), 0666)
		if writeFileErr != nil {
			panic(fmt.Sprintf("failed to write file to load from: %v", writeFileErr))
		}

		// setup
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUsersConfig := mock_jelly.NewMockAPIConfig(ctrl)
		mockUsersConfig.EXPECT().Set("vriska", 88888888).Return(nil)

		// setup env
		env := &Environment{
			apiConfigProviders: map[string]func() jelly.APIConfig{
				"users": func() jelly.APIConfig { return mockUsersConfig },
			},
		}

		actual, err := env.Load(confPath)

		if !assert.NoError(err) {
			return
		}

		assert.Equal(expect, actual)
	})
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
