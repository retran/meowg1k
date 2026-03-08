package starlark

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

func TestValidateToolParamsRegex(t *testing.T) {
	runtime := NewRuntime(t.TempDir())
	tool := &Tool{
		Name: "commit",
		Params: map[string]*Param{
			"ticket": {
				Type:         "string",
				Required:     true,
				Pattern:      "^[A-Z]+-\\d+$",
				PatternRegex: regexp.MustCompile(`^[A-Z]+-\d+$`),
			},
		},
	}

	params := map[string]starlark.Value{
		"ticket": starlark.String("123"),
	}

	err := ValidateToolParams(runtime, runtime.Registry(), tool, params)
	if err == nil {
		t.Fatalf("expected validation error for invalid ticket")
	}

	var validationErr *ParamValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ParamValidationError, got %v", err)
	}

	if !strings.Contains(validationErr.Reason, "pattern") {
		t.Fatalf("unexpected error message: %s", validationErr.Reason)
	}
}

func TestValidateToolParamsChoices(t *testing.T) {
	runtime := NewRuntime(t.TempDir())
	tool := &Tool{
		Name: "commit",
		Params: map[string]*Param{
			"type": {
				Type:    "string",
				Choices: []any{"feat", "fix"},
			},
		},
	}

	params := map[string]starlark.Value{
		"type": starlark.String("bugfix"),
	}

	err := ValidateToolParams(runtime, runtime.Registry(), tool, params)
	if err == nil {
		t.Fatalf("expected validation error for invalid choice")
	}
}

func TestValidateToolParamsRange(t *testing.T) {
	runtime := NewRuntime(t.TempDir())
	minVal := 1.0
	maxVal := 5.0
	tool := &Tool{
		Name: "search",
		Params: map[string]*Param{
			"limit": {
				Type: "int",
				Min:  &minVal,
				Max:  &maxVal,
			},
		},
	}

	params := map[string]starlark.Value{
		"limit": starlark.MakeInt(10),
	}

	err := ValidateToolParams(runtime, runtime.Registry(), tool, params)
	if err == nil {
		t.Fatalf("expected validation error for range constraint")
	}
}

func TestValidateToolParamsDynamicValidatorTool(t *testing.T) {
	runtime := NewRuntime(t.TempDir())
	validatorFn := mustLoadFunction(t, `
def validator(ctx):
    if not ctx.value.startswith("PROJ-"):
        return "Ticket must belong to PROJ"
    return True
`, "validator")

	validatorTool := &Tool{
		Name: "jira_validator",
		Params: map[string]*Param{
			"value": {Type: "string", Required: true},
		},
		Handler: validatorFn,
	}

	tool := &Tool{
		Name: "commit",
		Params: map[string]*Param{
			"ticket": {
				Type:          "string",
				Required:      true,
				ValidatorTool: validatorTool,
			},
		},
	}

	params := map[string]starlark.Value{
		"ticket": starlark.String("BUG-1"),
	}

	err := ValidateToolParams(runtime, runtime.Registry(), tool, params)
	if err == nil {
		t.Fatalf("expected validation error from validator tool")
	}

	var validationErr *ParamValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ParamValidationError, got %v", err)
	}

	if !strings.Contains(validationErr.Reason, "PROJ") {
		t.Fatalf("unexpected validator message: %s", validationErr.Reason)
	}
}

func TestValidateToolParamsDynamicValidatorFunction(t *testing.T) {
	runtime := NewRuntime(t.TempDir())
	validatorFn := mustLoadFunction(t, `
def validate(ctx):
    if len(ctx.value) < 3:
        return "value too short"
    return True
`, "validate")

	tool := &Tool{
		Name: "write",
		Params: map[string]*Param{
			"prompt": {
				Type:          "string",
				Required:      true,
				ValidatorFunc: validatorFn,
			},
		},
	}

	params := map[string]starlark.Value{
		"prompt": starlark.String("hi"),
	}

	err := ValidateToolParams(runtime, runtime.Registry(), tool, params)
	if err == nil {
		t.Fatalf("expected validation error from validator function")
	}
}

func TestSanitizeParamValueTrimsWhitespace(t *testing.T) {
	param := &Param{Type: "string"}
	value := starlark.String("  hello  ")
	sanitized := SanitizeParamValue(param, value)
	got := sanitized.(starlark.String).GoString()
	if got != "hello" {
		t.Fatalf("expected trimmed value 'hello', got %q", got)
	}
}

func TestValidateToolParamsRejectsWhitespaceOnlyInput(t *testing.T) {
	runtime := NewRuntime(t.TempDir())
	tool := &Tool{
		Name: "search",
		Params: map[string]*Param{
			"query": {Type: "string", Required: true},
		},
	}

	raw := starlark.String("   ")
	params := map[string]starlark.Value{
		"query": SanitizeParamValue(tool.Params["query"], raw),
	}

	err := ValidateToolParams(runtime, runtime.Registry(), tool, params)
	if err == nil {
		t.Fatalf("expected validation error for whitespace-only input")
	}
}

func mustLoadFunction(t *testing.T, src, name string) *starlark.Function {
	t.Helper()
	thread := &starlark.Thread{Name: "validator-test"}
	globals, err := starlark.ExecFileOptions(&syntax.FileOptions{}, thread, name+".star", src, nil)
	if err != nil {
		t.Fatalf("failed to load function: %v", err)
	}

	fn, ok := globals[name].(*starlark.Function)
	if !ok {
		t.Fatalf("symbol %s is not a function", name)
	}
	return fn
}

// TestParamValidationError_Error tests the Error method.
func TestParamValidationError_Error(t *testing.T) {
	err := &ParamValidationError{
		Param:  "max_results",
		Reason: "value must be between 1 and 100",
	}

	msg := err.Error()
	assert.Contains(t, msg, "max_results")
	assert.Contains(t, msg, "value must be between 1 and 100")
	assert.Contains(t, msg, "invalid")
}

// TestValidateToolParams_EmptyTool tests validation with nil tool.
func TestValidateToolParams_EmptyTool(t *testing.T) {
	runtime := NewRuntime(t.TempDir())

	// nil tool should not error
	err := ValidateToolParams(runtime, runtime.Registry(), nil, nil)
	assert.NoError(t, err)
}

// TestValidateToolParams_MinMaxLength tests min/max length constraints.
func TestValidateToolParams_MinMaxLength(t *testing.T) {
	runtime := NewRuntime(t.TempDir())

	t.Run("validates min length", func(t *testing.T) {
		minLen := 5
		tool := &Tool{
			Name: "search",
			Params: map[string]*Param{
				"query": {
					Type:   "string",
					MinLen: &minLen,
				},
			},
		}

		params := map[string]starlark.Value{
			"query": starlark.String("hi"), // Too short
		}

		err := ValidateToolParams(runtime, runtime.Registry(), tool, params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "length must be >= 5")
	})

	t.Run("validates max length", func(t *testing.T) {
		maxLen := 10
		tool := &Tool{
			Name: "search",
			Params: map[string]*Param{
				"query": {
					Type:   "string",
					MaxLen: &maxLen,
				},
			},
		}

		params := map[string]starlark.Value{
			"query": starlark.String("this is a very long query string"), // Too long
		}

		err := ValidateToolParams(runtime, runtime.Registry(), tool, params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "length must be <= 10")
	})

	t.Run("accepts valid length", func(t *testing.T) {
		minLen := 3
		maxLen := 10
		tool := &Tool{
			Name: "search",
			Params: map[string]*Param{
				"query": {
					Type:   "string",
					MinLen: &minLen,
					MaxLen: &maxLen,
				},
			},
		}

		params := map[string]starlark.Value{
			"query": starlark.String("hello"), // Just right
		}

		err := ValidateToolParams(runtime, runtime.Registry(), tool, params)
		assert.NoError(t, err)
	})
}

// TestValidateToolParams_TypeConversion tests type conversion errors.
func TestValidateToolParams_TypeConversion(t *testing.T) {
	runtime := NewRuntime(t.TempDir())

	t.Run("rejects wrong type for bool", func(t *testing.T) {
		tool := &Tool{
			Name: "toggle",
			Params: map[string]*Param{
				"enabled": {Type: "bool", Required: true},
			},
		}

		params := map[string]starlark.Value{
			"enabled": starlark.String("yes"), // Wrong type
		}

		err := ValidateToolParams(runtime, runtime.Registry(), tool, params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected bool")
	})

	t.Run("rejects wrong type for int", func(t *testing.T) {
		tool := &Tool{
			Name: "count",
			Params: map[string]*Param{
				"limit": {Type: "int", Required: true},
			},
		}

		params := map[string]starlark.Value{
			"limit": starlark.String("ten"), // Wrong type
		}

		err := ValidateToolParams(runtime, runtime.Registry(), tool, params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected int")
	})

	t.Run("accepts int as float", func(t *testing.T) {
		tool := &Tool{
			Name: "calculate",
			Params: map[string]*Param{
				"factor": {Type: "float", Required: true},
			},
		}

		params := map[string]starlark.Value{
			"factor": starlark.MakeInt(5), // Int should convert to float
		}

		err := ValidateToolParams(runtime, runtime.Registry(), tool, params)
		assert.NoError(t, err)
	})
}

// TestValidateToolParams_OptionalParameters tests optional parameter handling.
func TestValidateToolParams_OptionalParameters(t *testing.T) {
	runtime := NewRuntime(t.TempDir())

	tool := &Tool{
		Name: "search",
		Params: map[string]*Param{
			"query":   {Type: "string", Required: true},
			"limit":   {Type: "int", Required: false},
			"verbose": {Type: "bool", Required: false},
		},
	}

	t.Run("accepts when optional params omitted", func(t *testing.T) {
		params := map[string]starlark.Value{
			"query": starlark.String("test"),
			// limit and verbose omitted
		}

		err := ValidateToolParams(runtime, runtime.Registry(), tool, params)
		assert.NoError(t, err)
	})

	t.Run("accepts when optional params provided", func(t *testing.T) {
		params := map[string]starlark.Value{
			"query":   starlark.String("test"),
			"limit":   starlark.MakeInt(10),
			"verbose": starlark.Bool(true),
		}

		err := ValidateToolParams(runtime, runtime.Registry(), tool, params)
		assert.NoError(t, err)
	})
}

// TestZeroValueForType tests the zeroValueForType helper function.
func TestZeroValueForType(t *testing.T) {
	tests := []struct {
		expected  starlark.Value
		name      string
		paramType string
	}{
		{
			name:      "bool type returns False",
			paramType: "bool",
			expected:  starlark.False,
		},
		{
			name:      "int type returns 0",
			paramType: "int",
			expected:  starlark.MakeInt(0),
		},
		{
			name:      "float type returns 0.0",
			paramType: "float",
			expected:  starlark.Float(0.0),
		},
		{
			name:      "string type returns empty string",
			paramType: "string",
			expected:  starlark.String(""),
		},
		{
			name:      "unknown type returns empty string",
			paramType: "custom",
			expected:  starlark.String(""),
		},
		{
			name:      "empty type returns empty string",
			paramType: "",
			expected:  starlark.String(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := zeroValueForType(tt.paramType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================================
// Phase 4 Tests: Comprehensive validation coverage for 75% target
// ============================================================================

// TestConvertParamValue_EdgeCases tests all type conversion branches.
func TestConvertParamValue_EdgeCases(t *testing.T) {
	t.Run("string with None", func(t *testing.T) {
		param := &Param{Type: "string"}
		result, err := convertParamValue(param, starlark.None)
		assert.NoError(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("string with wrong type", func(t *testing.T) {
		param := &Param{Type: "string"}
		result, err := convertParamValue(param, starlark.MakeInt(123))
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "expected string")
	})

	t.Run("int out of range", func(t *testing.T) {
		param := &Param{Type: "int"}
		// Create a very large starlark.Int that won't fit in int64
		bigInt := starlark.MakeUint64(^uint64(0)) // Max uint64
		result, err := convertParamValue(param, bigInt)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "out of range")
	})

	t.Run("float with wrong type", func(t *testing.T) {
		param := &Param{Type: "float"}
		result, err := convertParamValue(param, starlark.String("not a float"))
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "expected float")
	})

	t.Run("float from int out of range", func(t *testing.T) {
		param := &Param{Type: "float"}
		bigInt := starlark.MakeUint64(^uint64(0))
		result, err := convertParamValue(param, bigInt)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "out of range")
	})

	t.Run("unsupported parameter type", func(t *testing.T) {
		param := &Param{Type: "custom_type"}
		result, err := convertParamValue(param, starlark.String("value"))
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "unsupported parameter type")
	})
}

// TestInterpretValidatorResult_AllBranches tests all result interpretation logic.
func TestInterpretValidatorResult_AllBranches(t *testing.T) {
	t.Run("None means success", func(t *testing.T) {
		err := interpretValidatorResult(starlark.None, "test_validator")
		assert.NoError(t, err)
	})

	t.Run("True means success", func(t *testing.T) {
		err := interpretValidatorResult(starlark.Bool(true), "test_validator")
		assert.NoError(t, err)
	})

	t.Run("False with label", func(t *testing.T) {
		err := interpretValidatorResult(starlark.Bool(false), "test_validator")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "test_validator")
		assert.Contains(t, err.Error(), "rejected the value")
	})

	t.Run("False without label", func(t *testing.T) {
		err := interpretValidatorResult(starlark.Bool(false), "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validator rejected the value")
		assert.NotContains(t, err.Error(), "''") // Should not include empty label
	})

	t.Run("non-empty string message", func(t *testing.T) {
		err := interpretValidatorResult(starlark.String("Custom error message"), "validator")
		assert.Error(t, err)
		assert.Equal(t, "Custom error message", err.Error())
	})

	t.Run("empty string means success", func(t *testing.T) {
		err := interpretValidatorResult(starlark.String(""), "validator")
		assert.NoError(t, err)
	})

	t.Run("whitespace-only string means success", func(t *testing.T) {
		err := interpretValidatorResult(starlark.String("   \n\t  "), "validator")
		assert.NoError(t, err)
	})

	t.Run("other types mean success", func(t *testing.T) {
		// Lists, dicts, etc. are treated as success
		err := interpretValidatorResult(starlark.NewList([]starlark.Value{}), "validator")
		assert.NoError(t, err)

		err = interpretValidatorResult(starlark.MakeInt(42), "validator")
		assert.NoError(t, err)
	})
}

// TestRunDynamicValidators_ErrorPaths tests error handling in dynamic validation.
func TestRunDynamicValidators_ErrorPaths(t *testing.T) {
	t.Run("no validators returns nil", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		param := &Param{Type: "string"}
		err := runDynamicValidators(runtime, runtime.Registry(), "test", param, starlark.String("value"))
		assert.NoError(t, err)
	})

	t.Run("nil runtime with validator tool", func(t *testing.T) {
		registry := NewRegistry()
		validatorTool := &Tool{
			Name:   "validator",
			Params: map[string]*Param{"value": {Type: "string"}},
			Handler: mustLoadFunction(t, `
def handler(ctx):
    return True
`, "handler"),
		}
		param := &Param{Type: "string", ValidatorTool: validatorTool}

		err := runDynamicValidators(nil, registry, "test", param, starlark.String("value"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "runtime is required")
	})

	t.Run("nil registry with validator tool", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		validatorTool := &Tool{
			Name:   "validator",
			Params: map[string]*Param{"value": {Type: "string"}},
			Handler: mustLoadFunction(t, `
def handler(ctx):
    return True
`, "handler"),
		}
		param := &Param{Type: "string", ValidatorTool: validatorTool}

		err := runDynamicValidators(runtime, nil, "test", param, starlark.String("value"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "registry is required")
	})

	t.Run("validator tool returns error", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		validatorFn := mustLoadFunction(t, `
def handler(ctx):
    fail("Validator failed")
`, "handler")

		validatorTool := &Tool{
			Name:    "failing_validator",
			Params:  map[string]*Param{"value": {Type: "string"}},
			Handler: validatorFn,
		}

		param := &Param{Type: "string", ValidatorTool: validatorTool}
		err := runDynamicValidators(runtime, runtime.Registry(), "test", param, starlark.String("value"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validator tool 'failing_validator' failed")
	})

	t.Run("validator tool returns false", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		validatorFn := mustLoadFunction(t, `
def handler(ctx):
    return False
`, "handler")

		validatorTool := &Tool{
			Name:    "rejecting_validator",
			Params:  map[string]*Param{"value": {Type: "string"}},
			Handler: validatorFn,
		}

		param := &Param{Type: "string", ValidatorTool: validatorTool}
		err := runDynamicValidators(runtime, runtime.Registry(), "test", param, starlark.String("value"))
		assert.Error(t, err)
		var validationErr *ParamValidationError
		assert.ErrorAs(t, err, &validationErr)
		assert.Contains(t, validationErr.Reason, "rejecting_validator")
	})

	t.Run("validator function returns error", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		validatorFn := mustLoadFunction(t, `
def handler(ctx):
    fail("Function validator failed")
`, "handler")

		param := &Param{Type: "string", ValidatorFunc: validatorFn}
		err := runDynamicValidators(runtime, runtime.Registry(), "test", param, starlark.String("value"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validator function failed")
	})

	t.Run("validator function returns rejection message", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		validatorFn := mustLoadFunction(t, `
def handler(ctx):
    return "Value is not acceptable"
`, "handler")

		param := &Param{Type: "string", ValidatorFunc: validatorFn}
		err := runDynamicValidators(runtime, runtime.Registry(), "test", param, starlark.String("value"))
		assert.Error(t, err)
		var validationErr *ParamValidationError
		assert.ErrorAs(t, err, &validationErr)
		assert.Contains(t, validationErr.Reason, "Value is not acceptable")
	})
}

// TestRunValidatorTool_EdgeCases tests validator tool execution edge cases.
func TestRunValidatorTool_EdgeCases(t *testing.T) {
	runtime := NewRuntime(t.TempDir())

	t.Run("nil tool", func(t *testing.T) {
		result, err := runValidatorTool(runtime, runtime.Registry(), nil, starlark.String("value"))
		assert.Error(t, err)
		assert.Equal(t, starlark.None, result)
		assert.Contains(t, err.Error(), "validator tool is nil")
	})

	t.Run("tool missing value parameter", func(t *testing.T) {
		tool := &Tool{
			Name: "bad_validator",
			Params: map[string]*Param{
				"other": {Type: "string"},
			},
			Handler: mustLoadFunction(t, `def h(ctx): return True`, "h"),
		}

		result, err := runValidatorTool(runtime, runtime.Registry(), tool, starlark.String("test"))
		assert.Error(t, err)
		assert.Equal(t, starlark.None, result)
		assert.Contains(t, err.Error(), "must declare a 'value' parameter")
	})

	t.Run("tool with additional parameters uses defaults", func(t *testing.T) {
		defaultVal := "default_value"
		validatorFn := mustLoadFunction(t, `
def handler(ctx):
    # Should receive default value for 'mode'
    if ctx.flags.mode == "default_value":
        return True
    return "mode was not set to default"
`, "handler")

		tool := &Tool{
			Name: "multi_param_validator",
			Params: map[string]*Param{
				"value": {Type: "string"},
				"mode":  {Type: "string", Default: defaultVal},
			},
			Handler: validatorFn,
		}

		result, err := runValidatorTool(runtime, runtime.Registry(), tool, starlark.String("test_value"))
		assert.NoError(t, err)
		assert.Equal(t, starlark.Bool(true), result)
	})

	t.Run("tool with parameter without default uses zero value", func(t *testing.T) {
		validatorFn := mustLoadFunction(t, `
def handler(ctx):
    # Should receive zero value for optional param
    if ctx.flags.count == 0:
        return True
    return "count was not zero"
`, "handler")

		tool := &Tool{
			Name: "zero_default_validator",
			Params: map[string]*Param{
				"value": {Type: "string"},
				"count": {Type: "int"}, // No default, should be zero
			},
			Handler: validatorFn,
		}

		result, err := runValidatorTool(runtime, runtime.Registry(), tool, starlark.String("test"))
		assert.NoError(t, err)
		assert.Equal(t, starlark.Bool(true), result)
	})

	t.Run("print is disabled in validator", func(t *testing.T) {
		validatorFn := mustLoadFunction(t, `
def handler(ctx):
    print("This should panic")
    return True
`, "handler")

		tool := &Tool{
			Name: "print_validator",
			Params: map[string]*Param{
				"value": {Type: "string"},
			},
			Handler: validatorFn,
		}

		// Should panic when print is called
		assert.Panics(t, func() {
			_, _ = runValidatorTool(runtime, runtime.Registry(), tool, starlark.String("test"))
		})
	})
}

// TestRunValidatorFunction_EdgeCases tests validator function execution.
func TestRunValidatorFunction_EdgeCases(t *testing.T) {
	runtime := NewRuntime(t.TempDir())

	t.Run("print is disabled in validator function", func(t *testing.T) {
		validatorFn := mustLoadFunction(t, `
def handler(ctx):
    print("This should panic")
    return True
`, "handler")

		assert.Panics(t, func() {
			_, _ = runValidatorFunction(runtime, runtime.Registry(), validatorFn, starlark.String("test"))
		})
	})

	t.Run("validator function receives value", func(t *testing.T) {
		validatorFn := mustLoadFunction(t, `
def handler(ctx):
    if ctx.flags.value == "expected":
        return True
    return "Got unexpected value: " + ctx.flags.value
`, "handler")

		result, err := runValidatorFunction(runtime, runtime.Registry(), validatorFn, starlark.String("expected"))
		assert.NoError(t, err)
		assert.Equal(t, starlark.Bool(true), result)
	})
}

// TestCreateValidatorContext_EdgeCases tests context creation for validators.
func TestCreateValidatorContext_EdgeCases(t *testing.T) {
	t.Run("nil runtime", func(t *testing.T) {
		flags := starlark.StringDict{"value": starlark.String("test")}
		params := map[string]starlark.Value{"value": starlark.String("test")}

		ctx, err := createValidatorContext(nil, NewRegistry(), flags, params)
		assert.Error(t, err)
		assert.Nil(t, ctx)
		assert.Contains(t, err.Error(), "runtime is required")
	})

	t.Run("nil registry", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		flags := starlark.StringDict{"value": starlark.String("test")}
		params := map[string]starlark.Value{"value": starlark.String("test")}

		ctx, err := createValidatorContext(runtime, nil, flags, params)
		assert.Error(t, err)
		assert.Nil(t, ctx)
		assert.Contains(t, err.Error(), "registry is required")
	})

	t.Run("creates context with all modules", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		flags := starlark.StringDict{"value": starlark.String("test")}
		params := map[string]starlark.Value{"value": starlark.String("test")}

		ctx, err := createValidatorContext(runtime, runtime.Registry(), flags, params)
		assert.NoError(t, err)
		assert.NotNil(t, ctx)

		// Verify all modules are present
		modules := []string{
			"flags", "args", "fs", "git", "llm", "shell", "index",
			"output", "json", "env", "ui", "path", "crypto", "time",
			"regexp", "http", "template", "stdin", "workspace", "run",
		}

		for _, mod := range modules {
			attr, err := ctx.Attr(mod)
			assert.NoError(t, err, "module %s should be present", mod)
			assert.NotNil(t, attr, "module %s should not be nil", mod)
		}
	})
}

// TestNormalizeValue tests value normalization.
func TestNormalizeValue(t *testing.T) {
	t.Run("nil becomes None", func(t *testing.T) {
		result := normalizeValue(nil)
		assert.Equal(t, starlark.None, result)
	})

	t.Run("non-nil value unchanged", func(t *testing.T) {
		value := starlark.String("test")
		result := normalizeValue(value)
		assert.Equal(t, value, result)
	})
}

// TestIsValueProvided tests value presence detection.
func TestIsValueProvided(t *testing.T) {
	t.Run("nil is not provided", func(t *testing.T) {
		param := &Param{Type: "string"}
		assert.False(t, isValueProvided(param, nil))
	})

	t.Run("None is not provided", func(t *testing.T) {
		param := &Param{Type: "string"}
		assert.False(t, isValueProvided(param, starlark.None))
	})

	t.Run("empty string is not provided for string type", func(t *testing.T) {
		param := &Param{Type: "string"}
		assert.False(t, isValueProvided(param, starlark.String("")))
	})

	t.Run("non-empty string is provided", func(t *testing.T) {
		param := &Param{Type: "string"}
		assert.True(t, isValueProvided(param, starlark.String("value")))
	})

	t.Run("zero int is provided", func(t *testing.T) {
		param := &Param{Type: "int"}
		assert.True(t, isValueProvided(param, starlark.MakeInt(0)))
	})

	t.Run("false bool is provided", func(t *testing.T) {
		param := &Param{Type: "bool"}
		assert.True(t, isValueProvided(param, starlark.Bool(false)))
	})
}

// TestSanitizeParamValue_EdgeCases tests sanitization edge cases.
func TestSanitizeParamValue_EdgeCases(t *testing.T) {
	t.Run("nil param returns value unchanged", func(t *testing.T) {
		value := starlark.String("  test  ")
		result := SanitizeParamValue(nil, value)
		assert.Equal(t, value, result)
	})

	t.Run("nil value returns nil", func(t *testing.T) {
		param := &Param{Type: "string"}
		result := SanitizeParamValue(param, nil)
		assert.Nil(t, result)
	})

	t.Run("non-string type unchanged", func(t *testing.T) {
		param := &Param{Type: "int"}
		value := starlark.MakeInt(42)
		result := SanitizeParamValue(param, value)
		assert.Equal(t, value, result)
	})

	t.Run("non-string value unchanged", func(t *testing.T) {
		param := &Param{Type: "string"}
		value := starlark.MakeInt(123) // Wrong type
		result := SanitizeParamValue(param, value)
		assert.Equal(t, value, result)
	})

	t.Run("already trimmed string unchanged", func(t *testing.T) {
		param := &Param{Type: "string"}
		value := starlark.String("test")
		result := SanitizeParamValue(param, value)
		assert.Equal(t, value, result) // Should be same object
	})

	t.Run("whitespace trimmed from string", func(t *testing.T) {
		param := &Param{Type: "string"}
		value := starlark.String("  \n\ttest\t\n  ")
		result := SanitizeParamValue(param, value)
		assert.Equal(t, starlark.String("test"), result)
	})
}

// TestToFloat64_EdgeCases tests numeric conversion.
func TestToFloat64_EdgeCases(t *testing.T) {
	t.Run("int converts", func(t *testing.T) {
		result, err := toFloat64(int(42))
		assert.NoError(t, err)
		assert.Equal(t, float64(42), result)
	})

	t.Run("int64 converts", func(t *testing.T) {
		result, err := toFloat64(int64(42))
		assert.NoError(t, err)
		assert.Equal(t, float64(42), result)
	})

	t.Run("float64 unchanged", func(t *testing.T) {
		result, err := toFloat64(float64(3.14))
		assert.NoError(t, err)
		assert.Equal(t, float64(3.14), result)
	})

	t.Run("string returns error", func(t *testing.T) {
		result, err := toFloat64("not a number")
		assert.Error(t, err)
		assert.Equal(t, float64(0), result)
		assert.Contains(t, err.Error(), "value must be numeric")
	})

	t.Run("bool returns error", func(t *testing.T) {
		result, err := toFloat64(true)
		assert.Error(t, err)
		assert.Equal(t, float64(0), result)
		assert.Contains(t, err.Error(), "got bool")
	})

	t.Run("slice returns error", func(t *testing.T) {
		result, err := toFloat64([]int{1, 2, 3})
		assert.Error(t, err)
		assert.Equal(t, float64(0), result)
		assert.Contains(t, err.Error(), "got []int")
	})
}

// TestValueInChoices tests choice validation.
func TestValueInChoices(t *testing.T) {
	t.Run("value in choices", func(t *testing.T) {
		choices := []any{"foo", "bar", "baz"}
		assert.True(t, valueInChoices("bar", choices))
	})

	t.Run("value not in choices", func(t *testing.T) {
		choices := []any{"foo", "bar", "baz"}
		assert.False(t, valueInChoices("qux", choices))
	})

	t.Run("empty choices", func(t *testing.T) {
		choices := []any{}
		assert.False(t, valueInChoices("anything", choices))
	})

	t.Run("numeric choices", func(t *testing.T) {
		choices := []any{1, 2, 3}
		assert.True(t, valueInChoices(2, choices))
		assert.False(t, valueInChoices(4, choices))
	})
}

// TestRunStaticChecks_TypeMismatches tests error handling for type mismatches.
func TestRunStaticChecks_TypeMismatches(t *testing.T) {
	t.Run("pattern on non-string", func(t *testing.T) {
		param := &Param{
			Type:         "int",
			PatternRegex: regexp.MustCompile(`\d+`),
		}
		err := runStaticChecks("test", param, 123) // int, not string
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "pattern validation requires string value")
	})

	t.Run("minLen on non-string", func(t *testing.T) {
		minLen := 5
		param := &Param{
			Type:   "int",
			MinLen: &minLen,
		}
		err := runStaticChecks("test", param, 123)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "min_len requires string value")
	})

	t.Run("maxLen on non-string", func(t *testing.T) {
		maxLen := 10
		param := &Param{
			Type:   "int",
			MaxLen: &maxLen,
		}
		err := runStaticChecks("test", param, 123)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max_len requires string value")
	})

	t.Run("min on non-numeric", func(t *testing.T) {
		minVal := 1.0
		param := &Param{
			Type: "string",
			Min:  &minVal,
		}
		err := runStaticChecks("test", param, "not a number")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "value must be numeric")
	})

	t.Run("max on non-numeric", func(t *testing.T) {
		maxVal := 10.0
		param := &Param{
			Type: "string",
			Max:  &maxVal,
		}
		err := runStaticChecks("test", param, "not a number")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "value must be numeric")
	})
}
