package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dekarrin/jelly"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func Test_Environment_DumpConfig(t *testing.T) {
	envEmpty := func(t *testing.T) *Environment {
		return &Environment{}
	}

	testCases := []struct {
		name     string
		envSetup func(t *testing.T) *Environment

		config jelly.Config
		expect map[string]interface{}
	}{
		{
			name:     "empty config, no format",
			config:   jelly.Config{},
			envSetup: envEmpty,
			expect: map[string]interface{}{
				"listen": ":0",
				"base":   "",
				"dbs":    map[string]interface{}{},
				"logging": map[string]interface{}{
					"enabled":  false,
					"provider": "none",
				},
				"authenticator": "",
			},
		},
		{
			name:     "empty config, YAML",
			envSetup: envEmpty,
			config:   jelly.Config{Format: jelly.YAML},
			expect: map[string]interface{}{
				"listen": ":0",
				"base":   "",
				"dbs":    map[string]interface{}{},
				"logging": map[string]interface{}{
					"enabled":  false,
					"provider": "none",
				},
				"authenticator": "",
			},
		},
		{
			name:     "empty config, JSON",
			envSetup: envEmpty,
			config:   jelly.Config{Format: jelly.JSON},
			expect: map[string]interface{}{
				"listen": ":0",
				"base":   "",
				"dbs":    map[string]interface{}{},
				"logging": map[string]interface{}{
					"enabled":  false,
					"provider": "none",
				},
				"authenticator": "",
			},
		},
		{
			name:     "full config - YAML",
			envSetup: envEmpty,
			config: jelly.Config{
				Format: jelly.YAML,
				Globals: jelly.Globals{
					Port:             80,
					Address:          "10.28.10.1",
					URIBase:          "v1/api/",
					MainAuthProvider: "john.egbert",
				},
				DBs: map[string]jelly.DatabaseConfig{
					"testdb": {
						Type:      jelly.DatabaseSQLite,
						DataFile:  "/var/lib/jelly/testdb.sqlite",
						Connector: "*",
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
					Provider: jelly.Jellog,
					File:     "/var/log/jelly.log",
				},
				APIs: map[string]jelly.APIConfig{
					"users": &testAPIConfig{
						CommonConfig: jelly.CommonConfig{
							Name:    "users",
							Enabled: true,
							Base:    "/auth/users",
							UsesDBs: []string{"testdb", "userdb"},
						},
						Vriska: 88888888,
					},
					"hits": &jelly.CommonConfig{
						Name:    "hits",
						Enabled: true,
						Base:    "/admin/hits",
						UsesDBs: []string{"hitsdb"},
					},
					"inprog": &jelly.CommonConfig{
						Name:    "inprog",
						Enabled: false,
						Base:    "/admin/inprog",
						UsesDBs: []string{"hitsdb"},
					},
				},
			},
			expect: map[string]interface{}{
				"listen":        "10.28.10.1:80",
				"base":          "v1/api/",
				"authenticator": "john.egbert",
				"dbs": map[string]interface{}{
					"testdb": map[string]interface{}{
						"type":      "sqlite",
						"file":      "/var/lib/jelly/testdb.sqlite",
						"connector": "*",
					},
					"userdb": map[string]interface{}{
						"type":      "inmem",
						"connector": "john",
					},
					"hitsdb": map[string]interface{}{
						"type": "owdb",
						"dir":  "/var/lib/jelly/hitsdb",
						"file": "hitsdb.owdb",
					},
				},
				"users": map[string]interface{}{
					"enabled": true,
					"base":    "/auth/users",
					"uses":    []interface{}{"testdb", "userdb"},
					"vriska":  88888888,
				},
				"hits": map[string]interface{}{
					"enabled": true,
					"base":    "/admin/hits",
					"uses":    []interface{}{"hitsdb"},
				},
				"inprog": map[string]interface{}{
					"enabled": false,
					"base":    "/admin/inprog",
					"uses":    []interface{}{"hitsdb"},
				},
				"logging": map[string]interface{}{
					"enabled":  true,
					"provider": "jellog",
					"file":     "/var/log/jelly.log",
				},
			},
		},
		{
			name:     "full config - JSON",
			envSetup: envEmpty,
			config: jelly.Config{
				Format: jelly.JSON,
				Globals: jelly.Globals{
					Port:             80,
					Address:          "10.28.10.1",
					URIBase:          "v1/api/",
					MainAuthProvider: "john.egbert",
				},
				DBs: map[string]jelly.DatabaseConfig{
					"testdb": {
						Type:      jelly.DatabaseSQLite,
						DataFile:  "/var/lib/jelly/testdb.sqlite",
						Connector: "*",
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
					Provider: jelly.Jellog,
					File:     "/var/log/jelly.log",
				},
				APIs: map[string]jelly.APIConfig{
					"users": &testAPIConfig{
						CommonConfig: jelly.CommonConfig{
							Name:    "users",
							Enabled: true,
							Base:    "/auth/users",
							UsesDBs: []string{"testdb", "userdb"},
						},
						Vriska: 88888888,
					},
					"hits": &jelly.CommonConfig{
						Name:    "hits",
						Enabled: true,
						Base:    "/admin/hits",
						UsesDBs: []string{"hitsdb"},
					},
					"inprog": &jelly.CommonConfig{
						Name:    "inprog",
						Enabled: false,
						Base:    "/admin/inprog",
						UsesDBs: []string{"hitsdb"},
					},
				},
			},
			expect: map[string]interface{}{
				"listen":        "10.28.10.1:80",
				"base":          "v1/api/",
				"authenticator": "john.egbert",
				"dbs": map[string]interface{}{
					"testdb": map[string]interface{}{
						"type":      "sqlite",
						"file":      "/var/lib/jelly/testdb.sqlite",
						"connector": "*",
					},
					"userdb": map[string]interface{}{
						"type":      "inmem",
						"connector": "john",
					},
					"hitsdb": map[string]interface{}{
						"type": "owdb",
						"dir":  "/var/lib/jelly/hitsdb",
						"file": "hitsdb.owdb",
					},
				},
				"users": map[string]interface{}{
					"enabled": true,
					"base":    "/auth/users",
					"uses":    []interface{}{"testdb", "userdb"},
					"vriska":  float64(88888888),
				},
				"hits": map[string]interface{}{
					"enabled": true,
					"base":    "/admin/hits",
					"uses":    []interface{}{"hitsdb"},
				},
				"inprog": map[string]interface{}{
					"enabled": false,
					"base":    "/admin/inprog",
					"uses":    []interface{}{"hitsdb"},
				},
				"logging": map[string]interface{}{
					"enabled":  true,
					"provider": "jellog",
					"file":     "/var/log/jelly.log",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			env := tc.envSetup(t)

			result := env.DumpConfig(tc.config)

			// attempt to parse into result map
			var resultMap map[string]interface{}
			if tc.config.Format == jelly.JSON {
				err := json.Unmarshal(result, &resultMap)
				if !assert.NoError(err, "parse result back into JSON failed") {
					return
				}
			} else {
				// fallback is jelly.YAML
				err := yaml.Unmarshal(result, &resultMap)
				if !assert.NoError(err, "parse result back into YAML failed") {
					return
				}
			}

			assert.Equal(tc.expect, resultMap)
		})
	}
}

func Test_Environment_LoadConfig(t *testing.T) {
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

	envEmpty := func(t *testing.T) *Environment {
		return &Environment{}
	}

	testCases := []struct {
		name     string
		envSetup func(t *testing.T) *Environment
		filename string
		content  string

		expect            jelly.Config
		expectErrContains string
	}{
		{
			name:              "invalid file extension",
			envSetup:          envEmpty,
			filename:          "config.txt",
			content:           "",
			expectErrContains: "incompatible format",
		},
		{
			name:     "yaml - empty config file",
			envSetup: envEmpty,
			filename: "config.yaml",
			content:  "",
			expect:   emptyYAMLConfig,
		},
		{
			name:     "yaml - listen - address:port",
			envSetup: envEmpty,
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
			envSetup: envEmpty,
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
			envSetup: envEmpty,
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
			envSetup: envEmpty,
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
			envSetup: envEmpty,
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
			envSetup: envEmpty,
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
			envSetup: envEmpty,
			filename: "config.json",
			content:  "",
			expect:   emptyJSONConfig,
		},
		{
			name:     "json - empty object",
			envSetup: envEmpty,
			filename: "config.json",
			content:  "{}",
			expect:   emptyJSONConfig,
		},
		{
			name:     "json - listen - address:port",
			envSetup: envEmpty,
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
			envSetup: envEmpty,
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
			envSetup: envEmpty,
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
			envSetup: envEmpty,
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
			envSetup: envEmpty,
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
			envSetup: envEmpty,
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
			envSetup: envEmpty,
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
			envSetup: envEmpty,
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
		}`,
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
		{
			name: "yaml - all options config file, registered conf reader",
			envSetup: func(t *testing.T) *Environment {
				env := &Environment{}
				env.RegisterConfigSection("users", func() jelly.APIConfig { return &testAPIConfig{} })
				return env
			},
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
					"users": &testAPIConfig{
						CommonConfig: jelly.CommonConfig{
							Name:    "users",
							Enabled: true,
							Base:    "/users",
							UsesDBs: []string{"testdb", "userdb"},
						},
						Vriska: 88888888,
					},
				},
				Format: jelly.YAML,
			},
		},
		{
			name: "json - all options config file, registered conf reader",
			envSetup: func(t *testing.T) *Environment {
				env := &Environment{}
				env.RegisterConfigSection("users", func() jelly.APIConfig { return &testAPIConfig{} })
				return env
			},
			filename: "config.json",
			content: `{
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
			}`,
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
					"users": &testAPIConfig{
						CommonConfig: jelly.CommonConfig{
							Name:    "users",
							Enabled: true,
							Base:    "/users",
							UsesDBs: []string{"testdb", "userdb"},
						},
						Vriska: 88888888,
					},
				},
				Format: jelly.JSON,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			env := tc.envSetup(t)

			// dump contents of config to a temp file
			tmpdir := t.TempDir()
			confPath := filepath.Join(tmpdir, tc.filename)
			writeFileErr := os.WriteFile(confPath, []byte(tc.content), 0666)
			if writeFileErr != nil {
				panic(fmt.Sprintf("failed to write file to load from: %v", writeFileErr))
			}

			// config file now exists, time to load it
			actual, err := env.LoadConfig(confPath)
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
}
