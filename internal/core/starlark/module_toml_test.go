// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

func TestTOMLModule_Parse(t *testing.T) {
	tomlModule := NewTOMLModule()

	t.Run("parses TOML string to Starlark dict", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := tomlModule.Members["parse"].(starlark.Callable)

		tomlStr := `
name = "test"
count = 42
active = true
`
		args := starlark.Tuple{starlark.String(tomlStr)}
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
		count, err := starlark.AsInt32(countVal)
		require.NoError(t, err)
		assert.Equal(t, 42, int(count))

		// Check active field
		activeVal, found, err := dict.Get(starlark.String("active"))
		require.NoError(t, err)
		require.True(t, found)
		active := bool(activeVal.(starlark.Bool))
		assert.True(t, active)
	})

	t.Run("parses TOML array", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := tomlModule.Members["parse"].(starlark.Callable)

		tomlStr := `
fruits = ["apple", "banana", "cherry"]
`
		args := starlark.Tuple{starlark.String(tomlStr)}
		result, err := starlark.Call(thread, parseFunc, args, nil)

		require.NoError(t, err)
		dict, ok := result.(*starlark.Dict)
		require.True(t, ok)

		fruitsVal, found, err := dict.Get(starlark.String("fruits"))
		require.NoError(t, err)
		require.True(t, found)

		fruitsList, ok := fruitsVal.(*starlark.List)
		require.True(t, ok, "fruits should be a list")
		assert.Equal(t, 3, fruitsList.Len())

		item, _ := starlark.AsString(fruitsList.Index(0))
		assert.Equal(t, "apple", item)
	})

	t.Run("parses nested TOML tables", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := tomlModule.Members["parse"].(starlark.Callable)

		tomlStr := `
[database]
host = "localhost"
port = 5432

[database.credentials]
username = "admin"
`
		args := starlark.Tuple{starlark.String(tomlStr)}
		result, err := starlark.Call(thread, parseFunc, args, nil)

		require.NoError(t, err)
		dict, ok := result.(*starlark.Dict)
		require.True(t, ok)

		// Check database.host
		dbVal, found, _ := dict.Get(starlark.String("database"))
		require.True(t, found)
		dbDict, ok := dbVal.(*starlark.Dict)
		require.True(t, ok)

		hostVal, found, _ := dbDict.Get(starlark.String("host"))
		require.True(t, found)
		host, _ := starlark.AsString(hostVal)
		assert.Equal(t, "localhost", host)

		// Check database.credentials.username
		credsVal, found, _ := dbDict.Get(starlark.String("credentials"))
		require.True(t, found)
		credsDict, ok := credsVal.(*starlark.Dict)
		require.True(t, ok)

		userVal, found, _ := credsDict.Get(starlark.String("username"))
		require.True(t, found)
		username, _ := starlark.AsString(userVal)
		assert.Equal(t, "admin", username)
	})

	t.Run("parses TOML array of tables", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := tomlModule.Members["parse"].(starlark.Callable)

		tomlStr := `
[[users]]
name = "Alice"
age = 30

[[users]]
name = "Bob"
age = 25
`
		args := starlark.Tuple{starlark.String(tomlStr)}
		result, err := starlark.Call(thread, parseFunc, args, nil)

		require.NoError(t, err)
		dict, ok := result.(*starlark.Dict)
		require.True(t, ok)

		usersVal, found, _ := dict.Get(starlark.String("users"))
		require.True(t, found)
		usersList, ok := usersVal.(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 2, usersList.Len())

		// Check first user
		user0, ok := usersList.Index(0).(*starlark.Dict)
		require.True(t, ok)
		nameVal, found, _ := user0.Get(starlark.String("name"))
		require.True(t, found)
		name, _ := starlark.AsString(nameVal)
		assert.Equal(t, "Alice", name)
	})

	t.Run("fails on invalid TOML", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := tomlModule.Members["parse"].(starlark.Callable)

		tomlStr := `invalid = toml syntax`
		args := starlark.Tuple{starlark.String(tomlStr)}
		_, err := starlark.Call(thread, parseFunc, args, nil)

		require.Error(t, err)
	})
}

func TestTOMLModule_Stringify(t *testing.T) {
	tomlModule := NewTOMLModule()

	t.Run("stringifies Starlark dict to TOML", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := tomlModule.Members["stringify"].(starlark.Callable)

		dict := starlark.NewDict(2)
		dict.SetKey(starlark.String("name"), starlark.String("test"))
		dict.SetKey(starlark.String("count"), starlark.MakeInt(42))

		args := starlark.Tuple{dict}
		result, err := starlark.Call(thread, stringifyFunc, args, nil)

		require.NoError(t, err)
		tomlStr, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Contains(t, tomlStr, `name = "test"`)
		assert.Contains(t, tomlStr, "count = 42")
	})

	t.Run("stringifies Starlark list to TOML array", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := tomlModule.Members["stringify"].(starlark.Callable)

		list := starlark.NewList([]starlark.Value{
			starlark.String("apple"),
			starlark.String("banana"),
			starlark.String("cherry"),
		})

		dict := starlark.NewDict(1)
		dict.SetKey(starlark.String("fruits"), list)

		args := starlark.Tuple{dict}
		result, err := starlark.Call(thread, stringifyFunc, args, nil)

		require.NoError(t, err)
		tomlStr, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Contains(t, tomlStr, "fruits = [")
		assert.Contains(t, tomlStr, `"apple"`)
		assert.Contains(t, tomlStr, `"banana"`)
	})

	t.Run("stringifies boolean values", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := tomlModule.Members["stringify"].(starlark.Callable)

		dict := starlark.NewDict(1)
		dict.SetKey(starlark.String("active"), starlark.Bool(true))

		args := starlark.Tuple{dict}
		result, err := starlark.Call(thread, stringifyFunc, args, nil)

		require.NoError(t, err)
		tomlStr, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Contains(t, tomlStr, "active = true")
	})

	t.Run("stringifies nested structures", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := tomlModule.Members["stringify"].(starlark.Callable)

		innerDict := starlark.NewDict(1)
		innerDict.SetKey(starlark.String("host"), starlark.String("localhost"))

		outerDict := starlark.NewDict(1)
		outerDict.SetKey(starlark.String("database"), innerDict)

		args := starlark.Tuple{outerDict}
		result, err := starlark.Call(thread, stringifyFunc, args, nil)

		require.NoError(t, err)
		tomlStr, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Contains(t, tomlStr, "[database]")
		assert.Contains(t, tomlStr, `host = "localhost"`)
	})
}

func TestTOMLModule_RoundTrip(t *testing.T) {
	tomlModule := NewTOMLModule()

	t.Run("parse and stringify round trip", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := tomlModule.Members["parse"].(starlark.Callable)
		stringifyFunc := tomlModule.Members["stringify"].(starlark.Callable)

		originalTOML := `
name = "Alice"
age = 30
active = true
`

		// Parse
		args := starlark.Tuple{starlark.String(originalTOML)}
		parsed, err := starlark.Call(thread, parseFunc, args, nil)
		require.NoError(t, err)

		// Stringify
		args = starlark.Tuple{parsed}
		result, err := starlark.Call(thread, stringifyFunc, args, nil)
		require.NoError(t, err)

		tomlStr, _ := starlark.AsString(result)
		// Values should be preserved
		assert.Contains(t, tomlStr, `name = "Alice"`)
		assert.Contains(t, tomlStr, "age = 30")
		assert.Contains(t, tomlStr, "active = true")
	})
}
