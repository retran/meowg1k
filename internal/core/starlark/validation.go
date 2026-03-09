// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

const (
	paramTypeString = "string"
	paramTypeBool   = "bool"
	paramTypeInt    = "int"
	paramTypeFloat  = "float"
)

// ParamValidationError represents a validation failure for a single parameter.
type ParamValidationError struct {
	Param  string
	Reason string
}

func (e *ParamValidationError) Error() string {
	return fmt.Sprintf("parameter '%s' is invalid: %s", e.Param, e.Reason)
}

// ValidateToolParams runs static and dynamic validation for all parameters in a tool.
func ValidateToolParams(runtime *Runtime, registry *Registry, tool *Tool, values map[string]starlark.Value) error {
	if tool == nil {
		return nil
	}

	for name, param := range tool.Params {
		value := starlark.Value(starlark.None)
		if values != nil {
			if v, ok := values[name]; ok {
				value = v
			}
		}

		if err := validateParam(runtime, registry, name, param, value); err != nil {
			return err
		}
	}

	return nil
}

func validateParam(runtime *Runtime, registry *Registry, name string, param *Param, value starlark.Value) error {
	normalized := normalizeValue(value)
	provided := isValueProvided(param, normalized)

	if param.Required && !provided {
		return &ParamValidationError{Param: name, Reason: "value is required"}
	}

	if !provided {
		return nil
	}

	converted, err := convertParamValue(param, normalized)
	if err != nil {
		return &ParamValidationError{Param: name, Reason: err.Error()}
	}

	if err := runStaticChecks(name, param, converted); err != nil {
		return err
	}

	return runDynamicValidators(runtime, registry, name, param, normalized)
}

func normalizeValue(value starlark.Value) starlark.Value {
	if value == nil {
		return starlark.None
	}
	return value
}

func isValueProvided(param *Param, value starlark.Value) bool {
	if value == nil {
		return false
	}
	if value == starlark.None {
		return false
	}

	if param.Type == paramTypeString {
		if s, ok := value.(starlark.String); ok {
			return s.GoString() != ""
		}
	}

	return true
}

func convertParamValue(param *Param, value starlark.Value) (any, error) { //nolint:gocognit // complexity inherent in converting all param types from Starlark values
	switch param.Type {
	case paramTypeString:
		if s, ok := value.(starlark.String); ok {
			return s.GoString(), nil
		}
		if value == starlark.None {
			return "", nil
		}
		return nil, fmt.Errorf("expected string, got %s", value.Type())
	case paramTypeBool:
		if b, ok := value.(starlark.Bool); ok {
			return bool(b), nil
		}
		return nil, fmt.Errorf("expected bool, got %s", value.Type())
	case paramTypeInt:
		switch v := value.(type) {
		case starlark.Int:
			i, ok := v.Int64()
			if !ok {
				return nil, fmt.Errorf("integer value out of range")
			}
			return int(i), nil
		default:
			return nil, fmt.Errorf("expected int, got %s", value.Type())
		}
	case paramTypeFloat:
		switch v := value.(type) {
		case starlark.Float:
			return float64(v), nil
		case starlark.Int:
			i, ok := v.Int64()
			if !ok {
				return nil, fmt.Errorf("numeric value out of range")
			}
			return float64(i), nil
		default:
			return nil, fmt.Errorf("expected float, got %s", value.Type())
		}
	default:
		return nil, fmt.Errorf("unsupported parameter type '%s'", param.Type)
	}
}

func runStaticChecks(name string, param *Param, value any) error { //nolint:gocognit,gocyclo // complexity inherent in validating all constraint combinations
	if len(param.Choices) > 0 {
		if !valueInChoices(value, param.Choices) {
			return &ParamValidationError{Param: name, Reason: fmt.Sprintf("value must be one of: %s", strings.Join(formatChoices(param.Choices), ", "))}
		}
	}

	if param.PatternRegex != nil {
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("parameter '%s' pattern validation requires string value", name)
		}
		if !param.PatternRegex.MatchString(str) {
			return &ParamValidationError{Param: name, Reason: fmt.Sprintf("value does not match pattern %s", param.Pattern)}
		}
	}

	if param.MinLen != nil {
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("parameter '%s' min_len requires string value", name)
		}
		if length := utf8.RuneCountInString(str); length < *param.MinLen {
			return &ParamValidationError{Param: name, Reason: fmt.Sprintf("length must be >= %d", *param.MinLen)}
		}
	}

	if param.MaxLen != nil {
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("parameter '%s' max_len requires string value", name)
		}
		if length := utf8.RuneCountInString(str); length > *param.MaxLen {
			return &ParamValidationError{Param: name, Reason: fmt.Sprintf("length must be <= %d", *param.MaxLen)}
		}
	}

	if param.Min != nil {
		number, err := toFloat64(value)
		if err != nil {
			return fmt.Errorf("parameter '%s': %w", name, err)
		}
		if number < *param.Min {
			return &ParamValidationError{Param: name, Reason: fmt.Sprintf("value must be >= %g", *param.Min)}
		}
	}

	if param.Max != nil {
		number, err := toFloat64(value)
		if err != nil {
			return fmt.Errorf("parameter '%s': %w", name, err)
		}
		if number > *param.Max {
			return &ParamValidationError{Param: name, Reason: fmt.Sprintf("value must be <= %g", *param.Max)}
		}
	}

	return nil
}

func runDynamicValidators(runtime *Runtime, registry *Registry, name string, param *Param, value starlark.Value) error {
	if param.ValidatorTool == nil && param.ValidatorFunc == nil {
		return nil
	}

	if runtime == nil {
		return fmt.Errorf("runtime is required for dynamic validation")
	}
	if registry == nil {
		return fmt.Errorf("registry is required for dynamic validation")
	}

	if param.ValidatorTool != nil {
		result, err := runValidatorTool(runtime, registry, param.ValidatorTool, value)
		if err != nil {
			return fmt.Errorf("validator tool '%s' failed: %w", param.ValidatorTool.Name, err)
		}
		if err := interpretValidatorResult(result, param.ValidatorTool.Name); err != nil {
			return &ParamValidationError{Param: name, Reason: err.Error()}
		}
	}

	if param.ValidatorFunc != nil {
		result, err := runValidatorFunction(runtime, registry, param.ValidatorFunc, value)
		if err != nil {
			return fmt.Errorf("validator function failed: %w", err)
		}
		if err := interpretValidatorResult(result, param.ValidatorFunc.Name()); err != nil {
			return &ParamValidationError{Param: name, Reason: err.Error()}
		}
	}

	return nil
}

func runValidatorTool(runtime *Runtime, registry *Registry, tool *Tool, value starlark.Value) (starlark.Value, error) {
	if tool == nil {
		return starlark.None, fmt.Errorf("validator tool is nil")
	}
	if _, ok := tool.Params["value"]; !ok {
		return starlark.None, fmt.Errorf("validator tool '%s' must declare a 'value' parameter", tool.Name)
	}

	flagsMembers := starlark.StringDict{}
	paramsMembers := make(map[string]starlark.Value)

	for paramName, param := range tool.Params {
		var assigned starlark.Value
		switch {
		case paramName == "value":
			assigned = normalizeValue(value)
		case param.Default != nil:
			assigned = convertGoValueToStarlark(param.Default)
		default:
			assigned = zeroValueForType(param.Type)
		}
		flagsMembers[paramName] = assigned
		paramsMembers[paramName] = assigned
	}

	ctx, err := createValidatorContext(runtime, registry, flagsMembers, paramsMembers)
	if err != nil {
		return starlark.None, err
	}

	thread := &starlark.Thread{Name: fmt.Sprintf("validator:%s", tool.Name)}
	thread.Print = func(*starlark.Thread, string) {
		panic("print() is disabled inside validator. Use ctx.output instead")
	}

	result, err := starlark.Call(thread, tool.Handler, starlark.Tuple{ctx}, nil)
	if err != nil {
		return starlark.None, fmt.Errorf("validator call failed: %w", err)
	}
	return result, nil
}

func runValidatorFunction(runtime *Runtime, registry *Registry, fn *starlark.Function, value starlark.Value) (starlark.Value, error) {
	flagsMembers := starlark.StringDict{
		"value": normalizeValue(value),
	}
	paramsMembers := map[string]starlark.Value{
		"value": normalizeValue(value),
	}

	ctx, err := createValidatorContext(runtime, registry, flagsMembers, paramsMembers)
	if err != nil {
		return starlark.None, err
	}

	thread := &starlark.Thread{Name: "validator:function"}
	thread.Print = func(*starlark.Thread, string) {
		panic("print() is disabled inside validator. Use ctx.output instead")
	}

	result, err := starlark.Call(thread, fn, starlark.Tuple{ctx}, nil)
	if err != nil {
		return starlark.None, fmt.Errorf("validator call failed: %w", err)
	}
	return result, nil
}

func createValidatorContext(runtime *Runtime, registry *Registry, flags starlark.StringDict, params map[string]starlark.Value) (*ContextWithParams, error) {
	if runtime == nil {
		return nil, fmt.Errorf("runtime is required for validator execution")
	}
	if registry == nil {
		return nil, fmt.Errorf("registry is required for validator execution")
	}

	flagsStruct := starlarkstruct.FromStringDict(starlarkstruct.Default, flags)
	argsStruct := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{})

	ctxMembers := starlark.StringDict{
		"flags":     flagsStruct,
		"args":      argsStruct,
		"fs":        runtime.CreateFSModuleForCtx(),
		"git":       runtime.CreateGitModuleForCtx(),
		"llm":       runtime.CreateLLMModuleForCtx(nil), // No session during validation
		"shell":     runtime.CreateShellModuleForCtx(),
		"index":     runtime.CreateIndexModuleForCtx(),
		"output":    runtime.CreateOutputModuleForCtx(),
		"json":      NewJSONModule(),
		"yaml":      NewYAMLModule(),
		"xml":       NewXMLModule(),
		"toml":      NewTOMLModule(),
		"csv":       NewCSVModule(),
		"env":       NewEnvModule(),
		"ui":        runtime.CreateUIModuleForCtx(0),
		"path":      NewPathModule(),
		"crypto":    NewCryptoModule(),
		"time":      NewTimeModule(),
		"regexp":    NewRegexpModule(),
		"http":      NewHTTPModule(),
		"template":  NewTemplateModule(runtime.WorkingDir()),
		"stdin":     runtime.CreateStdinModuleForCtx(),
		"workspace": starlark.String(runtime.WorkingDir()),
		"run":       CreateRunFunction(registry, runtime, nil, 0), // No session during validation
	}

	ctxStruct := starlarkstruct.FromStringDict(starlarkstruct.Default, ctxMembers)
	return CreateContextWithParams(ctxStruct, params), nil
}

func zeroValueForType(paramType string) starlark.Value {
	switch paramType {
	case paramTypeBool:
		return starlark.False
	case paramTypeInt:
		return starlark.MakeInt(0)
	case paramTypeFloat:
		return starlark.Float(0.0)
	default:
		return starlark.String("")
	}
}

func interpretValidatorResult(result starlark.Value, label string) error {
	switch v := result.(type) {
	case starlark.NoneType:
		return nil
	case starlark.Bool:
		if bool(v) {
			return nil
		}
		if label != "" {
			return fmt.Errorf("validator '%s' rejected the value", label)
		}
		return fmt.Errorf("validator rejected the value")
	case starlark.String:
		msg := strings.TrimSpace(v.GoString())
		if msg == "" {
			return nil
		}
		return fmt.Errorf("%s", msg)
	default:
		return nil
	}
}

func valueInChoices(value any, choices []any) bool {
	for _, choice := range choices {
		if reflect.DeepEqual(value, choice) {
			return true
		}
	}
	return false
}

func formatChoices(choices []any) []string {
	formatted := make([]string, 0, len(choices))
	for _, choice := range choices {
		formatted = append(formatted, fmt.Sprint(choice))
	}
	return formatted
}

func toFloat64(value any) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	default:
		return 0, fmt.Errorf("value must be numeric, got %T", value)
	}
}

// SanitizeParamValue normalizes user-provided strings before validation/usage.
func SanitizeParamValue(param *Param, value starlark.Value) starlark.Value {
	if param == nil || value == nil {
		return value
	}

	if param.Type != paramTypeString {
		return value
	}

	str, ok := value.(starlark.String)
	if !ok {
		return value
	}

	trimmed := strings.TrimSpace(str.GoString())
	if trimmed == str.GoString() {
		return value
	}

	return starlark.String(trimmed)
}
