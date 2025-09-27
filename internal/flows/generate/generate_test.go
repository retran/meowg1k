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

package generate

import (
	"context"
	"testing"
	"time"

	mdGateway "github.com/retran/meowg1k/internal/models/gateway"
	mdLLM "github.com/retran/meowg1k/internal/models/llm"
	mdProfile "github.com/retran/meowg1k/internal/models/profile"
	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/internal/services/task"
)

// Mock implementations for testing

type mockGatewayFactory struct{}

func (m *mockGatewayFactory) NewGenerationGateway(ctx context.Context, profile *mdProfile.ResolvedProfile) (gateway.GenerationGateway, error) {
	return &mockGenerationGateway{}, nil
}

func (m *mockGatewayFactory) NewEmbeddingsGateway(ctx context.Context, profile *mdProfile.ResolvedProfile) (gateway.EmbeddingsGateway, error) {
	return &mockEmbeddingsGateway{}, nil
}

type mockGenerationGateway struct{}

func (m *mockGenerationGateway) GenerateContent(ctx context.Context, request *mdGateway.GenerateContentRequest) (string, error) {
	return "Generated content", nil
}

type mockEmbeddingsGateway struct{}

func (m *mockEmbeddingsGateway) ComputeEmbeddings(ctx context.Context, request *mdGateway.ComputeEmbeddingsRequest) ([]mdGateway.Embedding, error) {
	return nil, nil
}

func (m *mockEmbeddingsGateway) ComputeDistance(first, second mdGateway.Embedding) (float64, error) {
	return 0.5, nil
}

type mockTaskService struct {
	config *task.TaskConfiguration
}

func (m *mockTaskService) Get() *task.TaskConfiguration {
	return m.config
}

type mockPromptProvider struct {
	prompt string
}

func (m *mockPromptProvider) GetUserPrompt() (string, error) {
	return m.prompt, nil
}

func (m *mockPromptProvider) GetSystemPrompt() (string, error) {
	return m.prompt, nil
}

func TestGenerateContentInput(t *testing.T) {
	profile := &mdProfile.ResolvedProfile{
		Provider:        mdGateway.OpenAI,
		Model:           "gpt-4",
		MaxInputTokens:  128000,
		MaxOutputTokens: 4096,
		Timeout:         5 * time.Minute,
		TokenizerType:   mdLLM.TokenizerCL100K,
	}

	input := GenerateContentInput{
		Profile:      profile,
		UserPrompt:   "Test user prompt",
		SystemPrompt: "Test system prompt",
	}

	if input.Profile != profile {
		t.Error("Expected profile to be set correctly")
	}

	if input.UserPrompt != "Test user prompt" {
		t.Errorf("Expected UserPrompt 'Test user prompt', got '%s'", input.UserPrompt)
	}

	if input.SystemPrompt != "Test system prompt" {
		t.Errorf("Expected SystemPrompt 'Test system prompt', got '%s'", input.SystemPrompt)
	}
}

func TestGenerateContentOutput(t *testing.T) {
	metadata := map[string]any{
		"tokens_used": 100,
		"model":       "gpt-4",
	}

	output := GenerateContentOutput{
		Content:  "Generated content",
		Metadata: metadata,
	}

	if output.Content != "Generated content" {
		t.Errorf("Expected Content 'Generated content', got '%s'", output.Content)
	}

	if output.Metadata == nil {
		t.Error("Expected Metadata to be set")
	}

	if output.Metadata["tokens_used"] != 100 {
		t.Errorf("Expected tokens_used 100, got %v", output.Metadata["tokens_used"])
	}
}

func TestNewGenerateContentActivityFactory(t *testing.T) {
	gatewayFactory := &mockGatewayFactory{}
	factory := NewGenerateContentActivityFactory(gatewayFactory)

	if factory == nil {
		t.Fatal("Factory should not be nil")
	}

	if factory.gatewayFactory != gatewayFactory {
		t.Error("Gateway factory should be set correctly")
	}
}

func TestNewGenerateContentFlowFactory(t *testing.T) {
	taskService := &mockTaskService{
		config: &task.TaskConfiguration{
			Name:         "test-task",
			Profile:      &mdProfile.ResolvedProfile{},
			SystemPrompt: "System prompt",
			UserPrompt:   "User prompt",
		},
	}
	userPromptProvider := &mockPromptProvider{prompt: "User prompt"}
	systemPromptProvider := &mockPromptProvider{prompt: "System prompt"}
	activityFactory := NewGenerateContentActivityFactory(&mockGatewayFactory{})

	flowFactory := NewGenerateContentFlowFactory(
		taskService,
		userPromptProvider,
		systemPromptProvider,
		activityFactory,
	)

	if flowFactory == nil {
		t.Fatal("Flow factory should not be nil")
	}

	if flowFactory.taskService != taskService {
		t.Error("Task service should be set correctly")
	}

	if flowFactory.userPromptProvider != userPromptProvider {
		t.Error("User prompt provider should be set correctly")
	}

	if flowFactory.systemPromptProvider != systemPromptProvider {
		t.Error("System prompt provider should be set correctly")
	}

	if flowFactory.generateContentActivityFactory != activityFactory {
		t.Error("Activity factory should be set correctly")
	}
}

func TestGenerateContentInputZeroValues(t *testing.T) {
	input := GenerateContentInput{}

	if input.Profile != nil {
		t.Error("Expected nil Profile")
	}

	if input.UserPrompt != "" {
		t.Error("Expected empty UserPrompt")
	}

	if input.SystemPrompt != "" {
		t.Error("Expected empty SystemPrompt")
	}
}

func TestGenerateContentOutputZeroValues(t *testing.T) {
	output := GenerateContentOutput{}

	if output.Content != "" {
		t.Error("Expected empty Content")
	}

	if output.Metadata != nil {
		t.Error("Expected nil Metadata")
	}
}

func TestGenerateContentInputWithVariousPrompts(t *testing.T) {
	testCases := []struct {
		name         string
		userPrompt   string
		systemPrompt string
	}{
		{"Empty prompts", "", ""},
		{"Only user prompt", "Hello", ""},
		{"Only system prompt", "", "You are helpful"},
		{"Both prompts", "Hello", "You are helpful"},
		{"Long prompts", "This is a very long user prompt that contains multiple sentences.", "You are a helpful assistant with expertise in many areas."},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := GenerateContentInput{
				UserPrompt:   tc.userPrompt,
				SystemPrompt: tc.systemPrompt,
			}

			if input.UserPrompt != tc.userPrompt {
				t.Errorf("Expected UserPrompt '%s', got '%s'", tc.userPrompt, input.UserPrompt)
			}

			if input.SystemPrompt != tc.systemPrompt {
				t.Errorf("Expected SystemPrompt '%s', got '%s'", tc.systemPrompt, input.SystemPrompt)
			}
		})
	}
}