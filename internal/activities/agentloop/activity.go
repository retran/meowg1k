// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package agentloop implements an iterative agent loop that can call tools.
package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/retran/meowg1k/internal/activities/draftcontent"
	"github.com/retran/meowg1k/internal/core/agent/tools"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/pkg/executor"
)

const defaultMaxIterations = 20

const (
	toolDirList        = "dir_list"
	toolFileRead       = "file_read"
	toolFileWrite      = "file_write"
	toolFileEdit       = "file_edit"
	toolFileMove       = "file_move"
	toolFileDelete     = "file_delete"
	toolGitUndo        = "git_undo"
	toolSearchSemantic = "search_semantic"
	toolSearchText     = "search_text"
	toolShellExec      = "shell_exec"
	toolPlanInit       = "plan_init"
	toolPlanRead       = "plan_read"
	toolPlanUpdateTask = "plan_update_task"
	toolMemStore       = "mem_store"
)

// Input defines the agent iteration input parameters.
type Input struct {
	ToolRegistry             *tools.Registry
	ToolDescriptionOverrides map[string]string
	Preset                   *preset.ResolvedPreset
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

		allowed := f.prepareAllowedTools(input.AllowedTools)
		stepLabel := f.setupExecutionContext(execCtx, input.StepName)
		maxIterations := f.resolveMaxIterations(input.MaxIterations)

		toolDefs := input.ToolRegistry.GetDefinitions(input.AllowedTools)
		applyToolDescriptionOverrides(toolDefs, input.ToolDescriptionOverrides)

		state := &loopState{
			transcript:  make([]transcriptEntry, 0),
			toolResults: make([]toolResult, 0),
		}

		for iteration := 1; iteration <= maxIterations; iteration++ {
			out, finished, err := f.runIteration(ctx, execCtx, iteration, input, toolDefs, allowed, state, stepLabel)
			if err != nil {
				return nil, err
			}
			if finished {
				return out, nil
			}
		}

		return nil, fmt.Errorf("agent iteration exceeded max iterations (%d)", maxIterations)
	}
}

func (f *Factory) runIteration(
	ctx context.Context,
	execCtx *executor.Context,
	iteration int,
	input *Input,
	toolDefs []gateway.ToolDefinition,
	allowed map[string]struct{},
	state *loopState,
	stepLabel string,
) (*Output, bool, error) {
	iterationCtx := execCtx.Child(fmt.Sprintf("Iteration#%d", iteration))

	resp, err := f.invokeLLM(
		ctx,
		iterationCtx,
		input.Preset,
		strings.TrimSpace(stringValue(input.SystemPrompt)),
		buildUserPrompt(stringValue(input.Goal), stringSliceValue(input.PriorSummaries), state),
		buildMessages(stringValue(input.Goal), stringSliceValue(input.PriorSummaries), state),
		toolDefs,
	)
	if err != nil {
		return nil, false, err
	}
	if resp == nil || resp.Response == nil {
		return nil, false, fmt.Errorf("nil LLM response")
	}

	hadToolCalls, pendingNarration := f.processBlocks(ctx, iterationCtx, resp.Response.Blocks, input.ToolRegistry, allowed, state)

	if hadToolCalls {
		return nil, false, nil
	}

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
	return &Output{Summary: finalSummary, Content: content, FinalMessage: finalMessage}, true, nil
}

func (f *Factory) prepareAllowedTools(allowedTools []string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(allowedTools))
	for _, name := range allowedTools {
		n := strings.TrimSpace(name)
		if n == "" {
			continue
		}
		allowed[n] = struct{}{}
	}
	return allowed
}

func (f *Factory) setupExecutionContext(execCtx *executor.Context, stepName string) string {
	name := strings.TrimSpace(stepName)
	if name == "" {
		name = "unknown"
	}
	stepLabel := strings.TrimSpace(cases.Title(language.English).String(name))
	if stepLabel == "" {
		stepLabel = "Unknown"
	}
	execCtx.SendRunning(getStepDisplayName(name))
	execCtx.SendProgress(fmt.Sprintf("Step started: %s", stepLabel))
	return stepLabel
}

func (f *Factory) resolveMaxIterations(inputMax *int) int {
	if inputMax != nil && *inputMax > 0 {
		return *inputMax
	}
	return defaultMaxIterations
}

func (f *Factory) processBlocks(
	ctx context.Context,
	iterationCtx *executor.Context,
	blocks []gateway.ContentBlock,
	registry *tools.Registry,
	allowed map[string]struct{},
	state *loopState,
) (hadToolCalls bool, pendingNarration []string) {
	hadToolCalls = false
	pendingNarration = make([]string, 0)
	for i := range blocks {
		block := blocks[i]
		switch block.Kind {
		case gateway.ContentBlockReasoning:
			if text := strings.TrimSpace(block.Text); text != "" {
				pendingNarration = append(pendingNarration, text)
				state.transcript = append(state.transcript, transcriptEntry{Kind: "reasoning", Text: text})
			}
		case gateway.ContentBlockText:
			if text := strings.TrimSpace(block.Text); text != "" {
				pendingNarration = append(pendingNarration, text)
				state.transcript = append(state.transcript, transcriptEntry{Kind: "text", Text: text})
			}
		case gateway.ContentBlockToolCall:
			if block.ToolCall == nil {
				continue
			}
			hadToolCalls = true
			f.handleToolCallBlock(ctx, iterationCtx, block.ToolCall, registry, allowed, state, &pendingNarration)
		}
	}
	return hadToolCalls, pendingNarration
}

func (f *Factory) handleToolCallBlock(
	ctx context.Context,
	iterationCtx *executor.Context,
	call *gateway.ToolCall,
	registry *tools.Registry,
	allowed map[string]struct{},
	state *loopState,
	pendingNarration *[]string,
) {
	args := call.Arguments
	if args == nil {
		args = map[string]any{}
	}

	// Display agent's narration before tool call if present
	narration := strings.Join(*pendingNarration, "\n\n")
	if strings.TrimSpace(narration) != "" {
		iterationCtx.SendProgress(strings.TrimSpace(narration))
	}
	*pendingNarration = (*pendingNarration)[:0]

	title := formatToolCallTitle(call.Name, args)
	details := formatToolCallDetails(*call, args, narration)
	iterationCtx.SendProgressWithDetails(title, details)
	toolCtx := iterationCtx.Child(title)

	result := f.executeToolWithSafety(ctx, toolCtx, registry, allowed, call.Name, args)

	state.toolResults = append(state.toolResults, toolResult{ID: call.ID, Name: call.Name, Args: args, Result: result})
	state.transcript = append(state.transcript, transcriptEntry{Kind: "tool_call", ToolName: call.Name, ToolID: call.ID, ToolArgs: args})
	state.transcript = append(state.transcript, transcriptEntry{Kind: "tool_result", ToolName: call.Name, ToolID: call.ID, ToolResult: result})
}

func (f *Factory) executeToolWithSafety(
	ctx context.Context,
	toolCtx *executor.Context,
	registry *tools.Registry,
	allowed map[string]struct{},
	name string,
	args map[string]any,
) any {
	_, toolAllowed := allowed[name]
	var (
		result  any
		toolErr error
	)
	if !toolAllowed {
		toolErr = fmt.Errorf("tool not allowed in this step: %s", name)
		result = map[string]any{"tool": name, "error": toolErr.Error()}
	} else {
		result, toolErr = registry.ExecuteTool(ctx, toolCtx, name, args)
	}
	if toolErr != nil {
		// Tool errors are expected sometimes (e.g., guessed paths). Don't fail the
		// activity/step; return the error back to the model as a normal tool result.
		toolCtx.SendProgressWithDetails("Tool returned error", toolErr.Error())
		result = map[string]any{"tool": name, "error": toolErr.Error()}
	} else {
		// Show tool result summary
		resultSummary := formatToolResult(name, result)
		if resultSummary != "" {
			toolCtx.SendCompletedWithDetails("Result", resultSummary)
		}
	}
	return result
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
	if input.Preset == nil {
		return fmt.Errorf("preset is nil")
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

	return append(messages, buildTranscriptMessages(state.transcript)...)
}

func buildTranscriptMessages(transcript []transcriptEntry) []gateway.Message {
	messages := make([]gateway.Message, 0, len(transcript))
	for i := range transcript {
		tr := &transcript[i]
		switch tr.Kind {
		case "text", "reasoning":
			if msg, ok := mapTextTranscriptEntry(tr); ok {
				messages = append(messages, msg)
			}
		case "tool_call":
			if msg, ok := mapToolCallTranscriptEntry(tr); ok {
				messages = append(messages, msg)
			}
		case "tool_result":
			if msg, ok := mapToolResultTranscriptEntry(tr); ok {
				messages = append(messages, msg)
			}
		}
	}
	return messages
}

func mapTextTranscriptEntry(tr *transcriptEntry) (gateway.Message, bool) {
	if strings.TrimSpace(tr.Text) == "" {
		return gateway.Message{}, false
	}
	return gateway.Message{Role: gateway.MessageRoleAssistant, Content: strings.TrimSpace(tr.Text)}, true
}

func mapToolCallTranscriptEntry(tr *transcriptEntry) (gateway.Message, bool) {
	args := map[string]any{}
	if tr.ToolArgs != nil {
		if m, ok := tr.ToolArgs.(map[string]any); ok {
			args = m
		}
	}
	calls := []gateway.ToolCall{{ID: tr.ToolID, Name: tr.ToolName, Arguments: args}}
	return gateway.Message{Role: gateway.MessageRoleAssistant, ToolCalls: calls}, true
}

func mapToolResultTranscriptEntry(tr *transcriptEntry) (gateway.Message, bool) {
	resultJSON := "{}"
	if tr.ToolResult != nil {
		if b, err := json.Marshal(tr.ToolResult); err == nil {
			resultJSON = string(b)
		}
	}
	return gateway.Message{Role: gateway.MessageRoleTool, ToolCallID: tr.ToolID, ToolName: tr.ToolName, Content: resultJSON}, true
}

func (f *Factory) invokeLLM(
	ctx context.Context,
	execCtx *executor.Context,
	resolvedPreset *preset.ResolvedPreset,
	systemPrompt string,
	userPrompt string,
	messages []gateway.Message,
	toolDefs []gateway.ToolDefinition,
) (*draftcontent.Output, error) {
	activity := f.invokeLLMFactory.NewActivity()
	out, err := activity(ctx, execCtx, &draftcontent.Input{
		Preset:       resolvedPreset,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Messages:     messages,
		Tools:        toolDefs,
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
	case toolDirList, toolFileRead, toolFileWrite, toolFileEdit, toolFileMove, toolFileDelete, toolGitUndo:
		return formatFileToolTitle(name, args)
	case toolSearchSemantic, toolSearchText:
		return formatSearchToolTitle(name, args)
	case toolShellExec:
		return formatShellToolTitle(args)
	case "git_diff":
		return formatGitDiffTitle(args)
	case toolPlanInit, toolPlanRead, toolPlanUpdateTask:
		return formatPlanToolTitle(name, args)
	case toolMemStore:
		return formatMemStoreTitle(args)
	case "util_summarize":
		return formatSummarizeTitle(args)
	case "agent_restart":
		return "Requesting a restart"
	default:
		return fmt.Sprintf("Using %s", name)
	}
}

func formatGitDiffTitle(args map[string]any) string {
	staged, ok := args["staged"].(bool)
	if !ok {
		staged = false
	}
	if staged {
		return "Getting staged diff"
	}
	return "Getting diff"
}

func formatMemStoreTitle(args map[string]any) string {
	fact, ok := args["fact"].(string)
	if !ok {
		fact = ""
	}
	fact = strings.TrimSpace(fact)
	if fact == "" {
		return "Memorizing"
	}
	return fmt.Sprintf("Memorizing: %s", truncateOneLine(fact, 80))
}

func formatSummarizeTitle(args map[string]any) string {
	t, ok := args["type"].(string)
	if !ok {
		t = ""
	}
	t = strings.TrimSpace(t)
	if t == "" {
		return "Summarizing"
	}
	return fmt.Sprintf("Summarizing (%s)", t)
}

func formatFileToolTitle(name string, args map[string]any) string {
	switch name {
	case toolDirList:
		return formatDirListTitle(args)
	case toolFileRead:
		return formatFileReadTitle(args)
	case toolFileWrite:
		return formatFileWriteTitle(args)
	case toolFileEdit:
		return formatFileEditTitle(args)
	case toolFileMove:
		return formatFileMoveTitle(args)
	case toolFileDelete:
		return formatFileDeleteTitle(args)
	case toolGitUndo:
		return formatGitUndoTitle(args)
	default:
		return "File operation"
	}
}

func formatDirListTitle(args map[string]any) string {
	dir, ok := args["dir"].(string)
	if !ok {
		dir = ""
	}
	dir = strings.TrimSpace(dir)
	if dir == "" || dir == "." {
		return "Listing (root)"
	}
	return fmt.Sprintf("Listing %s", dir)
}

func formatFileReadTitle(args map[string]any) string {
	p, ok := args["path"].(string)
	if !ok {
		p = ""
	}
	p = strings.TrimSpace(p)
	if p == "" {
		return "Reading a file"
	}
	return fmt.Sprintf("Reading %s", p)
}

func formatFileWriteTitle(args map[string]any) string {
	p, ok := args["path"].(string)
	if !ok {
		p = ""
	}
	p = strings.TrimSpace(p)
	if p == "" {
		return "Writing a file"
	}
	return fmt.Sprintf("Writing %s", p)
}

func formatFileEditTitle(args map[string]any) string {
	p, ok := args["path"].(string)
	if !ok {
		p = ""
	}
	p = strings.TrimSpace(p)
	if p == "" {
		return "Editing a file"
	}
	return fmt.Sprintf("Editing %s", p)
}

func formatFileMoveTitle(args map[string]any) string {
	src, ok := args["source_path"].(string)
	if !ok {
		src = ""
	}
	dst, ok2 := args["dest_path"].(string)
	if !ok2 {
		dst = ""
	}
	src = strings.TrimSpace(src)
	dst = strings.TrimSpace(dst)
	if src != "" && dst != "" {
		return fmt.Sprintf("Moving %s → %s", src, dst)
	}
	return "Moving a file"
}

func formatFileDeleteTitle(args map[string]any) string {
	p, ok := args["path"].(string)
	if !ok {
		p = ""
	}
	p = strings.TrimSpace(p)
	if p == "" {
		return "Deleting a file"
	}
	return fmt.Sprintf("Deleting %s", p)
}

func formatGitUndoTitle(args map[string]any) string {
	p, ok := args["path"].(string)
	if !ok {
		p = ""
	}
	p = strings.TrimSpace(p)
	if p == "" {
		return "Reverting uncommitted changes"
	}
	return fmt.Sprintf("Reverting %s", p)
}

func formatSearchToolTitle(name string, args map[string]any) string {
	switch name {
	case toolSearchSemantic:
		q, ok := args["query"].(string)
		if !ok {
			q = ""
		}
		q = strings.TrimSpace(q)
		if q == "" {
			return "Searching the codebase"
		}
		return fmt.Sprintf("Searching: %s", truncateOneLine(q, 80))
	case toolSearchText:
		pattern, ok := args["pattern"].(string)
		if !ok {
			pattern = ""
		}
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			return "Searching text"
		}
		return fmt.Sprintf("Searching text: %s", truncateOneLine(pattern, 80))
	default:
		return "Searching"
	}
}

func formatShellToolTitle(args map[string]any) string {
	cmd, ok := args["command"].(string)
	if !ok {
		cmd = ""
	}
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
}

func formatPlanToolTitle(name string, args map[string]any) string {
	switch name {
	case toolPlanInit:
		if rawTasks, ok := args["tasks"].([]any); ok {
			return fmt.Sprintf("Creating a plan (%d tasks)", len(rawTasks))
		}
		return "Creating a plan"
	case toolPlanRead:
		return "Reading plan"
	case toolPlanUpdateTask:
		id, ok := args["id"].(string)
		status, ok2 := args["status"].(string)
		id = strings.TrimSpace(id)
		status = strings.TrimSpace(status)
		if ok && ok2 && id != "" && status != "" {
			return fmt.Sprintf("Updating task %s → %s", id, status)
		}
		return "Updating a task"
	default:
		return "Plan operation"
	}
}

func formatToolCallDetails(call gateway.ToolCall, args map[string]any, narration string) string {
	var parts []string

	// Add agent's narration/thinking if present
	narration = strings.TrimSpace(narration)
	if narration != "" {
		parts = append(parts, narration)
	}

	// Add tool call arguments (abbreviated for common cases)
	if len(args) > 0 {
		switch call.Name {
		case toolFileRead, toolFileWrite, toolFileEdit, toolDirList:
			parts = append(parts, formatFileToolDetails(call.Name, args)...)
		case toolSearchSemantic, toolSearchText:
			parts = append(parts, formatSearchToolDetails(args)...)
		case toolShellExec:
			parts = append(parts, formatShellToolDetails(args)...)
		case toolMemStore:
			if fact, ok := args["fact"].(string); ok && fact != "" {
				parts = append(parts, truncateOneLine(fact, 120))
			}
		case toolPlanInit:
			if rawTasks, ok := args["tasks"].([]any); ok {
				parts = append(parts, fmt.Sprintf("%d tasks", len(rawTasks)))
			}
		case toolPlanUpdateTask:
			if info := formatPlanUpdateTaskDetails(args); info != "" {
				parts = append(parts, info)
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n")
}

func formatFileToolDetails(name string, args map[string]any) []string {
	switch name {
	case toolFileRead:
		return formatFileReadDetails(args)
	case toolFileWrite, toolFileEdit:
		return formatFileWriteOrEditDetails(args)
	case toolDirList:
		return formatDirListDetails(args)
	default:
		return nil
	}
}

func formatFileReadDetails(args map[string]any) []string {
	var parts []string
	if path, ok := args["path"].(string); ok && path != "" {
		parts = append(parts, path)
		if startLine, ok := args["start_line"].(float64); ok {
			if endLine, ok := args["end_line"].(float64); ok {
				parts = append(parts, fmt.Sprintf("Lines %d-%d", int(startLine), int(endLine)))
			}
		}
	}
	return parts
}

func formatFileWriteOrEditDetails(args map[string]any) []string {
	if path, ok := args["path"].(string); ok && path != "" {
		return []string{path}
	}
	return nil
}

func formatDirListDetails(args map[string]any) []string {
	if dir, ok := args["dir"].(string); ok && dir != "" {
		return []string{dir}
	}
	return nil
}

func formatSearchToolDetails(args map[string]any) []string {
	var parts []string
	if q, ok := args["query"].(string); ok && q != "" {
		parts = append(parts, truncateOneLine(q, 120))
	} else if pattern, ok := args["pattern"].(string); ok && pattern != "" {
		parts = append(parts, truncateOneLine(pattern, 120))
	}
	return parts
}

func formatShellToolDetails(args map[string]any) []string {
	cmd, ok := args["command"].(string)
	if !ok || cmd == "" {
		return nil
	}

	rawArgs, ok := args["args"].([]any)
	if !ok || len(rawArgs) == 0 {
		return []string{cmd}
	}

	argStrs := make([]string, 0, len(rawArgs))
	for i := range rawArgs {
		if s, ok := rawArgs[i].(string); ok {
			argStrs = append(argStrs, s)
		}
	}
	return []string{fmt.Sprintf("%s %s", cmd, strings.Join(argStrs, " "))}
}

func formatPlanUpdateTaskDetails(args map[string]any) string {
	var taskInfo []string
	if id, ok := args["id"].(string); ok && id != "" {
		taskInfo = append(taskInfo, fmt.Sprintf("task=%s", id))
	}
	if status, ok := args["status"].(string); ok && status != "" {
		taskInfo = append(taskInfo, fmt.Sprintf("status=%s", status))
	}
	if len(taskInfo) > 0 {
		return strings.Join(taskInfo, ", ")
	}
	return ""
}

func formatToolResult(toolName string, result any) string {
	if result == nil {
		return ""
	}

	switch toolName {
	case toolDirList, toolFileRead, toolFileWrite, toolFileEdit, "file_move", "file_delete", "git_undo":
		return formatFileToolResult(toolName, result)
	case toolSearchSemantic, toolSearchText:
		return formatSearchToolResult(toolName, result)
	case toolShellExec:
		return formatShellToolResult(result)
	case toolMemStore:
		return "Fact memorized"
	case toolPlanInit, "plan_read", toolPlanUpdateTask:
		return formatPlanToolResult(toolName, result)
	case "util_summarize":
		return formatSummarizeResult(result)
	}

	return ""
}

func formatShellToolResult(result any) string {
	if resultMap, ok := result.(map[string]any); ok {
		if exitCode, ok := resultMap["exit_code"].(float64); ok {
			if exitCode == 0 {
				return "Command executed successfully"
			}
			return fmt.Sprintf("Command exited with code %d", int(exitCode))
		}
	}
	return ""
}

func formatSummarizeResult(result any) string {
	if resultMap, ok := result.(map[string]any); ok {
		if summary, ok := resultMap["summary"].(string); ok {
			return fmt.Sprintf("Summary: %s", truncateOneLine(summary, 100))
		}
	}
	return ""
}

func formatFileToolResult(toolName string, result any) string {
	resultMap, ok := result.(map[string]any)
	if !ok {
		return ""
	}

	switch toolName {
	case toolDirList:
		return formatDirListResult(resultMap)
	case toolFileRead:
		return formatFileReadResult(resultMap)
	case toolFileWrite:
		return formatFileWriteResult(resultMap)
	case toolFileEdit:
		return formatFileEditResult(resultMap)
	case toolFileMove:
		return formatFileMoveResult(resultMap)
	case toolFileDelete:
		return formatFileDeleteResult(resultMap)
	case toolGitUndo:
		return formatGitUndoResult(resultMap)
	default:
		return ""
	}
}

func getAnyKey(m map[string]any, keys ...string) (any, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v, true
		}
	}
	return nil, false
}

func formatDirListResult(resultMap map[string]any) string {
	if val, ok := getAnyKey(resultMap, "Files", "files"); ok {
		if files, ok := val.([]any); ok {
			return fmt.Sprintf("Listed %d entries", len(files))
		}
	}
	return ""
}

func formatFileReadResult(resultMap map[string]any) string {
	if val, ok := getAnyKey(resultMap, "Content", "content"); ok {
		if content, ok := val.(string); ok {
			lines := strings.Count(content, "\n") + 1
			return fmt.Sprintf("Read %d lines (%d bytes)", lines, len(content))
		}
	}
	return ""
}

func formatFileWriteResult(resultMap map[string]any) string {
	if val, ok := getAnyKey(resultMap, "Written", "written"); ok {
		if written, ok := val.(bool); ok && written {
			return "File written successfully"
		}
	}
	return ""
}

func formatFileEditResult(resultMap map[string]any) string {
	if val, ok := getAnyKey(resultMap, "Applied", "applied"); ok {
		if applied, ok := val.(bool); ok && applied {
			return "Edit applied successfully"
		}
	}
	return ""
}

func formatFileMoveResult(resultMap map[string]any) string {
	if val, ok := getAnyKey(resultMap, "Moved", "moved"); ok {
		if moved, ok := val.(bool); ok && moved {
			return "File moved successfully"
		}
	}
	return ""
}

func formatFileDeleteResult(resultMap map[string]any) string {
	if val, ok := getAnyKey(resultMap, "Deleted", "deleted"); ok {
		if deleted, ok := val.(bool); ok && deleted {
			return "File deleted successfully"
		}
	}
	return ""
}

func formatGitUndoResult(resultMap map[string]any) string {
	if val, ok := getAnyKey(resultMap, "Restored", "restored"); ok {
		if restored, ok := val.(bool); ok && restored {
			return "Changes reverted successfully"
		}
	}
	return ""
}

func formatSearchToolResult(toolName string, result any) string {
	resultMap, ok := result.(map[string]any)
	if !ok {
		return ""
	}

	switch toolName {
	case toolSearchSemantic:
		if chunks, ok := resultMap["Chunks"].([]any); ok {
			return fmt.Sprintf("Found %d relevant code chunks", len(chunks))
		} else if chunks, ok := resultMap["chunks"].([]any); ok {
			return fmt.Sprintf("Found %d relevant code chunks", len(chunks))
		}
	case toolSearchText:
		if matches, ok := resultMap["matches"].([]any); ok {
			return fmt.Sprintf("Found %d matches", len(matches))
		} else if output, ok := resultMap["output"].(string); ok {
			matchCount := strings.Count(output, "\n")
			if matchCount > 0 {
				return fmt.Sprintf("Found matches in %d lines", matchCount)
			}
		}
	}
	return ""
}

func formatPlanToolResult(toolName string, result any) string {
	resultMap, ok := result.(map[string]any)
	if !ok {
		return ""
	}

	switch toolName {
	case toolPlanInit:
		if tasks, ok := resultMap["tasks"].([]any); ok {
			return fmt.Sprintf("Plan created with %d tasks", len(tasks))
		}
	case toolPlanRead:
		if tasks, ok := resultMap["tasks"].([]any); ok {
			return fmt.Sprintf("Retrieved %d tasks", len(tasks))
		}
	case toolPlanUpdateTask:
		return "Task updated"
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
