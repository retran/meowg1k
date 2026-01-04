// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package agentloop

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/internal/core/agent/tools"
	"github.com/retran/meowg1k/internal/activities/draftcontent"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/pkg/executor"
)

func TestValidateInputErrors(t *testing.T) {
	err := validateInput(nil, nil)
	assert.Error(t, err)

	factory := &Factory{}
	err = validateInput(factory, nil)
	assert.Error(t, err)

	err = validateInput(factory, &Input{})
	assert.Error(t, err)

	err = validateInput(factory, &Input{Preset: &preset.ResolvedPreset{Model: "m"}})
	assert.Error(t, err)
}

func TestPrepareAllowedTools(t *testing.T) {
	factory := &Factory{}
	allowed := factory.prepareAllowedTools([]string{" read_file ", "", "search_code"})
	assert.Contains(t, allowed, "read_file")
	assert.Contains(t, allowed, "search_code")
}

func TestSetupExecutionContext(t *testing.T) {
	factory := &Factory{}
	exec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)
	label := factory.setupExecutionContext(execCtx, "plan")
	assert.Equal(t, "Plan", label)

	label = factory.setupExecutionContext(execCtx, " ")
	assert.Equal(t, "Unknown", label)
}

func TestResolveMaxIterations(t *testing.T) {
	factory := &Factory{}
	assert.Equal(t, defaultMaxIterations, factory.resolveMaxIterations(nil))

	zero := 0
	assert.Equal(t, defaultMaxIterations, factory.resolveMaxIterations(&zero))

	two := 2
	assert.Equal(t, 2, factory.resolveMaxIterations(&two))
}

func TestRenderTranscriptOutput(t *testing.T) {
	output := renderTranscriptOutput([]transcriptEntry{
		{Kind: "text", Text: "first"},
		{Kind: "reasoning", Text: "second"},
		{Kind: "tool_call", ToolName: "read_file"},
	})
	assert.Equal(t, "first\n\nsecond", output)
	assert.Equal(t, "", renderTranscriptOutput(nil))
}

func TestBuildUserPromptAndMessages(t *testing.T) {
	state := &loopState{
		toolResults: []toolResult{{ID: "1", Name: "read_file", Result: map[string]any{"ok": true}}},
		transcript: []transcriptEntry{
			{Kind: "text", Text: "hello"},
			{Kind: "tool_call", ToolName: "read_file", ToolID: "1", ToolArgs: map[string]any{"path": "a.txt"}},
			{Kind: "tool_result", ToolName: "read_file", ToolID: "1", ToolResult: map[string]any{"content": "hi"}},
		},
	}
	prompt := buildUserPrompt("goal", []string{"sum1"}, state)
	assert.Contains(t, prompt, "Goal:")
	assert.Contains(t, prompt, "PriorSummaries:")
	assert.Contains(t, prompt, "ToolResults:")
	assert.Contains(t, prompt, "Transcript:")

	msgs := buildMessages("goal", []string{"sum1"}, state)
	require.Len(t, msgs, 4)
	assert.Equal(t, gateway.MessageRoleUser, msgs[0].Role)
	assert.Equal(t, gateway.MessageRoleAssistant, msgs[1].Role)
	assert.Equal(t, gateway.MessageRoleAssistant, msgs[2].Role)
	assert.Equal(t, gateway.MessageRoleTool, msgs[3].Role)
}

func TestBuildTranscriptMessages(t *testing.T) {
	transcript := []transcriptEntry{
		{Kind: "text", Text: "hello"},
		{Kind: "tool_call", ToolName: "read_file", ToolID: "1", ToolArgs: map[string]any{"path": "a.txt"}},
		{Kind: "tool_result", ToolName: "read_file", ToolID: "1", ToolResult: map[string]any{"content": "hi"}},
	}
	msgs := buildTranscriptMessages(transcript)
	require.Len(t, msgs, 3)
	assert.Equal(t, gateway.MessageRoleAssistant, msgs[0].Role)
	assert.Equal(t, gateway.MessageRoleAssistant, msgs[1].Role)
	assert.Equal(t, gateway.MessageRoleTool, msgs[2].Role)
}

func TestApplyToolDescriptionOverrides(t *testing.T) {
	defs := []gateway.ToolDefinition{
		{Name: "read_file", Description: "old"},
		{Name: "search_code", Description: "old"},
	}
	applyToolDescriptionOverrides(defs, map[string]string{
		"READ_FILE":  "new",
		"  ":         "skip",
		"search_code": "newer",
	})
	assert.Equal(t, "new", defs[0].Description)
	assert.Equal(t, "newer", defs[1].Description)
}

func TestFormatToolCallTitleAndDetails(t *testing.T) {
	title := formatToolCallTitle(toolFileRead, map[string]any{"path": "a.txt"})
	assert.Equal(t, "Reading a.txt", title)

	details := formatToolCallDetails(gateway.ToolCall{Name: toolFileRead}, map[string]any{
		"path":       "a.txt",
		"start_line": float64(1),
		"end_line":   float64(2),
	}, "thinking")
	assert.Contains(t, details, "thinking")
	assert.Contains(t, details, "a.txt")
	assert.Contains(t, details, "Lines 1-2")

	title = formatToolCallTitle("", map[string]any{})
	assert.Equal(t, "Using a tool", title)
}

func TestFormatToolResult(t *testing.T) {
	result := formatToolResult(toolDirList, map[string]any{"files": []any{"a", "b"}})
	assert.Contains(t, result, "Listed 2 entries")

	result = formatToolResult(toolFileRead, map[string]any{"content": "a\nb"})
	assert.Contains(t, result, "Read 2 lines")

	result = formatToolResult(toolShellExec, map[string]any{"exit_code": float64(1)})
	assert.Equal(t, "Command exited with code 1", result)

	result = formatToolResult(toolMemStore, map[string]any{})
	assert.Equal(t, "Fact memorized", result)

	result = formatToolResult(toolPlanInit, map[string]any{"tasks": []any{"a", "b"}})
	assert.Equal(t, "Plan created with 2 tasks", result)

	result = formatToolResult("util_summarize", map[string]any{"summary": "short summary"})
	assert.Contains(t, result, "Summary: short summary")
}

func TestFormatToolTitles(t *testing.T) {
	assert.Equal(t, "Getting staged diff", formatGitDiffTitle(map[string]any{"staged": true}))
	assert.Equal(t, "Getting diff", formatGitDiffTitle(map[string]any{}))

	assert.Equal(t, "Memorizing: hello", formatMemStoreTitle(map[string]any{"fact": "hello"}))
	assert.Equal(t, "Summarizing (diff)", formatSummarizeTitle(map[string]any{"type": "diff"}))

	assert.Equal(t, "Listing (root)", formatDirListTitle(map[string]any{"dir": "."}))
	assert.Equal(t, "Writing a file", formatFileWriteTitle(map[string]any{}))
	assert.Contains(t, formatFileMoveTitle(map[string]any{"source_path": "a.txt", "dest_path": "b.txt"}), "Moving")
	assert.Equal(t, "Reverting uncommitted changes", formatGitUndoTitle(map[string]any{}))

	assert.Contains(t, formatSearchToolTitle(toolSearchSemantic, map[string]any{"query": "foo"}), "Searching:")
	assert.Contains(t, formatSearchToolTitle(toolSearchText, map[string]any{"pattern": "bar"}), "Searching text:")

	assert.Contains(t, formatShellToolTitle(map[string]any{"command": "ls", "args": []any{"-la"}}), "Running:")
	assert.Equal(t, "Creating a plan (2 tasks)", formatPlanToolTitle(toolPlanInit, map[string]any{"tasks": []any{"a", "b"}}))
	assert.Equal(t, "Updating task T1 → done", formatPlanToolTitle(toolPlanUpdateTask, map[string]any{"id": "T1", "status": "done"}))
}

func TestFormatToolDetails(t *testing.T) {
	args := map[string]any{
		"path":       "a.txt",
		"start_line": float64(1),
		"end_line":   float64(2),
	}
	details := formatFileReadDetails(args)
	assert.Equal(t, []string{"a.txt", "Lines 1-2"}, details)

	assert.Equal(t, []string{"a.txt"}, formatFileWriteOrEditDetails(map[string]any{"path": "a.txt"}))
	assert.Equal(t, []string{"."}, formatDirListDetails(map[string]any{"dir": "."}))
	assert.Equal(t, []string{"query"}, formatSearchToolDetails(map[string]any{"query": "query"}))
	assert.Equal(t, []string{"ls -la"}, formatShellToolDetails(map[string]any{"command": "ls", "args": []any{"-la"}}))
	assert.Equal(t, "task=T1, status=done", formatPlanUpdateTaskDetails(map[string]any{"id": "T1", "status": "done"}))
}

func TestFormatToolCallDetailsPlanUpdate(t *testing.T) {
	details := formatToolCallDetails(gateway.ToolCall{Name: toolPlanUpdateTask}, map[string]any{
		"id":     "T1",
		"status": "done",
	}, "")
	assert.Equal(t, "task=T1, status=done", details)
}

func TestFormatSearchToolResult(t *testing.T) {
	result := formatSearchToolResult(toolSearchSemantic, map[string]any{"chunks": []any{"a"}})
	assert.Equal(t, "Found 1 relevant code chunks", result)

	result = formatSearchToolResult(toolSearchText, map[string]any{"output": "a\nb\n"})
	assert.Equal(t, "Found matches in 2 lines", result)
}

func TestFormatFileToolResult(t *testing.T) {
	assert.Equal(t, "File written successfully", formatFileWriteResult(map[string]any{"written": true}))
	assert.Equal(t, "Edit applied successfully", formatFileEditResult(map[string]any{"applied": true}))
	assert.Equal(t, "File moved successfully", formatFileMoveResult(map[string]any{"moved": true}))
	assert.Equal(t, "File deleted successfully", formatFileDeleteResult(map[string]any{"deleted": true}))
	assert.Equal(t, "Changes reverted successfully", formatGitUndoResult(map[string]any{"restored": true}))
}

func TestFormatPlanToolResult(t *testing.T) {
	assert.Equal(t, "Retrieved 1 tasks", formatPlanToolResult(toolPlanRead, map[string]any{"tasks": []any{"a"}}))
	assert.Equal(t, "Task updated", formatPlanToolResult(toolPlanUpdateTask, map[string]any{"ok": true}))
}

func TestStringHelpers(t *testing.T) {
	assert.Equal(t, "", stringValue(nil))
	value := "x"
	assert.Equal(t, "x", stringValue(&value))

	assert.Nil(t, stringSliceValue(nil))
	parts := []string{"a"}
	assert.Equal(t, []string{"a"}, stringSliceValue(&parts))
}

func TestTruncateOneLine(t *testing.T) {
	out := truncateOneLine("a\nb", 10)
	assert.False(t, strings.Contains(out, "\n"))

	out = truncateOneLine("abcdef", 5)
	assert.True(t, strings.HasSuffix(out, "…"))
}

func TestGetStepDisplayName(t *testing.T) {
	assert.Equal(t, "I'm planning", getStepDisplayName("plan"))
	assert.Equal(t, "I'm working on step custom", getStepDisplayName("custom"))
}

func TestBuildMessagesEmpty(t *testing.T) {
	msgs := buildMessages(" ", nil, nil)
	assert.Len(t, msgs, 0)
}

func TestBuildUserPromptEmptyState(t *testing.T) {
	prompt := buildUserPrompt("goal", nil, nil)
	assert.Contains(t, prompt, "Goal:")
}

func TestSetupExecutionContextUnknown(t *testing.T) {
	factory := &Factory{}
	exec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)
	label := factory.setupExecutionContext(execCtx, "  ")
	assert.Equal(t, "Unknown", label)
}

func TestExecuteToolWithSafetyDisallowed(t *testing.T) {
	factory := &Factory{}
	exec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)
	registry := tools.NewRegistry()
	result := factory.executeToolWithSafety(context.Background(), execCtx, registry, map[string]struct{}{}, "read_file", map[string]any{})
	resultMap, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Contains(t, resultMap["error"], "tool not allowed")
}

func TestProcessBlocksNarration(t *testing.T) {
	factory := &Factory{}
	exec := executor.NewExecutor(0)
	iterCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)
	state := &loopState{}

	blocks := []gateway.ContentBlock{
		{Kind: gateway.ContentBlockReasoning, Text: "think"},
		{Kind: gateway.ContentBlockText, Text: "say"},
	}
	hadTools, pending := factory.processBlocks(context.Background(), iterCtx, blocks, tools.NewRegistry(), map[string]struct{}{}, state)
	assert.False(t, hadTools)
	assert.Equal(t, []string{"think", "say"}, pending)
	assert.Len(t, state.transcript, 2)
}

func TestProcessBlocksToolCall(t *testing.T) {
	factory := &Factory{}
	exec := executor.NewExecutor(0)
	iterCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)
	state := &loopState{}

	registry := tools.NewRegistry()
	registry.Register(tools.Tool{
		Definition: gateway.ToolDefinition{Name: toolFileRead},
		Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
			_ = ctx
			_ = execCtx
			_ = args
			return map[string]any{"content": "line1\nline2"}, nil
		},
	})

	blocks := []gateway.ContentBlock{
		{Kind: gateway.ContentBlockToolCall, ToolCall: &gateway.ToolCall{
			ID:        "1",
			Name:      toolFileRead,
			Arguments: map[string]any{"path": "a.txt"},
		}},
	}
	hadTools, pending := factory.processBlocks(context.Background(), iterCtx, blocks, registry, map[string]struct{}{toolFileRead: {}}, state)
	assert.True(t, hadTools)
	assert.Len(t, pending, 0)
	assert.Len(t, state.toolResults, 1)
	assert.Len(t, state.transcript, 2)
}

func TestExecuteToolWithSafetyError(t *testing.T) {
	factory := &Factory{}
	exec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	registry := tools.NewRegistry()
	registry.Register(tools.Tool{
		Definition: gateway.ToolDefinition{Name: toolFileRead},
		Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
			_ = ctx
			_ = execCtx
			_ = args
			return nil, errors.New("boom")
		},
	})

	result := factory.executeToolWithSafety(context.Background(), execCtx, registry, map[string]struct{}{toolFileRead: {}}, toolFileRead, map[string]any{})
	resultMap, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Contains(t, resultMap["error"], "boom")
}

type fakeDraftContentFactory struct {
	outputs []*draftcontent.Output
	inputs  []*draftcontent.Input
}

func (f *fakeDraftContentFactory) NewActivity() executor.Activity[*draftcontent.Input, *draftcontent.Output] {
	return func(ctx context.Context, execCtx *executor.Context, input *draftcontent.Input) (*draftcontent.Output, error) {
		_ = ctx
		_ = execCtx
		f.inputs = append(f.inputs, input)
		if len(f.outputs) == 0 {
			return &draftcontent.Output{Response: &gateway.GenerateContentResponse{}}, nil
		}
		out := f.outputs[0]
		f.outputs = f.outputs[1:]
		return out, nil
	}
}

func TestRunIterationNilResponse(t *testing.T) {
	fake := &fakeDraftContentFactory{
		outputs: []*draftcontent.Output{{Response: nil}},
	}
	factory, err := NewFactory(fake)
	require.NoError(t, err)

	exec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)
	input := &Input{
		ToolRegistry: tools.NewRegistry(),
		Preset:       &preset.ResolvedPreset{Model: "model"},
		StepName:     "step",
	}

	_, _, err = factory.runIteration(context.Background(), execCtx, 1, input, nil, map[string]struct{}{}, &loopState{}, "Step")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil LLM response")
}

func TestActivityMaxIterationsExceeded(t *testing.T) {
	fake := &fakeDraftContentFactory{
		outputs: []*draftcontent.Output{
			{
				Response: &gateway.GenerateContentResponse{
					Blocks: []gateway.ContentBlock{{
						Kind: gateway.ContentBlockToolCall,
						ToolCall: &gateway.ToolCall{
							ID:        "1",
							Name:      toolFileRead,
							Arguments: map[string]any{},
						},
					}},
				},
			},
		},
	}
	factory, err := NewFactory(fake)
	require.NoError(t, err)

	exec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)
	max := 1
	activity := factory.NewActivity()
	_, err = activity(context.Background(), execCtx, &Input{
		ToolRegistry:  tools.NewRegistry(),
		Preset:        &preset.ResolvedPreset{Model: "model"},
		MaxIterations: &max,
		StepName:      "step",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeded max iterations")
}
