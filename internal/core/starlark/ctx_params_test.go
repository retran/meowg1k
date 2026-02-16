// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// TestContextWithParams_Attr tests the Attr method for parameter access
func TestContextWithParams_Attr(t *testing.T) {
	t.Run("retrieves injected parameter", func(t *testing.T) {
		baseCtx := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"workspace": starlark.String("/tmp/test"),
		})

		params := map[string]starlark.Value{
			"query":       starlark.String("test query"),
			"max_results": starlark.MakeInt(10),
		}

		ctx := CreateContextWithParams(baseCtx, params)

		// Access injected parameter
		queryVal, err := ctx.Attr("query")
		require.NoError(t, err)
		query, ok := starlark.AsString(queryVal)
		require.True(t, ok)
		assert.Equal(t, "test query", query)

		// Access another injected parameter
		maxResultsVal, err := ctx.Attr("max_results")
		require.NoError(t, err)
		maxResults, ok := maxResultsVal.(starlark.Int)
		require.True(t, ok)
		maxInt, _ := maxResults.Int64()
		assert.Equal(t, int64(10), maxInt)
	})

	t.Run("falls back to base context attributes", func(t *testing.T) {
		baseCtx := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"workspace": starlark.String("/tmp/test"),
			"fs":        starlark.String("fs_module"),
		})

		params := map[string]starlark.Value{
			"query": starlark.String("test query"),
		}

		ctx := CreateContextWithParams(baseCtx, params)

		// Access base context attribute
		workspaceVal, err := ctx.Attr("workspace")
		require.NoError(t, err)
		workspace, ok := starlark.AsString(workspaceVal)
		require.True(t, ok)
		assert.Equal(t, "/tmp/test", workspace)

		fsVal, err := ctx.Attr("fs")
		require.NoError(t, err)
		fs, ok := starlark.AsString(fsVal)
		require.True(t, ok)
		assert.Equal(t, "fs_module", fs)
	})

	t.Run("parameters take precedence over base context", func(t *testing.T) {
		baseCtx := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"value": starlark.String("base value"),
		})

		params := map[string]starlark.Value{
			"value": starlark.String("param value"),
		}

		ctx := CreateContextWithParams(baseCtx, params)

		// Parameter should shadow base context attribute
		valueVal, err := ctx.Attr("value")
		require.NoError(t, err)
		value, ok := starlark.AsString(valueVal)
		require.True(t, ok)
		assert.Equal(t, "param value", value)
	})

	t.Run("returns error for non-existent attribute", func(t *testing.T) {
		baseCtx := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"workspace": starlark.String("/tmp/test"),
		})

		params := map[string]starlark.Value{
			"query": starlark.String("test query"),
		}

		ctx := CreateContextWithParams(baseCtx, params)

		// Try to access non-existent attribute
		_, err := ctx.Attr("nonexistent")
		assert.Error(t, err)
		// The error message comes from the base starlarkstruct, not our custom message
		assert.Contains(t, err.Error(), "nonexistent")
	})
}

// TestContextWithParams_AttrNames tests the AttrNames method
func TestContextWithParams_AttrNames(t *testing.T) {
	t.Run("returns all parameter and base context names", func(t *testing.T) {
		baseCtx := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"workspace": starlark.String("/tmp/test"),
			"fs":        starlark.String("fs_module"),
		})

		params := map[string]starlark.Value{
			"query":       starlark.String("test query"),
			"max_results": starlark.MakeInt(10),
		}

		ctx := CreateContextWithParams(baseCtx, params)

		names := ctx.AttrNames()

		// Should contain parameter names
		assert.Contains(t, names, "query")
		assert.Contains(t, names, "max_results")

		// Should contain base context names
		assert.Contains(t, names, "workspace")
		assert.Contains(t, names, "fs")

		// Should have at least 4 names
		assert.GreaterOrEqual(t, len(names), 4)
	})

	t.Run("works with empty parameters", func(t *testing.T) {
		baseCtx := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"workspace": starlark.String("/tmp/test"),
		})

		params := map[string]starlark.Value{}

		ctx := CreateContextWithParams(baseCtx, params)

		names := ctx.AttrNames()

		// Should only contain base context names
		assert.Contains(t, names, "workspace")
		assert.GreaterOrEqual(t, len(names), 1)
	})
}

// TestContextWithParams_StarlarkInterface tests Starlark interface methods
func TestContextWithParams_StarlarkInterface(t *testing.T) {
	t.Run("String returns ctx", func(t *testing.T) {
		baseCtx := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{})
		params := map[string]starlark.Value{}
		ctx := CreateContextWithParams(baseCtx, params)

		assert.Equal(t, "ctx", ctx.String())
	})

	t.Run("Type returns context", func(t *testing.T) {
		baseCtx := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{})
		params := map[string]starlark.Value{}
		ctx := CreateContextWithParams(baseCtx, params)

		assert.Equal(t, "context", ctx.Type())
	})

	t.Run("Truth returns true", func(t *testing.T) {
		baseCtx := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{})
		params := map[string]starlark.Value{}
		ctx := CreateContextWithParams(baseCtx, params)

		assert.Equal(t, starlark.True, ctx.Truth())
	})

	t.Run("Hash returns error", func(t *testing.T) {
		baseCtx := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{})
		params := map[string]starlark.Value{}
		ctx := CreateContextWithParams(baseCtx, params)

		_, err := ctx.Hash()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unhashable")
	})

	t.Run("Freeze works without panic", func(t *testing.T) {
		baseCtx := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{})
		params := map[string]starlark.Value{}
		ctx := CreateContextWithParams(baseCtx, params)

		// Should not panic
		assert.NotPanics(t, func() {
			ctx.Freeze()
		})
	})
}

// TestContextWithParams_Integration tests integration with Starlark scripts
func TestContextWithParams_Integration(t *testing.T) {
	t.Run("parameter access in Starlark script", func(t *testing.T) {
		baseCtx := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"workspace": starlark.String("/workspace"),
		})

		params := map[string]starlark.Value{
			"query":  starlark.String("find bugs"),
			"limit":  starlark.MakeInt(5),
			"urgent": starlark.Bool(true),
		}

		ctx := CreateContextWithParams(baseCtx, params)

		// Test accessing parameters via Starlark
		thread := &starlark.Thread{Name: "test"}
		predeclared := starlark.StringDict{
			"ctx": ctx,
		}

		script := `
query = ctx.query
limit = ctx.limit
urgent = ctx.urgent
workspace = ctx.workspace
`
		globals, err := starlark.ExecFile(thread, "test.star", script, predeclared)
		require.NoError(t, err)

		// Verify values
		queryVal := globals["query"]
		require.NotNil(t, queryVal, "query should be set")
		query, ok := starlark.AsString(queryVal)
		require.True(t, ok, "query should be a string, got %T", queryVal)
		assert.Equal(t, "find bugs", query)

		limitVal := globals["limit"]
		require.NotNil(t, limitVal, "limit should be set")
		limitInt, ok := limitVal.(starlark.Int)
		require.True(t, ok, "limit should be an int, got %T", limitVal)
		limit, _ := limitInt.Int64()
		assert.Equal(t, int64(5), limit)

		urgentVal := globals["urgent"]
		require.NotNil(t, urgentVal, "urgent should be set")
		assert.Equal(t, starlark.Bool(true), urgentVal, "urgent should be true")

		workspaceVal := globals["workspace"]
		require.NotNil(t, workspaceVal, "workspace should be set")
		workspace, ok := starlark.AsString(workspaceVal)
		require.True(t, ok, "workspace should be a string, got %T", workspaceVal)
		assert.Equal(t, "/workspace", workspace)
	})
}
