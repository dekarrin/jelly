package config

import (
	"strings"
	"time"
)

// Bundle contains configuration specific to an API and global server properties
// to make a complete API-specific view of a config. It also makes accessing
// properties a little less cumbersome via its particular GetX functions.
type Bundle struct {
	api APIConfig
	g   Globals
}

func NewBundle(api APIConfig, g Globals) Bundle {
	return Bundle{api: api, g: g}
}

// ServerPort returns the port that the server the API is being initialized for
// will listen on.
func (bnd Bundle) ServerPort() int {
	return bnd.g.Port
}

// ServerAddress returns the address that the server the API is being
// initialized for will listen on.
func (bnd Bundle) ServerAddress() string {
	return bnd.g.Address
}

// ServerBase returns the base path that all APIs in the server are mounted at.
// It will perform any needed normalization of the base string to ensure that it
// is non-empty, starts with a slash, and does not end with a slash except if it
// is "/". Can be useful for establishing "complete" paths to entities, although
// if a complete base path to the API is needed, [Bundle.Base] can be called.
func (bnd Bundle) ServerBase() string {
	base := bnd.g.URIBase

	for len(base) > 0 && base[len(base)-1] == '/' {
		// do not end with a slash, please
		base = base[:len(base)-1]
	}
	if len(base) == 0 || base[0] != '/' {
		base = "/" + base
	}

	return strings.ToLower(base)
}

// ServerUnauthDelay returns the amount of time that the server is configured to
// wait before serving an error resposne to unauthenticated requests. This is
// often needed for passing into Endpoint.
func (bnd Bundle) ServerUnauthDelay() time.Duration {
	return bnd.g.UnauthDelay()
}

// Base returns the complete URIBase path configured for any methods. This takes
// ServerBase() and APIBase() and appends them together, handling
// doubled-slashes.
func (bnd Bundle) Base() string {
	svBase := bnd.ServerBase()
	apiBase := bnd.APIBase()

	var base string
	if svBase != "" && svBase != "/" {
		base = svBase
	}
	if apiBase != "" && apiBase != "/" {
		if base != "" {
			base = base + apiBase
		} else {
			base = apiBase
		}
	}

	if base == "" {
		base = "/"
	}

	return base
}

// Has returns whether the given key exists in the API config.
func (bnd Bundle) Has(key string) bool {
	return apiHas(bnd.api, key)
}

// Name returns the name of the API as read from the API config.
//
// This is a convenience function equivalent to calling bnd.Get(KeyAPIName).
func (bnd Bundle) Name() string {
	return bnd.Get(KeyAPIName)
}

// APIBase returns the base path of the API that its routes are all mounted at.
// It will perform any needed normalization of the base string to ensure that it
// is non-empty, starts with a slash, and does not end with a slash except if it
// is "/". The returned base path is relative to the ServerBase; combine both
// ServerBase and APIBase to get the complete URI base path, or call Base() to
// do it for you.
//
// This is a convenience function equivalent to calling bnd.Get(KeyAPIBase).
func (bnd Bundle) APIBase() string {
	base := bnd.Get(KeyAPIBase)

	for len(base) > 0 && base[len(base)-1] == '/' {
		// do not end with a slash, please
		base = base[:len(base)-1]
	}
	if len(base) == 0 || base[0] != '/' {
		base = "/" + base
	}

	return strings.ToLower(base)
}

// UsesDBs returns the list of database names that the API is configured to
// connect to, in the order they were listed in config.
//
// This is a convenience function equivalent to calling
// bnd.GetSlice(KeyAPIUsesDBs).
func (bnd Bundle) UsesDBs() []string {
	return bnd.GetSlice(KeyAPIUsesDBs)
}

// Enabled returns whether the API was set to be enabled. Since this is required
// for an API to be initialized, this will always be true for an API receiving a
// Bundle in its Init method.
//
// This is a convenience function equivalent to calling
// bnd.GetBool(KeyAPIEnabled).
func (bnd Bundle) Enabled() bool {
	return bnd.GetBool(KeyAPIEnabled)
}

// Get retrieves the value of a string-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) Get(key string) string {
	var v string

	if !bnd.Has(key) {
		return v
	}

	return Get[string](bnd.api, key)
}

// GetByteSlice retrieves the value of a []byte-typed API configuration key. If
// it doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) GetByteSlice(key string) []byte {
	var v []byte

	if !bnd.Has(key) {
		return v
	}

	return Get[[]byte](bnd.api, key)
}

// GetSlice retrieves the value of a []string-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) GetSlice(key string) []string {
	var v []string

	if !bnd.Has(key) {
		return v
	}

	return Get[[]string](bnd.api, key)
}

// GetIntSlice retrieves the value of a []int-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) GetIntSlice(key string) []int {
	var v []int

	if !bnd.Has(key) {
		return v
	}

	return Get[[]int](bnd.api, key)
}

// GetBoolSlice retrieves the value of a []bool-typed API configuration key. If
// it doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) GetBoolSlice(key string) []bool {
	var v []bool

	if !bnd.Has(key) {
		return v
	}

	return Get[[]bool](bnd.api, key)
}

// GetFloatSlice retrieves the value of a []float64-typed API configuration key.
// If it doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) GetFloatSlice(key string) []float64 {
	var v []float64

	if !bnd.Has(key) {
		return v
	}

	return Get[[]float64](bnd.api, key)
}

// GetBool retrieves the value of a bool-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) GetBool(key string) bool {
	var v bool

	if !bnd.Has(key) {
		return v
	}

	return Get[bool](bnd.api, key)
}

// GetFloat retrieves the value of a float64-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) GetFloat(key string) float64 {
	var v float64

	if !bnd.Has(key) {
		return v
	}

	return Get[float64](bnd.api, key)
}

// GetTime retrieves the value of a time.Time-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) GetTime(key string) time.Time {
	var v time.Time

	if !bnd.Has(key) {
		return v
	}

	return Get[time.Time](bnd.api, key)
}

// GetInt retrieves the value of an int-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) GetInt(key string) int {
	var v int

	if !bnd.Has(key) {
		return v
	}

	return Get[int](bnd.api, key)
}

// GetInt8 retrieves the value of an int8-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) GetInt8(key string) int8 {
	var v int8

	if !bnd.Has(key) {
		return v
	}

	return Get[int8](bnd.api, key)
}

// GetInt16 retrieves the value of an int16-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) GetInt16(key string) int16 {
	var v int16

	if !bnd.Has(key) {
		return v
	}

	return Get[int16](bnd.api, key)
}

// GetInt32 retrieves the value of an int32-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) GetInt32(key string) int32 {
	var v int32

	if !bnd.Has(key) {
		return v
	}

	return Get[int32](bnd.api, key)
}

// GetInt64 retrieves the value of an int64-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) GetInt64(key string) int64 {
	var v int64

	if !bnd.Has(key) {
		return v
	}

	return Get[int64](bnd.api, key)
}

// GetUint retrieves the value of a uint-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) GetUint(key string) uint {
	var v uint

	if !bnd.Has(key) {
		return v
	}

	return Get[uint](bnd.api, key)
}

// GetUint8 retrieves the value of a uint8-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) GetUint8(key string) uint8 {
	var v uint8

	if !bnd.Has(key) {
		return v
	}

	return Get[uint8](bnd.api, key)
}

// GetUint16 retrieves the value of a uint16-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) GetUint16(key string) uint16 {
	var v uint16

	if !bnd.Has(key) {
		return v
	}

	return Get[uint16](bnd.api, key)
}

// GetUint32 retrieves the value of a uint32-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) GetUint32(key string) uint32 {
	var v uint32

	if !bnd.Has(key) {
		return v
	}

	return Get[uint32](bnd.api, key)
}

// GetUint64 retrieves the value of a uint64-typed API configuration key. If it
// doesn't exist in the config, the zero-value is returned.
func (bnd Bundle) GetUint64(key string) uint64 {
	var v uint64

	if !bnd.Has(key) {
		return v
	}

	return Get[uint64](bnd.api, key)
}
