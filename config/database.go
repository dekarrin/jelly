package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dekarrin/jelly/db/owdb"
	"github.com/dekarrin/jelly/internal/authuserdao/inmem"
	"github.com/dekarrin/jelly/internal/authuserdao/sqlite"
	"github.com/dekarrin/jelly/types"
)

// ConnectorRegistry holds registered connecter functions for opening store
// structs on database connections.
//
// The zero value can be immediately used and will have the built-in default and
// pre-rolled connectors available. This can be disabled by setting
// DisableDefaults to true before attempting to use it.
type ConnectorRegistry struct {
	DisableDefaults bool
	reg             map[types.DBType]map[string]func(types.DatabaseConfig) (types.Store, error)
}

func (cr *ConnectorRegistry) initDefaults() {
	// TODO: follow initDefaults pattern on all env-y structs

	if cr.reg == nil {
		cr.reg = map[types.DBType]map[string]func(types.DatabaseConfig) (types.Store, error){
			types.DatabaseInMemory: {},
			types.DatabaseSQLite:   {},
			types.DatabaseOWDB:     {},
		}

		if !cr.DisableDefaults {
			cr.reg[types.DatabaseInMemory]["authuser"] = func(d types.DatabaseConfig) (types.Store, error) {
				return inmem.NewAuthUserStore(), nil
			}
			cr.reg[types.DatabaseSQLite]["authuser"] = func(db types.DatabaseConfig) (types.Store, error) {
				err := os.MkdirAll(db.DataDir, 0770)
				if err != nil {
					return nil, fmt.Errorf("create data dir: %w", err)
				}

				store, err := sqlite.NewAuthUserStore(db.DataDir)
				if err != nil {
					return nil, fmt.Errorf("initialize sqlite: %w", err)
				}

				return store, nil
			}
			cr.reg[types.DatabaseOWDB]["*"] = func(db types.DatabaseConfig) (types.Store, error) {
				err := os.MkdirAll(db.DataDir, 0770)
				if err != nil {
					return nil, fmt.Errorf("create data dir: %w", err)
				}

				fullPath := filepath.Join(db.DataDir, db.DataFile)
				store, err := owdb.Open(fullPath)
				if err != nil {
					return nil, fmt.Errorf("initialize owdb: %w", err)
				}

				return store, nil
			}
		}
	}
}

func (cr *ConnectorRegistry) Register(engine types.DBType, name string, connector func(types.DatabaseConfig) (types.Store, error)) error {
	if connector == nil {
		return fmt.Errorf("connector function cannot be nil")
	}

	cr.initDefaults()

	engConns, ok := cr.reg[engine]
	if !ok {
		return fmt.Errorf("%q is not a supported DB type", engine)
	}

	normName := strings.ToLower(name)
	if _, ok := engConns[normName]; ok && normName != "*" {
		return fmt.Errorf("duplicate connector registration; %q/%q already has a registered connector", engine, normName)
	}

	engConns[normName] = connector
	cr.reg[engine] = engConns
	return nil
}

// List returns an alphabetized list of all currently registered connector
// names for an engine.
func (cr *ConnectorRegistry) List(engine types.DBType) []string {
	cr.initDefaults()

	engConns := cr.reg[engine]

	names := make([]string, len(engConns))

	var cur int
	for k := range engConns {
		names[cur] = k
	}

	sort.Strings(names)
	return names
}

// Connect opens a connection to the configured database, returning a generic
// db.Store. The Store can then be cast to the appropriate type by APIs in
// their init method.
func (cr *ConnectorRegistry) Connect(db types.DatabaseConfig) (types.Store, error) {
	cr.initDefaults()

	engConns := cr.reg[db.Type]

	normName := strings.ToLower(db.Connector)
	connector, ok := engConns[normName]
	if !ok {
		connector, ok = engConns["*"]
		if !ok {
			var additionalInfo = "DB does not specify connector"
			if normName != "" && normName != "*" {
				additionalInfo = fmt.Sprintf("%q/%q is not a registered connector", db.Type, normName)
			}
			return nil, fmt.Errorf("%s and %q has no default \"*\" connector registered", additionalInfo, db.Type)
		}
	}

	return connector(db)
}
