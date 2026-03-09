// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func TestShellModule_Exec(t *testing.T) {
	t.Run("executes command and returns stdout", func(t *testing.T) {
		tmpDir := t.TempDir()
		rt := NewRuntime(tmpDir)
		shellModule := rt.createShellModule()

		// Get the exec function
		shellStruct := shellModule.(*starlarkstruct.Struct)
		execVal, err := shellStruct.Attr("exec")
		require.NoError(t, err)
		execFunc := execVal.(starlark.Callable)

		// Call shell.exec(command="echo hello")
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("command"), starlark.String("echo hello")},
		}
		result, err := starlark.Call(thread, execFunc, nil, kwargs)

		require.NoError(t, err)
		resultStruct, ok := result.(*starlarkstruct.Struct)
		require.True(t, ok, "shell.exec() should return a struct")

		// Check stdout
		stdoutVal, err := resultStruct.Attr("stdout")
		require.NoError(t, err)
		stdout, _ := starlark.AsString(stdoutVal)
		assert.Contains(t, stdout, "hello")

		// Check exit code
		exitCodeVal, err := resultStruct.Attr("exit_code")
		require.NoError(t, err)
		exitCodeInt, ok := exitCodeVal.(starlark.Int)
		require.True(t, ok)
		exitCode, _ := exitCodeInt.Int64()
		assert.Equal(t, int64(0), exitCode)
	})

	t.Run("captures stderr", func(t *testing.T) {
		tmpDir := t.TempDir()
		rt := NewRuntime(tmpDir)
		shellModule := rt.createShellModule()

		// Get the exec function
		shellStruct := shellModule.(*starlarkstruct.Struct)
		execVal, err := shellStruct.Attr("exec")
		require.NoError(t, err)
		execFunc := execVal.(starlark.Callable)

		// Call shell.exec(command="echo error >&2")
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("command"), starlark.String("echo error >&2")},
		}
		result, err := starlark.Call(thread, execFunc, nil, kwargs)

		require.NoError(t, err)
		resultStruct, ok := result.(*starlarkstruct.Struct)
		require.True(t, ok)

		// Check stderr
		stderrVal, err := resultStruct.Attr("stderr")
		require.NoError(t, err)
		stderr, _ := starlark.AsString(stderrVal)
		assert.Contains(t, stderr, "error")
	})

	t.Run("returns non-zero exit code on failure", func(t *testing.T) {
		tmpDir := t.TempDir()
		rt := NewRuntime(tmpDir)
		shellModule := rt.createShellModule()

		// Get the exec function
		shellStruct := shellModule.(*starlarkstruct.Struct)
		execVal, err := shellStruct.Attr("exec")
		require.NoError(t, err)
		execFunc := execVal.(starlark.Callable)

		// Call shell.exec(command="exit 42")
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("command"), starlark.String("exit 42")},
		}
		result, err := starlark.Call(thread, execFunc, nil, kwargs)

		require.NoError(t, err)
		resultStruct, ok := result.(*starlarkstruct.Struct)
		require.True(t, ok)

		// Check exit code
		exitCodeVal, err := resultStruct.Attr("exit_code")
		require.NoError(t, err)
		exitCodeInt, ok := exitCodeVal.(starlark.Int)
		require.True(t, ok)
		exitCode, _ := exitCodeInt.Int64()
		assert.Equal(t, int64(42), exitCode)
	})

	t.Run("executes command in working directory", func(t *testing.T) {
		// Skip on Windows as pwd command may not be available
		if runtime.GOOS == "windows" {
			t.Skip("Skipping on Windows")
		}

		tmpDir := t.TempDir()
		rt := NewRuntime(tmpDir)
		shellModule := rt.createShellModule()

		// Get the exec function
		shellStruct := shellModule.(*starlarkstruct.Struct)
		execVal, err := shellStruct.Attr("exec")
		require.NoError(t, err)
		execFunc := execVal.(starlark.Callable)

		// Call shell.exec(command="pwd")
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("command"), starlark.String("pwd")},
		}
		result, err := starlark.Call(thread, execFunc, nil, kwargs)

		require.NoError(t, err)
		resultStruct, ok := result.(*starlarkstruct.Struct)
		require.True(t, ok)

		// Check stdout contains tmpDir
		stdoutVal, err := resultStruct.Attr("stdout")
		require.NoError(t, err)
		stdout, _ := starlark.AsString(stdoutVal)
		assert.Contains(t, stdout, tmpDir)
	})

	t.Run("handles multiline output", func(t *testing.T) {
		tmpDir := t.TempDir()
		rt := NewRuntime(tmpDir)
		shellModule := rt.createShellModule()

		// Get the exec function
		shellStruct := shellModule.(*starlarkstruct.Struct)
		execVal, err := shellStruct.Attr("exec")
		require.NoError(t, err)
		execFunc := execVal.(starlark.Callable)

		// Call shell.exec(command="echo line1 && echo line2")
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("command"), starlark.String("echo line1 && echo line2")},
		}
		result, err := starlark.Call(thread, execFunc, nil, kwargs)

		require.NoError(t, err)
		resultStruct, ok := result.(*starlarkstruct.Struct)
		require.True(t, ok)

		// Check stdout contains both lines
		stdoutVal, err := resultStruct.Attr("stdout")
		require.NoError(t, err)
		stdout, _ := starlark.AsString(stdoutVal)
		assert.Contains(t, stdout, "line1")
		assert.Contains(t, stdout, "line2")
	})
}
