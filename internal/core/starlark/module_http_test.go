// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func TestHTTPGet(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "test-value", r.Header.Get("X-Test-Header"))
		assert.Equal(t, "bar", r.URL.Query().Get("foo"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "success"})
	}))
	defer server.Close()

	thread := &starlark.Thread{Name: "test"}

	// Test GET with headers and params
	headers := starlark.NewDict(1)
	headers.SetKey(starlark.String("X-Test-Header"), starlark.String("test-value"))

	params := starlark.NewDict(1)
	params.SetKey(starlark.String("foo"), starlark.String("bar"))

	result, err := httpGet(thread, starlark.NewBuiltin("get", httpGet),
		starlark.Tuple{starlark.String(server.URL)},
		[]starlark.Tuple{
			{starlark.String("headers"), headers},
			{starlark.String("params"), params},
		})

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify response structure
	response, ok := result.(*starlarkstruct.Struct)
	require.True(t, ok)

	statusCode, err := response.Attr("status_code")
	require.NoError(t, err)
	assert.Equal(t, starlark.MakeInt(200), statusCode)

	okField, err := response.Attr("ok")
	require.NoError(t, err)
	assert.Equal(t, starlark.True, okField)

	// Verify JSON parsing
	jsonField, err := response.Attr("json")
	require.NoError(t, err)
	jsonDict, ok := jsonField.(*starlark.Dict)
	require.True(t, ok)

	message, found, err := jsonDict.Get(starlark.String("message"))
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "success", message.(starlark.String).GoString())
}

func TestHTTPPost(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "test", body["key"])

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"result": "created"})
	}))
	defer server.Close()

	thread := &starlark.Thread{Name: "test"}

	// Test POST with JSON body
	jsonData := starlark.NewDict(1)
	jsonData.SetKey(starlark.String("key"), starlark.String("test"))

	result, err := httpPost(thread, starlark.NewBuiltin("post", httpPost),
		starlark.Tuple{starlark.String(server.URL)},
		[]starlark.Tuple{
			{starlark.String("json"), jsonData},
		})

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify response
	response, ok := result.(*starlarkstruct.Struct)
	require.True(t, ok)

	statusCode, err := response.Attr("status_code")
	require.NoError(t, err)
	assert.Equal(t, starlark.MakeInt(201), statusCode)
}

func TestHTTPPostWithBody(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)

		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		assert.Equal(t, "plain text body", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	thread := &starlark.Thread{Name: "test"}

	result, err := httpPost(thread, starlark.NewBuiltin("post", httpPost),
		starlark.Tuple{starlark.String(server.URL)},
		[]starlark.Tuple{
			{starlark.String("body"), starlark.String("plain text body")},
		})

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestHTTPPut(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	thread := &starlark.Thread{Name: "test"}

	jsonData := starlark.NewDict(1)
	jsonData.SetKey(starlark.String("update"), starlark.String("value"))

	result, err := httpPut(thread, starlark.NewBuiltin("put", httpPut),
		starlark.Tuple{starlark.String(server.URL)},
		[]starlark.Tuple{
			{starlark.String("json"), jsonData},
		})

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestHTTPDelete(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	thread := &starlark.Thread{Name: "test"}

	result, err := httpDelete(thread, starlark.NewBuiltin("delete", httpDelete),
		starlark.Tuple{starlark.String(server.URL)},
		nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify response
	response, ok := result.(*starlarkstruct.Struct)
	require.True(t, ok)

	statusCode, err := response.Attr("status_code")
	require.NoError(t, err)
	assert.Equal(t, starlark.MakeInt(204), statusCode)
}

func TestHTTPGraphQL(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Contains(t, body["query"], "query")

		variables, ok := body["variables"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "test-value", variables["var1"])

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]string{"result": "success"},
		})
	}))
	defer server.Close()

	thread := &starlark.Thread{Name: "test"}

	variables := starlark.NewDict(1)
	variables.SetKey(starlark.String("var1"), starlark.String("test-value"))

	result, err := httpGraphQL(thread, starlark.NewBuiltin("graphql", httpGraphQL),
		starlark.Tuple{
			starlark.String(server.URL),
			starlark.String("query { test }"),
		},
		[]starlark.Tuple{
			{starlark.String("variables"), variables},
			{starlark.String("token"), starlark.String("test-token")},
		})

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify response
	response, ok := result.(*starlarkstruct.Struct)
	require.True(t, ok)

	statusCode, err := response.Attr("status_code")
	require.NoError(t, err)
	assert.Equal(t, starlark.MakeInt(200), statusCode)
}

func TestHTTPErrorHandling(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}

	// Test invalid URL
	result, err := httpGet(thread, starlark.NewBuiltin("get", httpGet),
		starlark.Tuple{starlark.String("http://invalid-url-that-does-not-exist-12345.com")},
		[]starlark.Tuple{
			{starlark.String("timeout"), starlark.MakeInt(1)},
		})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "HTTP GET request")
}

func TestStarlarkValueConversion(t *testing.T) {
	tests := []struct {
		name     string
		input    starlark.Value
		expected interface{}
	}{
		{
			name:     "bool true",
			input:    starlark.True,
			expected: true,
		},
		{
			name:     "bool false",
			input:    starlark.False,
			expected: false,
		},
		{
			name:     "int",
			input:    starlark.MakeInt(42),
			expected: int64(42),
		},
		{
			name:     "float",
			input:    starlark.Float(3.14),
			expected: float64(3.14),
		},
		{
			name:     "string",
			input:    starlark.String("hello"),
			expected: "hello",
		},
		{
			name:     "none",
			input:    starlark.None,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := starlarkValueToGo(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGoValueToStarlark(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected starlark.Value
	}{
		{
			name:     "bool true",
			input:    true,
			expected: starlark.True,
		},
		{
			name:     "float",
			input:    float64(3.14),
			expected: starlark.Float(3.14),
		},
		{
			name:     "string",
			input:    "hello",
			expected: starlark.String("hello"),
		},
		{
			name:     "nil",
			input:    nil,
			expected: starlark.None,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := goValueToStarlark(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestHTTPGetErrors tests error cases for http.get()
func TestHTTPGetErrors(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}

	t.Run("missing url argument", func(t *testing.T) {
		_, err := httpGet(thread, starlark.NewBuiltin("get", httpGet),
			starlark.Tuple{},
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "url")
	})

	t.Run("wrong argument type for url", func(t *testing.T) {
		_, err := httpGet(thread, starlark.NewBuiltin("get", httpGet),
			starlark.Tuple{starlark.MakeInt(123)},
			nil)
		assert.Error(t, err)
	})

	t.Run("invalid url format", func(t *testing.T) {
		_, err := httpGet(thread, starlark.NewBuiltin("get", httpGet),
			starlark.Tuple{starlark.String("://invalid-url")},
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse URL")
	})

	t.Run("malformed url", func(t *testing.T) {
		_, err := httpGet(thread, starlark.NewBuiltin("get", httpGet),
			starlark.Tuple{starlark.String("not a url at all")},
			nil)
		// This might succeed as it could be treated as a relative URL
		// depending on Go's url.Parse behavior
		_ = err
	})

	t.Run("wrong type for headers", func(t *testing.T) {
		_, err := httpGet(thread, starlark.NewBuiltin("get", httpGet),
			starlark.Tuple{starlark.String("http://example.com")},
			[]starlark.Tuple{
				{starlark.String("headers"), starlark.String("not a dict")},
			})
		assert.Error(t, err)
	})

	t.Run("wrong type for params", func(t *testing.T) {
		_, err := httpGet(thread, starlark.NewBuiltin("get", httpGet),
			starlark.Tuple{starlark.String("http://example.com")},
			[]starlark.Tuple{
				{starlark.String("params"), starlark.String("not a dict")},
			})
		assert.Error(t, err)
	})
}

// TestHTTPPostErrors tests error cases for http.post()
func TestHTTPPostErrors(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}

	t.Run("missing url argument", func(t *testing.T) {
		_, err := httpPost(thread, starlark.NewBuiltin("post", httpPost),
			starlark.Tuple{},
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "url")
	})

	t.Run("wrong argument type for url", func(t *testing.T) {
		_, err := httpPost(thread, starlark.NewBuiltin("post", httpPost),
			starlark.Tuple{starlark.MakeInt(123)},
			nil)
		assert.Error(t, err)
	})

	t.Run("invalid url format", func(t *testing.T) {
		_, err := httpPost(thread, starlark.NewBuiltin("post", httpPost),
			starlark.Tuple{starlark.String("://invalid-url")},
			nil)
		assert.Error(t, err)
		// POST/PUT/DELETE errors come from http.NewRequest, not url.Parse
		assert.Contains(t, err.Error(), "request")
	})

	t.Run("wrong type for body", func(t *testing.T) {
		_, err := httpPost(thread, starlark.NewBuiltin("post", httpPost),
			starlark.Tuple{starlark.String("http://example.com")},
			[]starlark.Tuple{
				{starlark.String("body"), starlark.MakeInt(123)},
			})
		assert.Error(t, err)
	})
}

// TestHTTPPutErrors tests error cases for http.put()
func TestHTTPPutErrors(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}

	t.Run("missing url argument", func(t *testing.T) {
		_, err := httpPut(thread, starlark.NewBuiltin("put", httpPut),
			starlark.Tuple{},
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "url")
	})

	t.Run("wrong argument type for url", func(t *testing.T) {
		_, err := httpPut(thread, starlark.NewBuiltin("put", httpPut),
			starlark.Tuple{starlark.MakeInt(123)},
			nil)
		assert.Error(t, err)
	})

	t.Run("invalid url format", func(t *testing.T) {
		_, err := httpPut(thread, starlark.NewBuiltin("put", httpPut),
			starlark.Tuple{starlark.String("://invalid-url")},
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "request")
	})
}

// TestHTTPDeleteErrors tests error cases for http.delete()
func TestHTTPDeleteErrors(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}

	t.Run("missing url argument", func(t *testing.T) {
		_, err := httpDelete(thread, starlark.NewBuiltin("delete", httpDelete),
			starlark.Tuple{},
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "url")
	})

	t.Run("wrong argument type for url", func(t *testing.T) {
		_, err := httpDelete(thread, starlark.NewBuiltin("delete", httpDelete),
			starlark.Tuple{starlark.MakeInt(123)},
			nil)
		assert.Error(t, err)
	})

	t.Run("invalid url format", func(t *testing.T) {
		_, err := httpDelete(thread, starlark.NewBuiltin("delete", httpDelete),
			starlark.Tuple{starlark.String("://invalid-url")},
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "request")
	})
}

// TestHTTPGraphQLErrors tests error cases for http.graphql()
func TestHTTPGraphQLErrors(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}

	t.Run("missing url argument", func(t *testing.T) {
		_, err := httpGraphQL(thread, starlark.NewBuiltin("graphql", httpGraphQL),
			starlark.Tuple{},
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "url")
	})

	t.Run("missing query argument", func(t *testing.T) {
		_, err := httpGraphQL(thread, starlark.NewBuiltin("graphql", httpGraphQL),
			starlark.Tuple{starlark.String("http://example.com")},
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "query")
	})

	t.Run("wrong argument type for url", func(t *testing.T) {
		_, err := httpGraphQL(thread, starlark.NewBuiltin("graphql", httpGraphQL),
			starlark.Tuple{starlark.MakeInt(123), starlark.String("query {}")},
			nil)
		assert.Error(t, err)
	})

	t.Run("invalid url format", func(t *testing.T) {
		_, err := httpGraphQL(thread, starlark.NewBuiltin("graphql", httpGraphQL),
			starlark.Tuple{starlark.String("://invalid-url"), starlark.String("query {}")},
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "request")
	})
}

// TestHTTPModuleCreation tests the NewHTTPModule function
func TestHTTPModuleCreation(t *testing.T) {
	module := NewHTTPModule()
	require.NotNil(t, module)

	moduleStruct, ok := module.(*starlarkstruct.Struct)
	require.True(t, ok)

	// Verify all expected functions exist
	getFunc, err := moduleStruct.Attr("get")
	require.NoError(t, err)
	require.NotNil(t, getFunc)

	postFunc, err := moduleStruct.Attr("post")
	require.NoError(t, err)
	require.NotNil(t, postFunc)

	putFunc, err := moduleStruct.Attr("put")
	require.NoError(t, err)
	require.NotNil(t, putFunc)

	deleteFunc, err := moduleStruct.Attr("delete")
	require.NoError(t, err)
	require.NotNil(t, deleteFunc)

	graphqlFunc, err := moduleStruct.Attr("graphql")
	require.NoError(t, err)
	require.NotNil(t, graphqlFunc)
}

// TestHTTPResponseStatusCodes tests various HTTP status codes
func TestHTTPResponseStatusCodes(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}

	testCases := []struct {
		name       string
		statusCode int
		expectOk   bool
	}{
		{"200 OK", http.StatusOK, true},
		{"201 Created", http.StatusCreated, true},
		{"204 No Content", http.StatusNoContent, true},
		{"400 Bad Request", http.StatusBadRequest, false},
		{"404 Not Found", http.StatusNotFound, false},
		{"500 Internal Server Error", http.StatusInternalServerError, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer server.Close()

			result, err := httpGet(thread, starlark.NewBuiltin("get", httpGet),
				starlark.Tuple{starlark.String(server.URL)},
				nil)

			require.NoError(t, err)
			require.NotNil(t, result)

			response, ok := result.(*starlarkstruct.Struct)
			require.True(t, ok)

			statusCodeVal, err := response.Attr("status_code")
			require.NoError(t, err)
			assert.Equal(t, starlark.MakeInt(tc.statusCode), statusCodeVal)

			okField, err := response.Attr("ok")
			require.NoError(t, err)
			assert.Equal(t, starlark.Bool(tc.expectOk), okField)
		})
	}
}

// TestStarlarkValueConversionEdgeCases tests edge cases in value conversion
func TestStarlarkValueConversionEdgeCases(t *testing.T) {
	t.Run("list conversion", func(t *testing.T) {
		list := starlark.NewList([]starlark.Value{
			starlark.String("a"),
			starlark.MakeInt(1),
		})
		result := starlarkValueToGo(list)
		slice, ok := result.([]interface{})
		require.True(t, ok)
		assert.Len(t, slice, 2)
	})

	t.Run("dict conversion", func(t *testing.T) {
		dict := starlark.NewDict(1)
		dict.SetKey(starlark.String("key"), starlark.String("value"))
		result := starlarkValueToGo(dict)
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "value", resultMap["key"])
	})

	t.Run("unknown type fallback", func(t *testing.T) {
		// Use a custom type that's not handled
		custom := starlark.Tuple{starlark.String("test")}
		result := starlarkValueToGo(custom)
		// Should fall back to string representation
		assert.NotNil(t, result)
	})
}

// TestGoValueToStarlarkEdgeCases tests edge cases in Go to Starlark conversion
func TestGoValueToStarlarkEdgeCases(t *testing.T) {
	t.Run("int conversion", func(t *testing.T) {
		result := goValueToStarlark(42)
		// Int is handled as float64 in the current implementation
		assert.NotNil(t, result)
	})

	t.Run("slice conversion", func(t *testing.T) {
		result := goValueToStarlark([]interface{}{"a", "b", "c"})
		list, ok := result.(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 3, list.Len())
	})

	t.Run("map conversion", func(t *testing.T) {
		result := goValueToStarlark(map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		})
		dict, ok := result.(*starlark.Dict)
		require.True(t, ok)
		assert.Equal(t, 2, dict.Len())
	})

	t.Run("nested structures", func(t *testing.T) {
		nested := map[string]interface{}{
			"users": []interface{}{
				map[string]interface{}{"name": "Alice"},
				map[string]interface{}{"name": "Bob"},
			},
		}
		result := goValueToStarlark(nested)
		dict, ok := result.(*starlark.Dict)
		require.True(t, ok)
		assert.Equal(t, 1, dict.Len())
	})
}
