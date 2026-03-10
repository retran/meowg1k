// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"regexp"
	"strings"

	"fmt"

	"go.starlark.net/starlark"
)

// Param represents a tool parameter definition.
type Param struct {
	Default       any
	PatternRegex  *regexp.Regexp
	ValidatorFunc *starlark.Function
	ValidatorTool *Tool
	MaxLen        *int
	MinLen        *int
	Max           *float64
	Min           *float64
	Description   string
	Pattern       string
	Type          string
	Short         string
	Choices       []any
	FromStdin     bool
	Required      bool
}

// Tool represents a reusable tool that can be called from CLI or other tools.
type Tool struct {
	Params      map[string]*Param
	Handler     *starlark.Function
	Name        string
	Description string
}

// Command represents a registered CLI command.
// Commands are now generated from Tools.
type Command struct {
	Handler         *starlark.Function
	Flags           map[string]*FlagDef
	Args            map[string]*ArgDef
	Tool            *Tool
	Name            string
	Description     string
	LongDescription string
}

// FlagDef defines a command-line flag.
type FlagDef struct {
	Default     any
	Short       string
	Type        string
	Description string
	// ParamKey is the original snake_case param name used to look up values in
	// the handler context (e.g. "custom_style"). FlagName (the map key) is the
	// kebab-case CLI flag (e.g. "custom-style").
	ParamKey  string
	Required  bool
	FromStdin bool
}

// ArgDef defines a positional argument.
type ArgDef struct {
	Default     any
	Description string
	Index       int
}

// Registry stores registered commands and tools.
type Registry struct {
	commands map[string]*Command
	tools    map[string]*Tool
}

// NewRegistry creates a new command registry.
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]*Command),
		tools:    make(map[string]*Tool),
	}
}

// Register adds a command to the registry.
func (r *Registry) Register(c *Command) error {
	if c == nil {
		return fmt.Errorf("command is nil")
	}

	if c.Name == "" {
		return fmt.Errorf("command name is required")
	}

	if c.Handler == nil {
		return fmt.Errorf("command handler is required")
	}

	r.commands[c.Name] = c
	return nil
}

// List returns all registered commands.
func (r *Registry) List() []*Command {
	result := make([]*Command, 0, len(r.commands))
	for _, c := range r.commands {
		result = append(result, c)
	}
	return result
}

// Get retrieves a command by name.
func (r *Registry) Get(name string) (*Command, bool) {
	cmd, exists := r.commands[name]
	return cmd, exists
}

// RegisterTool adds a tool to the registry.
func (r *Registry) RegisterTool(t *Tool) error {
	if t == nil {
		return fmt.Errorf("tool is nil")
	}

	if t.Name == "" {
		return fmt.Errorf("tool name is required")
	}

	if t.Handler == nil {
		return fmt.Errorf("tool handler is required")
	}

	r.tools[t.Name] = t
	return nil
}

// GetTool retrieves a tool by name.
func (r *Registry) GetTool(name string) (*Tool, bool) {
	tool, exists := r.tools[name]
	return tool, exists
}

// ListTools returns all registered tools.
func (r *Registry) ListTools() []*Tool {
	result := make([]*Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

// CommandFromTool creates a Command from a Tool.
func (r *Registry) CommandFromTool(tool *Tool, nameOverride string) (*Command, error) {
	if tool == nil {
		return nil, fmt.Errorf("tool is nil")
	}
	if tool.Handler == nil {
		return nil, fmt.Errorf("handler is required")
	}

	name := tool.Name
	if nameOverride != "" {
		name = nameOverride
	}

	cmd := &Command{
		Name:        name,
		Description: tool.Description,
		Handler:     tool.Handler,
		Flags:       make(map[string]*FlagDef),
		Args:        make(map[string]*ArgDef),
		Tool:        tool,
	}

	for paramName, param := range tool.Params {
		flagName := strings.ReplaceAll(paramName, "_", "-")
		cmd.Flags[flagName] = &FlagDef{
			Short:       param.Short,
			Type:        param.Type,
			Default:     param.Default,
			Required:    param.Required,
			FromStdin:   param.FromStdin,
			Description: buildFlagDescription(param),
			ParamKey:    paramName,
		}
	}

	return cmd, nil
}

// buildFlagDescription creates a comprehensive flag description
// that includes the base description plus additional metadata like choices,
// default values, and constraints.
func buildFlagDescription(param *Param) string { //nolint:gocognit,gocyclo // complexity inherent in building descriptive flag help with all constraints
	desc := param.Description

	// Add choices information if available and not already mentioned in description
	if len(param.Choices) > 0 { //nolint:nestif // nested check to avoid duplicating choice info already in description
		// Only add "Possible values" if description doesn't already list them
		hasChoicesInDesc := false
		for _, choice := range param.Choices {
			if strings.Contains(desc, fmt.Sprintf("%v", choice)) {
				hasChoicesInDesc = true
				break
			}
		}

		if !hasChoicesInDesc {
			desc += "\nPossible values: "
			for i, choice := range param.Choices {
				if i > 0 {
					desc += ", "
				}
				desc += fmt.Sprintf("%v", choice)
			}
		}
	}

	// Add range constraints for numeric types
	if param.Min != nil || param.Max != nil {
		switch {
		case param.Min != nil && param.Max != nil:
			desc += fmt.Sprintf("\nRange: [%v, %v]", *param.Min, *param.Max)
		case param.Min != nil:
			desc += fmt.Sprintf("\nMinimum: %v", *param.Min)
		case param.Max != nil:
			desc += fmt.Sprintf("\nMaximum: %v", *param.Max)
		}
	}

	// Add length constraints for strings
	if param.MinLen != nil || param.MaxLen != nil {
		switch {
		case param.MinLen != nil && param.MaxLen != nil:
			desc += fmt.Sprintf("\nLength: [%d, %d]", *param.MinLen, *param.MaxLen)
		case param.MinLen != nil:
			desc += fmt.Sprintf("\nMinimum length: %d", *param.MinLen)
		case param.MaxLen != nil:
			desc += fmt.Sprintf("\nMaximum length: %d", *param.MaxLen)
		}
	}

	// Add pattern constraint if set
	if param.Pattern != "" {
		desc += fmt.Sprintf("\nPattern: %s", param.Pattern)
	}

	// Add stdin note if applicable
	if param.FromStdin {
		desc += "\n(can be read from stdin if not provided)"
	}

	return desc
}

// ToolSchema represents an LLM tool definition (OpenAI/Gemini format).
type ToolSchema struct {
	Parameters  map[string]interface{} `json:"parameters"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
}

// GenerateToolSchema generates an LLM tool schema for a tool.
func (t *Tool) GenerateToolSchema() ToolSchema { //nolint:gocognit // complexity inherent in generating schema for all parameter types and constraints
	properties := make(map[string]interface{})
	required := make([]string, 0)

	for paramName, param := range t.Params {
		paramType := param.Type
		switch paramType {
		case paramTypeInt:
			paramType = "integer"
		case paramTypeBool:
			paramType = "boolean"
		case paramTypeFloat:
			paramType = "number"
		}

		schema := map[string]interface{}{
			"type":        paramType,
			"description": param.Description,
		}

		if len(param.Choices) > 0 {
			schema["enum"] = param.Choices
		}
		if param.Pattern != "" {
			schema["pattern"] = param.Pattern
		}
		if param.Min != nil {
			schema["minimum"] = *param.Min
		}
		if param.Max != nil {
			schema["maximum"] = *param.Max
		}
		if param.MinLen != nil {
			schema["minLength"] = *param.MinLen
		}
		if param.MaxLen != nil {
			schema["maxLength"] = *param.MaxLen
		}

		properties[paramName] = schema

		if param.Required {
			required = append(required, paramName)
		}
	}

	parameters := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		parameters["required"] = required
	}

	return ToolSchema{
		Name:        t.Name,
		Description: t.Description,
		Parameters:  parameters,
	}
}

// GenerateAllToolSchemas generates LLM schemas for all registered tools.
func (r *Registry) GenerateAllToolSchemas() []ToolSchema {
	schemas := make([]ToolSchema, 0, len(r.tools))
	for _, tool := range r.tools {
		schemas = append(schemas, tool.GenerateToolSchema())
	}
	return schemas
}
