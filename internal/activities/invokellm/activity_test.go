/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package invokellm

import (
	"context"
	"errors"
	"testing"

	"github.com/retran/meowg1k/internal/services/profile"
	"github.com/retran/meowg1k/internal/testutil/gatewaymocks"
	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/future"
)

// mockExecutor is a mock implementation of executor.Executor
type mockExecutor struct{}

func (m *mockExecutor) RunActivity(ctx context.Context, executorCtx *executor.Context, name string, activity executor.Activity[any, any], input any) *future.Future[any] {
	return nil
}

func (m *mockExecutor) RunFlow(ctx context.Context, name string, flow executor.Flow, retryPolicy *executor.RetryPolicy) error {
	return nil
}

func TestNewFactory(t *testing.T) {
	gwFactory := &gatewaymocks.MockGatewayFactory{}
	factory := NewFactory(gwFactory)

	if factory == nil {
		t.Error("NewFactory returned nil")
	}
}

func TestInvokeLLMActivity_Success(t *testing.T) {
	gwFactory := &gatewaymocks.MockGatewayFactory{
		GenerationGateway: &gatewaymocks.MockGenerationGateway{
			Content: "Generated content",
		},
	}

	factory := NewFactory(gwFactory)
	activity := factory.NewActivity()

	ctx := context.Background()
	executorCtx := executor.NewContext("test", nil, &mockExecutor{})

	input := &Input{
		Profile: &profile.ResolvedProfile{
			Provider: "test",
			Model:    "test-model",
		},
		SystemPrompt: "System prompt",
		UserPrompt:   "User prompt",
	}

	result, err := activity(ctx, executorCtx, input)
	if err != nil {
		t.Errorf("Activity failed: %v", err)
	}

	output, ok := result.(*Output)
	if !ok {
		t.Errorf("Expected *Output, got %T", result)
	}

	if output.Content != "Generated content" {
		t.Errorf("Expected 'Generated content', got '%s'", output.Content)
	}
}

func TestInvokeLLMActivity_NilInput(t *testing.T) {
	gwFactory := &gatewaymocks.MockGatewayFactory{}
	factory := NewFactory(gwFactory)
	activity := factory.NewActivity()

	ctx := context.Background()
	executorCtx := executor.NewContext("test", nil, &mockExecutor{})

	_, err := activity(ctx, executorCtx, nil)
	if err != executor.ErrInputCannotBeNil {
		t.Errorf("Expected ErrInputCannotBeNil, got %v", err)
	}
}

func TestInvokeLLMActivity_InvalidInputType(t *testing.T) {
	gwFactory := &gatewaymocks.MockGatewayFactory{}
	factory := NewFactory(gwFactory)
	activity := factory.NewActivity()

	ctx := context.Background()
	executorCtx := executor.NewContext("test", nil, &mockExecutor{})

	_, err := activity(ctx, executorCtx, "invalid input")
	if !errors.Is(err, executor.ErrInvalidInputType) {
		t.Errorf("Expected ErrInvalidInputType, got %v", err)
	}
}

func TestInvokeLLMActivity_GatewayError(t *testing.T) {
	gwFactory := &gatewaymocks.MockGatewayFactory{
		Err: errors.New("gateway creation failed"),
	}

	factory := NewFactory(gwFactory)
	activity := factory.NewActivity()

	ctx := context.Background()
	executorCtx := executor.NewContext("test", nil, &mockExecutor{})

	input := &Input{
		Profile: &profile.ResolvedProfile{
			Provider: "test",
			Model:    "test-model",
		},
		SystemPrompt: "System prompt",
		UserPrompt:   "User prompt",
	}

	_, err := activity(ctx, executorCtx, input)
	if err == nil {
		t.Error("Expected error from gateway creation")
	}
}

func TestInvokeLLMActivity_GenerationError(t *testing.T) {
	gwFactory := &gatewaymocks.MockGatewayFactory{
		GenerationGateway: &gatewaymocks.MockGenerationGateway{
			Err: errors.New("generation failed"),
		},
	}

	factory := NewFactory(gwFactory)
	activity := factory.NewActivity()

	ctx := context.Background()
	executorCtx := executor.NewContext("test", nil, &mockExecutor{})

	input := &Input{
		Profile: &profile.ResolvedProfile{
			Provider: "test",
			Model:    "test-model",
		},
		SystemPrompt: "System prompt",
		UserPrompt:   "User prompt",
	}

	_, err := activity(ctx, executorCtx, input)
	if err == nil {
		t.Error("Expected error from generation")
	}
}
