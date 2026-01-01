package agentloop

import (
	"context"
	"testing"

	"github.com/retran/meowg1k/internal/activities/draftcontent"
	"github.com/retran/meowg1k/internal/core/agent/tools"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/pkg/executor"
)

type fakeDraftFactory struct {
	inputs    []*draftcontent.Input
	responses []*draftcontent.Output
	calls     int
}

func (f *fakeDraftFactory) NewActivity() executor.Activity[*draftcontent.Input, *draftcontent.Output] {
	return func(ctx context.Context, execCtx *executor.Context, input *draftcontent.Input) (*draftcontent.Output, error) {
		_ = ctx
		_ = execCtx
		f.calls++
		f.inputs = append(f.inputs, input)
		if len(f.responses) == 0 {
			return &draftcontent.Output{Response: &gateway.GenerateContentResponse{}}, nil
		}
		resp := f.responses[0]
		f.responses = f.responses[1:]
		return resp, nil
	}
}

func TestAgentLoop_ContinuesAfterToolCallsEvenWithText(t *testing.T) {
	ctx := context.Background()
	exec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	reg := tools.NewRegistry()
	toolCalled := 0
	reg.Register(tools.Tool{
		Definition: gateway.ToolDefinition{Name: "list_files"},
		Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
			_ = ctx
			_ = execCtx
			_ = args
			toolCalled++
			return map[string]any{"files": []string{"a.go"}}, nil
		},
	})

	fake := &fakeDraftFactory{responses: []*draftcontent.Output{
		{
			Response: &gateway.GenerateContentResponse{Blocks: []gateway.ContentBlock{
				{Kind: gateway.ContentBlockText, Text: "I will inspect the repo."},
				{Kind: gateway.ContentBlockToolCall, ToolCall: &gateway.ToolCall{ID: "1", Name: "list_files", Arguments: map[string]any{}}},
				{Kind: gateway.ContentBlockText, Text: "Next I will read agentloop."},
			}},
		},
		{
			Response: &gateway.GenerateContentResponse{Blocks: []gateway.ContentBlock{
				{Kind: gateway.ContentBlockText, Text: "Final answer."},
			}},
		},
	}}

	factory, err := NewFactory(fake)
	if err != nil {
		t.Fatalf("NewFactory error: %v", err)
	}

	goal := "explain agent implementation"
	maxIt := 5
	out, err := factory.NewActivity()(ctx, execCtx, &Input{
		ToolRegistry:  reg,
		AllowedTools:  []string{"list_files"},
		StepName:      "discover",
		Profile:       &profile.ResolvedProfile{Model: "test", MaxOutputTokens: 32},
		Goal:          &goal,
		MaxIterations: &maxIt,
	})
	if err != nil {
		t.Fatalf("agent loop error: %v", err)
	}
	if out == nil {
		t.Fatalf("expected output")
	}
	if out.Content != "I will inspect the repo.\n\nNext I will read agentloop.\n\nFinal answer." {
		t.Fatalf("unexpected content: %q", out.Content)
	}
	if out.FinalMessage != "Final answer." {
		t.Fatalf("unexpected final message: %q", out.FinalMessage)
	}
	if fake.calls != 2 {
		t.Fatalf("expected 2 LLM calls, got %d", fake.calls)
	}
	if toolCalled != 1 {
		t.Fatalf("expected tool to be called once, got %d", toolCalled)
	}

	if len(fake.inputs) < 2 {
		t.Fatalf("expected at least 2 inputs captured")
	}
	secondMsgs := fake.inputs[1].Messages
	foundToolResult := false
	for _, m := range secondMsgs {
		if m.Role == gateway.MessageRoleTool && m.ToolCallID == "1" && m.ToolName == "list_files" {
			foundToolResult = true
			break
		}
	}
	if !foundToolResult {
		t.Fatalf("expected tool result message in second iteration messages")
	}
}

func TestAgentLoop_NoToolsStepRejectsToolCalls(t *testing.T) {
	ctx := context.Background()
	exec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	reg := tools.NewRegistry()
	toolCalled := 0
	reg.Register(tools.Tool{
		Definition: gateway.ToolDefinition{Name: "list_files"},
		Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
			_ = ctx
			_ = execCtx
			_ = args
			toolCalled++
			return map[string]any{"files": []string{"a.go"}}, nil
		},
	})

	fake := &fakeDraftFactory{responses: []*draftcontent.Output{
		{
			Response: &gateway.GenerateContentResponse{Blocks: []gateway.ContentBlock{
				{Kind: gateway.ContentBlockText, Text: "I will try to use a tool."},
				{Kind: gateway.ContentBlockToolCall, ToolCall: &gateway.ToolCall{ID: "1", Name: "list_files", Arguments: map[string]any{}}},
			}},
		},
		{
			Response: &gateway.GenerateContentResponse{Blocks: []gateway.ContentBlock{
				{Kind: gateway.ContentBlockText, Text: "Final answer without tools."},
			}},
		},
	}}

	factory, err := NewFactory(fake)
	if err != nil {
		t.Fatalf("NewFactory error: %v", err)
	}

	goal := "report"
	maxIt := 5
	out, err := factory.NewActivity()(ctx, execCtx, &Input{
		ToolRegistry:  reg,
		AllowedTools:  nil,
		StepName:      "final",
		Profile:       &profile.ResolvedProfile{Model: "test", MaxOutputTokens: 32},
		Goal:          &goal,
		MaxIterations: &maxIt,
	})
	if err != nil {
		t.Fatalf("agent loop error: %v", err)
	}
	if out == nil {
		t.Fatalf("expected output")
	}
	if toolCalled != 0 {
		t.Fatalf("expected tool not to be executed, got %d", toolCalled)
	}
	if out.Content != "I will try to use a tool.\n\nFinal answer without tools." {
		t.Fatalf("unexpected content: %q", out.Content)
	}
	if out.FinalMessage != "Final answer without tools." {
		t.Fatalf("unexpected final message: %q", out.FinalMessage)
	}
}
