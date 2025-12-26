// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package agentstep implements a single agent step activity with tool use.
package agentstep

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/invokellm"
	"github.com/retran/meowg1k/internal/core/agent"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/pkg/executor"
)

// ToolRunner executes a tool call and returns a result.
type ToolRunner interface {
	RunTool(ctx context.Context, execCtx *executor.Context, tool, mode string, params json.RawMessage, profile *profile.ResolvedProfile, systemPrompt string) (*ToolResult, error)
}

// ToolResult represents the outcome of a tool call.
type ToolResult struct {
	Data interface{} `json:"data"`
	Tool string      `json:"tool"`
	Mode string      `json:"mode"`
}

// Input defines the agent step input parameters.
type Input struct {
	ToolRunner     ToolRunner
	Profile        *profile.ResolvedProfile
	StepConfig     *agent.StepConfig
	PriorSummaries *[]string
	Goal           *string
	SystemPrompt   *string
}

// Output defines the agent step output.
type Output struct {
	Summary string
	Content string
}

// Factory builds agent step activities.
type Factory struct {
	invokeLLMFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output]
}

// NewFactory creates a new agent step factory.
func NewFactory(invokeLLMFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output]) (*Factory, error) {
	if invokeLLMFactory == nil {
		return nil, fmt.Errorf("invokeLLMFactory is nil")
	}
	return &Factory{invokeLLMFactory: invokeLLMFactory}, nil
}

// NewActivity returns the agent step activity implementation.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, execCtx *executor.Context, input *Input) (*Output, error) {
		return f.runStep(ctx, execCtx, input)
	}
}

func (f *Factory) runStep(ctx context.Context, execCtx *executor.Context, input *Input) (*Output, error) {
	if err := validateInput(f, input); err != nil {
		return nil, err
	}

	stepName := resolveStepName(input.StepConfig)

	// Use more user-friendly step names with emojis
	displayName := getStepDisplayName(stepName)
	execCtx.SendRunning(displayName)

	toolDefs, toolNameMap := buildToolDefinitions(input.StepConfig)
	return f.runStepLoop(ctx, execCtx, input, stepName, toolDefs, toolNameMap)
}

func getStepDisplayName(stepName string) string {
	switch strings.ToLower(stepName) {
	case "research":
		return "🧠 Researching..."
	case "plan":
		return "📝 Planning..."
	case "execute":
		return "🚀 Executing..."
	case "verify":
		return "✅ Verifying..."
	default:
		return fmt.Sprintf("Agent step: %s", stepName)
	}
}

func resolveStepName(stepConfig *agent.StepConfig) string {
	if stepConfig == nil {
		return "unknown"
	}
	if stepConfig.Index >= 0 && stepConfig.Index < len(agent.StepOrder) {
		return strings.ToLower(agent.StepOrder[stepConfig.Index])
	}
	return "unknown"
}

func (f *Factory) runStepLoop(ctx context.Context, execCtx *executor.Context, input *Input, stepName string, toolDefs []gateway.ToolDefinition, toolNameMap map[string]toolSpec) (*Output, error) {
	toolResults := make([]*ToolResult, 0)
	systemPrompt := stringValue(input.SystemPrompt)
	goal := stringValue(input.Goal)
	summaries := stringSliceValue(input.PriorSummaries)
	for {
		response, err := f.invokeLLM(ctx, execCtx, input.Profile, buildSystemPrompt(systemPrompt, input.StepConfig), buildUserPrompt(goal, summaries, toolResults, stepName), toolDefs)
		if err != nil {
			return nil, err
		}

		output, result, err := handleResponse(ctx, execCtx, input, stepName, toolDefs, toolNameMap, response)
		if err != nil {
			return nil, err
		}
		if output != nil {
			return output, nil
		}
		if result != nil {
			toolResults = append(toolResults, result)
		}
	}
}

func validateInput(factory *Factory, input *Input) error {
	if factory == nil {
		return fmt.Errorf("factory is nil")
	}
	if input == nil {
		return fmt.Errorf("input cannot be nil")
	}
	if input.StepConfig == nil {
		return fmt.Errorf("step config is nil")
	}
	if input.Profile == nil {
		return fmt.Errorf("profile is nil")
	}
	if input.ToolRunner == nil {
		return fmt.Errorf("tool runner is nil")
	}
	return nil
}

func handleResponse(ctx context.Context, execCtx *executor.Context, input *Input, stepName string, toolDefs []gateway.ToolDefinition, toolNameMap map[string]toolSpec, response *invokellm.Output) (*Output, *ToolResult, error) {
	if len(response.ToolCalls) > 0 {
		return handleToolCalls(ctx, execCtx, input, stepName, toolNameMap, response.ToolCalls)
	}

	parsed, err := parseAgentResponse(response.Content)
	if err != nil {
		return handleNonJSONResponse(execCtx, stepName, toolDefs, response.Content, err)
	}

	switch parsed.Type {
	case "final":
		summary := parsed.Summary
		if summary == "" {
			summary = parsed.Content
		}
		execCtx.SendCompleted(fmt.Sprintf("Completed: %s", stepName))
		return &Output{Summary: strings.TrimSpace(summary), Content: strings.TrimSpace(parsed.Content)}, nil, nil
	case "tool":
		if !input.StepConfig.AllowsToolMode(parsed.Tool, parsed.Mode) {
			return nil, nil, fmt.Errorf("tool %q mode %q not allowed in step %q", parsed.Tool, parsed.Mode, stepName)
		}
		result, err := input.ToolRunner.RunTool(ctx, execCtx, parsed.Tool, parsed.Mode, parsed.Params, input.Profile, stringValue(input.SystemPrompt))
		if err != nil {
			return nil, nil, fmt.Errorf("tool %q mode %q failed: %w", parsed.Tool, parsed.Mode, err)
		}
		return nil, result, nil
	default:
		return nil, nil, fmt.Errorf("unsupported response type %q", parsed.Type)
	}
}

func handleToolCalls(ctx context.Context, execCtx *executor.Context, input *Input, stepName string, toolNameMap map[string]toolSpec, calls []gateway.ToolCall) (*Output, *ToolResult, error) {
	if len(calls) > 1 {
		return nil, nil, fmt.Errorf("multiple tool calls returned; only one is supported per step")
	}
	toolCall := calls[0]
	spec, ok := toolNameMap[strings.ToLower(toolCall.Name)]
	if !ok {
		return nil, nil, fmt.Errorf("unknown tool call %q", toolCall.Name)
	}
	if !input.StepConfig.AllowsToolMode(spec.Tool, spec.Mode) {
		return nil, nil, fmt.Errorf("tool %q mode %q not allowed in step %q", spec.Tool, spec.Mode, stepName)
	}
	args := toolCall.Arguments
	if args == nil {
		args = map[string]any{}
	}
	params, err := json.Marshal(args)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal tool call args: %w", err)
	}
	result, err := input.ToolRunner.RunTool(ctx, execCtx, spec.Tool, spec.Mode, params, input.Profile, stringValue(input.SystemPrompt))
	if err != nil {
		return nil, nil, fmt.Errorf("tool %q mode %q failed: %w", spec.Tool, spec.Mode, err)
	}
	return nil, result, nil
}

func handleNonJSONResponse(execCtx *executor.Context, stepName string, toolDefs []gateway.ToolDefinition, content string, parseErr error) (*Output, *ToolResult, error) {
	if len(toolDefs) == 0 {
		return nil, nil, parseErr
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, nil, parseErr
	}
	execCtx.SendCompleted(fmt.Sprintf("Completed: %s", stepName))
	return &Output{Summary: trimmed, Content: trimmed}, nil, nil
}

func buildSystemPrompt(basePrompt string, stepConfig *agent.StepConfig) string {
	toolSchema := `If tool calling is not used, respond with JSON only. Use one of:
{"type":"tool","tool":"<tool>","mode":"<mode>","params":{...}}
{"type":"final","content":"...","summary":"..."}
`

	toolsAndModes := buildToolModeSummary(stepConfig)
	if toolsAndModes == "" {
		toolsAndModes = "workspace(list, read, write, replace, delete, mkdir, stat, exists); search(embeddings); git(status, diff, show, log, branch, current_branch, stage, commit); summarize(text, file, diff); plan(add, complete, list); memory(add, list); command(run); patch(apply)"
	}
	toolSchema += "\nAllowed tools and modes: " + toolsAndModes + "\n"
	toolSchema += "Modes must match the allowed list exactly. Use params for options (example: workspace.list uses depth for recursion; workspace.delete uses recursive boolean), not new modes.\n"
	toolSchema += "If tool calling is available, prefer tool calls instead of emitting a JSON tool response.\n"

	stepPrompt := basePrompt
	if stepConfig != nil && stepConfig.SystemPrompt != "" {
		stepPrompt = stepConfig.SystemPrompt
	}

	return strings.TrimSpace(stepPrompt + "\n\n" + toolSchema)
}

func buildToolModeSummary(stepConfig *agent.StepConfig) string {
	if stepConfig == nil {
		return ""
	}
	if len(stepConfig.Tools) == 0 {
		return ""
	}

	parts := make([]string, 0, len(stepConfig.Tools))
	for _, tool := range stepConfig.Tools {
		modeSet := stepConfig.ToolModes[strings.ToLower(tool)]
		if len(modeSet) == 0 {
			parts = append(parts, fmt.Sprintf("%s(*)", tool))
			continue
		}

		modes := make([]string, 0, len(modeSet))
		for mode := range modeSet {
			modes = append(modes, mode)
		}
		parts = append(parts, fmt.Sprintf("%s(%s)", tool, strings.Join(modes, ", ")))
	}

	return strings.Join(parts, "; ")
}

func buildUserPrompt(goal string, summaries []string, toolResults []*ToolResult, stepName string) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Goal:\n%s\n\n", strings.TrimSpace(goal)))
	builder.WriteString(fmt.Sprintf("Step: %s\n\n", stepName))

	if len(summaries) > 0 {
		builder.WriteString("Prior step summaries:\n")
		for i, summary := range summaries {
			builder.WriteString(fmt.Sprintf("%d) %s\n", i+1, summary))
		}
		builder.WriteString("\n")
	}

	if len(toolResults) > 0 {
		builder.WriteString("Tool results:\n")
		for i, result := range toolResults {
			jsonResult, err := json.Marshal(result)
			if err != nil {
				builder.WriteString(fmt.Sprintf("%d) <failed to marshal tool result: %v>\n", i+1, err))
				continue
			}
			builder.WriteString(fmt.Sprintf("%d) %s\n", i+1, string(jsonResult)))
		}
		builder.WriteString("\n")
	}

	builder.WriteString("Respond with the next tool call or final output.")
	return builder.String()
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringSliceValue(value *[]string) []string {
	if value == nil {
		return nil
	}
	return *value
}

type toolSpec struct {
	Tool string
	Mode string
}

type toolSchema struct {
	Parameters  map[string]any
	Description string
}

func buildToolDefinitions(stepConfig *agent.StepConfig) (definitions []gateway.ToolDefinition, toolMap map[string]toolSpec) {
	if stepConfig == nil || len(stepConfig.Tools) == 0 {
		return nil, map[string]toolSpec{}
	}

	allowedTools := make(map[string]bool, len(stepConfig.Tools))
	for _, tool := range stepConfig.Tools {
		allowedTools[strings.ToLower(tool)] = true
	}

	definitions = make([]gateway.ToolDefinition, 0)
	toolMap = make(map[string]toolSpec)
	schemas := toolSchemas()

	for tool, modes := range schemas {
		if !allowedTools[tool] {
			continue
		}

		allowedModes := stepConfig.ToolModes[tool]
		for mode, schema := range modes {
			if len(allowedModes) > 0 && !allowedModes[mode] {
				continue
			}
			name := strings.ToLower(fmt.Sprintf("%s_%s", tool, mode))
			definitions = append(definitions, gateway.ToolDefinition{
				Name:        name,
				Description: schema.Description,
				Parameters:  schema.Parameters,
			})
			toolMap[name] = toolSpec{Tool: tool, Mode: mode}
		}
	}

	return definitions, toolMap
}

func toolSchemas() map[string]map[string]toolSchema {
	return map[string]map[string]toolSchema{
		"workspace": workspaceToolSchemas(),
		"search":    searchToolSchemas(),
		"git":       gitToolSchemas(),
		"summarize": summarizeToolSchemas(),
		"plan":      planToolSchemas(),
		"memory":    memoryToolSchemas(),
		"command":   commandToolSchemas(),
		"patch":     patchToolSchemas(),
	}
}

func workspaceToolSchemas() map[string]toolSchema {
	return map[string]toolSchema{
		"list": {
			Description: "List workspace entries under a path.",
			Parameters: schemaObject(map[string]any{
				"path":          schemaString("Relative path to list."),
				"depth":         schemaInteger("Maximum depth to traverse."),
				"include_files": schemaBool("Whether to include files."),
				"include_dirs":  schemaBool("Whether to include directories."),
			}, nil),
		},
		"read": {
			Description: "Read a file from the workspace.",
			Parameters: schemaObject(map[string]any{
				"path":       schemaString("Relative file path."),
				"start_line": schemaInteger("Start line (1-based)."),
				"end_line":   schemaInteger("End line (1-based)."),
			}, []string{"path"}),
		},
		"write": {
			Description: "Write content to a file in the workspace.",
			Parameters: schemaObject(map[string]any{
				"path":      schemaString("Relative file path."),
				"content":   schemaString("File content."),
				"create":    schemaBool("Create file if missing."),
				"overwrite": schemaBool("Overwrite file if it exists."),
			}, []string{"path", "content"}),
		},
		"replace": {
			Description: "Replace text in a workspace file.",
			Parameters: schemaObject(map[string]any{
				"path":          schemaString("Relative file path."),
				"old_text":      schemaString("Text to replace."),
				"new_text":      schemaString("Replacement text."),
				"occurrence":    schemaString("Replacement mode: all, first, single."),
				"require_match": schemaBool("Require a match to succeed."),
			}, []string{"path", "old_text", "new_text"}),
		},
		"delete": {
			Description: "Delete a workspace file or directory.",
			Parameters: schemaObject(map[string]any{
				"path":      schemaString("Relative path to delete."),
				"recursive": schemaBool("Allow recursive deletion for directories."),
			}, []string{"path"}),
		},
		"mkdir": {
			Description: "Create a workspace directory.",
			Parameters: schemaObject(map[string]any{
				"path":    schemaString("Relative path to create."),
				"parents": schemaBool("Create parent directories."),
			}, []string{"path"}),
		},
		"stat": {
			Description: "Get file metadata for a workspace path.",
			Parameters: schemaObject(map[string]any{
				"path": schemaString("Relative path to stat."),
			}, []string{"path"}),
		},
		"exists": {
			Description: "Check whether a workspace path exists.",
			Parameters: schemaObject(map[string]any{
				"path": schemaString("Relative path to check."),
			}, []string{"path"}),
		},
	}
}

func searchToolSchemas() map[string]toolSchema {
	return map[string]toolSchema{
		"embeddings": {
			Description: "Search indexed workspace content.",
			Parameters: schemaObject(map[string]any{
				"query_text": schemaString("Search query text."),
			}, []string{"query_text"}),
		},
	}
}

func gitToolSchemas() map[string]toolSchema {
	return map[string]toolSchema{
		"status": {
			Description: "Get git status.",
			Parameters:  schemaObject(map[string]any{}, nil),
		},
		"diff": {
			Description: "Get git diff for a ref/path.",
			Parameters: schemaObject(map[string]any{
				"ref":  schemaString("Git ref."),
				"path": schemaString("Optional path filter."),
			}, nil),
		},
		"show": {
			Description: "Show a git commit.",
			Parameters: schemaObject(map[string]any{
				"ref": schemaString("Git ref."),
			}, nil),
		},
		"log": {
			Description: "Show git log.",
			Parameters: schemaObject(map[string]any{
				"limit": schemaInteger("Maximum number of entries."),
				"path":  schemaString("Optional path filter."),
			}, nil),
		},
		"branch": {
			Description: "List git branches.",
			Parameters:  schemaObject(map[string]any{}, nil),
		},
		"current_branch": {
			Description: "Get the current git branch.",
			Parameters:  schemaObject(map[string]any{}, nil),
		},
		"stage": {
			Description: "Stage git paths.",
			Parameters: schemaObject(map[string]any{
				"paths": schemaArray(schemaString("Path to stage.")),
			}, []string{"paths"}),
		},
		"commit": {
			Description: "Create a git commit.",
			Parameters: schemaObject(map[string]any{
				"message": schemaString("Commit message."),
			}, []string{"message"}),
		},
	}
}

func summarizeToolSchemas() map[string]toolSchema {
	return map[string]toolSchema{
		"text": {
			Description: "Summarize text content.",
			Parameters: schemaObject(map[string]any{
				"text": schemaString("Text to summarize."),
			}, []string{"text"}),
		},
		"file": {
			Description: "Summarize a workspace file.",
			Parameters: schemaObject(map[string]any{
				"path": schemaString("Relative file path."),
			}, []string{"path"}),
		},
		"diff": {
			Description: "Summarize a git diff.",
			Parameters: schemaObject(map[string]any{
				"ref":  schemaString("Git ref."),
				"path": schemaString("Optional path filter."),
			}, nil),
		},
	}
}

func planToolSchemas() map[string]toolSchema {
	return map[string]toolSchema{
		"add": {
			Description: "Add a plan task.",
			Parameters: schemaObject(map[string]any{
				"text": schemaString("Task description."),
			}, []string{"text"}),
		},
		"complete": {
			Description: "Mark a plan task complete.",
			Parameters: schemaObject(map[string]any{
				"task_id": schemaInteger("Task ID."),
			}, []string{"task_id"}),
		},
		"list": {
			Description: "List plan tasks.",
			Parameters:  schemaObject(map[string]any{}, nil),
		},
	}
}

func memoryToolSchemas() map[string]toolSchema {
	return map[string]toolSchema{
		"add": {
			Description: "Add a memory note for later steps.",
			Parameters: schemaObject(map[string]any{
				"text": schemaString("Memory note to store."),
			}, []string{"text"}),
		},
		"list": {
			Description: "List stored memory notes.",
			Parameters:  schemaObject(map[string]any{}, nil),
		},
	}
}

func commandToolSchemas() map[string]toolSchema {
	return map[string]toolSchema{
		"run": {
			Description: "Run a shell command.",
			Parameters: schemaObject(map[string]any{
				"command": schemaString("Shell command to execute."),
			}, []string{"command"}),
		},
	}
}

func patchToolSchemas() map[string]toolSchema {
	return map[string]toolSchema{
		"apply": {
			Description: "Apply a unified diff patch.",
			Parameters: schemaObject(map[string]any{
				"diff": schemaString("Unified diff patch."),
			}, []string{"diff"}),
		},
	}
}

func schemaObject(properties map[string]any, required []string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func schemaString(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
	}
}

func schemaInteger(description string) map[string]any {
	return map[string]any{
		"type":        "integer",
		"description": description,
	}
}

func schemaBool(description string) map[string]any {
	return map[string]any{
		"type":        "boolean",
		"description": description,
	}
}

func schemaArray(item map[string]any) map[string]any {
	return map[string]any{
		"type":  "array",
		"items": item,
	}
}

type agentResponse struct {
	Content string          `json:"content"`
	Summary string          `json:"summary"`
	Type    string          `json:"type"`
	Tool    string          `json:"tool"`
	Mode    string          `json:"mode"`
	Params  json.RawMessage `json:"params"`
}

func parseAgentResponse(content string) (*agentResponse, error) {
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("empty agent response")
	}

	jsonPayload := extractJSONPayload(content)
	if jsonPayload == "" {
		return nil, fmt.Errorf("failed to find JSON in response")
	}

	var parsed agentResponse
	if err := json.Unmarshal([]byte(jsonPayload), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse agent response: %w", err)
	}

	parsed.Type = strings.ToLower(strings.TrimSpace(parsed.Type))
	parsed.Tool = strings.ToLower(strings.TrimSpace(parsed.Tool))
	parsed.Mode = strings.ToLower(strings.TrimSpace(parsed.Mode))

	if parsed.Type == "" {
		return nil, fmt.Errorf("response type is required")
	}

	return &parsed, nil
}

func extractJSONPayload(content string) string {
	start := strings.Index(content, "```")
	if start >= 0 {
		segment := content[start+3:]
		if strings.HasPrefix(strings.TrimSpace(segment), "json") {
			segment = segment[strings.Index(segment, "\n")+1:]
		}
		end := strings.Index(segment, "```")
		if end >= 0 {
			return strings.TrimSpace(segment[:end])
		}
	}

	open := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if open >= 0 && end > open {
		return strings.TrimSpace(content[open : end+1])
	}

	return ""
}

func (f *Factory) invokeLLM(
	ctx context.Context,
	execCtx *executor.Context,
	resolvedProfile *profile.ResolvedProfile,
	systemPrompt string,
	userPrompt string,
	tools []gateway.ToolDefinition,
) (*invokellm.Output, error) {
	input := &invokellm.Input{
		Profile:      resolvedProfile,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Tools:        tools,
	}

	activity := f.invokeLLMFactory.NewActivity()
	output, err := executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "InvokeLLM", activity, input)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke LLM: %w", err)
	}

	return output, nil
}
