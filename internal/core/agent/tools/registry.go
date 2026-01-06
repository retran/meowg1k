// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/pkg/executor"
)

// Handler executes a tool.
type Handler func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error)

// Tool represents a registered tool.
type Tool struct {
	Handler    Handler
	Definition gateway.ToolDefinition
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
		if suggestion := r.suggestToolName(toolName); suggestion != "" {
			return nil, fmt.Errorf("tool not found: %s (did you mean %s?)", toolName, suggestion)
		}
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}
	return tool.Handler(ctx, execCtx, args)
}

func (r *Registry) suggestToolName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	// Common LLM hallucinations / legacy names.
	switch name {
	case "execute_shell", "run_terminal":
		if _, ok := r.Get("run_shell"); ok {
			return "run_shell"
		}
	}

	return ""
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

// BindArgs converts map[string]any to struct using JSON marshaling.
func BindArgs(args map[string]any, target any) error {
	b, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("failed to marshal args: %w", err)
	}
	if err := json.Unmarshal(b, target); err != nil {
		return fmt.Errorf("failed to unmarshal to target: %w", err)
	}
	return nil
}
