// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

func TestXMLModule_Parse(t *testing.T) {
	xmlModule := NewXMLModule()

	t.Run("parses simple XML to Starlark dict", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := xmlModule.Members["parse"].(starlark.Callable)

		xmlStr := `<root><name>test</name><count>42</count></root>`
		args := starlark.Tuple{starlark.String(xmlStr)}
		result, err := starlark.Call(thread, parseFunc, args, nil)

		require.NoError(t, err)
		dict, ok := result.(*starlark.Dict)
		require.True(t, ok, "parse should return a dict")

		// Check root element
		rootVal, found, err := dict.Get(starlark.String("root"))
		require.NoError(t, err)
		require.True(t, found)
		rootDict, ok := rootVal.(*starlark.Dict)
		require.True(t, ok)

		// Check name field
		nameVal, found, err := rootDict.Get(starlark.String("name"))
		require.NoError(t, err)
		require.True(t, found)
		name, _ := starlark.AsString(nameVal)
		assert.Equal(t, "test", name)
	})

	t.Run("parses XML with attributes", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := xmlModule.Members["parse"].(starlark.Callable)

		xmlStr := `<user id="123" role="admin">Alice</user>`
		args := starlark.Tuple{starlark.String(xmlStr)}
		result, err := starlark.Call(thread, parseFunc, args, nil)

		require.NoError(t, err)
		dict, ok := result.(*starlark.Dict)
		require.True(t, ok)

		// Check user element exists
		userVal, found, err := dict.Get(starlark.String("user"))
		require.NoError(t, err)
		require.True(t, found)

		// Attributes should be accessible
		userDict, ok := userVal.(*starlark.Dict)
		require.True(t, ok)

		// Should have attributes
		attrsVal, found, _ := userDict.Get(starlark.String("-id"))
		if found {
			id, _ := starlark.AsString(attrsVal)
			assert.Equal(t, "123", id)
		}
	})

	t.Run("parses nested XML structures", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := xmlModule.Members["parse"].(starlark.Callable)

		xmlStr := `
<config>
  <database>
    <host>localhost</host>
    <port>5432</port>
  </database>
</config>
`
		args := starlark.Tuple{starlark.String(xmlStr)}
		result, err := starlark.Call(thread, parseFunc, args, nil)

		require.NoError(t, err)
		dict, ok := result.(*starlark.Dict)
		require.True(t, ok)

		// Navigate to config.database.host
		configVal, found, _ := dict.Get(starlark.String("config"))
		require.True(t, found)
		configDict, ok := configVal.(*starlark.Dict)
		require.True(t, ok)

		dbVal, found, _ := configDict.Get(starlark.String("database"))
		require.True(t, found)
		dbDict, ok := dbVal.(*starlark.Dict)
		require.True(t, ok)

		hostVal, found, _ := dbDict.Get(starlark.String("host"))
		require.True(t, found)
		host, _ := starlark.AsString(hostVal)
		assert.Equal(t, "localhost", host)
	})

	t.Run("fails on invalid XML", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := xmlModule.Members["parse"].(starlark.Callable)

		xmlStr := `<invalid><unclosed>`
		args := starlark.Tuple{starlark.String(xmlStr)}
		_, err := starlark.Call(thread, parseFunc, args, nil)

		require.Error(t, err)
	})
}

func TestXMLModule_Stringify(t *testing.T) {
	xmlModule := NewXMLModule()

	t.Run("stringifies Starlark dict to XML", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := xmlModule.Members["stringify"].(starlark.Callable)

		innerDict := starlark.NewDict(2)
		innerDict.SetKey(starlark.String("name"), starlark.String("test"))
		innerDict.SetKey(starlark.String("count"), starlark.MakeInt(42))

		dict := starlark.NewDict(1)
		dict.SetKey(starlark.String("root"), innerDict)

		args := starlark.Tuple{dict}
		result, err := starlark.Call(thread, stringifyFunc, args, nil)

		require.NoError(t, err)
		xmlStr, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Contains(t, xmlStr, "<root>")
		assert.Contains(t, xmlStr, "</root>")
		assert.Contains(t, xmlStr, "<name>test</name>")
	})

	t.Run("stringifies with indentation", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := xmlModule.Members["stringify"].(starlark.Callable)

		innerDict := starlark.NewDict(1)
		innerDict.SetKey(starlark.String("key"), starlark.String("value"))

		dict := starlark.NewDict(1)
		dict.SetKey(starlark.String("root"), innerDict)

		args := starlark.Tuple{dict}
		kwargs := []starlark.Tuple{
			{starlark.String("indent"), starlark.Bool(true)},
		}
		result, err := starlark.Call(thread, stringifyFunc, args, kwargs)

		require.NoError(t, err)
		xmlStr, ok := starlark.AsString(result)
		require.True(t, ok)
		// Should have indentation
		assert.Contains(t, xmlStr, "\n")
	})

	t.Run("stringifies with custom root element", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := xmlModule.Members["stringify"].(starlark.Callable)

		dict := starlark.NewDict(1)
		dict.SetKey(starlark.String("name"), starlark.String("Alice"))

		args := starlark.Tuple{dict}
		kwargs := []starlark.Tuple{
			{starlark.String("root"), starlark.String("user")},
		}
		result, err := starlark.Call(thread, stringifyFunc, args, kwargs)

		require.NoError(t, err)
		xmlStr, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Contains(t, xmlStr, "<user>")
		assert.Contains(t, xmlStr, "</user>")
	})

	t.Run("stringifies nested structures", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		stringifyFunc := xmlModule.Members["stringify"].(starlark.Callable)

		dbDict := starlark.NewDict(1)
		dbDict.SetKey(starlark.String("host"), starlark.String("localhost"))

		configDict := starlark.NewDict(1)
		configDict.SetKey(starlark.String("database"), dbDict)

		rootDict := starlark.NewDict(1)
		rootDict.SetKey(starlark.String("config"), configDict)

		args := starlark.Tuple{rootDict}
		result, err := starlark.Call(thread, stringifyFunc, args, nil)

		require.NoError(t, err)
		xmlStr, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Contains(t, xmlStr, "<config>")
		assert.Contains(t, xmlStr, "<database>")
		assert.Contains(t, xmlStr, "<host>localhost</host>")
	})
}

func TestXMLModule_RoundTrip(t *testing.T) {
	xmlModule := NewXMLModule()

	t.Run("parse and stringify round trip", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		parseFunc := xmlModule.Members["parse"].(starlark.Callable)
		stringifyFunc := xmlModule.Members["stringify"].(starlark.Callable)

		originalXML := `<root><name>Alice</name><age>30</age></root>`

		// Parse
		args := starlark.Tuple{starlark.String(originalXML)}
		parsed, err := starlark.Call(thread, parseFunc, args, nil)
		require.NoError(t, err)

		// Stringify
		args = starlark.Tuple{parsed}
		result, err := starlark.Call(thread, stringifyFunc, args, nil)
		require.NoError(t, err)

		xmlStr, _ := starlark.AsString(result)
		// Values should be preserved
		assert.Contains(t, xmlStr, "<root>")
		assert.Contains(t, xmlStr, "<name>Alice</name>")
		assert.Contains(t, xmlStr, "<age>30</age>")
	})
}
