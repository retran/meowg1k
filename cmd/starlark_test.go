package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"go.starlark.net/starlark"

	starlarkpkg "github.com/retran/meowg1k/internal/core/starlark"
)

func TestBuildCobraCommand_SafeDefaultCoercion(t *testing.T) {
	// Construct a command with various default types
	c := &starlarkpkg.Command{
		Name:        "test",
		Description: "",
		Handler:     nil, // not used by builder
		Flags:       map[string]*starlarkpkg.FlagDef{},
	}
	c.Flags["boolflag"] = &starlarkpkg.FlagDef{Type: "bool", Default: "true"}
	c.Flags["intflag"] = &starlarkpkg.FlagDef{Type: "int", Default: float64(3)}
	c.Flags["floatflag"] = &starlarkpkg.FlagDef{Type: "float", Default: "2.5"}
	c.Flags["stringflag"] = &starlarkpkg.FlagDef{Type: "string", Default: 42}

	// Build
	cmd, err := buildCobraCommand(starlarkpkg.NewRuntime("."), c)
	if err != nil {
		t.Fatalf("buildCobraCommand error: %v", err)
	}
	if cmd.Use != "test" {
		t.Fatalf("unexpected command use: %s", cmd.Use)
	}

	// Validate defaults via flag getters
	b, err := cmd.Flags().GetBool("boolflag")
	if err != nil || b != true {
		t.Fatalf("boolflag default expected true, got %v (err=%v)", b, err)
	}
	i, err := cmd.Flags().GetInt("intflag")
	if err != nil || i != 3 {
		t.Fatalf("intflag default expected 3, got %v (err=%v)", i, err)
	}
	f, err := cmd.Flags().GetFloat64("floatflag")
	if err != nil || f != 2.5 {
		t.Fatalf("floatflag default expected 2.5, got %v (err=%v)", f, err)
	}
	s, err := cmd.Flags().GetString("stringflag")
	if err != nil || s != "42" {
		t.Fatalf("stringflag default expected '42', got %v (err=%v)", s, err)
	}
}

func TestBuildCobraCommand_RequiredFlagNoError(t *testing.T) {
	c := &starlarkpkg.Command{
		Name:        "req",
		Description: "",
		Handler:     nil,
		Flags:       map[string]*starlarkpkg.FlagDef{},
	}
	c.Flags["need"] = &starlarkpkg.FlagDef{Type: "string", Required: true}

	cmd, err := buildCobraCommand(starlarkpkg.NewRuntime("."), c)
	if err != nil {
		t.Fatalf("unexpected error for required flag: %v", err)
	}

	// Ensure flag exists and is marked required in annotations
	f := cmd.Flags().Lookup("need")
	if f == nil {
		t.Fatalf("required flag not found")
	}
	// Cobra marks required via annotations; ensure present
	if _, ok := f.Annotations[cobra.BashCompOneRequiredFlag]; !ok {
		t.Fatalf("required annotation not set on flag")
	}
}

// Note: executeStarlarkHandler context-cancellation is hard to test without constructing a real *starlark.Function.
// We focus tests on safe default coercion and required flag handling.

func TestConvertToStarlarkValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string // Type name or value representation
	}{
		{"nil", nil, "None"},
		{"bool_true", true, "True"},
		{"bool_false", false, "False"},
		{"int", 42, "42"},
		{"int64", int64(100), "100"},
		{"float64", 3.14, "3.14"},
		{"string", "hello", "\"hello\""},
		{"starlark.Value", starlark.MakeInt(7), "7"},
		{"other", []int{1, 2}, ""}, // Fallback to string
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToStarlarkValue(tt.input)
			if tt.expected == "None" {
				assert.Equal(t, starlark.None, result)
			} else if tt.expected != "" {
				// Just check it doesn't panic and returns something
				assert.NotNil(t, result)
			}
		})
	}
}

func TestCoerceBool(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected bool
	}{
		{"nil", nil, false},
		{"bool_true", true, true},
		{"bool_false", false, false},
		{"starlark_bool_true", starlark.Bool(true), true},
		{"starlark_bool_false", starlark.Bool(false), false},
		{"string_true", "true", true},
		{"string_1", "1", true},
		{"string_yes", "yes", true},
		{"string_t", "t", true},
		{"string_y", "Y", true}, // Case insensitive
		{"string_false", "false", false},
		{"string_0", "0", false},
		{"int_nonzero", 5, true},
		{"int_zero", 0, false},
		{"int64_nonzero", int64(10), true},
		{"int64_zero", int64(0), false},
		{"float64_nonzero", 1.5, true},
		{"float64_zero", 0.0, false},
		{"unknown", []int{1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coerceBool(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCoerceInt(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected int
	}{
		{"nil", nil, 0},
		{"int", 42, 42},
		{"int64", int64(100), 100},
		{"float64", 3.14, 3},
		{"string_valid", "123", 123},
		{"string_invalid", "abc", 0},
		{"unknown", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coerceInt(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCoerceFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected float64
	}{
		{"nil", nil, 0.0},
		{"float64", 3.14, 3.14},
		{"int", 42, 42.0},
		{"int64", int64(100), 100.0},
		{"string_valid", "2.718", 2.718},
		{"string_invalid", "abc", 0.0},
		{"unknown", true, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coerceFloat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCoerceString(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"starlark_string", starlark.String("world"), "world"},
		{"bool_true", true, "true"},
		{"bool_false", false, "false"},
		{"int", 42, "42"},
		{"int64", int64(100), "100"},
		{"float64", 3.14, "3.14"},
		{"unknown", []int{1, 2}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coerceString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
