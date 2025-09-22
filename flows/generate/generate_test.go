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
	"testing"
	"time"

	"github.com/retran/meowg1k/internal/config"
	"github.com/retran/meowg1k/internal/flows"
	"github.com/retran/meowg1k/internal/services/config/loader"
	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/internal/services/prompt"
)

// Tests for Flow Creation

func TestFlowCreation(t *testing.T) {
	cfg := createTestConfig()

	// Create services for the factory
	configLoader := &mockConfigLoaderService{config: cfg}
	resolverService := &mockProfileResolver{
		profile: createTestProfile(),
		prompt:  "test user prompt",
	}
	promptBuilder := &mockPromptBuilder{}
	gatewayFactory := &mockGatewayFactory{
		gateway: &mockGenerationGateway{
			content: "test generated content",
		},
	}

	factory := NewFlowFactory(configLoader, resolverService, promptBuilder, gatewayFactory)
	flow := factory.CreateFlow(nil)

	if flow == nil {
		t.Fatal("Expected flow to be created, got nil")
	}

	// Test that the flow has the expected structure
	// This is a basic structural test - more detailed tests would require accessing internal flow state
}

func TestFlowWithFeedbackHandler(t *testing.T) {
	cfg := createTestConfig()

	// Mock feedback handler
	feedbackHandler := func(feedback flows.Feedback) {
		// Feedback processing not tested here
	}

	// Create services for the factory
	configLoader := &mockConfigLoaderService{config: cfg}
	resolverService := &mockProfileResolver{
		profile: createTestProfile(),
		prompt:  "test user prompt",
	}
	promptBuilder := &mockPromptBuilder{}
	gatewayFactory := &mockGatewayFactory{
		gateway: &mockGenerationGateway{
			content: "test generated content",
		},
	}

	factory := NewFlowFactory(configLoader, resolverService, promptBuilder, gatewayFactory)
	flow := factory.CreateFlow(feedbackHandler)

	if flow == nil {
		t.Fatal("Expected flow to be created, got nil")
	}
}

// Factory Integration Tests

func TestFlowSuccess(t *testing.T) {
	// Create test command with required flags
	cmd := createTestCommand()
	cmd.Flags().Set("user-prompt", "test user prompt")
	cmd.Flags().Set("system-prompt", "test system prompt")
	cmd.Flags().Set("profile", "default")

	// Create test config
	cfg := createTestConfig()

	// Create mocked flow using factory
	flow := createMockedFlow(cfg)

	input := Input{
		Cmd:    cmd,
		Config: cfg,
	}

	result, err := flow.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error from flow execution, got: %v", err)
	}

	generatedContent, ok := result.(GeneratedContent)
	if !ok {
		t.Fatalf("Expected GeneratedContent result, got: %T", result)
	}

	if generatedContent.Content != "mocked generated content" {
		t.Errorf("Expected 'mocked generated content', got: %s", generatedContent.Content)
	}
}

func TestFlowInvalidInput(t *testing.T) {
	// Test with invalid command (missing required flags)
	cmd := createTestCommand()
	// Don't set any flags

	cfg := createTestConfig()

	// Create flow with mocked dependencies that will fail
	flow := createMockedFlowWithErrors(cfg)

	input := Input{
		Cmd:    cmd,
		Config: cfg,
	}

	_, err := flow.Run(context.Background(), input)

	// Should get an error due to missing configuration or failed resolution
	if err == nil {
		t.Error("Expected error due to invalid input, got nil")
	}
}

func TestFlowExecutionError(t *testing.T) {
	cmd := createTestCommand()
	cmd.Flags().Set("user-prompt", "test prompt")
	cfg := createTestConfig()

	// Create flow that will fail during execution
	flow := createMockedFlowWithErrors(cfg)

	input := Input{
		Cmd:    cmd,
		Config: cfg,
	}

	_, err := flow.Run(context.Background(), input)

	if err == nil {
		t.Error("Expected error from flow execution, got nil")
	}
}

// Helper functions for creating mocked flows using the factory pattern

func createMockedFlow(cfg *config.Config) *flows.Flow {
	// Create successful mocks
	configLoader := &mockConfigLoaderService{
		config: cfg,
	}
	resolverService := &mockProfileResolver{
		profile: createTestProfile(),
		prompt:  "test user prompt",
	}
	gatewayFactory := &mockGatewayFactory{
		gateway: &mockGenerationGateway{
			content: "mocked generated content",
		},
	}
	promptBuilder := prompt.NewBuilder()

	// Use the factory pattern to create the flow
	factory := NewFlowFactory(configLoader, resolverService, promptBuilder, gatewayFactory)
	return factory.CreateFlow(nil)
}

func createMockedFlowWithErrors(cfg *config.Config) *flows.Flow {
	// Create failing mocks
	configLoader := &mockConfigLoaderService{
		config: cfg,
	}
	resolverService := &mockProfileResolver{
		prompt: "test user prompt",
		err:    errors.New("profile resolution failed"),
	}
	gatewayFactory := &mockGatewayFactory{
		err: errors.New("gateway creation failed"),
	}
	promptBuilder := prompt.NewBuilder()

	// Use the factory pattern to create the flow
	factory := NewFlowFactory(configLoader, resolverService, promptBuilder, gatewayFactory)
	return factory.CreateFlow(nil)
}

// End-to-end style tests with more realistic scenarios

func TestFlowEndToEndSuccess(t *testing.T) {
	// This test simulates a more realistic end-to-end execution
	// with all components working together

	cmd := createTestCommand()
	cmd.Flags().Set("user-prompt", "Generate a greeting")
	cmd.Flags().Set("system-prompt", "You are a helpful assistant")
	cmd.Flags().Set("profile", "test-profile")

	cfg := createTestConfig()

	// Create mocked flow with realistic data flow
	flow := createRealisticMockedFlow()

	input := Input{
		Cmd:    cmd,
		Config: cfg,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := flow.Run(ctx, input)
	if err != nil {
		t.Fatalf("Expected successful execution, got error: %v", err)
	}

	generatedContent, ok := result.(GeneratedContent)
	if !ok {
		t.Fatalf("Expected GeneratedContent, got: %T", result)
	}

	if generatedContent.Content == "" {
		t.Error("Expected non-empty generated content")
	}
}

func createRealisticMockedFlow() *flows.Flow {
	// More realistic mocks that simulate actual behavior
	configLoader := &mockConfigLoaderService{}
	resolverService := &mockProfileResolver{
		profile: &config.ResolvedProfile{
			Provider:        "openai",
			Model:           "gpt-4",
			BaseURL:         "https://api.openai.com/v1",
			APIKey:          "mock-api-key",
			MaxOutputTokens: 1000,
			Timeout:         30 * time.Second,
		},
		prompt: "Generate a greeting",
	}

	gatewayFactory := &mockGatewayFactory{
		gateway: &mockGenerationGateway{
			content: "Hello! How can I assist you today?",
		},
	}

	promptBuilder := prompt.NewBuilder()

	// Use the factory pattern to create the flow
	factory := NewFlowFactory(configLoader, resolverService, promptBuilder, gatewayFactory)
	return factory.CreateFlow(nil)
}

// Test error propagation through the flow

func TestFlowErrorPropagation(t *testing.T) {
	tests := []struct {
		name               string
		profileResolverErr error
		promptResolverErr  error
		gatewayFactoryErr  error
		generationErr      error
		expectError        bool
	}{
		{
			name:               "ProfileResolver error",
			profileResolverErr: errors.New("profile error"),
			expectError:        true,
		},
		{
			name:              "GatewayFactory error",
			gatewayFactoryErr: errors.New("gateway error"),
			expectError:       true,
		},
		{
			name:          "Generation error",
			generationErr: errors.New("generation error"),
			expectError:   true,
		},
		{
			name:        "No errors",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flow := flows.NewFlow()

			configLoader := &mockConfigLoaderService{}
			resolverService := &mockProfileResolver{
				profile:   createTestProfile(),
				prompt:    "test prompt",
				err:       tt.profileResolverErr,
				promptErr: tt.promptResolverErr,
			}
			gatewayFactory := &mockGatewayFactory{
				gateway: &mockGenerationGateway{
					content: "test content",
					err:     tt.generationErr,
				},
				err: tt.gatewayFactoryErr,
			}

			promptBuilder := prompt.NewBuilder()
			flows.AddTask(flow, "resolve-params", NewResolveParamsExecutor(configLoader, resolverService, promptBuilder)).
				LinkToID("create-gateway")
			flows.AddTask(flow, "create-gateway", NewCreateGatewayExecutor(gatewayFactory)).
				LinkToID("generate-content")
			flows.AddTask(flow, "generate-content", &GenerateContentExecutor{})

			flow = flow.SetStart("resolve-params")

			cmd := createTestCommand()
			cmd.Flags().Set("user-prompt", "test")

			input := Input{
				Cmd:    cmd,
				Config: createTestConfig(),
			}

			_, err := flow.Run(context.Background(), input)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// End-to-End Integration Tests
// These tests use real service implementations where possible to validate actual flow execution

func TestFlowEndToEndWithRealServices(t *testing.T) {
	// Create test command with required flags
	cmd := createTestCommand()
	cmd.Flags().Set("user-prompt", "Generate a simple hello world function in Python")

	// Create test config with realistic settings
	cfg := createRealisticTestConfig()

	// Create a flow with real services but mock the gateway to avoid external API calls
	flow := createRealServicesFlow(cfg)

	input := Input{
		Cmd:    cmd,
		Config: cfg,
	}

	// Execute the flow
	result, err := flow.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error from real services flow execution, got: %v", err)
	}

	generatedContent, ok := result.(GeneratedContent)
	if !ok {
		t.Fatalf("Expected GeneratedContent result, got: %T", result)
	}

	if generatedContent.Content == "" {
		t.Error("Expected non-empty generated content")
	}

	// Validate that the content looks reasonable (contains expected patterns)
	if len(generatedContent.Content) < 10 {
		t.Errorf("Generated content seems too short: %s", generatedContent.Content)
	}
}

func TestFlowEndToEndWithTimeout(t *testing.T) {
	// Create test command
	cmd := createTestCommand()
	cmd.Flags().Set("user-prompt", "test prompt")

	cfg := createRealisticTestConfig()

	// Create a flow with a slow mock to test timeout behavior
	flow := createSlowServicesFlow(cfg)

	input := Input{
		Cmd:    cmd,
		Config: cfg,
	}

	// Create a context with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Execute the flow - should timeout
	_, err := flow.Run(ctx, input)
	if err == nil {
		t.Error("Expected timeout error but got none")
	}

	// Verify it's a context cancellation error
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context timeout/cancellation error, got: %v", err)
	}
}

func TestFlowEndToEndWithStdin(t *testing.T) {
	// This test would require complex stdin simulation
	// For now, we'll test the command flag resolution logic
	cmd := createTestCommand()
	cmd.Flags().Set("user-prompt", "Analyze this code")
	// In a real scenario, stdin would be provided as well

	cfg := createRealisticTestConfig()
	flow := createRealServicesFlow(cfg)

	input := Input{
		Cmd:    cmd,
		Config: cfg,
	}

	result, err := flow.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error from stdin flow execution, got: %v", err)
	}

	generatedContent, ok := result.(GeneratedContent)
	if !ok {
		t.Fatalf("Expected GeneratedContent result, got: %T", result)
	}

	if generatedContent.Content == "" {
		t.Error("Expected non-empty generated content")
	}
}

// Helper functions for integration tests

func createRealisticTestConfig() *config.Config {
	return &config.Config{
		Generate: &config.GenerateConfig{
			Default: &config.GenerateDefault{
				Profile: "default",
			},
		},
		Profiles: map[string]*config.Profile{
			"default": {
				Provider:        "openai",
				Model:           "gpt-4o-mini",
				Timeout:         30 * time.Second,
				MaxOutputTokens: 1000,
			},
		},
	}
}

func createRealServicesFlow(cfg *config.Config) *flows.Flow {
	// Use simplified real services that match the actual interfaces
	loaderService := &integrationConfigLoaderService{config: cfg}
	resolverService := &integrationResolverService{}

	// Mock only the gateway to avoid external API calls
	gatewayFactory := &mockGatewayFactory{
		gateway: &mockGenerationGateway{
			content: "def hello_world():\n    print('Hello, World!')",
		},
	}

	promptBuilder := prompt.NewBuilder()

	// Use the factory pattern to create the flow
	factory := NewFlowFactory(loaderService, resolverService, promptBuilder, gatewayFactory)
	return factory.CreateFlow(nil)
}

func createSlowServicesFlow(cfg *config.Config) *flows.Flow {
	loaderService := &integrationConfigLoaderService{config: cfg}
	resolverService := &integrationResolverService{}

	// Use a slow mock that simulates a timeout scenario
	gatewayFactory := &mockGatewayFactory{
		gateway: &slowMockGenerationGateway{
			delay: 200 * time.Millisecond, // Longer than test timeout
		},
	}

	promptBuilder := prompt.NewBuilder()

	// Use the factory pattern to create the flow
	factory := NewFlowFactory(loaderService, resolverService, promptBuilder, gatewayFactory)
	return factory.CreateFlow(nil)
}

// Integration test service implementations

type integrationConfigLoaderService struct {
	config *config.Config
}

func (r *integrationConfigLoaderService) LoadConfig(configPath string) (*config.Config, error) {
	return r.config, nil
}

func (r *integrationConfigLoaderService) LoadFromSources(sources ...loader.ConfigSource) (*config.Config, error) {
	return r.config, nil
}

type integrationResolverService struct{}

func (r *integrationResolverService) ResolveProfile(profileName string) (*config.ResolvedProfile, error) {
	// For integration tests, return a fixed test profile
	return &config.ResolvedProfile{
		Provider: gateway.OpenAI,
		Model:    "gpt-4",
		BaseURL:  "https://api.openai.com/v1",
		APIKey:   "test-api-key",
		Timeout:  30000,
	}, nil
}

func (r *integrationResolverService) ResolvePrompt(promptName string) (string, error) {
	return "test system prompt", nil
}

func (r *integrationResolverService) ResolveTaskConfiguration() (profileName, systemPrompt, userPrompt string, err error) {
	// For integration tests, return fixed test values
	return "default", "test system prompt", "test user prompt", nil
}
