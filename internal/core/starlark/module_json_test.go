// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

func TestJSONModule_Parse(t *testing.T) {
	jsonModule := NewJSONModule()

	t.Run("parses JSON string to Starlark dict", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := jsonModule.Members["parse"].(starlark.Callable)

		jsonStr := `{"name": "test", "count": 42, "active": true}`
		args := starlark.Tuple{starlark.String(jsonStr)}
		result, err := starlark.Call(thread, parseFunc, args, nil)

		require.NoError(t, err)
		dict, ok := result.(*starlark.Dict)
		require.True(t, ok, "parse should return a dict")

		// Check name field
		nameVal, found, err := dict.Get(starlark.String("name"))
		require.NoError(t, err)
		require.True(t, found)
		name, _ := starlark.AsString(nameVal)
		assert.Equal(t, "test", name)

		// Check count field
		countVal, found, err := dict.Get(starlark.String("count"))
		require.NoError(t, err)
		require.True(t, found)
		// JSON numbers are parsed as float64
		count, ok := starlark.AsFloat(countVal)
		require.True(t, ok)
		assert.Equal(t, 42.0, count)

		// Check active field
		activeVal, found, err := dict.Get(starlark.String("active"))
		require.NoError(t, err)
		require.True(t, found)
		active := bool(activeVal.(starlark.Bool))
		assert.True(t, active)
	})

	t.Run("parses JSON array to Starlark list", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := jsonModule.Members["parse"].(starlark.Callable)

		jsonStr := `["apple", "banana", "cherry"]`
		args := starlark.Tuple{starlark.String(jsonStr)}
		result, err := starlark.Call(thread, parseFunc, args, nil)

		require.NoError(t, err)
		list, ok := result.(*starlark.List)
		require.True(t, ok, "parse should return a list")
		assert.Equal(t, 3, list.Len())

		item, _ := starlark.AsString(list.Index(0))
		assert.Equal(t, "apple", item)
	})

	t.Run("parses JSON null to None", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := jsonModule.Members["parse"].(starlark.Callable)

		jsonStr := `null`
		args := starlark.Tuple{starlark.String(jsonStr)}
		result, err := starlark.Call(thread, parseFunc, args, nil)

		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)
	})

	t.Run("parses nested JSON structures", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := jsonModule.Members["parse"].(starlark.Callable)

		jsonStr := `{"user": {"name": "Alice", "age": 30}, "tags": ["admin", "user"]}`
		args := starlark.Tuple{starlark.String(jsonStr)}
		result, err := starlark.Call(thread, parseFunc, args, nil)

		require.NoError(t, err)
		dict, ok := result.(*starlark.Dict)
		require.True(t, ok)

		// Check user.name
		userVal, found, _ := dict.Get(starlark.String("user"))
		require.True(t, found)
		userDict, ok := userVal.(*starlark.Dict)
		require.True(t, ok)

		nameVal, found, _ := userDict.Get(starlark.String("name"))
		require.True(t, found)
		name, _ := starlark.AsString(nameVal)
		assert.Equal(t, "Alice", name)

		// Check tags array
		tagsVal, found, _ := dict.Get(starlark.String("tags"))
		require.True(t, found)
		tagsList, ok := tagsVal.(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 2, tagsList.Len())
	})

	t.Run("fails on invalid JSON", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := jsonModule.Members["parse"].(starlark.Callable)

		jsonStr := `{invalid json}`
		args := starlark.Tuple{starlark.String(jsonStr)}
		_, err := starlark.Call(thread, parseFunc, args, nil)

		require.Error(t, err)
	})
}

func TestJSONModule_Stringify(t *testing.T) {
	jsonModule := NewJSONModule()

	t.Run("stringifies Starlark dict to JSON", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := jsonModule.Members["stringify"].(starlark.Callable)

		dict := starlark.NewDict(2)
		dict.SetKey(starlark.String("name"), starlark.String("test"))
		dict.SetKey(starlark.String("count"), starlark.MakeInt(42))

		args := starlark.Tuple{dict}
		result, err := starlark.Call(thread, stringifyFunc, args, nil)

		require.NoError(t, err)
		jsonStr, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Contains(t, jsonStr, `"name":"test"`)
		assert.Contains(t, jsonStr, `"count":42`)
	})

	t.Run("stringifies Starlark list to JSON array", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := jsonModule.Members["stringify"].(starlark.Callable)

		list := starlark.NewList([]starlark.Value{
			starlark.String("apple"),
			starlark.String("banana"),
			starlark.MakeInt(123),
		})

		args := starlark.Tuple{list}
		result, err := starlark.Call(thread, stringifyFunc, args, nil)

		require.NoError(t, err)
		jsonStr, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Equal(t, `["apple","banana",123]`, jsonStr)
	})

	t.Run("stringifies with indentation", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := jsonModule.Members["stringify"].(starlark.Callable)

		dict := starlark.NewDict(1)
		dict.SetKey(starlark.String("key"), starlark.String("value"))

		args := starlark.Tuple{dict}
		kwargs := []starlark.Tuple{
			{starlark.String("indent"), starlark.MakeInt(2)},
		}
		result, err := starlark.Call(thread, stringifyFunc, args, kwargs)

		require.NoError(t, err)
		jsonStr, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Contains(t, jsonStr, "\n") // Should have newlines when indented
		assert.Contains(t, jsonStr, "  ") // Should have indentation
	})

	t.Run("stringifies None to null", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := jsonModule.Members["stringify"].(starlark.Callable)

		args := starlark.Tuple{starlark.None}
		result, err := starlark.Call(thread, stringifyFunc, args, nil)

		require.NoError(t, err)
		jsonStr, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Equal(t, "null", jsonStr)
	})

	t.Run("stringifies boolean values", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := jsonModule.Members["stringify"].(starlark.Callable)

		args := starlark.Tuple{starlark.Bool(true)}
		result, err := starlark.Call(thread, stringifyFunc, args, nil)

		require.NoError(t, err)
		jsonStr, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Equal(t, "true", jsonStr)
	})

	t.Run("stringifies nested structures", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := jsonModule.Members["stringify"].(starlark.Callable)

		innerDict := starlark.NewDict(1)
		innerDict.SetKey(starlark.String("age"), starlark.MakeInt(30))

		outerDict := starlark.NewDict(2)
		outerDict.SetKey(starlark.String("user"), innerDict)
		outerDict.SetKey(starlark.String("active"), starlark.Bool(true))

		args := starlark.Tuple{outerDict}
		result, err := starlark.Call(thread, stringifyFunc, args, nil)

		require.NoError(t, err)
		jsonStr, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Contains(t, jsonStr, `"user"`)
		assert.Contains(t, jsonStr, `"age":30`)
	})
}

func TestJSONModule_RoundTrip(t *testing.T) {
	jsonModule := NewJSONModule()

	t.Run("parse and stringify round trip", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := jsonModule.Members["parse"].(starlark.Callable)
		stringifyFunc := jsonModule.Members["stringify"].(starlark.Callable)

		originalJSON := `{"name":"Alice","age":30,"active":true}`

		// Parse
		args := starlark.Tuple{starlark.String(originalJSON)}
		parsed, err := starlark.Call(thread, parseFunc, args, nil)
		require.NoError(t, err)

		// Stringify
		args = starlark.Tuple{parsed}
		result, err := starlark.Call(thread, stringifyFunc, args, nil)
		require.NoError(t, err)

		jsonStr, _ := starlark.AsString(result)
		// Values should be preserved (exact formatting may differ)
		assert.Contains(t, jsonStr, `"name":"Alice"`)
		assert.Contains(t, jsonStr, `"age":30`)
		assert.Contains(t, jsonStr, `"active":true`)
	})
}
