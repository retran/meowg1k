// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

// TestEnvGet tests env.get() function
func TestEnvGet(t *testing.T) {
	// Set a test environment variable
	testKey := "MEOWG1K_TEST_VAR"
	testValue := "test_value_123"
	require.NoError(t, os.Setenv(testKey, testValue))
	defer os.Unsetenv(testKey)

	envModule := NewEnvModule()
	getFunc := envModule.Members["get"]

	thread := &starlark.Thread{Name: "test"}

	// Test getting existing variable
	args := starlark.Tuple{starlark.String(testKey)}
	result, err := starlark.Call(thread, getFunc, args, nil)
	require.NoError(t, err)

	resultStr, ok := result.(starlark.String)
	require.True(t, ok, "result should be a string")
	assert.Equal(t, testValue, string(resultStr))
}

// TestEnvGetNonExistent tests env.get() with non-existent variable
func TestEnvGetNonExistent(t *testing.T) {
	envModule := NewEnvModule()
	getFunc := envModule.Members["get"]

	thread := &starlark.Thread{Name: "test"}

	// Test getting non-existent variable (should return None)
	args := starlark.Tuple{starlark.String("MEOWG1K_NONEXISTENT_VAR")}
	result, err := starlark.Call(thread, getFunc, args, nil)
	require.NoError(t, err)

	assert.Equal(t, starlark.None, result)
}

// TestEnvGetWithDefault tests env.get() with default value
func TestEnvGetWithDefault(t *testing.T) {
	envModule := NewEnvModule()
	getFunc := envModule.Members["get"]

	thread := &starlark.Thread{Name: "test"}

	// Test getting non-existent variable with default value
	kwargs := []starlark.Tuple{
		{starlark.String("key"), starlark.String("MEOWG1K_NONEXISTENT_VAR")},
		{starlark.String("default"), starlark.String("default_value")},
	}
	result, err := starlark.Call(thread, getFunc, starlark.Tuple{}, kwargs)
	require.NoError(t, err)

	resultStr, ok := result.(starlark.String)
	require.True(t, ok, "result should be a string")
	assert.Equal(t, "default_value", string(resultStr))
}

// TestEnvSet tests env.set() function
func TestEnvSet(t *testing.T) {
	testKey := "MEOWG1K_TEST_SET_VAR"
	testValue := "new_value_456"
	defer os.Unsetenv(testKey)

	envModule := NewEnvModule()
	setFunc := envModule.Members["set"]

	thread := &starlark.Thread{Name: "test"}

	// Test setting a variable
	args := starlark.Tuple{
		starlark.String(testKey),
		starlark.String(testValue),
	}
	result, err := starlark.Call(thread, setFunc, args, nil)
	require.NoError(t, err)
	assert.Equal(t, starlark.None, result)

	// Verify the variable was set
	actualValue := os.Getenv(testKey)
	assert.Equal(t, testValue, actualValue)
}

// TestEnvSetOverwrite tests env.set() overwrites existing variable
func TestEnvSetOverwrite(t *testing.T) {
	testKey := "MEOWG1K_TEST_OVERWRITE_VAR"
	originalValue := "original_value"
	newValue := "new_value"

	require.NoError(t, os.Setenv(testKey, originalValue))
	defer os.Unsetenv(testKey)

	envModule := NewEnvModule()
	setFunc := envModule.Members["set"]

	thread := &starlark.Thread{Name: "test"}

	// Test overwriting the variable
	args := starlark.Tuple{
		starlark.String(testKey),
		starlark.String(newValue),
	}
	result, err := starlark.Call(thread, setFunc, args, nil)
	require.NoError(t, err)
	assert.Equal(t, starlark.None, result)

	// Verify the variable was overwritten
	actualValue := os.Getenv(testKey)
	assert.Equal(t, newValue, actualValue)
}

// TestEnvList tests env.list() function
func TestEnvList(t *testing.T) {
	// Set a couple of test variables
	testKey1 := "MEOWG1K_TEST_LIST_VAR1"
	testValue1 := "value1"
	testKey2 := "MEOWG1K_TEST_LIST_VAR2"
	testValue2 := "value2"

	require.NoError(t, os.Setenv(testKey1, testValue1))
	require.NoError(t, os.Setenv(testKey2, testValue2))
	defer func() {
		os.Unsetenv(testKey1)
		os.Unsetenv(testKey2)
	}()

	envModule := NewEnvModule()
	listFunc := envModule.Members["list"]

	thread := &starlark.Thread{Name: "test"}

	// Test listing all environment variables
	result, err := starlark.Call(thread, listFunc, starlark.Tuple{}, nil)
	require.NoError(t, err)

	dict, ok := result.(*starlark.Dict)
	require.True(t, ok, "result should be a dict")

	// Verify the dict contains our test variables
	val1, found, err := dict.Get(starlark.String(testKey1))
	require.NoError(t, err)
	require.True(t, found, "test key 1 should be in dict")
	assert.Equal(t, testValue1, string(val1.(starlark.String)))

	val2, found, err := dict.Get(starlark.String(testKey2))
	require.NoError(t, err)
	require.True(t, found, "test key 2 should be in dict")
	assert.Equal(t, testValue2, string(val2.(starlark.String)))

	// Verify the dict has at least some entries (system env vars)
	assert.Greater(t, dict.Len(), 0)
}

// TestEnvListEmpty tests env.list() returns dict even if env is minimal
func TestEnvListEmpty(t *testing.T) {
	envModule := NewEnvModule()
	listFunc := envModule.Members["list"]

	thread := &starlark.Thread{Name: "test"}

	result, err := starlark.Call(thread, listFunc, starlark.Tuple{}, nil)
	require.NoError(t, err)

	dict, ok := result.(*starlark.Dict)
	require.True(t, ok, "result should be a dict")

	// Should have at least PATH or similar system variables
	assert.Greater(t, dict.Len(), 0)
}

// TestEnvGetSetIntegration tests getting and setting in sequence
func TestEnvGetSetIntegration(t *testing.T) {
	testKey := "MEOWG1K_INTEGRATION_VAR"
	testValue := "integration_value"
	defer os.Unsetenv(testKey)

	envModule := NewEnvModule()
	getFunc := envModule.Members["get"]
	setFunc := envModule.Members["set"]

	thread := &starlark.Thread{Name: "test"}

	// First, verify variable doesn't exist
	args := starlark.Tuple{starlark.String(testKey)}
	result, err := starlark.Call(thread, getFunc, args, nil)
	require.NoError(t, err)
	assert.Equal(t, starlark.None, result)

	// Set the variable
	setArgs := starlark.Tuple{
		starlark.String(testKey),
		starlark.String(testValue),
	}
	_, err = starlark.Call(thread, setFunc, setArgs, nil)
	require.NoError(t, err)

	// Get the variable again and verify it's set
	result, err = starlark.Call(thread, getFunc, args, nil)
	require.NoError(t, err)

	resultStr, ok := result.(starlark.String)
	require.True(t, ok, "result should be a string")
	assert.Equal(t, testValue, string(resultStr))
}

// TestEnvGetErrors tests error cases for env.get()
func TestEnvGetErrors(t *testing.T) {
	envModule := NewEnvModule()
	getFunc := envModule.Members["get"]
	thread := &starlark.Thread{Name: "test"}

	// Test with no arguments
	_, err := starlark.Call(thread, getFunc, starlark.Tuple{}, nil)
	assert.Error(t, err, "should error with no arguments")

	// Test with wrong argument type
	args := starlark.Tuple{starlark.MakeInt(123)}
	_, err = starlark.Call(thread, getFunc, args, nil)
	assert.Error(t, err, "should error with non-string argument")
}

// TestEnvSetErrors tests error cases for env.set()
func TestEnvSetErrors(t *testing.T) {
	envModule := NewEnvModule()
	setFunc := envModule.Members["set"]
	thread := &starlark.Thread{Name: "test"}

	// Test with no arguments
	_, err := starlark.Call(thread, setFunc, starlark.Tuple{}, nil)
	assert.Error(t, err, "should error with no arguments")

	// Test with only one argument
	args := starlark.Tuple{starlark.String("KEY")}
	_, err = starlark.Call(thread, setFunc, args, nil)
	assert.Error(t, err, "should error with only one argument")

	// Test with too many arguments
	args = starlark.Tuple{
		starlark.String("KEY1"),
		starlark.String("VALUE1"),
		starlark.String("EXTRA"),
	}
	_, err = starlark.Call(thread, setFunc, args, nil)
	assert.Error(t, err, "should error with too many arguments")
}

// TestEnvListErrors tests error cases for env.list()
func TestEnvListErrors(t *testing.T) {
	envModule := NewEnvModule()
	listFunc := envModule.Members["list"]
	thread := &starlark.Thread{Name: "test"}

	// Test with unexpected arguments
	args := starlark.Tuple{starlark.String("unexpected")}
	_, err := starlark.Call(thread, listFunc, args, nil)
	assert.Error(t, err, "should error with unexpected arguments")
}
