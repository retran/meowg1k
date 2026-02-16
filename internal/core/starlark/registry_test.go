// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

func TestNewRegistry(t *testing.T) {
	t.Run("creates empty registry", func(t *testing.T) {
		registry := NewRegistry()

		require.NotNil(t, registry)
		assert.NotNil(t, registry.commands)
		assert.NotNil(t, registry.tools)
		assert.Len(t, registry.List(), 0)
		assert.Len(t, registry.ListTools(), 0)
	})
}

// TestBuildFlagDescription tests the buildFlagDescription helper function
func TestBuildFlagDescription(t *testing.T) {
	t.Run("returns basic description unchanged", func(t *testing.T) {
		param := &Param{
			Description: "A simple parameter",
		}

		result := buildFlagDescription(param)
		assert.Equal(t, "A simple parameter", result)
	})

	t.Run("appends choices when not in description", func(t *testing.T) {
		param := &Param{
			Description: "Select a type",
			Choices:     []any{"feat", "fix", "docs"},
		}

		result := buildFlagDescription(param)
		assert.Contains(t, result, "Possible values:")
		assert.Contains(t, result, "feat")
		assert.Contains(t, result, "fix")
		assert.Contains(t, result, "docs")
	})

	t.Run("does not append choices if already in description", func(t *testing.T) {
		param := &Param{
			Description: "Select a type: feat, fix, or docs",
			Choices:     []any{"feat", "fix", "docs"},
		}

		result := buildFlagDescription(param)
		// Should not add "Possible values:" because choices are mentioned
		assert.NotContains(t, result, "Possible values:")
		// Original description should remain
		assert.Contains(t, result, "Select a type: feat, fix, or docs")
	})

	t.Run("appends min and max range for numbers", func(t *testing.T) {
		min := 1.0
		max := 10.0
		param := &Param{
			Description: "Number of results",
			Min:         &min,
			Max:         &max,
		}

		result := buildFlagDescription(param)
		assert.Contains(t, result, "Range: [1, 10]")
	})

	t.Run("appends only minimum when max not set", func(t *testing.T) {
		min := 5.0
		param := &Param{
			Description: "Minimum value",
			Min:         &min,
		}

		result := buildFlagDescription(param)
		assert.Contains(t, result, "Minimum: 5")
		assert.NotContains(t, result, "Range:")
	})

	t.Run("appends only maximum when min not set", func(t *testing.T) {
		max := 100.0
		param := &Param{
			Description: "Maximum value",
			Max:         &max,
		}

		result := buildFlagDescription(param)
		assert.Contains(t, result, "Maximum: 100")
		assert.NotContains(t, result, "Range:")
	})

	t.Run("appends min and max length for strings", func(t *testing.T) {
		minLen := 3
		maxLen := 50
		param := &Param{
			Description: "Username",
			MinLen:      &minLen,
			MaxLen:      &maxLen,
		}

		result := buildFlagDescription(param)
		assert.Contains(t, result, "Length: [3, 50]")
	})

	t.Run("appends only minimum length when max not set", func(t *testing.T) {
		minLen := 5
		param := &Param{
			Description: "Password",
			MinLen:      &minLen,
		}

		result := buildFlagDescription(param)
		assert.Contains(t, result, "Minimum length: 5")
		assert.NotContains(t, result, "Length:")
	})

	t.Run("appends only maximum length when min not set", func(t *testing.T) {
		maxLen := 140
		param := &Param{
			Description: "Tweet",
			MaxLen:      &maxLen,
		}

		result := buildFlagDescription(param)
		assert.Contains(t, result, "Maximum length: 140")
		assert.NotContains(t, result, "Length:")
	})

	t.Run("appends pattern constraint", func(t *testing.T) {
		param := &Param{
			Description: "Email address",
			Pattern:     "^[a-z0-9._%+-]+@[a-z0-9.-]+\\.[a-z]{2,}$",
		}

		result := buildFlagDescription(param)
		assert.Contains(t, result, "Pattern:")
		assert.Contains(t, result, "^[a-z0-9._%+-]+@[a-z0-9.-]+\\.[a-z]{2,}$")
	})

	t.Run("appends stdin note when FromStdin is true", func(t *testing.T) {
		param := &Param{
			Description: "Input data",
			FromStdin:   true,
		}

		result := buildFlagDescription(param)
		assert.Contains(t, result, "can be read from stdin")
	})

	t.Run("combines multiple constraints", func(t *testing.T) {
		minLen := 5
		maxLen := 100
		param := &Param{
			Description: "Commit message",
			MinLen:      &minLen,
			MaxLen:      &maxLen,
			Pattern:     "^[A-Z]",
			FromStdin:   true,
		}

		result := buildFlagDescription(param)
		assert.Contains(t, result, "Commit message")
		assert.Contains(t, result, "Length: [5, 100]")
		assert.Contains(t, result, "Pattern: ^[A-Z]")
		assert.Contains(t, result, "can be read from stdin")
	})

	t.Run("handles integer choices", func(t *testing.T) {
		param := &Param{
			Description: "Priority level",
			Choices:     []any{1, 2, 3, 4, 5},
		}

		result := buildFlagDescription(param)
		assert.Contains(t, result, "Possible values:")
		assert.Contains(t, result, "1")
		assert.Contains(t, result, "5")
	})
}

// TestRegistry_Register tests command registration
func TestRegistry_Register(t *testing.T) {
	createTestHandler := func(t *testing.T) *starlark.Function {
		t.Helper()
		thread := &starlark.Thread{Name: "test"}
		globals, _ := starlark.ExecFile(thread, "test.star", `def handler(ctx): return None`, nil)
		return globals["handler"].(*starlark.Function)
	}

	t.Run("registers command", func(t *testing.T) {
		registry := NewRegistry()
		cmd := &Command{
			Name:        "test-cmd",
			Description: "A test command",
			Handler:     createTestHandler(t),
		}

		err := registry.Register(cmd)

		assert.NoError(t, err)
		commands := registry.List()
		require.Len(t, commands, 1)
		assert.Equal(t, "test-cmd", commands[0].Name)
	})

	t.Run("fails when command is nil", func(t *testing.T) {
		registry := NewRegistry()

		err := registry.Register(nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "command is nil")
	})

	t.Run("fails when command name is empty", func(t *testing.T) {
		registry := NewRegistry()
		cmd := &Command{
			Name:        "",
			Description: "No name",
			Handler:     createTestHandler(t),
		}

		err := registry.Register(cmd)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("fails when command has no handler", func(t *testing.T) {
		registry := NewRegistry()
		cmd := &Command{
			Name:        "no-handler",
			Description: "No handler",
		}

		err := registry.Register(cmd)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handler is required")
	})

	t.Run("registers duplicate command name", func(t *testing.T) {
		registry := NewRegistry()
		cmd1 := &Command{Name: "duplicate", Description: "First", Handler: createTestHandler(t)}
		cmd2 := &Command{Name: "duplicate", Description: "Second", Handler: createTestHandler(t)}

		err := registry.Register(cmd1)
		require.NoError(t, err)

		// Note: Current implementation allows overwriting, no error
		err = registry.Register(cmd2)
		assert.NoError(t, err)
	})

	t.Run("registers multiple commands", func(t *testing.T) {
		registry := NewRegistry()
		cmd1 := &Command{Name: "cmd1", Description: "First", Handler: createTestHandler(t)}
		cmd2 := &Command{Name: "cmd2", Description: "Second", Handler: createTestHandler(t)}

		err := registry.Register(cmd1)
		require.NoError(t, err)
		err = registry.Register(cmd2)
		require.NoError(t, err)

		commands := registry.List()
		assert.Len(t, commands, 2)
	})
}

func TestRegistry_Get(t *testing.T) {
	createTestHandler := func(t *testing.T) *starlark.Function {
		t.Helper()
		thread := &starlark.Thread{Name: "test"}
		globals, _ := starlark.ExecFile(thread, "test.star", `def handler(ctx): return None`, nil)
		return globals["handler"].(*starlark.Function)
	}

	t.Run("gets existing command", func(t *testing.T) {
		registry := NewRegistry()
		cmd := &Command{Name: "test", Description: "Test command", Handler: createTestHandler(t)}
		registry.Register(cmd)

		retrieved, exists := registry.Get("test")

		assert.True(t, exists)
		assert.NotNil(t, retrieved)
		assert.Equal(t, "test", retrieved.Name)
	})

	t.Run("returns false for non-existent command", func(t *testing.T) {
		registry := NewRegistry()

		_, exists := registry.Get("nonexistent")

		assert.False(t, exists)
	})
}

func TestRegistry_List(t *testing.T) {
	createTestHandler := func(t *testing.T) *starlark.Function {
		t.Helper()
		thread := &starlark.Thread{Name: "test"}
		globals, _ := starlark.ExecFile(thread, "test.star", `def handler(ctx): return None`, nil)
		return globals["handler"].(*starlark.Function)
	}

	t.Run("returns empty list initially", func(t *testing.T) {
		registry := NewRegistry()

		commands := registry.List()

		assert.Len(t, commands, 0)
	})

	t.Run("returns all registered commands", func(t *testing.T) {
		registry := NewRegistry()
		h := createTestHandler(t)
		registry.Register(&Command{Name: "cmd1", Handler: h})
		registry.Register(&Command{Name: "cmd2", Handler: h})
		registry.Register(&Command{Name: "cmd3", Handler: h})

		commands := registry.List()

		assert.Len(t, commands, 3)
	})
}

func TestRegistry_RegisterTool(t *testing.T) {
	// Helper to create a test handler
	createTestHandler := func(t *testing.T) *starlark.Function {
		t.Helper()
		thread := &starlark.Thread{Name: "test"}
		globals, err := starlark.ExecFile(thread, "test.star", `def handler(ctx): return None`, nil)
		require.NoError(t, err)
		handler, ok := globals["handler"].(*starlark.Function)
		require.True(t, ok)
		return handler
	}

	t.Run("registers tool", func(t *testing.T) {
		registry := NewRegistry()
		tool := &Tool{
			Name:        "test-tool",
			Description: "A test tool",
			Handler:     createTestHandler(t),
		}

		err := registry.RegisterTool(tool)

		assert.NoError(t, err)
		tools := registry.ListTools()
		require.Len(t, tools, 1)
		assert.Equal(t, "test-tool", tools[0].Name)
	})

	t.Run("fails when tool is nil", func(t *testing.T) {
		registry := NewRegistry()

		err := registry.RegisterTool(nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool is nil")
	})

	t.Run("fails when tool name is empty", func(t *testing.T) {
		registry := NewRegistry()
		tool := &Tool{
			Name:        "",
			Description: "No name",
			Handler:     createTestHandler(t),
		}

		err := registry.RegisterTool(tool)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("fails when tool has no handler", func(t *testing.T) {
		registry := NewRegistry()
		tool := &Tool{
			Name:        "no-handler",
			Description: "No handler",
		}

		err := registry.RegisterTool(tool)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handler is required")
	})

	t.Run("registers duplicate tool name", func(t *testing.T) {
		registry := NewRegistry()
		tool1 := &Tool{Name: "duplicate", Description: "First", Handler: createTestHandler(t)}
		tool2 := &Tool{Name: "duplicate", Description: "Second", Handler: createTestHandler(t)}

		err := registry.RegisterTool(tool1)
		require.NoError(t, err)

		// Note: Current implementation allows overwriting
		err = registry.RegisterTool(tool2)
		assert.NoError(t, err)
	})

	t.Run("registers multiple tools", func(t *testing.T) {
		registry := NewRegistry()
		tool1 := &Tool{Name: "tool1", Description: "First", Handler: createTestHandler(t)}
		tool2 := &Tool{Name: "tool2", Description: "Second", Handler: createTestHandler(t)}

		err := registry.RegisterTool(tool1)
		require.NoError(t, err)
		err = registry.RegisterTool(tool2)
		require.NoError(t, err)

		tools := registry.ListTools()
		assert.Len(t, tools, 2)
	})
}

func TestRegistry_GetTool(t *testing.T) {
	createTestHandler := func(t *testing.T) *starlark.Function {
		t.Helper()
		thread := &starlark.Thread{Name: "test"}
		globals, _ := starlark.ExecFile(thread, "test.star", `def handler(ctx): return None`, nil)
		return globals["handler"].(*starlark.Function)
	}

	t.Run("gets existing tool", func(t *testing.T) {
		registry := NewRegistry()
		tool := &Tool{Name: "test-tool", Description: "Test tool", Handler: createTestHandler(t)}
		registry.RegisterTool(tool)

		retrieved, exists := registry.GetTool("test-tool")

		assert.True(t, exists)
		assert.NotNil(t, retrieved)
		assert.Equal(t, "test-tool", retrieved.Name)
	})

	t.Run("returns false for non-existent tool", func(t *testing.T) {
		registry := NewRegistry()

		_, exists := registry.GetTool("nonexistent")

		assert.False(t, exists)
	})
}

func TestRegistry_ListTools(t *testing.T) {
	createTestHandler := func(t *testing.T) *starlark.Function {
		t.Helper()
		thread := &starlark.Thread{Name: "test"}
		globals, _ := starlark.ExecFile(thread, "test.star", `def handler(ctx): return None`, nil)
		return globals["handler"].(*starlark.Function)
	}

	t.Run("returns empty list initially", func(t *testing.T) {
		registry := NewRegistry()

		tools := registry.ListTools()

		assert.Len(t, tools, 0)
	})

	t.Run("returns all registered tools", func(t *testing.T) {
		registry := NewRegistry()
		h := createTestHandler(t)
		registry.RegisterTool(&Tool{Name: "tool1", Handler: h})
		registry.RegisterTool(&Tool{Name: "tool2", Handler: h})
		registry.RegisterTool(&Tool{Name: "tool3", Handler: h})

		tools := registry.ListTools()

		assert.Len(t, tools, 3)
	})
}

func TestRegistry_CommandFromTool(t *testing.T) {
	createTestHandler := func(t *testing.T) *starlark.Function {
		t.Helper()
		thread := &starlark.Thread{Name: "test"}
		globals, err := starlark.ExecFile(thread, "test.star", `def handler(ctx): return "result"`, nil)
		require.NoError(t, err)
		handler, ok := globals["handler"].(*starlark.Function)
		require.True(t, ok)
		return handler
	}

	t.Run("creates command from tool with all parameters", func(t *testing.T) {
		registry := NewRegistry()

		tool := &Tool{
			Name:        "test-tool",
			Description: "Test tool",
			Handler:     createTestHandler(t),
			Params: map[string]*Param{
				"name": {
					Type:        "string",
					Description: "Name parameter",
					Required:    true,
				},
				"count": {
					Type:        "int",
					Description: "Count parameter",
					Default:     10,
				},
			},
		}

		cmd, err := registry.CommandFromTool(tool, "")

		assert.NoError(t, err)
		assert.NotNil(t, cmd)
		assert.Equal(t, "test-tool", cmd.Name)
		assert.Equal(t, "Test tool", cmd.Description)
		assert.Same(t, tool.Handler, cmd.Handler)
		assert.Same(t, tool, cmd.Tool)
		assert.Len(t, cmd.Flags, 2)
	})

	t.Run("creates command with overridden name", func(t *testing.T) {
		registry := NewRegistry()

		tool := &Tool{
			Name:        "original-name",
			Description: "Test tool",
			Handler:     createTestHandler(t),
		}

		cmd, err := registry.CommandFromTool(tool, "override-name")

		assert.NoError(t, err)
		assert.Equal(t, "override-name", cmd.Name)
	})

	t.Run("fails when tool is nil", func(t *testing.T) {
		registry := NewRegistry()

		_, err := registry.CommandFromTool(nil, "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool is nil")
	})

	t.Run("fails when tool has no handler", func(t *testing.T) {
		registry := NewRegistry()
		tool := &Tool{
			Name:        "no-handler",
			Description: "Tool without handler",
		}

		_, err := registry.CommandFromTool(tool, "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handler is required")
	})
}

func TestTool_GenerateToolSchema(t *testing.T) {
	t.Run("generates schema for tool with parameters", func(t *testing.T) {
		tool := &Tool{
			Name:        "test-tool",
			Description: "A test tool for testing",
			Params: map[string]*Param{
				"name": {
					Type:        "string",
					Description: "User name",
					Required:    true,
				},
				"age": {
					Type:        "int",
					Description: "User age",
					Default:     18,
				},
			},
		}

		schema := tool.GenerateToolSchema()

		assert.Equal(t, "test-tool", schema.Name)
		assert.Equal(t, "A test tool for testing", schema.Description)
		assert.NotNil(t, schema.Parameters)

		// Check that parameters map contains expected structure
		params, ok := schema.Parameters["properties"]
		assert.True(t, ok)
		assert.NotNil(t, params)
	})

	t.Run("generates schema for tool without parameters", func(t *testing.T) {
		tool := &Tool{
			Name:        "simple-tool",
			Description: "No parameters",
			Params:      map[string]*Param{},
		}

		schema := tool.GenerateToolSchema()

		assert.Equal(t, "simple-tool", schema.Name)
		assert.NotNil(t, schema.Parameters)
	})

	t.Run("includes choices in schema", func(t *testing.T) {
		tool := &Tool{
			Name: "choice-tool",
			Params: map[string]*Param{
				"level": {
					Type:        "string",
					Description: "Log level",
					Choices:     []interface{}{"debug", "info", "warn", "error"},
				},
			},
		}

		schema := tool.GenerateToolSchema()

		props, _ := schema.Parameters["properties"].(map[string]interface{})
		levelSchema, _ := props["level"].(map[string]interface{})
		assert.Equal(t, []interface{}{"debug", "info", "warn", "error"}, levelSchema["enum"])
	})

	t.Run("includes pattern in schema", func(t *testing.T) {
		tool := &Tool{
			Name: "pattern-tool",
			Params: map[string]*Param{
				"ticket": {
					Type:    "string",
					Pattern: "^[A-Z]+-\\d+$",
				},
			},
		}

		schema := tool.GenerateToolSchema()

		props, _ := schema.Parameters["properties"].(map[string]interface{})
		ticketSchema, _ := props["ticket"].(map[string]interface{})
		assert.Equal(t, "^[A-Z]+-\\d+$", ticketSchema["pattern"])
	})

	t.Run("includes min/max for numbers", func(t *testing.T) {
		min := 1.0
		max := 100.0
		tool := &Tool{
			Name: "range-tool",
			Params: map[string]*Param{
				"score": {
					Type: "float",
					Min:  &min,
					Max:  &max,
				},
			},
		}

		schema := tool.GenerateToolSchema()

		props, _ := schema.Parameters["properties"].(map[string]interface{})
		scoreSchema, _ := props["score"].(map[string]interface{})
		assert.Equal(t, 1.0, scoreSchema["minimum"])
		assert.Equal(t, 100.0, scoreSchema["maximum"])
	})

	t.Run("includes minLength/maxLength for strings", func(t *testing.T) {
		minLen := 5
		maxLen := 50
		tool := &Tool{
			Name: "length-tool",
			Params: map[string]*Param{
				"name": {
					Type:   "string",
					MinLen: &minLen,
					MaxLen: &maxLen,
				},
			},
		}

		schema := tool.GenerateToolSchema()

		props, _ := schema.Parameters["properties"].(map[string]interface{})
		nameSchema, _ := props["name"].(map[string]interface{})
		assert.Equal(t, 5, nameSchema["minLength"])
		assert.Equal(t, 50, nameSchema["maxLength"])
	})

	t.Run("maps type names to JSON Schema types", func(t *testing.T) {
		tool := &Tool{
			Name: "type-mapping-tool",
			Params: map[string]*Param{
				"count": {Type: "int"},
				"flag":  {Type: "bool"},
				"score": {Type: "float"},
				"name":  {Type: "string"},
			},
		}

		schema := tool.GenerateToolSchema()

		props, _ := schema.Parameters["properties"].(map[string]interface{})
		countSchema, _ := props["count"].(map[string]interface{})
		flagSchema, _ := props["flag"].(map[string]interface{})
		scoreSchema, _ := props["score"].(map[string]interface{})
		nameSchema, _ := props["name"].(map[string]interface{})

		assert.Equal(t, "integer", countSchema["type"])
		assert.Equal(t, "boolean", flagSchema["type"])
		assert.Equal(t, "number", scoreSchema["type"])
		assert.Equal(t, "string", nameSchema["type"])
	})

	t.Run("includes required parameters", func(t *testing.T) {
		tool := &Tool{
			Name: "required-tool",
			Params: map[string]*Param{
				"required1": {Type: "string", Required: true},
				"required2": {Type: "int", Required: true},
				"optional":  {Type: "string", Required: false},
			},
		}

		schema := tool.GenerateToolSchema()

		required, _ := schema.Parameters["required"].([]string)
		assert.Len(t, required, 2)
		assert.Contains(t, required, "required1")
		assert.Contains(t, required, "required2")
		assert.NotContains(t, required, "optional")
	})

	t.Run("omits required field when no parameters are required", func(t *testing.T) {
		tool := &Tool{
			Name: "all-optional-tool",
			Params: map[string]*Param{
				"opt1": {Type: "string", Required: false},
				"opt2": {Type: "int", Required: false},
			},
		}

		schema := tool.GenerateToolSchema()

		_, hasRequired := schema.Parameters["required"]
		assert.False(t, hasRequired)
	})
}

func TestRegistry_GenerateAllToolSchemas(t *testing.T) {
	createTestHandler := func(t *testing.T) *starlark.Function {
		t.Helper()
		thread := &starlark.Thread{Name: "test"}
		globals, _ := starlark.ExecFile(thread, "test.star", `def handler(ctx): return None`, nil)
		return globals["handler"].(*starlark.Function)
	}

	t.Run("generates schemas for all tools", func(t *testing.T) {
		registry := NewRegistry()

		h := createTestHandler(t)
		tool1 := &Tool{Name: "tool1", Description: "First tool", Handler: h}
		tool2 := &Tool{Name: "tool2", Description: "Second tool", Handler: h}

		registry.RegisterTool(tool1)
		registry.RegisterTool(tool2)

		schemas := registry.GenerateAllToolSchemas()

		assert.Len(t, schemas, 2)
		names := []string{schemas[0].Name, schemas[1].Name}
		assert.Contains(t, names, "tool1")
		assert.Contains(t, names, "tool2")
	})

	t.Run("returns empty list when no tools registered", func(t *testing.T) {
		registry := NewRegistry()

		schemas := registry.GenerateAllToolSchemas()

		assert.Len(t, schemas, 0)
	})
}
