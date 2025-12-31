// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/pkg/executor"
)

// Handler executes a tool.
type Handler func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error)

// Tool represents a registered tool.
type Tool struct {
	Definition gateway.ToolDefinition
	Handler    Handler
}

// Registry manages the available tools.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool Tool) {
	r.tools[tool.Definition.Name] = tool
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// ExecuteTool executes a tool by name.
func (r *Registry) ExecuteTool(ctx context.Context, execCtx *executor.Context, toolName string, args map[string]any) (any, error) {
	tool, ok := r.Get(toolName)
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}
	return tool.Handler(ctx, execCtx, args)
}

// GetDefinitions returns the definitions for the specified tool names.
func (r *Registry) GetDefinitions(names []string) []gateway.ToolDefinition {
	defs := make([]gateway.ToolDefinition, 0, len(names))
	for _, name := range names {
		if tool, ok := r.Get(name); ok {
			defs = append(defs, tool.Definition)
		}
	}
	return defs
}

// Helper to convert map[string]any to struct using JSON
func BindArgs(args map[string]any, target any) error {
	b, err := json.Marshal(args)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, target)
}
