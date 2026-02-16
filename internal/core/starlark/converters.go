// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import "go.starlark.net/starlark"

// goToStarlark converts a Go value to a Starlark value.
// Supports: nil, bool, int, int64, float64, string, []interface{}, []map[string]interface{}, map[string]interface{}
func goToStarlark(v interface{}) starlark.Value {
	switch val := v.(type) {
	case nil:
		return starlark.None
	case bool:
		return starlark.Bool(val)
	case int:
		return starlark.MakeInt(val)
	case int64:
		return starlark.MakeInt64(val)
	case float64:
		return starlark.Float(val)
	case string:
		return starlark.String(val)
	case []interface{}:
		items := make([]starlark.Value, len(val))
		for i, item := range val {
			items[i] = goToStarlark(item)
		}
		return starlark.NewList(items)
	case []map[string]interface{}:
		// Handle TOML array-of-tables
		items := make([]starlark.Value, len(val))
		for i, item := range val {
			items[i] = goToStarlark(item)
		}
		return starlark.NewList(items)
	case map[string]interface{}:
		dict := starlark.NewDict(len(val))
		for k, v := range val {
			dict.SetKey(starlark.String(k), goToStarlark(v))
		}
		return dict
	default:
		return starlark.None
	}
}

// starlarkToGo converts a Starlark value to a Go value.
// Supports: String, Int, Bool, Float, List, Dict
func starlarkToGo(val starlark.Value) any {
	switch v := val.(type) {
	case starlark.String:
		return string(v)
	case starlark.Int:
		i, _ := v.Int64()
		return int(i)
	case starlark.Bool:
		return bool(v)
	case starlark.Float:
		return float64(v)
	case *starlark.List:
		result := make([]any, v.Len())
		for i := 0; i < v.Len(); i++ {
			result[i] = starlarkToGo(v.Index(i))
		}
		return result
	case *starlark.Dict:
		result := make(map[string]any)
		for _, item := range v.Items() {
			if key, ok := item[0].(starlark.String); ok {
				result[string(key)] = starlarkToGo(item[1])
			}
		}
		return result
	default:
		return nil
	}
}
