// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

// TestParamValue tests the ParamValue wrapper type
func TestParamValue(t *testing.T) {
	param := &Param{
		Type:     "string",
		Required: true,
	}
	paramVal := &ParamValue{Param: param}

	t.Run("String method", func(t *testing.T) {
		str := paramVal.String()
		assert.Equal(t, "Param(string)", str)
	})

	t.Run("Type method", func(t *testing.T) {
		typ := paramVal.Type()
		assert.Equal(t, "param", typ)
	})

	t.Run("Freeze method", func(t *testing.T) {
		// Should not panic
		paramVal.Freeze()
	})

	t.Run("Truth method", func(t *testing.T) {
		truth := paramVal.Truth()
		assert.Equal(t, starlark.True, truth)
	})

	t.Run("Hash method", func(t *testing.T) {
		_, err := paramVal.Hash()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unhashable")
	})
}

// TestToolValue tests the ToolValue wrapper type
func TestToolValue(t *testing.T) {
	tool := &Tool{
		Name:        "test_tool",
		Description: "A test tool",
	}
	toolVal := &ToolValue{Tool: tool}

	t.Run("String method", func(t *testing.T) {
		str := toolVal.String()
		assert.Equal(t, "Tool(test_tool)", str)
	})

	t.Run("Type method", func(t *testing.T) {
		typ := toolVal.Type()
		assert.Equal(t, "tool", typ)
	})

	t.Run("Freeze method", func(t *testing.T) {
		// Should not panic
		toolVal.Freeze()
	})

	t.Run("Truth method", func(t *testing.T) {
		truth := toolVal.Truth()
		assert.Equal(t, starlark.True, truth)
	})

	t.Run("Hash method", func(t *testing.T) {
		_, err := toolVal.Hash()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unhashable")
	})
}

// TestCreateParamFunction tests the meow.param() builtin
func TestCreateParamFunction(t *testing.T) {
	paramFunc := CreateParamFunction()
	require.NotNil(t, paramFunc)

	thread := &starlark.Thread{Name: "test"}

	t.Run("basic string param", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("string")}
		result, err := starlark.Call(thread, paramFunc, args, nil)
		require.NoError(t, err)

		paramVal, ok := result.(*ParamValue)
		require.True(t, ok)
		assert.Equal(t, "string", paramVal.Param.Type)
	})

	t.Run("param with description", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("int")}
		kwargs := []starlark.Tuple{
			{starlark.String("desc"), starlark.String("A test parameter")},
		}
		result, err := starlark.Call(thread, paramFunc, args, kwargs)
		require.NoError(t, err)

		paramVal, ok := result.(*ParamValue)
		require.True(t, ok)
		assert.Equal(t, "int", paramVal.Param.Type)
		assert.Equal(t, "A test parameter", paramVal.Param.Description)
	})

	t.Run("param with required flag", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("bool")}
		kwargs := []starlark.Tuple{
			{starlark.String("required"), starlark.Bool(true)},
		}
		result, err := starlark.Call(thread, paramFunc, args, kwargs)
		require.NoError(t, err)

		paramVal, ok := result.(*ParamValue)
		require.True(t, ok)
		assert.True(t, paramVal.Param.Required)
	})

	t.Run("param with short flag", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("string")}
		kwargs := []starlark.Tuple{
			{starlark.String("short"), starlark.String("s")},
		}
		result, err := starlark.Call(thread, paramFunc, args, kwargs)
		require.NoError(t, err)

		paramVal, ok := result.(*ParamValue)
		require.True(t, ok)
		assert.Equal(t, "s", paramVal.Param.Short)
	})

	t.Run("param with default value", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("string")}
		kwargs := []starlark.Tuple{
			{starlark.String("default"), starlark.String("default_value")},
		}
		result, err := starlark.Call(thread, paramFunc, args, kwargs)
		require.NoError(t, err)

		paramVal, ok := result.(*ParamValue)
		require.True(t, ok)
		assert.Equal(t, "default_value", paramVal.Param.Default)
	})

	t.Run("param with from_stdin flag", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("string")}
		kwargs := []starlark.Tuple{
			{starlark.String("from_stdin"), starlark.Bool(true)},
		}
		result, err := starlark.Call(thread, paramFunc, args, kwargs)
		require.NoError(t, err)

		paramVal, ok := result.(*ParamValue)
		require.True(t, ok)
		assert.True(t, paramVal.Param.FromStdin)
	})

	t.Run("param with choices", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("string")}
		choices := starlark.NewList([]starlark.Value{
			starlark.String("option1"),
			starlark.String("option2"),
			starlark.String("option3"),
		})
		kwargs := []starlark.Tuple{
			{starlark.String("choices"), choices},
		}
		result, err := starlark.Call(thread, paramFunc, args, kwargs)
		require.NoError(t, err)

		paramVal, ok := result.(*ParamValue)
		require.True(t, ok)
		require.Len(t, paramVal.Param.Choices, 3)
		assert.Equal(t, "option1", paramVal.Param.Choices[0])
		assert.Equal(t, "option2", paramVal.Param.Choices[1])
		assert.Equal(t, "option3", paramVal.Param.Choices[2])
	})

	t.Run("param with pattern", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("string")}
		kwargs := []starlark.Tuple{
			{starlark.String("pattern"), starlark.String("^[a-z]+$")},
		}
		result, err := starlark.Call(thread, paramFunc, args, kwargs)
		require.NoError(t, err)

		paramVal, ok := result.(*ParamValue)
		require.True(t, ok)
		assert.Equal(t, "^[a-z]+$", paramVal.Param.Pattern)
		assert.NotNil(t, paramVal.Param.PatternRegex)
	})

	t.Run("param with min constraint", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("int")}
		kwargs := []starlark.Tuple{
			{starlark.String("min"), starlark.MakeInt(10)},
		}
		result, err := starlark.Call(thread, paramFunc, args, kwargs)
		require.NoError(t, err)

		paramVal, ok := result.(*ParamValue)
		require.True(t, ok)
		require.NotNil(t, paramVal.Param.Min)
		assert.Equal(t, float64(10), *paramVal.Param.Min)
	})

	t.Run("param with max constraint", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("int")}
		kwargs := []starlark.Tuple{
			{starlark.String("max"), starlark.MakeInt(100)},
		}
		result, err := starlark.Call(thread, paramFunc, args, kwargs)
		require.NoError(t, err)

		paramVal, ok := result.(*ParamValue)
		require.True(t, ok)
		require.NotNil(t, paramVal.Param.Max)
		assert.Equal(t, float64(100), *paramVal.Param.Max)
	})

	t.Run("param with min_len constraint", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("string")}
		kwargs := []starlark.Tuple{
			{starlark.String("min_len"), starlark.MakeInt(5)},
		}
		result, err := starlark.Call(thread, paramFunc, args, kwargs)
		require.NoError(t, err)

		paramVal, ok := result.(*ParamValue)
		require.True(t, ok)
		require.NotNil(t, paramVal.Param.MinLen)
		assert.Equal(t, 5, *paramVal.Param.MinLen)
	})

	t.Run("param with max_len constraint", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("string")}
		kwargs := []starlark.Tuple{
			{starlark.String("max_len"), starlark.MakeInt(50)},
		}
		result, err := starlark.Call(thread, paramFunc, args, kwargs)
		require.NoError(t, err)

		paramVal, ok := result.(*ParamValue)
		require.True(t, ok)
		require.NotNil(t, paramVal.Param.MaxLen)
		assert.Equal(t, 50, *paramVal.Param.MaxLen)
	})
}

// TestCreateParamFunctionErrors tests error cases for meow.param()
func TestCreateParamFunctionErrors(t *testing.T) {
	paramFunc := CreateParamFunction()
	thread := &starlark.Thread{Name: "test"}

	t.Run("missing type argument", func(t *testing.T) {
		_, err := starlark.Call(thread, paramFunc, starlark.Tuple{}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires type")
	})

	t.Run("non-string type argument", func(t *testing.T) {
		args := starlark.Tuple{starlark.MakeInt(123)}
		_, err := starlark.Call(thread, paramFunc, args, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be string")
	})

	t.Run("invalid pattern", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("string")}
		kwargs := []starlark.Tuple{
			{starlark.String("pattern"), starlark.String("[invalid(")},
		}
		_, err := starlark.Call(thread, paramFunc, args, kwargs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pattern")
	})
}

// TestConvertChoices tests the convertChoices helper function
func TestConvertChoices(t *testing.T) {
	t.Run("list of strings", func(t *testing.T) {
		choices := starlark.NewList([]starlark.Value{
			starlark.String("a"),
			starlark.String("b"),
			starlark.String("c"),
		})
		result, err := convertChoices(choices)
		require.NoError(t, err)
		assert.Equal(t, []interface{}{"a", "b", "c"}, result)
	})

	t.Run("list of ints", func(t *testing.T) {
		choices := starlark.NewList([]starlark.Value{
			starlark.MakeInt(1),
			starlark.MakeInt(2),
			starlark.MakeInt(3),
		})
		result, err := convertChoices(choices)
		require.NoError(t, err)
		assert.Equal(t, []interface{}{1, 2, 3}, result)
	})

	t.Run("list of floats", func(t *testing.T) {
		choices := starlark.NewList([]starlark.Value{
			starlark.Float(1.5),
			starlark.Float(2.5),
		})
		result, err := convertChoices(choices)
		require.NoError(t, err)
		assert.Equal(t, []interface{}{1.5, 2.5}, result)
	})

	t.Run("list of bools", func(t *testing.T) {
		choices := starlark.NewList([]starlark.Value{
			starlark.Bool(true),
			starlark.Bool(false),
		})
		result, err := convertChoices(choices)
		require.NoError(t, err)
		assert.Equal(t, []interface{}{true, false}, result)
	})

	t.Run("non-list value", func(t *testing.T) {
		_, err := convertChoices(starlark.String("not a list"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be a list")
	})
}

// TestConvertIntConstraint tests the convertIntConstraint helper
func TestConvertIntConstraint(t *testing.T) {
	t.Run("int value", func(t *testing.T) {
		result, err := convertIntConstraint(starlark.MakeInt(42))
		require.NoError(t, err)
		assert.Equal(t, 42, result)
	})

	t.Run("non-int value", func(t *testing.T) {
		_, err := convertIntConstraint(starlark.String("not an int"))
		assert.Error(t, err)
	})
}

// TestConvertStarlarkValue tests the convertStarlarkValue helper
func TestConvertStarlarkValue(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		result := convertStarlarkValue(starlark.String("test"))
		assert.Equal(t, "test", result)
	})

	t.Run("int", func(t *testing.T) {
		result := convertStarlarkValue(starlark.MakeInt(42))
		assert.Equal(t, 42, result)
	})

	t.Run("float", func(t *testing.T) {
		result := convertStarlarkValue(starlark.Float(3.14))
		assert.Equal(t, 3.14, result)
	})

	t.Run("bool", func(t *testing.T) {
		result := convertStarlarkValue(starlark.Bool(true))
		assert.Equal(t, true, result)
	})

	t.Run("None", func(t *testing.T) {
		result := convertStarlarkValue(starlark.None)
		assert.Nil(t, result)
	})
}

// TestConvertNumericConstraint tests the convertNumericConstraint helper
func TestConvertNumericConstraint(t *testing.T) {
	t.Run("int value", func(t *testing.T) {
		result, err := convertNumericConstraint(starlark.MakeInt(42))
		require.NoError(t, err)
		assert.Equal(t, float64(42), result)
	})

	t.Run("float value", func(t *testing.T) {
		result, err := convertNumericConstraint(starlark.Float(3.14))
		require.NoError(t, err)
		assert.Equal(t, 3.14, result)
	})

	t.Run("non-numeric value", func(t *testing.T) {
		_, err := convertNumericConstraint(starlark.String("not a number"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be int or float")
	})
}

// TestConvertIntConstraintEdgeCases tests edge cases for convertIntConstraint
func TestConvertIntConstraintEdgeCases(t *testing.T) {
	t.Run("normal int", func(t *testing.T) {
		result, err := convertIntConstraint(starlark.MakeInt(100))
		require.NoError(t, err)
		assert.Equal(t, 100, result)
	})

	t.Run("string value error", func(t *testing.T) {
		_, err := convertIntConstraint(starlark.String("not an int"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be int")
	})

	t.Run("float value error", func(t *testing.T) {
		_, err := convertIntConstraint(starlark.Float(3.14))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be int")
	})
}

// TestConvertStarlarkValueEdgeCases tests edge cases for convertStarlarkValue
func TestConvertStarlarkValueEdgeCases(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		result := convertStarlarkValue(starlark.String("test"))
		assert.Equal(t, "test", result)
	})

	t.Run("int", func(t *testing.T) {
		result := convertStarlarkValue(starlark.MakeInt(42))
		assert.Equal(t, 42, result)
	})

	t.Run("float", func(t *testing.T) {
		result := convertStarlarkValue(starlark.Float(3.14))
		assert.Equal(t, 3.14, result)
	})

	t.Run("bool true", func(t *testing.T) {
		result := convertStarlarkValue(starlark.Bool(true))
		assert.Equal(t, true, result)
	})

	t.Run("bool false", func(t *testing.T) {
		result := convertStarlarkValue(starlark.Bool(false))
		assert.Equal(t, false, result)
	})

	t.Run("None", func(t *testing.T) {
		result := convertStarlarkValue(starlark.None)
		assert.Nil(t, result)
	})

	t.Run("unknown type returns nil", func(t *testing.T) {
		// Use a Tuple which is not handled in convertStarlarkValue
		result := convertStarlarkValue(starlark.Tuple{starlark.String("test")})
		assert.Nil(t, result)
	})
}

// TestCreateParamFunctionValidatorCases tests validator parameter handling
func TestCreateParamFunctionValidatorCases(t *testing.T) {
	paramFunc := CreateParamFunction()
	thread := &starlark.Thread{Name: "test"}

	t.Run("param with None validator", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("string")}
		kwargs := []starlark.Tuple{
			{starlark.String("validator"), starlark.None},
		}
		result, err := starlark.Call(thread, paramFunc, args, kwargs)
		require.NoError(t, err)

		paramVal, ok := result.(*ParamValue)
		require.True(t, ok)
		assert.Nil(t, paramVal.Param.ValidatorTool)
		assert.Nil(t, paramVal.Param.ValidatorFunc)
	})
}

// TestCreateToolsModule tests module creation
func TestCreateToolsModule(t *testing.T) {
	registry := NewRegistry()
	module := CreateToolsModule(registry)

	require.NotNil(t, module)
	assert.Equal(t, "meow_tools", module.Name)

	// Verify all expected functions are present
	_, ok := module.Members["param"]
	assert.True(t, ok, "param function should be present")

	_, ok = module.Members["tool"]
	assert.True(t, ok, "tool function should be present")

	_, ok = module.Members["command"]
	assert.True(t, ok, "command function should be present")
}

func TestCreateToolFunction(t *testing.T) {
	t.Run("creates tool successfully", func(t *testing.T) {
		registry := NewRegistry()
		toolFunc := CreateToolFunction(registry)
		require.NotNil(t, toolFunc)

		thread := &starlark.Thread{Name: "test"}

		// Create a simple handler function
		handlerCode := `def handler(ctx): return "result"`
		globals, err := starlark.ExecFile(thread, "test.star", handlerCode, nil)
		require.NoError(t, err)
		handler := globals["handler"]

		// Call tool()
		kwargs := []starlark.Tuple{
			{starlark.String("name"), starlark.String("test-tool")},
			{starlark.String("handler"), handler},
		}
		result, err := starlark.Call(thread, toolFunc, nil, kwargs)

		require.NoError(t, err)
		toolVal, ok := result.(*ToolValue)
		require.True(t, ok)
		assert.Equal(t, "test-tool", toolVal.Tool.Name)
		assert.NotNil(t, toolVal.Tool.Handler)
	})

	t.Run("error when name is missing", func(t *testing.T) {
		registry := NewRegistry()
		toolFunc := CreateToolFunction(registry)

		thread := &starlark.Thread{Name: "test"}

		// Create a handler but don't provide name
		handlerCode := `def handler(ctx): return "result"`
		globals, err := starlark.ExecFile(thread, "test.star", handlerCode, nil)
		require.NoError(t, err)
		handler := globals["handler"]

		kwargs := []starlark.Tuple{
			{starlark.String("handler"), handler},
		}
		_, err = starlark.Call(thread, toolFunc, nil, kwargs)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool name is required")
	})

	t.Run("error when handler is missing", func(t *testing.T) {
		registry := NewRegistry()
		toolFunc := CreateToolFunction(registry)

		thread := &starlark.Thread{Name: "test"}

		kwargs := []starlark.Tuple{
			{starlark.String("name"), starlark.String("test-tool")},
		}
		_, err := starlark.Call(thread, toolFunc, nil, kwargs)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool handler is required")
	})
}

func TestCreateCommandFunction(t *testing.T) {
	t.Run("creates command from tool successfully", func(t *testing.T) {
		registry := NewRegistry()
		commandFunc := CreateCommandFunction(registry)
		require.NotNil(t, commandFunc)

		thread := &starlark.Thread{Name: "test"}

		// Create a tool first
		handlerCode := `def handler(ctx): return "result"`
		globals, err := starlark.ExecFile(thread, "test.star", handlerCode, nil)
		require.NoError(t, err)
		handler := globals["handler"]

		tool := &Tool{
			Name:    "test-tool",
			Handler: handler.(*starlark.Function),
		}
		toolVal := &ToolValue{Tool: tool}

		// Call command() with the tool
		args := starlark.Tuple{toolVal}
		result, err := starlark.Call(thread, commandFunc, args, nil)

		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)

		// Verify command was registered
		cmd, ok := registry.Get("test-tool")
		require.True(t, ok)
		require.NotNil(t, cmd)
		assert.Equal(t, "test-tool", cmd.Name)
	})

	t.Run("creates command with name override", func(t *testing.T) {
		registry := NewRegistry()
		commandFunc := CreateCommandFunction(registry)

		thread := &starlark.Thread{Name: "test"}

		handlerCode := `def handler(ctx): return "result"`
		globals, err := starlark.ExecFile(thread, "test.star", handlerCode, nil)
		require.NoError(t, err)
		handler := globals["handler"]

		tool := &Tool{
			Name:    "original-name",
			Handler: handler.(*starlark.Function),
		}
		toolVal := &ToolValue{Tool: tool}

		// Call command() with name override
		args := starlark.Tuple{toolVal}
		kwargs := []starlark.Tuple{
			{starlark.String("name"), starlark.String("new-name")},
		}
		result, err := starlark.Call(thread, commandFunc, args, kwargs)

		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)

		// Verify command was registered with new name
		cmd, ok := registry.Get("new-name")
		require.True(t, ok)
		require.NotNil(t, cmd)
		assert.Equal(t, "new-name", cmd.Name)
	})

	t.Run("error when no arguments provided", func(t *testing.T) {
		registry := NewRegistry()
		commandFunc := CreateCommandFunction(registry)

		thread := &starlark.Thread{Name: "test"}

		_, err := starlark.Call(thread, commandFunc, nil, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires a tool as first argument")
	})

	t.Run("error when first argument is not a tool", func(t *testing.T) {
		registry := NewRegistry()
		commandFunc := CreateCommandFunction(registry)

		thread := &starlark.Thread{Name: "test"}

		// Pass a string instead of a tool
		args := starlark.Tuple{starlark.String("not-a-tool")}
		_, err := starlark.Call(thread, commandFunc, args, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "first argument must be a tool")
	})

	t.Run("error when tool has nil handler", func(t *testing.T) {
		registry := NewRegistry()
		commandFunc := CreateCommandFunction(registry)

		thread := &starlark.Thread{Name: "test"}

		// Create a tool with nil handler
		tool := &Tool{
			Name:    "invalid-tool",
			Handler: nil,
		}
		toolVal := &ToolValue{Tool: tool}

		args := starlark.Tuple{toolVal}
		_, err := starlark.Call(thread, commandFunc, args, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handler is required")
	})
}
