// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package invokellm

import (
	"context"
	"errors"
	"testing"

	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockGenerationGateway is a mock implementation of GenerationGateway for testing.
type mockGenerationGateway struct {
	Err     error
	Content string
}

func (m *mockGenerationGateway) GenerateContent(ctx context.Context, request *domainGateway.GenerateContentRequest) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	return m.Content, nil
}

type mockToolCallingGateway struct {
	Response    *domainGateway.GenerateContentResponse
	ResponseErr error
	GenerateErr error
	Content     string
}

func (m *mockToolCallingGateway) GenerateContent(ctx context.Context, request *domainGateway.GenerateContentRequest) (string, error) {
	if m.GenerateErr != nil {
		return "", m.GenerateErr
	}
	return m.Content, nil
}

func (m *mockToolCallingGateway) GenerateContentWithTools(ctx context.Context, request *domainGateway.GenerateContentRequest, tools []domainGateway.ToolDefinition) (*domainGateway.GenerateContentResponse, error) {
	return m.Response, m.ResponseErr
}

// mockGenerationGatewayFactory is a mock implementation of GenerationGatewayFactory for testing.
type mockGenerationGatewayFactory struct {
	Gateway ports.GenerationGateway
	Err     error
}

func (m *mockGenerationGatewayFactory) NewGenerationGateway(ctx context.Context, resolvedProfile *profile.ResolvedProfile) (ports.GenerationGateway, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Gateway != nil {
		return m.Gateway, nil
	}
	return &mockGenerationGateway{Content: "test content"}, nil
}

func TestNewFactory(t *testing.T) {
	gwFactory := &mockGenerationGatewayFactory{}
	factory, err := NewFactory(gwFactory)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}

	if factory == nil {
		t.Error("NewFactory returned nil")
	}
}

func TestInvokeLLMActivity_Success(t *testing.T) {
	gwFactory := &mockGenerationGatewayFactory{
		Gateway: &mockGenerationGateway{
			Content: "Generated content",
		},
	}

	factory, err := NewFactory(gwFactory)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}
	activity := factory.NewActivity()

	ctx := context.Background()
	executorCtx := executor.NewContext("test", nil, nil)

	input := &Input{
		Profile: &profile.ResolvedProfile{
			Provider: "test",
			Model:    "test-model",
		},
		SystemPrompt: "System prompt",
		UserPrompt:   "User prompt",
	}

	output, err := activity(ctx, executorCtx, input)
	if err != nil {
		t.Errorf("Activity failed: %v", err)
	}

	if output.Content != "Generated content" {
		t.Errorf("Expected 'Generated content', got '%s'", output.Content)
	}
}

func TestInvokeLLMActivity_NilInput(t *testing.T) {
	gwFactory := &mockGenerationGatewayFactory{}
	factory, err := NewFactory(gwFactory)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}

	activity := factory.NewActivity()

	ctx := context.Background()
	executorCtx := executor.NewContext("test", nil, nil)

	_, err = activity(ctx, executorCtx, nil)
	if err == nil {
		t.Error("Expected error for nil input, got nil")
	}
}

func TestInvokeLLMActivity_GatewayError(t *testing.T) {
	gwFactory := &mockGenerationGatewayFactory{
		Err: errors.New("gateway creation failed"),
	}

	factory, err := NewFactory(gwFactory)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}

	activity := factory.NewActivity()

	ctx := context.Background()
	executorCtx := executor.NewContext("test", nil, nil)

	input := &Input{
		Profile: &profile.ResolvedProfile{
			Provider: "test",
			Model:    "test-model",
		},
		SystemPrompt: "System prompt",
		UserPrompt:   "User prompt",
	}

	_, err = activity(ctx, executorCtx, input)
	if err == nil {
		t.Error("Expected error from gateway creation")
	}
}

func TestInvokeLLMActivity_ToolCalls(t *testing.T) {
	gwFactory := &mockGenerationGatewayFactory{
		Gateway: &mockToolCallingGateway{
			Response: &domainGateway.GenerateContentResponse{
				Content: "tool response",
				ToolCalls: []domainGateway.ToolCall{
					{
						ID:        "call-1",
						Name:      "workspace_read",
						Arguments: map[string]any{"path": "README.md"},
					},
				},
			},
		},
	}

	factory, err := NewFactory(gwFactory)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}
	activity := factory.NewActivity()

	ctx := context.Background()
	executorCtx := executor.NewContext("test", nil, nil)

	input := &Input{
		Profile: &profile.ResolvedProfile{
			Provider: "test",
			Model:    "test-model",
		},
		SystemPrompt: "System prompt",
		UserPrompt:   "User prompt",
		Tools: []domainGateway.ToolDefinition{
			{Name: "workspace_read"},
		},
	}

	output, err := activity(ctx, executorCtx, input)
	if err != nil {
		t.Errorf("Activity failed: %v", err)
	}
	if output.Content != "tool response" {
		t.Errorf("Expected content 'tool response', got '%s'", output.Content)
	}
	if len(output.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(output.ToolCalls))
	}
	if output.ToolCalls[0].Name != "workspace_read" {
		t.Fatalf("Expected tool name workspace_read, got %q", output.ToolCalls[0].Name)
	}
}

func TestInvokeLLMActivity_ToolCallsFallback(t *testing.T) {
	gwFactory := &mockGenerationGatewayFactory{
		Gateway: &mockToolCallingGateway{
			Content:     "fallback content",
			ResponseErr: domainGateway.ErrToolCallingNotSupported,
		},
	}

	factory, err := NewFactory(gwFactory)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}
	activity := factory.NewActivity()

	ctx := context.Background()
	executorCtx := executor.NewContext("test", nil, nil)

	input := &Input{
		Profile: &profile.ResolvedProfile{
			Provider: "test",
			Model:    "test-model",
		},
		SystemPrompt: "System prompt",
		UserPrompt:   "User prompt",
		Tools: []domainGateway.ToolDefinition{
			{Name: "workspace_read"},
		},
	}

	output, err := activity(ctx, executorCtx, input)
	if err != nil {
		t.Errorf("Activity failed: %v", err)
	}
	if output.Content != "fallback content" {
		t.Errorf("Expected content 'fallback content', got '%s'", output.Content)
	}
	if len(output.ToolCalls) != 0 {
		t.Fatalf("Expected no tool calls, got %d", len(output.ToolCalls))
	}
}

func TestInvokeLLMActivity_ToolCallsError(t *testing.T) {
	gwFactory := &mockGenerationGatewayFactory{
		Gateway: &mockToolCallingGateway{
			ResponseErr: errors.New("tool error"),
		},
	}

	factory, err := NewFactory(gwFactory)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}
	activity := factory.NewActivity()

	ctx := context.Background()
	executorCtx := executor.NewContext("test", nil, nil)

	input := &Input{
		Profile: &profile.ResolvedProfile{
			Provider: "test",
			Model:    "test-model",
		},
		SystemPrompt: "System prompt",
		UserPrompt:   "User prompt",
		Tools: []domainGateway.ToolDefinition{
			{Name: "workspace_read"},
		},
	}

	_, err = activity(ctx, executorCtx, input)
	if err == nil {
		t.Fatal("Expected error for tool call failure")
	}
}

func TestInvokeLLMActivity_GenerationError(t *testing.T) {
	gwFactory := &mockGenerationGatewayFactory{
		Gateway: &mockGenerationGateway{
			Err: errors.New("generation failed"),
		},
	}

	factory, err := NewFactory(gwFactory)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}

	activity := factory.NewActivity()

	ctx := context.Background()
	executorCtx := executor.NewContext("test", nil, nil)

	input := &Input{
		Profile: &profile.ResolvedProfile{
			Provider: "test",
			Model:    "test-model",
		},
		SystemPrompt: "System prompt",
		UserPrompt:   "User prompt",
	}

	_, err = activity(ctx, executorCtx, input)
	if err == nil {
		t.Error("Expected error from generation")
	}
}

func TestNewFactory_NilGatewayFactory(t *testing.T) {
	_, err := NewFactory(nil)
	if err == nil {
		t.Fatal("expected error for nil gateway factory, got nil")
	}
	expectedMsg := "gateway factory cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestNewActivity_NilFactory(t *testing.T) {
	var factory *Factory
	activity := factory.NewActivity()

	ctx := context.Background()
	executorCtx := executor.NewContext("test", nil, nil)
	input := &Input{
		Profile:      &profile.ResolvedProfile{Provider: "test", Model: "test-model"},
		SystemPrompt: "system",
		UserPrompt:   "user",
	}

	_, err := activity(ctx, executorCtx, input)
	if err == nil {
		t.Fatal("expected error for nil factory, got nil")
	}
	expectedMsg := "invoke LLM factory is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}
