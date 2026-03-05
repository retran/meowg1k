// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"
	"regexp"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ParamValue wraps a Param for use in Starlark
type ParamValue struct {
	Param *Param
}

var _ starlark.Value = (*ParamValue)(nil)

func (p *ParamValue) String() string        { return fmt.Sprintf("Param(%s)", p.Param.Type) }
func (p *ParamValue) Type() string          { return "param" }
func (p *ParamValue) Freeze()               {}
func (p *ParamValue) Truth() starlark.Bool  { return starlark.True }
func (p *ParamValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: param") }

// ToolValue wraps a Tool for use in Starlark
type ToolValue struct {
	Tool *Tool
}

var _ starlark.Value = (*ToolValue)(nil)

func (t *ToolValue) String() string        { return fmt.Sprintf("Tool(%s)", t.Tool.Name) }
func (t *ToolValue) Type() string          { return "tool" }
func (t *ToolValue) Freeze()               {}
func (t *ToolValue) Truth() starlark.Bool  { return starlark.True }
func (t *ToolValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: tool") }

// CreateParamFunction creates the meow.param() builtin
func CreateParamFunction() *starlark.Builtin {
	return starlark.NewBuiltin("param", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if args.Len() < 1 {
			return nil, fmt.Errorf("param() requires type as first argument")
		}

		typeVal, ok := args.Index(0).(starlark.String)
		if !ok {
			return nil, fmt.Errorf("param type must be string")
		}

		param := &Param{
			Type: typeVal.GoString(),
		}

		for _, kv := range kwargs {
			if len(kv) != 2 {
				continue
			}
			key, ok := kv[0].(starlark.String)
			if !ok {
				continue
			}

			switch key.GoString() {
			case "default":
				param.Default = convertStarlarkValue(kv[1])
			case "short":
				if s, ok := kv[1].(starlark.String); ok {
					param.Short = s.GoString()
				}
			case "desc":
				if s, ok := kv[1].(starlark.String); ok {
					param.Description = s.GoString()
				}
			case "required":
				if b, ok := kv[1].(starlark.Bool); ok {
					param.Required = bool(b)
				}
			case "from_stdin":
				if b, ok := kv[1].(starlark.Bool); ok {
					param.FromStdin = bool(b)
				}
			case "choices":
				vals, err := convertChoices(kv[1])
				if err != nil {
					return nil, err
				}
				param.Choices = vals
			case "pattern":
				if s, ok := kv[1].(starlark.String); ok {
					re, err := regexp.Compile(s.GoString())
					if err != nil {
						return nil, fmt.Errorf("invalid pattern for param: %w", err)
					}
					param.Pattern = s.GoString()
					param.PatternRegex = re
				}
			case "min":
				val, err := convertNumericConstraint(kv[1])
				if err != nil {
					return nil, fmt.Errorf("min must be int or float: %w", err)
				}
				param.Min = &val
			case "max":
				val, err := convertNumericConstraint(kv[1])
				if err != nil {
					return nil, fmt.Errorf("max must be int or float: %w", err)
				}
				param.Max = &val
			case "min_len":
				val, err := convertIntConstraint(kv[1])
				if err != nil {
					return nil, fmt.Errorf("min_len must be int: %w", err)
				}
				param.MinLen = &val
			case "max_len":
				val, err := convertIntConstraint(kv[1])
				if err != nil {
					return nil, fmt.Errorf("max_len must be int: %w", err)
				}
				param.MaxLen = &val
			case "validator":
				switch v := kv[1].(type) {
				case *ToolValue:
					param.ValidatorTool = v.Tool
				case *starlark.Function:
					param.ValidatorFunc = v
				case starlark.NoneType:
					param.ValidatorTool = nil
					param.ValidatorFunc = nil
				default:
					return nil, fmt.Errorf("validator must be a meow.tool or function")
				}
			}
		}

		return &ParamValue{Param: param}, nil
	})
}

// CreateToolFunction creates the meow.tool() builtin
func CreateToolFunction(registry *Registry) *starlark.Builtin {
	return starlark.NewBuiltin("tool", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		tool := &Tool{
			Params: make(map[string]*Param),
		}

		for _, kv := range kwargs {
			if len(kv) != 2 {
				continue
			}
			key, ok := kv[0].(starlark.String)
			if !ok {
				continue
			}

			switch key.GoString() {
			case "name":
				if s, ok := kv[1].(starlark.String); ok {
					tool.Name = s.GoString()
				}
			case "description":
				if s, ok := kv[1].(starlark.String); ok {
					tool.Description = s.GoString()
				}
			case "handler":
				if h, ok := kv[1].(*starlark.Function); ok {
					tool.Handler = h
				}
			case "params":
				if dict, ok := kv[1].(*starlark.Dict); ok {
					for _, item := range dict.Items() {
						if len(item) != 2 {
							continue
						}
						paramName, ok := item[0].(starlark.String)
						if !ok {
							continue
						}
						paramVal, ok := item[1].(*ParamValue)
						if !ok {
							continue
						}
						tool.Params[paramName.GoString()] = paramVal.Param
					}
				}
			}
		}

		if tool.Name == "" {
			return nil, fmt.Errorf("tool name is required")
		}
		if tool.Handler == nil {
			return nil, fmt.Errorf("tool handler is required")
		}

		if err := registry.RegisterTool(tool); err != nil {
			return nil, err
		}

		return &ToolValue{Tool: tool}, nil
	})
}

// CreateCommandFunction creates the meow.command() builtin for auto-mapping tools to commands
func CreateCommandFunction(registry *Registry) *starlark.Builtin {
	return starlark.NewBuiltin("command", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var toolVal *ToolValue
		var nameOverride string

		if args.Len() < 1 {
			return nil, fmt.Errorf("command() requires a tool as first argument")
		}

		var ok bool
		toolVal, ok = args.Index(0).(*ToolValue)
		if !ok {
			return nil, fmt.Errorf("first argument must be a tool")
		}

		for _, kv := range kwargs {
			if len(kv) != 2 {
				continue
			}
			key, ok := kv[0].(starlark.String)
			if !ok {
				continue
			}
			if key.GoString() == "name" {
				if s, ok := kv[1].(starlark.String); ok {
					nameOverride = s.GoString()
				}
			}
		}

		cmd, err := registry.CommandFromTool(toolVal.Tool, nameOverride)
		if err != nil {
			return nil, err
		}

		if err := registry.Register(cmd); err != nil {
			return nil, err
		}

		return starlark.None, nil
	})
}

// CreateToolsModule creates the enhanced meow module with tool support
func CreateToolsModule(registry *Registry) *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "meow_tools",
		Members: starlark.StringDict{
			"param":   CreateParamFunction(),
			"tool":    CreateToolFunction(registry),
			"command": CreateCommandFunction(registry),
		},
	}
}

func convertChoices(value starlark.Value) ([]any, error) {
	iterable, ok := value.(starlark.Iterable)
	if !ok {
		return nil, fmt.Errorf("choices must be a list or tuple")
	}

	iter := iterable.Iterate()
	defer iter.Done()

	result := make([]any, 0)
	var item starlark.Value
	for iter.Next(&item) {
		result = append(result, convertStarlarkValue(item))
	}

	return result, nil
}

func convertNumericConstraint(value starlark.Value) (float64, error) {
	switch v := value.(type) {
	case starlark.Int:
		i, ok := v.Int64()
		if !ok {
			return 0, fmt.Errorf("numeric constraint out of range")
		}
		return float64(i), nil
	case starlark.Float:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("value must be int or float, got %s", v.Type())
	}
}

func convertIntConstraint(value starlark.Value) (int, error) {
	switch v := value.(type) {
	case starlark.Int:
		i, ok := v.Int64()
		if !ok {
			return 0, fmt.Errorf("integer constraint out of range")
		}
		return int(i), nil
	default:
		return 0, fmt.Errorf("value must be int, got %s", v.Type())
	}
}

// Helper function to convert Starlark values to Go values
func convertStarlarkValue(v starlark.Value) any {
	switch val := v.(type) {
	case starlark.String:
		return val.GoString()
	case starlark.Int:
		if i, ok := val.Int64(); ok {
			return int(i)
		}
		return 0
	case starlark.Float:
		return float64(val)
	case starlark.Bool:
		return bool(val)
	case starlark.NoneType:
		return nil
	default:
		return nil
	}
}
