// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

func TestCSVModule_Parse(t *testing.T) {
	csvModule := NewCSVModule()

	t.Run("parses CSV without headers to list of lists", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := csvModule.Members["parse"].(starlark.Callable)

		csvStr := `Alice,30,Engineer
Bob,25,Designer
Carol,35,Manager`

		args := starlark.Tuple{starlark.String(csvStr)}
		result, err := starlark.Call(thread, parseFunc, args, nil)

		require.NoError(t, err)
		list, ok := result.(*starlark.List)
		require.True(t, ok, "parse should return a list")
		assert.Equal(t, 3, list.Len())

		// Check first row
		row0, ok := list.Index(0).(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 3, row0.Len())

		name, _ := starlark.AsString(row0.Index(0))
		assert.Equal(t, "Alice", name)

		age, _ := starlark.AsString(row0.Index(1))
		assert.Equal(t, "30", age)
	})

	t.Run("parses CSV with headers to list of dicts", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := csvModule.Members["parse"].(starlark.Callable)

		csvStr := `name,age,role
Alice,30,Engineer
Bob,25,Designer`

		args := starlark.Tuple{starlark.String(csvStr)}
		kwargs := []starlark.Tuple{
			{starlark.String("has_header"), starlark.Bool(true)},
		}
		result, err := starlark.Call(thread, parseFunc, args, kwargs)

		require.NoError(t, err)
		list, ok := result.(*starlark.List)
		require.True(t, ok, "parse should return a list")
		assert.Equal(t, 2, list.Len()) // 2 data rows (header excluded)

		// Check first row
		row0, ok := list.Index(0).(*starlark.Dict)
		require.True(t, ok, "each row should be a dict when has_header=true")

		nameVal, found, _ := row0.Get(starlark.String("name"))
		require.True(t, found)
		name, _ := starlark.AsString(nameVal)
		assert.Equal(t, "Alice", name)

		ageVal, found, _ := row0.Get(starlark.String("age"))
		require.True(t, found)
		age, _ := starlark.AsString(ageVal)
		assert.Equal(t, "30", age)
	})

	t.Run("parses CSV with custom delimiter", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := csvModule.Members["parse"].(starlark.Callable)

		csvStr := `Alice	30	Engineer
Bob	25	Designer`

		args := starlark.Tuple{starlark.String(csvStr)}
		kwargs := []starlark.Tuple{
			{starlark.String("delimiter"), starlark.String("\t")},
		}
		result, err := starlark.Call(thread, parseFunc, args, kwargs)

		require.NoError(t, err)
		list, ok := result.(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 2, list.Len())

		row0, ok := list.Index(0).(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 3, row0.Len())

		name, _ := starlark.AsString(row0.Index(0))
		assert.Equal(t, "Alice", name)
	})

	t.Run("parses empty CSV", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := csvModule.Members["parse"].(starlark.Callable)

		csvStr := ``

		args := starlark.Tuple{starlark.String(csvStr)}
		result, err := starlark.Call(thread, parseFunc, args, nil)

		require.NoError(t, err)
		list, ok := result.(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 0, list.Len())
	})

	t.Run("parses CSV with quoted fields", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := csvModule.Members["parse"].(starlark.Callable)

		csvStr := `"Alice, Jr.",30,"Software Engineer"
"Bob",25,"UX Designer"`

		args := starlark.Tuple{starlark.String(csvStr)}
		result, err := starlark.Call(thread, parseFunc, args, nil)

		require.NoError(t, err)
		list, ok := result.(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 2, list.Len())

		row0, ok := list.Index(0).(*starlark.List)
		require.True(t, ok)

		name, _ := starlark.AsString(row0.Index(0))
		assert.Equal(t, "Alice, Jr.", name) // Comma preserved in quoted field
	})
}

func TestCSVModule_Stringify(t *testing.T) {
	csvModule := NewCSVModule()

	t.Run("stringifies list of lists to CSV", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := csvModule.Members["stringify"].(starlark.Callable)

		row1 := starlark.NewList([]starlark.Value{
			starlark.String("Alice"),
			starlark.MakeInt(30),
			starlark.String("Engineer"),
		})
		row2 := starlark.NewList([]starlark.Value{
			starlark.String("Bob"),
			starlark.MakeInt(25),
			starlark.String("Designer"),
		})

		list := starlark.NewList([]starlark.Value{row1, row2})

		args := starlark.Tuple{list}
		result, err := starlark.Call(thread, stringifyFunc, args, nil)

		require.NoError(t, err)
		csvStr, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Contains(t, csvStr, "Alice,30,Engineer")
		assert.Contains(t, csvStr, "Bob,25,Designer")
	})

	t.Run("stringifies list of dicts to CSV with headers", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := csvModule.Members["stringify"].(starlark.Callable)

		row1 := starlark.NewDict(3)
		row1.SetKey(starlark.String("name"), starlark.String("Alice"))
		row1.SetKey(starlark.String("age"), starlark.MakeInt(30))
		row1.SetKey(starlark.String("role"), starlark.String("Engineer"))

		row2 := starlark.NewDict(3)
		row2.SetKey(starlark.String("name"), starlark.String("Bob"))
		row2.SetKey(starlark.String("age"), starlark.MakeInt(25))
		row2.SetKey(starlark.String("role"), starlark.String("Designer"))

		list := starlark.NewList([]starlark.Value{row1, row2})

		args := starlark.Tuple{list}
		kwargs := []starlark.Tuple{
			{starlark.String("headers"), starlark.NewList([]starlark.Value{
				starlark.String("name"),
				starlark.String("age"),
				starlark.String("role"),
			})},
		}
		result, err := starlark.Call(thread, stringifyFunc, args, kwargs)

		require.NoError(t, err)
		csvStr, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Contains(t, csvStr, "name,age,role")
		assert.Contains(t, csvStr, "Alice,30,Engineer")
		assert.Contains(t, csvStr, "Bob,25,Designer")
	})

	t.Run("stringifies with custom delimiter", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := csvModule.Members["stringify"].(starlark.Callable)

		row1 := starlark.NewList([]starlark.Value{
			starlark.String("Alice"),
			starlark.MakeInt(30),
		})

		list := starlark.NewList([]starlark.Value{row1})

		args := starlark.Tuple{list}
		kwargs := []starlark.Tuple{
			{starlark.String("delimiter"), starlark.String("\t")},
		}
		result, err := starlark.Call(thread, stringifyFunc, args, kwargs)

		require.NoError(t, err)
		csvStr, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Contains(t, csvStr, "Alice\t30")
	})

	t.Run("stringifies empty list", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := csvModule.Members["stringify"].(starlark.Callable)

		list := starlark.NewList([]starlark.Value{})

		args := starlark.Tuple{list}
		result, err := starlark.Call(thread, stringifyFunc, args, nil)

		require.NoError(t, err)
		csvStr, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Equal(t, "", csvStr)
	})

	t.Run("stringifies with fields containing commas", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := csvModule.Members["stringify"].(starlark.Callable)

		row1 := starlark.NewList([]starlark.Value{
			starlark.String("Alice, Jr."),
			starlark.String("Software Engineer"),
		})

		list := starlark.NewList([]starlark.Value{row1})

		args := starlark.Tuple{list}
		result, err := starlark.Call(thread, stringifyFunc, args, nil)

		require.NoError(t, err)
		csvStr, ok := starlark.AsString(result)
		require.True(t, ok)
		// Fields with commas should be quoted
		assert.Contains(t, csvStr, `"Alice, Jr."`)
	})
}

func TestCSVModule_RoundTrip(t *testing.T) {
	csvModule := NewCSVModule()

	t.Run("parse and stringify round trip without headers", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := csvModule.Members["parse"].(starlark.Callable)
		stringifyFunc := csvModule.Members["stringify"].(starlark.Callable)

		originalCSV := `Alice,30,Engineer
Bob,25,Designer`

		// Parse
		args := starlark.Tuple{starlark.String(originalCSV)}
		parsed, err := starlark.Call(thread, parseFunc, args, nil)
		require.NoError(t, err)

		// Stringify
		args = starlark.Tuple{parsed}
		result, err := starlark.Call(thread, stringifyFunc, args, nil)
		require.NoError(t, err)

		csvStr, _ := starlark.AsString(result)
		// Values should be preserved
		assert.Contains(t, csvStr, "Alice,30,Engineer")
		assert.Contains(t, csvStr, "Bob,25,Designer")
	})

	t.Run("parse and stringify round trip with headers", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := csvModule.Members["parse"].(starlark.Callable)
		stringifyFunc := csvModule.Members["stringify"].(starlark.Callable)

		originalCSV := `name,age,role
Alice,30,Engineer
Bob,25,Designer`

		// Parse
		args := starlark.Tuple{starlark.String(originalCSV)}
		kwargs := []starlark.Tuple{
			{starlark.String("has_header"), starlark.Bool(true)},
		}
		parsed, err := starlark.Call(thread, parseFunc, args, kwargs)
		require.NoError(t, err)

		// Stringify
		args = starlark.Tuple{parsed}
		kwargs = []starlark.Tuple{
			{starlark.String("headers"), starlark.NewList([]starlark.Value{
				starlark.String("name"),
				starlark.String("age"),
				starlark.String("role"),
			})},
		}
		result, err := starlark.Call(thread, stringifyFunc, args, kwargs)
		require.NoError(t, err)

		csvStr, _ := starlark.AsString(result)
		// Values should be preserved
		assert.Contains(t, csvStr, "name,age,role")
		assert.Contains(t, csvStr, "Alice,30,Engineer")
		assert.Contains(t, csvStr, "Bob,25,Designer")
	})
}
