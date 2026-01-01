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
	"github.com/retran/meowg1k/internal/core/agent/tools"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/pkg/executor"
)

const defaultMaxIterations = 20

// Input defines the agent iteration input parameters.
type Input struct {
	ToolRegistry             *tools.Registry
	ToolDescriptionOverrides map[string]string
	Profile                  *profile.ResolvedProfile
	PriorSummaries           *[]string
	Goal                     *string
	SystemPrompt             *string
	MaxIterations            *int
	StepName                 string
	AllowedTools             []string
}

// Output defines the agent iteration output.
type Output struct {
	// Content is the full user-facing output for the step, including narration
	// emitted in tool-call iterations plus the final answer.
	Content string
	// FinalMessage is the message emitted in the first iteration with no tool calls.
	// This is typically what subsequent steps should use as the prior step output.
	FinalMessage string
	Summary      string
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

		allowed := make(map[string]struct{}, len(input.AllowedTools))
		for _, name := range input.AllowedTools {
			n := strings.TrimSpace(name)
			if n == "" {
				continue
			}
			allowed[n] = struct{}{}
		}

		stepName := strings.TrimSpace(input.StepName)
		if stepName == "" {
			stepName = "unknown"
		}
		stepLabel := strings.TrimSpace(strings.Title(stepName))
		if stepLabel == "" {
			stepLabel = "Unknown"
		}
		execCtx.SendRunning(getStepDisplayName(stepName))
		execCtx.SendProgress(fmt.Sprintf("Step started: %s", stepLabel))

		maxIterations := defaultMaxIterations
		if input.MaxIterations != nil && *input.MaxIterations > 0 {
			maxIterations = *input.MaxIterations
		}

		toolDefs := input.ToolRegistry.GetDefinitions(input.AllowedTools)
		applyToolDescriptionOverrides(toolDefs, input.ToolDescriptionOverrides)

		state := &loopState{
			transcript:  make([]transcriptEntry, 0),
			toolResults: make([]toolResult, 0),
		}

		for iteration := 1; iteration <= maxIterations; iteration++ {
			iterationCtx := execCtx.Child(fmt.Sprintf("Iteration#%d", iteration))

			messages := buildMessages(stringValue(input.Goal), stringSliceValue(input.PriorSummaries), state)
			resp, err := f.invokeLLM(
				ctx,
				iterationCtx,
				input.Profile,
				strings.TrimSpace(stringValue(input.SystemPrompt)),
				buildUserPrompt(stringValue(input.Goal), stringSliceValue(input.PriorSummaries), state),
				messages,
				toolDefs,
			)
			if err != nil {
				return nil, err
			}
			if resp == nil || resp.Response == nil {
				return nil, fmt.Errorf("nil LLM response")
			}

			blocks := resp.Response.Blocks

			hadToolCalls := false
			pendingNarration := make([]string, 0)
			for _, block := range blocks {
				switch block.Kind {
				case gateway.ContentBlockReasoning:
					text := strings.TrimSpace(block.Text)
					if text != "" {
						pendingNarration = append(pendingNarration, text)
						state.transcript = append(state.transcript, transcriptEntry{Kind: "reasoning", Text: text})
					}
				case gateway.ContentBlockText:
					text := strings.TrimSpace(block.Text)
					if text != "" {
						pendingNarration = append(pendingNarration, text)
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

					title := formatToolCallTitle(call.Name, args)
					details := formatToolCallDetails(call, args, strings.Join(pendingNarration, "\n\n"))
					pendingNarration = pendingNarration[:0]
					iterationCtx.SendProgressWithDetails(title, details)
					toolCtx := iterationCtx.Child(title)

					_, toolAllowed := allowed[call.Name]
					var (
						result  any
						toolErr error
					)
					if !toolAllowed {
						toolErr = fmt.Errorf("tool not allowed in this step: %s", call.Name)
						result = map[string]any{"tool": call.Name, "error": toolErr.Error()}
					} else {
						result, toolErr = input.ToolRegistry.ExecuteTool(ctx, toolCtx, call.Name, args)
					}
					if toolErr != nil {
						// Tool errors are expected sometimes (e.g., guessed paths). Don't fail the
						// activity/step; return the error back to the model as a normal tool result.
						toolCtx.SendProgressWithDetails("Tool returned error", toolErr.Error())
						result = map[string]any{"tool": call.Name, "error": toolErr.Error()}
					}

					state.toolResults = append(state.toolResults, toolResult{ID: call.ID, Name: call.Name, Args: args, Result: result})
					state.transcript = append(state.transcript, transcriptEntry{Kind: "tool_call", ToolName: call.Name, ToolID: call.ID, ToolArgs: args})
					state.transcript = append(state.transcript, transcriptEntry{Kind: "tool_result", ToolName: call.Name, ToolID: call.ID, ToolResult: result})
				}
			}

			if hadToolCalls {
				continue
			}

			if !hadToolCalls {
				finalMessage := strings.TrimSpace(strings.Join(pendingNarration, "\n\n"))
				content := renderTranscriptOutput(state.transcript)
				finalSummary := strings.TrimSpace(resp.Response.Reasoning())
				if finalSummary == "" {
					finalSummary = finalMessage
				}
				if strings.TrimSpace(finalMessage) != "" {
					iterationCtx.SendProgressWithDetails("Agent output", finalMessage)
				}
				execCtx.SendCompletedWithDetails(fmt.Sprintf("Step finished: %s", stepLabel), fmt.Sprintf("iterations=%d", iteration))
				return &Output{Summary: finalSummary, Content: content, FinalMessage: finalMessage}, nil
			}
		}

		return nil, fmt.Errorf("agent iteration exceeded max iterations (%d)", maxIterations)
	}
}

func applyToolDescriptionOverrides(defs []gateway.ToolDefinition, overrides map[string]string) {
	if len(defs) == 0 || len(overrides) == 0 {
		return
	}

	normalized := make(map[string]string, len(overrides))
	for k, v := range overrides {
		key := strings.ToLower(strings.TrimSpace(k))
		val := strings.TrimSpace(v)
		if key == "" || val == "" {
			continue
		}
		normalized[key] = val
	}
	if len(normalized) == 0 {
		return
	}

	for i := range defs {
		name := strings.ToLower(strings.TrimSpace(defs[i].Name))
		if name == "" {
			continue
		}
		if desc, ok := normalized[name]; ok {
			defs[i].Description = desc
		}
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
	ToolArgs   any    `json:"tool_args,omitempty"`
	Kind       string `json:"kind"`
	Text       string `json:"text,omitempty"`
	ToolID     string `json:"tool_id,omitempty"`
	ToolName   string `json:"tool_name,omitempty"`
}

type loopState struct {
	toolResults []toolResult
	transcript  []transcriptEntry
}

func renderTranscriptOutput(transcript []transcriptEntry) string {
	if len(transcript) == 0 {
		return ""
	}

	parts := make([]string, 0, len(transcript))
	for _, tr := range transcript {
		switch tr.Kind {
		case "text", "reasoning":
			seg := strings.TrimSpace(tr.Text)
			if seg == "" {
				continue
			}
			parts = append(parts, seg)
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func validateInput(factory *Factory, input *Input) error {
	if factory == nil {
		return fmt.Errorf("factory is nil")
	}
	if input == nil {
		return fmt.Errorf("input cannot be nil")
	}
	if input.Profile == nil {
		return fmt.Errorf("profile is nil")
	}
	if input.ToolRegistry == nil {
		return fmt.Errorf("tool registry is nil")
	}
	return nil
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

	return strings.TrimSpace(sb.String())
}

func buildMessages(goal string, summaries []string, state *loopState) []gateway.Message {
	goal = strings.TrimSpace(goal)

	var user strings.Builder
	if goal != "" {
		user.WriteString("Goal:\n")
		user.WriteString(goal)
		user.WriteString("\n")
	}
	if len(summaries) > 0 {
		user.WriteString("\nPriorSummaries:\n")
		for _, s := range summaries {
			trim := strings.TrimSpace(s)
			if trim == "" {
				continue
			}
			user.WriteString("- ")
			user.WriteString(trim)
			user.WriteString("\n")
		}
	}

	messages := []gateway.Message{}
	if strings.TrimSpace(user.String()) != "" {
		messages = append(messages, gateway.Message{Role: gateway.MessageRoleUser, Content: strings.TrimSpace(user.String())})
	}

	if state == nil || len(state.transcript) == 0 {
		return messages
	}

	for _, tr := range state.transcript {
		switch tr.Kind {
		case "text", "reasoning":
			if strings.TrimSpace(tr.Text) == "" {
				continue
			}
			messages = append(messages, gateway.Message{Role: gateway.MessageRoleAssistant, Content: strings.TrimSpace(tr.Text)})
		case "tool_call":
			args := map[string]any{}
			if tr.ToolArgs != nil {
				if m, ok := tr.ToolArgs.(map[string]any); ok {
					args = m
				}
			}
			calls := []gateway.ToolCall{{ID: tr.ToolID, Name: tr.ToolName, Arguments: args}}
			messages = append(messages, gateway.Message{Role: gateway.MessageRoleAssistant, ToolCalls: calls})
		case "tool_result":
			resultJSON := "{}"
			if tr.ToolResult != nil {
				if b, err := json.Marshal(tr.ToolResult); err == nil {
					resultJSON = string(b)
				}
			}
			messages = append(messages, gateway.Message{Role: gateway.MessageRoleTool, ToolCallID: tr.ToolID, ToolName: tr.ToolName, Content: resultJSON})
		}
	}

	return messages
}

func (f *Factory) invokeLLM(
	ctx context.Context,
	execCtx *executor.Context,
	resolvedProfile *profile.ResolvedProfile,
	systemPrompt string,
	userPrompt string,
	messages []gateway.Message,
	tools []gateway.ToolDefinition,
) (*draftcontent.Output, error) {
	activity := f.invokeLLMFactory.NewActivity()
	out, err := activity(ctx, execCtx, &draftcontent.Input{
		Profile:      resolvedProfile,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Messages:     messages,
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

func formatToolCallTitle(name string, args map[string]any) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Using a tool"
	}

	switch name {
	case "list_files":
		dir, _ := args["dir"].(string)
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return "Listing files"
		}
		return fmt.Sprintf("Listing files in %s", dir)
	case "read_file":
		p, _ := args["path"].(string)
		p = strings.TrimSpace(p)
		if p == "" {
			return "Reading a file"
		}
		return fmt.Sprintf("Reading %s", p)
	case "write_file":
		p, _ := args["path"].(string)
		p = strings.TrimSpace(p)
		if p == "" {
			return "Writing a file"
		}
		return fmt.Sprintf("Writing %s", p)
	case "edit_file":
		p, _ := args["path"].(string)
		p = strings.TrimSpace(p)
		if p == "" {
			return "Editing a file"
		}
		return fmt.Sprintf("Editing %s", p)
	case "search_code":
		q, _ := args["query"].(string)
		q = strings.TrimSpace(q)
		if q == "" {
			return "Searching the codebase"
		}
		return fmt.Sprintf("Searching: %s", truncateOneLine(q, 80))
	case "run_shell":
		cmd, _ := args["command"].(string)
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			return "Running a shell command"
		}
		if rawArgs, ok := args["args"].([]any); ok && len(rawArgs) > 0 {
			parts := make([]string, 0, len(rawArgs)+1)
			parts = append(parts, cmd)
			for _, a := range rawArgs {
				if s, ok := a.(string); ok {
					parts = append(parts, s)
				}
			}
			return fmt.Sprintf("Running: %s", truncateOneLine(strings.Join(parts, " "), 100))
		}
		return fmt.Sprintf("Running: %s", truncateOneLine(cmd, 100))
	case "get_diff":
		staged, _ := args["staged"].(bool)
		if staged {
			return "Getting staged diff"
		}
		return "Getting diff"
	case "create_plan":
		if rawTasks, ok := args["tasks"].([]any); ok {
			return fmt.Sprintf("Creating a plan (%d tasks)", len(rawTasks))
		}
		return "Creating a plan"
	case "update_task":
		id, _ := args["id"].(string)
		status, _ := args["status"].(string)
		id = strings.TrimSpace(id)
		status = strings.TrimSpace(status)
		if id != "" && status != "" {
			return fmt.Sprintf("Updating task %s → %s", id, status)
		}
		return "Updating a task"
	case "memorize_fact":
		fact, _ := args["fact"].(string)
		fact = strings.TrimSpace(fact)
		if fact == "" {
			return "Saving a note"
		}
		return fmt.Sprintf("Saving a note: %s", truncateOneLine(fact, 80))
	case "summarize":
		t, _ := args["type"].(string)
		t = strings.TrimSpace(t)
		if t == "" {
			return "Summarizing"
		}
		return fmt.Sprintf("Summarizing (%s)", t)
	case "restart_with_instruction":
		return "Requesting a restart"
	default:
		return fmt.Sprintf("Using %s", name)
	}
}

func formatToolCallDetails(_ gateway.ToolCall, _ map[string]any, narration string) string {
	narration = strings.TrimSpace(narration)
	if narration != "" {
		return narration
	}
	return ""
}

func truncateOneLine(s string, maxLimit int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.TrimSpace(s)
	if maxLimit <= 0 {
		return s
	}
	if len(s) <= maxLimit {
		return s
	}
	if maxLimit <= 1 {
		return s[:maxLimit]
	}
	return s[:maxLimit-1] + "…"
}
