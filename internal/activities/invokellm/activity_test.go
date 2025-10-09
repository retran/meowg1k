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

	coreGateway "github.com/retran/meowg1k/internal/core/gateway"
	"github.com/retran/meowg1k/internal/core/profile"
	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockGenerationGateway is a mock implementation of GenerationGateway for testing.
type mockGenerationGateway struct {
	Content string
	Err     error
}

func (m *mockGenerationGateway) GenerateContent(ctx context.Context, request *coreGateway.GenerateContentRequest) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	return m.Content, nil
}

// mockGenerationGatewayFactory is a mock implementation of GenerationGatewayFactory for testing.
type mockGenerationGatewayFactory struct {
	Gateway gateway.GenerationGateway
	Err     error
}

func (m *mockGenerationGatewayFactory) NewGenerationGateway(ctx context.Context, profile *profile.ResolvedProfile) (gateway.GenerationGateway, error) {
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
