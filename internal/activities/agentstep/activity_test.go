// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package agentstep

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/retran/meowg1k/internal/activities/invokellm"
	agentconfig "github.com/retran/meowg1k/internal/core/agent"
	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/pkg/executor"
)

type mockInvokeFactory struct {
	outputs []*invokellm.Output
	index   int
}

func (m *mockInvokeFactory) NewActivity() executor.Activity[*invokellm.Input, *invokellm.Output] {
	return func(_ context.Context, _ *executor.Context, _ *invokellm.Input) (*invokellm.Output, error) {
		if m.index >= len(m.outputs) {
			return &invokellm.Output{Content: `{"type":"final","content":"done"}`}, nil
		}
		out := m.outputs[m.index]
		m.index++
		return out, nil
	}
}

type mockToolRunner struct {
	calls int
}

func (m *mockToolRunner) RunTool(_ context.Context, _ *executor.Context, tool, mode string, _ json.RawMessage, _ *profile.ResolvedProfile, _ string) (*ToolResult, error) {
	m.calls++
	return &ToolResult{Tool: tool, Mode: mode, Data: map[string]string{"ok": "true"}}, nil
}

func stringPtr(value string) *string {
	return &value
}

func TestAgentStepToolThenFinal(t *testing.T) {
	factory, err := NewFactory(&mockInvokeFactory{
		outputs: []*invokellm.Output{
			{Content: `{"type":"tool","tool":"plan","mode":"list","params":{}}`},
			{Content: `{"type":"final","content":"all good","summary":"done"}`},
		},
	})
	if err != nil {
		t.Fatalf("failed to create factory: %v", err)
	}

	stepConfig := &agentconfig.StepConfig{
		SystemPrompt: "Plan step",
		Tools:        []string{"plan"},
		ToolModes: map[string]map[string]bool{
			"plan": {"list": true},
		},
		Index: 1,
	}

	exec := executor.NewExecutor(1)
	execCtx := executor.NewContext("agent-step", executor.NoOpFeedbackHandler, exec)

	runner := &mockToolRunner{}
	activity := factory.NewActivity()
	output, err := activity(context.Background(), execCtx, &Input{
		Goal:         stringPtr("Test goal"),
		Profile:      &profile.ResolvedProfile{Model: "test"},
		SystemPrompt: stringPtr("Plan step"),
		StepConfig:   stepConfig,
		ToolRunner:   runner,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if output.Summary != "done" {
		t.Fatalf("expected summary 'done', got %q", output.Summary)
	}
	if runner.calls != 1 {
		t.Fatalf("expected tool runner to be called once, got %d", runner.calls)
	}
}

func TestAgentStepToolCallResponse(t *testing.T) {
	factory, err := NewFactory(&mockInvokeFactory{
		outputs: []*invokellm.Output{
			{
				ToolCalls: []domainGateway.ToolCall{
					{
						ID:        "call-1",
						Name:      "workspace_read",
						Arguments: map[string]any{"path": "README.md"},
					},
				},
			},
			{Content: `{"type":"final","content":"done","summary":"ok"}`},
		},
	})
	if err != nil {
		t.Fatalf("failed to create factory: %v", err)
	}

	stepConfig := &agentconfig.StepConfig{
		SystemPrompt: "Plan step",
		Tools:        []string{"workspace"},
		ToolModes: map[string]map[string]bool{
			"workspace": {"read": true},
		},
		Index: 1,
	}

	exec := executor.NewExecutor(1)
	execCtx := executor.NewContext("agent-step", executor.NoOpFeedbackHandler, exec)

	runner := &mockToolRunner{}
	activity := factory.NewActivity()
	output, err := activity(context.Background(), execCtx, &Input{
		Goal:         stringPtr("Test goal"),
		Profile:      &profile.ResolvedProfile{Model: "test"},
		SystemPrompt: stringPtr("Plan step"),
		StepConfig:   stepConfig,
		ToolRunner:   runner,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if output.Summary != "ok" {
		t.Fatalf("expected summary 'ok', got %q", output.Summary)
	}
	if runner.calls != 1 {
		t.Fatalf("expected tool runner to be called once, got %d", runner.calls)
	}
}

func TestAgentStepNonJSONResponseWithTools(t *testing.T) {
	factory, err := NewFactory(&mockInvokeFactory{
		outputs: []*invokellm.Output{
			{Content: "Not JSON"},
		},
	})
	if err != nil {
		t.Fatalf("failed to create factory: %v", err)
	}

	stepConfig := &agentconfig.StepConfig{
		SystemPrompt: "Plan step",
		Tools:        []string{"plan"},
		ToolModes: map[string]map[string]bool{
			"plan": {"list": true},
		},
		Index: 1,
	}

	exec := executor.NewExecutor(1)
	execCtx := executor.NewContext("agent-step", executor.NoOpFeedbackHandler, exec)

	runner := &mockToolRunner{}
	activity := factory.NewActivity()
	output, err := activity(context.Background(), execCtx, &Input{
		Goal:         stringPtr("Test goal"),
		Profile:      &profile.ResolvedProfile{Model: "test"},
		SystemPrompt: stringPtr("Plan step"),
		StepConfig:   stepConfig,
		ToolRunner:   runner,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if output.Summary != "Not JSON" {
		t.Fatalf("expected summary 'Not JSON', got %q", output.Summary)
	}
	if runner.calls != 0 {
		t.Fatalf("expected tool runner to not be called, got %d", runner.calls)
	}
}

func TestAgentStepDisallowedTool(t *testing.T) {
	factory, err := NewFactory(&mockInvokeFactory{
		outputs: []*invokellm.Output{
			{Content: `{"type":"tool","tool":"workspace","mode":"read","params":{"path":"a"}}`},
		},
	})
	if err != nil {
		t.Fatalf("failed to create factory: %v", err)
	}

	stepConfig := &agentconfig.StepConfig{
		SystemPrompt: "Plan step",
		Tools:        []string{"plan"},
		ToolModes: map[string]map[string]bool{
			"plan": {"list": true},
		},
		Index: 1,
	}

	exec := executor.NewExecutor(1)
	execCtx := executor.NewContext("agent-step", executor.NoOpFeedbackHandler, exec)

	activity := factory.NewActivity()
	_, err = activity(context.Background(), execCtx, &Input{
		Goal:         stringPtr("Test goal"),
		Profile:      &profile.ResolvedProfile{Model: "test"},
		SystemPrompt: stringPtr("Plan step"),
		StepConfig:   stepConfig,
		ToolRunner:   &mockToolRunner{},
	})
	if err == nil {
		t.Fatal("expected error for disallowed tool")
	}
}

func TestParseAgentResponseWithCodeBlock(t *testing.T) {
	content := "```json\n{\"type\":\"final\",\"content\":\"ok\"}\n```"
	parsed, err := parseAgentResponse(content)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if parsed.Type != "final" {
		t.Fatalf("expected type final, got %q", parsed.Type)
	}
}

func TestParseAgentResponseMissingType(t *testing.T) {
	content := "{\"content\":\"ok\"}"
	_, err := parseAgentResponse(content)
	if err == nil {
		t.Fatal("expected error for missing type")
	}
}
