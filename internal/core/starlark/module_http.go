// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"bytes"
	"context"
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
func httpGet(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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
		return nil, fmt.Errorf("http.get: %w", err)
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL '%s': %w", urlStr, err)
	}

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

	req, err := http.NewRequestWithContext(context.Background(), "GET", parsedURL.String(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request for '%s': %w", urlStr, err)
	}

	if headersDict != nil {
		addHeadersToRequest(req, headersDict)
	}

	return executeHTTPRequest(req, timeout, urlStr)
}

// httpPostOrPut implements the shared logic for http.post() and http.put().
func httpPostOrPut(method string, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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
		return nil, fmt.Errorf("%s: %w", b.Name(), err)
	}

	var bodyReader io.Reader
	contentType := ""

	switch {
	case jsonData != nil && jsonData != starlark.None:
		jsonBytes, err := starlarkValueToJSON(jsonData)
		if err != nil {
			return nil, fmt.Errorf("failed to encode JSON body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBytes)
		contentType = "application/json"
	case body != "":
		bodyReader = strings.NewReader(body)
	default:
		bodyReader = http.NoBody
	}

	req, err := http.NewRequestWithContext(context.Background(), method, urlStr, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s request for '%s': %w", method, urlStr, err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	if headersDict != nil {
		addHeadersToRequest(req, headersDict)
	}

	return executeHTTPRequest(req, timeout, urlStr)
}

// httpPost implements http.post().
func httpPost(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return httpPostOrPut("POST", b, args, kwargs)
}

// httpPut implements http.put().
func httpPut(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return httpPostOrPut("PUT", b, args, kwargs)
}

// httpDelete implements http.delete().
func httpDelete(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var urlStr string
	var headersDict *starlark.Dict
	timeout := 30

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"url", &urlStr,
		"headers?", &headersDict,
		"timeout?", &timeout,
	); err != nil {
		return nil, fmt.Errorf("http.delete: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), "DELETE", urlStr, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create DELETE request for '%s': %w", urlStr, err)
	}

	if headersDict != nil {
		addHeadersToRequest(req, headersDict)
	}

	return executeHTTPRequest(req, timeout, urlStr)
}

// httpGraphQL implements http.graphql().
func httpGraphQL(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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
		return nil, fmt.Errorf("http.graphql: %w", err)
	}

	body := map[string]interface{}{
		"query": query,
	}

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

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to encode GraphQL body: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", urlStr, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphQL request for '%s': %w", urlStr, err)
	}

	req.Header.Set("Content-Type", "application/json")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return executeHTTPRequest(req, timeout, urlStr)
}

// addHeadersToRequest adds headers from a Starlark dict to an HTTP request.
func addHeadersToRequest(req *http.Request, headersDict *starlark.Dict) {
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
}

// executeHTTPRequest executes an HTTP request and returns a structured response.
func executeHTTPRequest(req *http.Request, timeout int, urlStr string) (starlark.Value, error) {
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	resp, err := client.Do(req) //nolint:gosec // URL is user-provided by design
	if err != nil {
		return nil, fmt.Errorf("HTTP %s request to '%s' failed: %w", req.Method, urlStr, err)
	}
	defer resp.Body.Close() //nolint:errcheck // deferred close errors are not critical

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from '%s': %w", urlStr, err)
	}

	headers := starlark.NewDict(len(resp.Header))
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers.SetKey(starlark.String(key), starlark.String(values[0])) //nolint:errcheck // starlark dict operations with known-compatible types
		}
	}

	var jsonBody starlark.Value = starlark.None
	var jsonData interface{}
	if err := json.Unmarshal(bodyBytes, &jsonData); err == nil {
		jsonBody = goValueToStarlark(jsonData)
	}

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
	data, err := json.Marshal(goVal)
	if err != nil {
		return nil, fmt.Errorf("json marshal failed: %w", err)
	}
	return data, nil
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
			dict.SetKey(starlark.String(key), goValueToStarlark(value)) //nolint:errcheck // starlark dict operations with known-compatible types
		}
		return dict
	default:
		return starlark.String(fmt.Sprint(v))
	}
}
