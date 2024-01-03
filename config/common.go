package config

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	KeyAPIName    = "name"
	KeyAPIBase    = "base"
	KeyAPIEnabled = "enabled"
	KeyAPIUsesDBs = "uses"
)

// Common holds configuration options common to all APIs.
type Common struct {
	// Name is the name of the API. Must be unique.
	Name string

	// Enabled is whether the API is to be enabled. By default, this is false in
	// all cases.
	Enabled bool

	// Base is the base URI that all paths will be rooted at, relative to the
	// server base path. This can be "/" (or "", which is equivalent) to
	// indicate that the API is to be based directly at the URIBase of the
	// server config that this API is a part of.
	Base string

	// UsesDBs is a list of names of data stores and authenticators that the API
	// uses directly. When Init is called, it is passed active connections to
	// each of the DBs. There must be a corresponding entry for each DB name in
	// the root DBs listing in the Config this API is a part of. The
	// Authenticators slice should contain only authenticators that are provided
	// by other APIs; see their documentation for which they provide.
	UsesDBs []string
}

// FillDefaults returns a new *Common identical to cc but with unset values set
// to their defaults and values normalized.
func (cc *Common) FillDefaults() APIConfig {
	newCC := new(Common)

	if newCC.Base == "" {
		newCC.Base = "/"
	}

	return newCC
}

// Validate returns an error if the Config has invalid field values set. Empty
// and unset values are considered invalid; if defaults are intended to be used,
// call Validate on the return value of FillDefaults.
func (cc *Common) Validate() error {
	if err := validateBaseURI(cc.Base); err != nil {
		return fmt.Errorf(KeyAPIBase+": %w", err)
	}

	return nil
}

func (cc *Common) Common() Common {
	return *cc
}

func (cc *Common) Keys() []string {
	return []string{KeyAPIName, KeyAPIEnabled, KeyAPIBase, KeyAPIUsesDBs}
}

func (cc *Common) Get(key string) interface{} {
	switch strings.ToLower(key) {
	case KeyAPIName:
		return cc.Name
	case KeyAPIEnabled:
		return cc.Enabled
	case KeyAPIBase:
		return cc.Base
	case KeyAPIUsesDBs:
		return cc.UsesDBs
	default:
		return nil
	}
}

func (cc *Common) Set(key string, value interface{}) error {
	switch strings.ToLower(key) {
	case KeyAPIName:
		if valueStr, ok := value.(string); ok {
			cc.Name = valueStr
			return nil
		} else {
			return fmt.Errorf("key '"+KeyAPIName+"' requires a string but got a %T", value)
		}
	case KeyAPIEnabled:
		if valueBool, ok := value.(bool); ok {
			cc.Enabled = valueBool
			return nil
		} else {
			return fmt.Errorf("key '"+KeyAPIEnabled+"' requires a bool but got a %T", value)
		}
	case KeyAPIBase:
		if valueStr, ok := value.(string); ok {
			cc.Base = valueStr
			return nil
		} else {
			return fmt.Errorf("key '"+KeyAPIBase+"' requires a string but got a %T", value)
		}
	case KeyAPIUsesDBs:
		if valueStrSlice, ok := value.([]string); ok {
			cc.UsesDBs = valueStrSlice
			return nil
		} else {
			return fmt.Errorf("key '"+KeyAPIUsesDBs+"' requires a []string but got a %T", value)
		}
	default:
		return fmt.Errorf("not a valid key: %q", key)
	}
}

func (cc *Common) SetFromString(key string, value string) error {
	switch strings.ToLower(key) {
	case KeyAPIName, KeyAPIBase:
		return cc.Set(key, value)
	case KeyAPIEnabled:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		return cc.Set(key, b)
	case KeyAPIUsesDBs:
		if value == "" {
			return cc.Set(key, []string{})
		}
		dbsStrSlice := strings.Split(value, ",")
		return cc.Set(key, dbsStrSlice)
	default:
		return fmt.Errorf("not a valid key: %q", key)
	}
}
