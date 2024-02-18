// Package config contains configuration options for the server as well as
// various config contstants.
package config

import (
	"fmt"
	"strings"

	"github.com/dekarrin/jelly/types"
)

// Get returns a value from an types.APIConfig. Panics if the given value is not of
// the given type or if there is an error retrieving it or if the given key does
// not exist.
func Get[E any](api types.APIConfig, key string) E {
	if !apiHas(api, key) {
		panic(fmt.Sprintf("config does not contain key %q", key))
	}
	v := api.Get(key)
	if typed, ok := v.(E); ok {
		return typed
	}

	var check E
	panic(fmt.Sprintf("key %q is not of type %T", key, check))
}

// TypedSlice takes a value that is passed to Set that is expected to be a slice
// of the given type and performs the required conversions. If a non-nil error
// is returned it will contain the key name automatically in its error string.
func TypedSlice[E any](key string, value interface{}) ([]E, error) {
	var typed E
	var typedValues []E

	if valueStr, ok := value.([]E); ok {
		typedValues = valueStr
		return typedValues, nil
	} else if valueSlice, ok := value.([]interface{}); ok {
		var ok bool
		for i := range valueSlice {
			if typed, ok = valueSlice[i].(E); ok {
				typedValues = append(typedValues, typed)
			} else {
				return nil, fmt.Errorf("%s[%d]: %q is not a valid string", key, i, valueSlice[i])
			}
		}
		return typedValues, nil
	} else {
		return nil, fmt.Errorf("key '%s' requires a %T but got a %T", key, typedValues, value)
	}
}

func apiHas(api types.APIConfig, key string) bool {
	needle := strings.ToLower(key)

	for _, k := range api.Keys() {
		if strings.ToLower(k) == needle {
			return true
		}
	}
	return false
}
