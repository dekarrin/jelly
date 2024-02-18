package server

import (
	"fmt"
	"strings"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/internal/config"
	"github.com/dekarrin/jelly/internal/middle"
	"github.com/dekarrin/jelly/types"
)

// Environment is a full Jelly environment that contains all parameters needed
// to run a server. Creating an Environment prior to config loading allows all
// required external functionality to be properly registered.
type Environment struct {
	componentProviders      map[string]func() jelly.API
	componentProvidersOrder []string

	confEnv *config.Environment

	middleProv *middle.Provider

	connectors *config.ConnectorRegistry

	DisableDefaults bool
}

func (env *Environment) initDefaults() {
	if env.componentProviders == nil {
		env.componentProviders = map[string]func() jelly.API{}
		env.componentProvidersOrder = []string{}
		env.confEnv = &config.Environment{DisableDefaults: env.DisableDefaults}
		env.middleProv = &middle.Provider{DisableDefaults: env.DisableDefaults}
		env.connectors = &config.ConnectorRegistry{DisableDefaults: env.DisableDefaults}
	}
}

// UseComponent enables the given component and its section in config. Required
// to be called at least once for every pre-rolled component in use (such as
// jelly/auth) prior to loading config that contains its section. Calling
// UseComponent twice with a component with the same name will cause a panic.
func (env *Environment) UseComponent(c jelly.Component) {
	env.initDefaults()

	normName := strings.ToLower(c.Name())
	if _, ok := env.componentProviders[normName]; ok {
		panic(fmt.Sprintf("duplicate component: %q is already in-use", c.Name()))
	}

	if err := env.RegisterConfigSection(normName, c.Config); err != nil {
		panic(fmt.Sprintf("register component config section: %v", err))
	}

	env.componentProviders[normName] = c.API
	env.componentProvidersOrder = append(env.componentProvidersOrder, normName)
}

// RegisterConfigSection registers a provider function, which creates an
// implementor of config.APIConfig, to the name of the config section that
// should be loaded into it. You must call this for every custom API config
// sections, or they will be given the default common config only at
// initialization.
func (env *Environment) RegisterConfigSection(name string, provider func() types.APIConfig) error {
	env.initDefaults()
	return env.confEnv.Register(name, provider)
}

// SetMainAuthenticator sets what the main authenticator in the middleware
// provider is. This provider will be used when obtaining middleware that uses
// an authenticator but no specific authenticator is specified. The name given
// must be the name of one previously registered with RegisterAuthenticator
func (env *Environment) SetMainAuthenticator(name string) error {
	env.initDefaults()
	return env.middleProv.RegisterMainAuthenticator(name)
}

// RegisterConnector allows the specification of database connection methods.
// The registered name can then be specified as the connector field of any DB
// in config whose type is the given engine.
func (env *Environment) RegisterConnector(engine types.DBType, name string, connector func(types.DatabaseConfig) (types.Store, error)) error {
	env.initDefaults()
	return env.connectors.Register(engine, name, connector)
}

// RegisterAuthenticator registers an authenticator for use with other
// components in a jelly framework environment. This is generally not called
// directly but can be. If attempting to register the authenticator of a
// jelly.Component such as jelly/auth.Component, consider calling UseComponent
// instead as that will automatically call RegisterAuthenticator for any
// authenticators the component provides.
func (env *Environment) RegisterAuthenticator(name string, authen types.Authenticator) error {
	env.initDefaults()
	return env.middleProv.RegisterAuthenticator(name, authen)
}

// LoadConfig loads a configuration from file. Ensure that UseComponent is first
// called on every component that will be configured (such as jelly/auth), and
// ensure RegisterConfigSection is called for each custom config section not
// associated with a component.
func (env *Environment) LoadConfig(file string) (types.Config, error) {
	env.initDefaults()
	return env.confEnv.Load(file)
}

// DumpConfig dumpes the given config to bytes. If Format is not set on the
// Config, YAML is assumed.
func (env *Environment) DumpConfig(cfg types.Config) []byte {
	env.initDefaults()
	return config.Dump(cfg)
}
