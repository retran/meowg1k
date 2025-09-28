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
	"errors"
	"strings"
	"testing"
	"time"

	mdGateway "github.com/retran/meowg1k/internal/models/gateway"
	mdLLM "github.com/retran/meowg1k/internal/models/llm"
	mdProfile "github.com/retran/meowg1k/internal/models/profile"
	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/internal/services/task"
	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/future"
)

// Test error definitions
var (
	errUserPromptError    = errors.New("user prompt error")
	errSystemPromptError  = errors.New("system prompt error")
	errActivityExecFailed = errors.New("activity execution failed")
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
	config *task.Configuration
}

func (m *mockTaskService) Get() *task.Configuration {
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

func TestContentInput(t *testing.T) {
	profile := &mdProfile.ResolvedProfile{
		Provider:        mdGateway.OpenAI,
		Model:           "gpt-4",
		MaxInputTokens:  128000,
		MaxOutputTokens: 4096,
		Timeout:         5 * time.Minute,
		TokenizerType:   mdLLM.TokenizerCL100K,
	}

	input := ContentInput{
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

func TestContentOutput(t *testing.T) {
	metadata := map[string]any{
		"tokens_used": 100,
		"model":       "gpt-4",
	}

	output := ContentOutput{
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

func TestNewContentActivityFactory(t *testing.T) {
	gatewayFactory := &mockGatewayFactory{}
	factory := NewActivityFactory(gatewayFactory)

	if factory == nil {
		t.Fatal("Factory should not be nil")
	}

	if factory.gatewayFactory != gatewayFactory {
		t.Error("Gateway factory should be set correctly")
	}
}

func TestNewContentFlowFactory(t *testing.T) {
	taskService := &mockTaskService{
		config: &task.Configuration{
			Name:         "test-task",
			Profile:      &mdProfile.ResolvedProfile{},
			SystemPrompt: "System prompt",
			UserPrompt:   "User prompt",
		},
	}
	userPromptProvider := &mockPromptProvider{prompt: "User prompt"}
	systemPromptProvider := &mockPromptProvider{prompt: "System prompt"}
	activityFactory := NewActivityFactory(&mockGatewayFactory{})

	flowFactory := NewFlowFactory(
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

	if flowFactory.activityFactory != activityFactory {
		t.Error("Activity factory should be set correctly")
	}
}

func TestContentInputZeroValues(t *testing.T) {
	input := ContentInput{}

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

func TestContentOutputZeroValues(t *testing.T) {
	output := ContentOutput{}

	if output.Content != "" {
		t.Error("Expected empty Content")
	}

	if output.Metadata != nil {
		t.Error("Expected nil Metadata")
	}
}

func TestContentInputWithVariousPrompts(t *testing.T) {
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
			input := ContentInput{
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

// Additional comprehensive tests for activity and flow functions

func TestGenerateContentActivityExecution(t *testing.T) {
	factory := NewActivityFactory(&mockGatewayFactory{})
	activity := factory.NewActivity()

	if activity == nil {
		t.Fatal("Activity function should not be nil")
	}

	// Create real executor context with no-op feedback handler
	executorCtx := executor.NewContext("test-activity", executor.NoOpFeedbackHandler, mockExecutor{})

	profile := &mdProfile.ResolvedProfile{
		Provider:        mdGateway.OpenAI,
		Model:           "gpt-4",
		MaxOutputTokens: 4096,
		TokenizerType:   mdLLM.TokenizerCL100K,
	}

	input := &ContentInput{
		Profile:      profile,
		UserPrompt:   "Test prompt",
		SystemPrompt: "Test system",
	}

	// Test successful execution
	ctx := context.Background()
	result, err := activity(ctx, executorCtx, input)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	output, ok := result.(*ContentOutput)
	if !ok {
		t.Fatal("Result should be ContentOutput")
	}

	if output.Content != "Generated content" {
		t.Errorf("Expected 'Generated content', got '%s'", output.Content)
	}
}

func TestGenerateContentActivityWithNilInput(t *testing.T) {
	factory := NewActivityFactory(&mockGatewayFactory{})
	activity := factory.NewActivity()

	executorCtx := executor.NewContext("test-activity", executor.NoOpFeedbackHandler, &mockExecutor{})

	// Test with nil input
	ctx := context.Background()
	_, err := activity(ctx, executorCtx, nil)

	if err == nil {
		t.Error("Expected error with nil input")
	}

	expectedError := "input cannot be nil"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestGenerateContentActivityWithInvalidInput(t *testing.T) {
	factory := NewActivityFactory(&mockGatewayFactory{})
	activity := factory.NewActivity()

	executorCtx := executor.NewContext("test-activity", executor.NoOpFeedbackHandler, &mockExecutor{})

	// Test with wrong input type
	ctx := context.Background()
	_, err := activity(ctx, executorCtx, "invalid input")

	if err == nil {
		t.Error("Expected error with invalid input type")
	}

	if !strings.Contains(err.Error(), "invalid input type") {
		t.Errorf("Expected 'invalid input type' error, got: %v", err)
	}
}

func TestGenerateContentFlowExecution(t *testing.T) {
	// Setup mocks
	profile := &mdProfile.ResolvedProfile{
		Provider: mdGateway.OpenAI,
		Model:    "gpt-4",
	}

	taskService := &mockTaskService{
		config: &task.Configuration{
			Name:         "test-task",
			Profile:      profile,
			SystemPrompt: "System prompt",
			UserPrompt:   "User prompt",
		},
	}

	userPromptProvider := &mockPromptProvider{prompt: "User prompt"}
	systemPromptProvider := &mockPromptProvider{prompt: "System prompt"}
	activityFactory := NewActivityFactory(&mockGatewayFactory{})

	flowFactory := NewFlowFactory(
		taskService,
		userPromptProvider,
		systemPromptProvider,
		activityFactory,
	)

	flow := flowFactory.NewFlow()
	if flow == nil {
		t.Fatal("Flow function should not be nil")
	}

	// Create real executor context
	executorCtx := executor.NewContext("test-flow", executor.NoOpFeedbackHandler, &mockExecutor{})

	// Test flow execution
	ctx := context.Background()
	err := flow(ctx, executorCtx)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestGenerateContentFlowWithPromptErrors(t *testing.T) {
	taskService := &mockTaskService{
		config: &task.Configuration{
			Name:    "test-task",
			Profile: &mdProfile.ResolvedProfile{},
		},
	}

	// Test with user prompt error
	t.Run("user prompt error", func(t *testing.T) {
		userPromptProvider := &mockPromptProviderWithError{err: errUserPromptError}
		systemPromptProvider := &mockPromptProvider{prompt: "System prompt"}
		activityFactory := NewActivityFactory(&mockGatewayFactory{})

		flowFactory := NewFlowFactory(
			taskService,
			userPromptProvider,
			systemPromptProvider,
			activityFactory,
		)

		flow := flowFactory.NewFlow()
		executorCtx := executor.NewContext("test-flow", executor.NoOpFeedbackHandler, &mockExecutor{})

		ctx := context.Background()
		err := flow(ctx, executorCtx)

		if err == nil {
			t.Error("Expected error with user prompt failure")
		}

		if !strings.Contains(err.Error(), "failed to get user prompt") {
			t.Errorf("Expected user prompt error, got: %v", err)
		}
	})

	// Test with system prompt error
	t.Run("system prompt error", func(t *testing.T) {
		userPromptProvider := &mockPromptProvider{prompt: "User prompt"}
		systemPromptProvider := &mockPromptProviderWithError{err: errSystemPromptError}
		activityFactory := NewActivityFactory(&mockGatewayFactory{})

		flowFactory := NewFlowFactory(
			taskService,
			userPromptProvider,
			systemPromptProvider,
			activityFactory,
		)

		flow := flowFactory.NewFlow()
		executorCtx := executor.NewContext("test-flow", executor.NoOpFeedbackHandler, &mockExecutor{})

		ctx := context.Background()
		err := flow(ctx, executorCtx)

		if err == nil {
			t.Error("Expected error with system prompt failure")
		}

		if !strings.Contains(err.Error(), "failed to get system prompt") {
			t.Errorf("Expected system prompt error, got: %v", err)
		}
	})
}

func TestGenerateContentFlowWithActivityError(t *testing.T) {
	taskService := &mockTaskService{
		config: &task.Configuration{
			Name:    "test-task",
			Profile: &mdProfile.ResolvedProfile{},
		},
	}

	userPromptProvider := &mockPromptProvider{prompt: "User prompt"}
	systemPromptProvider := &mockPromptProvider{prompt: "System prompt"}
	activityFactory := NewActivityFactory(&mockGatewayFactory{})

	flowFactory := NewFlowFactory(
		taskService,
		userPromptProvider,
		systemPromptProvider,
		activityFactory,
	)

	flow := flowFactory.NewFlow()

	// Mock executor that returns error
	executorCtx := executor.NewContext("test-flow", executor.NoOpFeedbackHandler, &mockExecutorWithError{err: errActivityExecFailed})

	ctx := context.Background()
	err := flow(ctx, executorCtx)

	if err == nil {
		t.Error("Expected error with activity failure")
	}

	if !strings.Contains(err.Error(), "failed to execute \"GenerateContent\" activity") {
		t.Errorf("Expected activity execution error, got: %v", err)
	}
}

// Extended mock implementations

type mockExecutor struct{}

func (m mockExecutor) RunActivity(
	ctx context.Context,
	parentCtx *executor.Context,
	name string,
	activity executor.Activity[any, any],
	input any,
) *future.Future[any] {
	// Create a future with the expected result
	f := future.NewFuture[any]()
	f.Complete(&ContentOutput{Content: "Generated content"})
	return f
}

func (m mockExecutor) RunFlow(
	ctx context.Context,
	name string,
	flow executor.Flow,
	retryPolicy *executor.RetryPolicy,
) error {
	return nil
}

type mockExecutorWithError struct {
	err error
}

func (m mockExecutorWithError) RunActivity(
	ctx context.Context,
	parentCtx *executor.Context,
	name string,
	activity executor.Activity[any, any],
	input any,
) *future.Future[any] {
	f := future.NewFuture[any]()
	f.CompleteWithError(m.err)
	return f
}

func (m mockExecutorWithError) RunFlow(
	ctx context.Context,
	name string,
	flow executor.Flow,
	retryPolicy *executor.RetryPolicy,
) error {
	return m.err
}

type mockPromptProviderWithError struct {
	err error
}

func (m *mockPromptProviderWithError) GetUserPrompt() (string, error) {
	return "", m.err
}

func (m *mockPromptProviderWithError) GetSystemPrompt() (string, error) {
	return "", m.err
}
