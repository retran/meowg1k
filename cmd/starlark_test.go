package cmd

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		input    any
		name     string
		expected bool
	}{
		{nil, "nil", false},
		{true, "bool_true", true},
		{false, "bool_false", false},
		{starlark.Bool(true), "starlark_bool_true", true},
		{starlark.Bool(false), "starlark_bool_false", false},
		{"true", "string_true", true},
		{"1", "string_1", true},
		{"yes", "string_yes", true},
		{"t", "string_t", true},
		{"Y", "string_y", true}, // Case insensitive
		{"false", "string_false", false},
		{"0", "string_0", false},
		{5, "int_nonzero", true},
		{0, "int_zero", false},
		{int64(10), "int64_nonzero", true},
		{int64(0), "int64_zero", false},
		{1.5, "float64_nonzero", true},
		{0.0, "float64_zero", false},
		{[]int{1}, "unknown", false},
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
		input    any
		name     string
		expected int
	}{
		{nil, "nil", 0},
		{42, "int", 42},
		{int64(100), "int64", 100},
		{3.14, "float64", 3},
		{"123", "string_valid", 123},
		{"abc", "string_invalid", 0},
		{true, "unknown", 0},
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
		input    any
		name     string
		expected float64
	}{
		{nil, "nil", 0.0},
		{3.14, "float64", 3.14},
		{42, "int", 42.0},
		{int64(100), "int64", 100.0},
		{"2.718", "string_valid", 2.718},
		{"abc", "string_invalid", 0.0},
		{true, "unknown", 0.0},
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

func TestIsEmptyStarlarkValue(t *testing.T) {
	tests := []struct {
		input    starlark.Value
		name     string
		expected bool
	}{
		{input: starlark.String(""), name: "empty string", expected: true},
		{input: starlark.String("hello"), name: "non-empty string", expected: false},
		{input: starlark.None, name: "none", expected: true},
		{input: starlark.Bool(true), name: "bool true", expected: false},
		{input: starlark.Bool(false), name: "bool false", expected: false},
		{input: starlark.MakeInt(0), name: "int zero", expected: false},
		{input: starlark.MakeInt(42), name: "int nonzero", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isEmptyStarlarkValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetFlagValue(t *testing.T) {
	const (
		flagTypeBool  = "bool"
		flagTypeInt   = "int"
		flagTypeFloat = "float"
	)

	tests := []struct {
		flagVal  interface{}
		name     string
		flagType string
	}{
		{flagVal: true, name: "bool flag", flagType: flagTypeBool},
		{flagVal: 42, name: "int flag", flagType: flagTypeInt},
		{flagVal: 3.14, name: "float flag", flagType: flagTypeFloat},
		{flagVal: "hello", name: "string flag", flagType: "string"},
		{flagVal: "test", name: "unknown type defaults to string", flagType: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}
			switch tt.flagType {
			case flagTypeBool:
				cmd.Flags().Bool("flag", tt.flagVal.(bool), "")
			case flagTypeInt:
				cmd.Flags().Int("flag", tt.flagVal.(int), "")
			case flagTypeFloat:
				cmd.Flags().Float64("flag", tt.flagVal.(float64), "")
			default:
				cmd.Flags().String("flag", "test", "")
			}

			result := getFlagValue(cmd, "flag", tt.flagType)
			assert.NotNil(t, result)
		})
	}
}

func TestGetFlagValue_NonExistentFlag(t *testing.T) {
	// Test that non-existent flags return zero values without panicking
	cmd := &cobra.Command{Use: "test"}

	assert.NotPanics(t, func() {
		result := getFlagValue(cmd, "nonexistent", "bool")
		assert.Equal(t, starlark.Bool(false), result)
	})

	assert.NotPanics(t, func() {
		result := getFlagValue(cmd, "nonexistent", "int")
		assert.Equal(t, starlark.MakeInt(0), result)
	})

	assert.NotPanics(t, func() {
		result := getFlagValue(cmd, "nonexistent", "float")
		assert.Equal(t, starlark.Float(0.0), result)
	})

	assert.NotPanics(t, func() {
		result := getFlagValue(cmd, "nonexistent", "string")
		assert.Equal(t, starlark.String(""), result)
	})
}

// ---------------------------------------------------------------------------
// resolveParamDef
// ---------------------------------------------------------------------------

func TestResolveParamDef_NoTool(t *testing.T) {
	cmd := &starlarkpkg.Command{
		Name:  "test",
		Flags: map[string]*starlarkpkg.FlagDef{},
		Tool:  nil, // no tool
	}
	result := resolveParamDef(cmd, "myflag")
	assert.Nil(t, result)
}

func TestResolveParamDef_Found(t *testing.T) {
	param := &starlarkpkg.Param{Required: true}
	cmd := &starlarkpkg.Command{
		Name:  "test",
		Flags: map[string]*starlarkpkg.FlagDef{},
		Tool: &starlarkpkg.Tool{
			Params: map[string]*starlarkpkg.Param{
				"myflag": param,
			},
		},
	}
	result := resolveParamDef(cmd, "myflag")
	assert.Equal(t, param, result)
}

func TestResolveParamDef_NotFound(t *testing.T) {
	cmd := &starlarkpkg.Command{
		Name:  "test",
		Flags: map[string]*starlarkpkg.FlagDef{},
		Tool: &starlarkpkg.Tool{
			Params: map[string]*starlarkpkg.Param{},
		},
	}
	result := resolveParamDef(cmd, "nonexistent")
	assert.Nil(t, result)
}

// ---------------------------------------------------------------------------
// applyStdinFallback
// ---------------------------------------------------------------------------

func TestApplyStdinFallback_NonEmpty_NotRequired(t *testing.T) {
	cobraCmd := &cobra.Command{Use: "test"}
	paramDef := &starlarkpkg.Param{Required: false, FromStdin: true}
	sr := &stdinReader{checked: true, content: "stdin content"}

	// Value is already non-empty, no stdin needed
	val := starlark.String("existing")
	result, err := applyStdinFallback(cobraCmd, "flag", paramDef, val, sr)
	assert.NoError(t, err)
	assert.Equal(t, val, result)
}

func TestApplyStdinFallback_EmptyRequired_NoStdin(t *testing.T) {
	cobraCmd := &cobra.Command{Use: "test"}
	cobraCmd.SetOut(nil) // suppress cobra output
	paramDef := &starlarkpkg.Param{Required: true, FromStdin: true}
	sr := &stdinReader{checked: true, content: ""} // stdin is empty

	_, err := applyStdinFallback(cobraCmd, "myflag", paramDef, starlark.None, sr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "myflag")
	assert.Contains(t, err.Error(), "required")
}

func TestApplyStdinFallback_EmptyNotRequired_NoStdin(t *testing.T) {
	cobraCmd := &cobra.Command{Use: "test"}
	paramDef := &starlarkpkg.Param{Required: false, FromStdin: true}
	sr := &stdinReader{checked: true, content: ""}

	result, err := applyStdinFallback(cobraCmd, "flag", paramDef, starlark.None, sr)
	assert.NoError(t, err)
	assert.Equal(t, starlark.None, result)
}

// ---------------------------------------------------------------------------
// checkContextCancelled
// ---------------------------------------------------------------------------

func TestCheckContextCancelled_NilContext(t *testing.T) {
	cobraCmd := &cobra.Command{Use: "test"}
	// cobra.Command without a context set returns nil from Context()
	err := checkContextCancelled(cobraCmd)
	assert.NoError(t, err)
}

func TestCheckContextCancelled_ActiveContext(t *testing.T) {
	cobraCmd := &cobra.Command{Use: "test"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cobraCmd.SetContext(ctx)
	err := checkContextCancelled(cobraCmd)
	assert.NoError(t, err)
}

func TestCheckContextCancelled_CancelledContext(t *testing.T) {
	cobraCmd := &cobra.Command{Use: "test"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	cobraCmd.SetContext(ctx)
	err := checkContextCancelled(cobraCmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

// ---------------------------------------------------------------------------
// logSessionWarning
// ---------------------------------------------------------------------------

func TestLogSessionWarning_NoError(t *testing.T) {
	// logSessionWarning should be a no-op when err is nil — no output, no panic.
	old := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	logSessionWarning("test warning", nil)

	w.Close()
	os.Stderr = old

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	assert.Empty(t, buf.String(), "expected no output when err is nil")
}

func TestLogSessionWarning_WithError(t *testing.T) {
	// logSessionWarning should write to stderr when err is non-nil.
	old := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	logSessionWarning("something failed", assert.AnError)

	w.Close()
	os.Stderr = old

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "something failed")
	assert.Contains(t, buf.String(), assert.AnError.Error())
}

// ---------------------------------------------------------------------------
// createSessionCleanup
// ---------------------------------------------------------------------------

func TestCreateSessionCleanup_NilService(t *testing.T) {
	ctx := context.Background()
	sess, cleanup, err := createSessionCleanup(ctx, nil, "test")
	assert.NoError(t, err)
	assert.Nil(t, sess)
	require.NotNil(t, cleanup)
	// cleanup should be a no-op
	assert.NotPanics(t, func() { cleanup(nil) })
	assert.NotPanics(t, func() { cleanup(assert.AnError) })
}
