// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// NewHTTPModule creates the http built-in module.
func NewHTTPModule() starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"get":     starlark.NewBuiltin("get", httpGet),
		"post":    starlark.NewBuiltin("post", httpPost),
		"put":     starlark.NewBuiltin("put", httpPut),
		"delete":  starlark.NewBuiltin("delete", httpDelete),
		"graphql": starlark.NewBuiltin("graphql", httpGraphQL),
	})
}

// httpGet implements http.get().
func httpGet(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var urlStr string
	var headersDict *starlark.Dict
	var paramsDict *starlark.Dict
	timeout := 30

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"url", &urlStr,
		"headers?", &headersDict,
		"params?", &paramsDict,
		"timeout?", &timeout,
	); err != nil {
		return nil, err
	}

	// Parse URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL '%s': %w", urlStr, err)
	}

	// Add query parameters
	if paramsDict != nil {
		query := parsedURL.Query()
		for _, item := range paramsDict.Items() {
			key, ok := starlark.AsString(item[0])
			if !ok {
				continue
			}
			value, ok := starlark.AsString(item[1])
			if !ok {
				continue
			}
			query.Add(key, value)
		}
		parsedURL.RawQuery = query.Encode()
	}

	// Create request
	req, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request for '%s': %w", urlStr, err)
	}

	// Add headers
	if headersDict != nil {
		if err := addHeadersToRequest(req, headersDict); err != nil {
			return nil, err
		}
	}

	// Execute request
	return executeHTTPRequest(req, timeout, urlStr)
}

// httpPost implements http.post().
func httpPost(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var urlStr string
	var body string
	var jsonData starlark.Value
	var headersDict *starlark.Dict
	timeout := 30

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"url", &urlStr,
		"body?", &body,
		"json?", &jsonData,
		"headers?", &headersDict,
		"timeout?", &timeout,
	); err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	contentType := ""

	// Handle JSON body
	if jsonData != nil && jsonData != starlark.None {
		jsonBytes, err := starlarkValueToJSON(jsonData)
		if err != nil {
			return nil, fmt.Errorf("failed to encode JSON body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBytes)
		contentType = "application/json"
	} else if body != "" {
		bodyReader = strings.NewReader(body)
	}

	// Create request
	req, err := http.NewRequest("POST", urlStr, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create POST request for '%s': %w", urlStr, err)
	}

	// Set content type if JSON
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	// Add headers
	if headersDict != nil {
		if err := addHeadersToRequest(req, headersDict); err != nil {
			return nil, err
		}
	}

	// Execute request
	return executeHTTPRequest(req, timeout, urlStr)
}

// httpPut implements http.put().
func httpPut(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var urlStr string
	var body string
	var jsonData starlark.Value
	var headersDict *starlark.Dict
	timeout := 30

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"url", &urlStr,
		"body?", &body,
		"json?", &jsonData,
		"headers?", &headersDict,
		"timeout?", &timeout,
	); err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	contentType := ""

	// Handle JSON body
	if jsonData != nil && jsonData != starlark.None {
		jsonBytes, err := starlarkValueToJSON(jsonData)
		if err != nil {
			return nil, fmt.Errorf("failed to encode JSON body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBytes)
		contentType = "application/json"
	} else if body != "" {
		bodyReader = strings.NewReader(body)
	}

	// Create request
	req, err := http.NewRequest("PUT", urlStr, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create PUT request for '%s': %w", urlStr, err)
	}

	// Set content type if JSON
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	// Add headers
	if headersDict != nil {
		if err := addHeadersToRequest(req, headersDict); err != nil {
			return nil, err
		}
	}

	// Execute request
	return executeHTTPRequest(req, timeout, urlStr)
}

// httpDelete implements http.delete().
func httpDelete(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var urlStr string
	var headersDict *starlark.Dict
	timeout := 30

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"url", &urlStr,
		"headers?", &headersDict,
		"timeout?", &timeout,
	); err != nil {
		return nil, err
	}

	// Create request
	req, err := http.NewRequest("DELETE", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create DELETE request for '%s': %w", urlStr, err)
	}

	// Add headers
	if headersDict != nil {
		if err := addHeadersToRequest(req, headersDict); err != nil {
			return nil, err
		}
	}

	// Execute request
	return executeHTTPRequest(req, timeout, urlStr)
}

// httpGraphQL implements http.graphql().
func httpGraphQL(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var urlStr string
	var query string
	var variablesDict *starlark.Dict
	var token string
	timeout := 30

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"url", &urlStr,
		"query", &query,
		"variables?", &variablesDict,
		"token?", &token,
		"timeout?", &timeout,
	); err != nil {
		return nil, err
	}

	// Build GraphQL request body
	body := map[string]interface{}{
		"query": query,
	}

	// Add variables if provided
	if variablesDict != nil {
		variables := make(map[string]interface{})
		for _, item := range variablesDict.Items() {
			key, ok := starlark.AsString(item[0])
			if !ok {
				continue
			}
			variables[key] = starlarkValueToGo(item[1])
		}
		body["variables"] = variables
	}

	// Encode body
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to encode GraphQL body: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", urlStr, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphQL request for '%s': %w", urlStr, err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add authorization token if provided
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Execute request
	return executeHTTPRequest(req, timeout, urlStr)
}

// addHeadersToRequest adds headers from a Starlark dict to an HTTP request.
func addHeadersToRequest(req *http.Request, headersDict *starlark.Dict) error {
	for _, item := range headersDict.Items() {
		key, ok := starlark.AsString(item[0])
		if !ok {
			continue
		}
		value, ok := starlark.AsString(item[1])
		if !ok {
			continue
		}
		req.Header.Set(key, value)
	}
	return nil
}

// executeHTTPRequest executes an HTTP request and returns a structured response.
func executeHTTPRequest(req *http.Request, timeout int, urlStr string) (starlark.Value, error) {
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP %s request to '%s' failed: %w", req.Method, urlStr, err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from '%s': %w", urlStr, err)
	}

	// Parse response headers
	headers := starlark.NewDict(len(resp.Header))
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers.SetKey(starlark.String(key), starlark.String(values[0]))
		}
	}

	// Try to parse as JSON
	var jsonBody starlark.Value = starlark.None
	var jsonData interface{}
	if err := json.Unmarshal(bodyBytes, &jsonData); err == nil {
		jsonBody = goValueToStarlark(jsonData)
	}

	// Build response struct
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"status_code": starlark.MakeInt(resp.StatusCode),
		"headers":     headers,
		"body":        starlark.String(string(bodyBytes)),
		"json":        jsonBody,
		"ok":          starlark.Bool(resp.StatusCode >= 200 && resp.StatusCode < 300),
	}), nil
}

// starlarkValueToJSON converts a Starlark value to JSON bytes.
func starlarkValueToJSON(val starlark.Value) ([]byte, error) {
	goVal := starlarkValueToGo(val)
	return json.Marshal(goVal)
}

// starlarkValueToGo converts a Starlark value to a Go value.
func starlarkValueToGo(val starlark.Value) interface{} {
	switch v := val.(type) {
	case starlark.NoneType:
		return nil
	case starlark.Bool:
		return bool(v)
	case starlark.Int:
		i, _ := v.Int64()
		return i
	case starlark.Float:
		return float64(v)
	case starlark.String:
		return string(v)
	case *starlark.List:
		result := make([]interface{}, v.Len())
		for i := 0; i < v.Len(); i++ {
			result[i] = starlarkValueToGo(v.Index(i))
		}
		return result
	case *starlark.Dict:
		result := make(map[string]interface{})
		for _, item := range v.Items() {
			key, ok := starlark.AsString(item[0])
			if ok {
				result[key] = starlarkValueToGo(item[1])
			}
		}
		return result
	default:
		return v.String()
	}
}

// goValueToStarlark converts a Go value (from JSON) to a Starlark value.
func goValueToStarlark(val interface{}) starlark.Value {
	if val == nil {
		return starlark.None
	}

	switch v := val.(type) {
	case bool:
		return starlark.Bool(v)
	case float64:
		return starlark.Float(v)
	case string:
		return starlark.String(v)
	case []interface{}:
		items := make([]starlark.Value, len(v))
		for i, item := range v {
			items[i] = goValueToStarlark(item)
		}
		return starlark.NewList(items)
	case map[string]interface{}:
		dict := starlark.NewDict(len(v))
		for key, value := range v {
			dict.SetKey(starlark.String(key), goValueToStarlark(value))
		}
		return dict
	default:
		return starlark.String(fmt.Sprint(v))
	}
}
