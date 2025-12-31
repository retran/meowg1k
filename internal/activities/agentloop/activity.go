// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package agentloop implements an iterative agent loop that can call tools.
package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/draftcontent"
	agentconfig "github.com/retran/meowg1k/internal/core/agent"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/pkg/executor"
)

const defaultMaxIterations = 20

// ToolExecutor executes a model-emitted tool call.
//
// Callers can inject an implementation when ready.
// Note: tool implementations are intentionally not provided yet.
type ToolExecutor interface {
	ExecuteTool(ctx context.Context, execCtx *executor.Context, toolName string, args map[string]any) (any, error)
}

// NoopToolExecutor returns a deterministic "not implemented" result.
// This keeps the loop functional while we revise tool implementations.
type NoopToolExecutor struct{}

func (NoopToolExecutor) ExecuteTool(_ context.Context, _ *executor.Context, toolName string, _ map[string]any) (any, error) {
	return map[string]any{"tool": toolName, "error": "tool not implemented"}, nil
}

// Input defines the agent iteration input parameters.
type Input struct {
	ToolExecutor   ToolExecutor
	Profile        *profile.ResolvedProfile
	StepConfig     *agentconfig.StepConfig
	PriorSummaries *[]string
	Goal           *string
	SystemPrompt   *string
	MaxIterations  *int
}

// Output defines the agent iteration output.
type Output struct {
	Content string
	Summary string
}

// Factory builds agent iteration activities.
type Factory struct {
	invokeLLMFactory executor.ActivityFactory[*draftcontent.Input, *draftcontent.Output]
}

// NewFactory creates a new agent iteration factory.
func NewFactory(invokeLLMFactory executor.ActivityFactory[*draftcontent.Input, *draftcontent.Output]) (*Factory, error) {
	if invokeLLMFactory == nil {
		return nil, fmt.Errorf("invokeLLMFactory is nil")
	}
	return &Factory{invokeLLMFactory: invokeLLMFactory}, nil
}

// NewActivity returns the agent iteration activity implementation.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, execCtx *executor.Context, input *Input) (*Output, error) {
		if err := validateInput(f, input); err != nil {
			return nil, err
		}

		stepName := resolveStepName(input.StepConfig)
		execCtx.SendRunning(getStepDisplayName(stepName))

		maxIterations := defaultMaxIterations
		if input.MaxIterations != nil && *input.MaxIterations > 0 {
			maxIterations = *input.MaxIterations
		}

		toolDefs := buildToolDefinitions(input.StepConfig)

		toolExec := input.ToolExecutor
		if toolExec == nil {
			toolExec = NoopToolExecutor{}
		}

		state := &loopState{
			transcript:  make([]transcriptEntry, 0),
			toolResults: make([]toolResult, 0),
		}

		for iteration := 1; iteration <= maxIterations; iteration++ {
			iterationCtx := execCtx.Child(fmt.Sprintf("Iteration#%d", iteration))

			resp, err := f.invokeLLM(
				ctx,
				iterationCtx,
				input.Profile,
				buildSystemPrompt(stringValue(input.SystemPrompt), input.StepConfig),
				buildUserPrompt(stringValue(input.Goal), stringSliceValue(input.PriorSummaries), state),
				toolDefs,
			)
			if err != nil {
				return nil, err
			}
			if resp == nil || resp.Response == nil {
				return nil, fmt.Errorf("nil LLM response")
			}

			blocks := resp.Response.Blocks
			iterationCtx.SendProgressWithDetails("agent.response", fmt.Sprintf("blocks=%d", len(blocks)))

			hadToolCalls := false
			for _, block := range blocks {
				switch block.Kind {
				case gateway.ContentBlockReasoning:
					text := strings.TrimSpace(block.Text)
					if text != "" {
						iterationCtx.SendProgressWithDetails("agent.thought", text)
						state.transcript = append(state.transcript, transcriptEntry{Kind: "reasoning", Text: text})
					}
				case gateway.ContentBlockText:
					text := strings.TrimSpace(block.Text)
					if text != "" {
						iterationCtx.SendProgressWithDetails("agent.output", text)
						state.transcript = append(state.transcript, transcriptEntry{Kind: "text", Text: text})
					}
				case gateway.ContentBlockToolCall:
					if block.ToolCall == nil {
						continue
					}
					hadToolCalls = true

					call := *block.ToolCall
					args := call.Arguments
					if args == nil {
						args = map[string]any{}
					}

					iterationCtx.SendProgressWithDetails("agent.tool_call", formatToolCallDetails(call))
					toolCtx := iterationCtx.Child(fmt.Sprintf("Tool:%s", call.Name))
					toolCtx.SendRunningWithDetails("I'm running a tool", fmt.Sprintf("name=%s", call.Name))

					result, toolErr := toolExec.ExecuteTool(ctx, toolCtx, call.Name, args)
					if toolErr != nil {
						toolCtx.SendFailedWithDetails(toolErr, "Tool failed", fmt.Sprintf("name=%s", call.Name))
						result = map[string]any{"tool": call.Name, "error": toolErr.Error()}
					}
					toolCtx.SendCompletedWithDetails("I've finished running the tool", fmt.Sprintf("name=%s", call.Name))

					state.toolResults = append(state.toolResults, toolResult{ID: call.ID, Name: call.Name, Args: args, Result: result})
					state.transcript = append(state.transcript, transcriptEntry{Kind: "tool_result", ToolName: call.Name, ToolID: call.ID, ToolResult: result})
				}
			}

			if !hadToolCalls {
				finalText := strings.TrimSpace(resp.Response.Text())
				finalSummary := strings.TrimSpace(resp.Response.Reasoning())
				if finalSummary == "" {
					finalSummary = finalText
				}
				execCtx.SendCompletedWithDetails("agent.loop.ended", fmt.Sprintf("iterations=%d", iteration))
				return &Output{Summary: finalSummary, Content: finalText}, nil
			}
		}

		return nil, fmt.Errorf("agent iteration exceeded max iterations (%d)", maxIterations)
	}
}

type toolResult struct {
	Result any            `json:"result"`
	Args   map[string]any `json:"args,omitempty"`
	ID     string         `json:"id,omitempty"`
	Name   string         `json:"name"`
}

type transcriptEntry struct {
	ToolResult any    `json:"tool_result,omitempty"`
	Kind       string `json:"kind"`
	Text       string `json:"text,omitempty"`
	ToolID     string `json:"tool_id,omitempty"`
	ToolName   string `json:"tool_name,omitempty"`
}

type loopState struct {
	toolResults []toolResult
	transcript  []transcriptEntry
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
	return nil
}

func resolveStepName(stepConfig *agentconfig.StepConfig) string {
	if stepConfig == nil {
		return "unknown"
	}
	if stepConfig.Index >= 0 && stepConfig.Index < len(agentconfig.StepOrder) {
		return strings.ToLower(agentconfig.StepOrder[stepConfig.Index])
	}
	return "unknown"
}

func getStepDisplayName(stepName string) string {
	switch strings.ToLower(stepName) {
	case "research":
		return "I'm researching"
	case "plan":
		return "I'm planning"
	case "execute":
		return "I'm executing"
	case "verify":
		return "I'm verifying"
	default:
		return fmt.Sprintf("I'm working on step %s", stepName)
	}
}

func buildToolDefinitions(stepConfig *agentconfig.StepConfig) []gateway.ToolDefinition {
	if stepConfig == nil {
		return nil
	}

	defs := make([]gateway.ToolDefinition, 0)
	for _, tool := range stepConfig.Tools {
		toolLower := strings.ToLower(strings.TrimSpace(tool))
		if toolLower == "" {
			continue
		}

		modes := stepConfig.ToolModes[toolLower]
		if len(modes) == 0 {
			defs = append(defs, gateway.ToolDefinition{
				Name:        toolLower,
				Description: "Tool (implementation pending)",
				Parameters:  map[string]any{"type": "object", "additionalProperties": true},
			})
			continue
		}

		for mode, allowed := range modes {
			if !allowed {
				continue
			}
			modeLower := strings.ToLower(strings.TrimSpace(mode))
			if modeLower == "" {
				continue
			}
			defs = append(defs, gateway.ToolDefinition{
				Name:        fmt.Sprintf("%s.%s", toolLower, modeLower),
				Description: "Tool (implementation pending)",
				Parameters:  map[string]any{"type": "object", "additionalProperties": true},
			})
		}
	}

	return defs
}

func buildSystemPrompt(basePrompt string, _ *agentconfig.StepConfig) string {
	return strings.TrimSpace(basePrompt)
}

func buildUserPrompt(goal string, summaries []string, state *loopState) string {
	goal = strings.TrimSpace(goal)

	var sb strings.Builder
	if goal != "" {
		sb.WriteString("Goal:\n")
		sb.WriteString(goal)
		sb.WriteString("\n\n")
	}

	if len(summaries) > 0 {
		sb.WriteString("PriorSummaries:\n")
		for _, s := range summaries {
			trim := strings.TrimSpace(s)
			if trim == "" {
				continue
			}
			sb.WriteString("- ")
			sb.WriteString(trim)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if state != nil && len(state.toolResults) > 0 {
		if b, err := json.MarshalIndent(state.toolResults, "", "  "); err == nil {
			sb.WriteString("ToolResults:\n")
			sb.Write(b)
			sb.WriteString("\n\n")
		}
	}

	if state != nil && len(state.transcript) > 0 {
		if b, err := json.MarshalIndent(state.transcript, "", "  "); err == nil {
			sb.WriteString("Transcript:\n")
			sb.Write(b)
			sb.WriteString("\n\n")
		}
	}

	sb.WriteString("Respond normally. If you need to use a tool, emit a tool call using the provided tool definitions.")

	return strings.TrimSpace(sb.String())
}

func (f *Factory) invokeLLM(
	ctx context.Context,
	execCtx *executor.Context,
	resolvedProfile *profile.ResolvedProfile,
	systemPrompt string,
	userPrompt string,
	tools []gateway.ToolDefinition,
) (*draftcontent.Output, error) {
	activity := f.invokeLLMFactory.NewActivity()
	out, err := activity(ctx, execCtx, &draftcontent.Input{
		Profile:      resolvedProfile,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Tools:        tools,
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func stringValue(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func stringSliceValue(ptr *[]string) []string {
	if ptr == nil {
		return nil
	}
	return *ptr
}

func formatToolCallDetails(call gateway.ToolCall) string {
	b, _ := json.Marshal(call.Arguments)
	if call.ID != "" {
		return fmt.Sprintf("id=%s name=%s args=%s", call.ID, call.Name, string(b))
	}
	return fmt.Sprintf("name=%s args=%s", call.Name, string(b))
}
