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

	"github.com/retran/meowg1k/internal/config"
	"github.com/retran/meowg1k/internal/flows"
	"github.com/retran/meowg1k/internal/services/config/loader"
	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/spf13/cobra"
)

// Mock implementations for testing

type mockConfigLoaderService struct {
	config *config.Config
	err    error
}

func (m *mockConfigLoaderService) LoadConfig(configPath string) (*config.Config, error) {
	return m.config, m.err
}

func (m *mockConfigLoaderService) LoadFromSources(sources ...loader.ConfigSource) (*config.Config, error) {
	return m.config, m.err
}

type mockProfileResolver struct {
	profile   *config.ResolvedProfile
	err       error // Profile resolution error
	prompt    string
	promptErr error // Prompt resolution error
}

func (m *mockProfileResolver) ResolveProfile(profileName string) (*config.ResolvedProfile, error) {
	return m.profile, m.err
}

func (m *mockProfileResolver) ResolvePrompt(promptName string) (string, error) {
	return m.prompt, m.promptErr
}

func (m *mockProfileResolver) ResolveTaskConfiguration() (profileName, systemPrompt, userPrompt string, err error) {
	if m.err != nil {
		return "", "", "", m.err
	}
	return "default", "test system prompt", "test user prompt", nil
}

type mockGatewayFactory struct {
	gateway gateway.GenerationGateway
	err     error
}

func (m *mockGatewayFactory) CreateGenerationGateway(ctx context.Context, provider gateway.Provider, baseURL, apiKey string) (gateway.GenerationGateway, error) {
	return m.gateway, m.err
}

func (m *mockGatewayFactory) CreateEmbeddingsGateway(ctx context.Context, provider gateway.Provider, baseURL, apiKey string) (gateway.EmbeddingsGateway, error) {
	return nil, errors.New("not implemented")
}

type mockGenerationGateway struct {
	content string
	err     error
}

func (m *mockGenerationGateway) GenerateContent(ctx context.Context, request *gateway.GenerateContentRequest) (string, error) {
	return m.content, m.err
}

type mockPromptBuilder struct {
	result string
	err    error
}

func (m *mockPromptBuilder) BuildUserPrompt(basePrompt string, stdinWrapper string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.result != "" {
		return m.result, nil
	}
	if basePrompt != "" {
		return basePrompt, nil
	}
	return "test user prompt", nil
}

func (m *mockPromptBuilder) CombinePrompts(parts ...string) string {
	var nonEmpty []string
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			nonEmpty = append(nonEmpty, strings.TrimSpace(part))
		}
	}
	return strings.Join(nonEmpty, "\n\n")
}

// Helper functions

func createTestCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().String("user-prompt", "", "User prompt")
	cmd.Flags().String("system-prompt", "", "System prompt")
	cmd.Flags().String("profile", "", "Profile name")
	return cmd
}

func createTestProfile() *config.ResolvedProfile {
	return &config.ResolvedProfile{
		Provider:        "openai",
		Model:           "gpt-4",
		BaseURL:         "https://api.openai.com/v1",
		APIKey:          "test-key",
		MaxOutputTokens: 1000,
		Timeout:         30 * time.Second,
	}
}

func createTestConfig() *config.Config {
	return &config.Config{
		Generate: &config.GenerateConfig{
			Default: &config.GenerateDefault{
				Profile: "test-profile",
			},
		},
	}
}

// Tests for ResolveParamsExecutor

func TestNewResolveParamsExecutor(t *testing.T) {
	configLoader := &mockConfigLoaderService{}
	resolverService := &mockProfileResolver{}
	promptBuilder := &mockPromptBuilder{}

	executor := NewResolveParamsExecutor(configLoader, resolverService, promptBuilder)

	if executor.LoaderService != configLoader {
		t.Error("LoaderService not set correctly")
	}
	if executor.ResolverService != resolverService {
		t.Error("ResolverService not set correctly")
	}
	if executor.PromptBuilder != promptBuilder {
		t.Error("PromptBuilder not set correctly")
	}
}

func TestResolveParamsExecutor_Execute_Success(t *testing.T) {
	profile := createTestProfile()
	configLoader := &mockConfigLoaderService{}
	resolverService := &mockProfileResolver{
		profile: profile,
		prompt:  "test user prompt",
	}
	promptBuilder := &mockPromptBuilder{}

	executor := NewResolveParamsExecutor(configLoader, resolverService, promptBuilder)

	cmd := createTestCommand()

	// Create config with system prompt
	cfg := createTestConfig()
	cfg.Generate.Default.SystemPrompt = "test system prompt"

	input := Input{
		Cmd:    cmd,
		Config: cfg,
	}

	result, outcome, err := executor.Execute(context.Background(), input)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if outcome.Type != flows.OutcomeSuccess {
		t.Errorf("Expected success outcome, got: %v", outcome.Type)
	}

	if result.Params.Profile != profile {
		t.Error("Profile not set correctly in result")
	}

	if result.Params.SystemPrompt != "test system prompt" {
		t.Errorf("Expected system prompt 'test system prompt', got: %s", result.Params.SystemPrompt)
	}

	// Note: UserPrompt testing would require fixing the ResolvePromptFromCommand mock
}

func TestResolveParamsExecutor_Execute_InvalidInput(t *testing.T) {
	configLoader := &mockConfigLoaderService{}
	resolverService := &mockProfileResolver{}
	promptBuilder := &mockPromptBuilder{}
	executor := NewResolveParamsExecutor(configLoader, resolverService, promptBuilder)

	_, _, err := executor.Execute(context.Background(), "invalid input")

	if err != flows.ErrInvalidInput {
		t.Errorf("Expected ErrInvalidInput, got: %v", err)
	}
}

func TestResolveParamsExecutor_Execute_ProfileResolverError(t *testing.T) {
	expectedErr := errors.New("profile resolution failed")
	configLoader := &mockConfigLoaderService{}
	resolverService := &mockProfileResolver{err: expectedErr}
	promptBuilder := &mockPromptBuilder{}

	executor := NewResolveParamsExecutor(configLoader, resolverService, promptBuilder)

	input := Input{
		Cmd:    createTestCommand(),
		Config: createTestConfig(),
	}

	_, _, err := executor.Execute(context.Background(), input)

	// The error should be wrapped, so check if it contains the original error
	if err == nil || !errors.Is(err, expectedErr) {
		t.Errorf("Expected error containing profile resolver error, got: %v", err)
	}
}

func TestResolveParamsExecutor_Execute_PromptResolverError(t *testing.T) {
	configLoader := &mockConfigLoaderService{}

	// Create a resolver that succeeds for profile but fails for prompt
	resolverService := &mockProfileResolver{
		profile:   createTestProfile(),
		prompt:    "",
		err:       nil,
		promptErr: errors.New("prompt resolution failed"),
	}
	promptBuilder := &mockPromptBuilder{err: errors.New("prompt building failed")}

	executor := NewResolveParamsExecutor(configLoader, resolverService, promptBuilder)

	input := Input{
		Cmd:    createTestCommand(),
		Config: createTestConfig(),
	}

	_, _, err := executor.Execute(context.Background(), input)

	// The error should be wrapped, so check if it contains the original error
	if err == nil {
		t.Error("Expected error from prompt building, got none")
	}

	if !strings.Contains(err.Error(), "failed to build user prompt") {
		t.Errorf("Expected error containing 'failed to build user prompt', got: %v", err)
	}
}

// Tests for CreateGatewayExecutor

func TestNewCreateGatewayExecutor(t *testing.T) {
	gatewayFactory := &mockGatewayFactory{}

	executor := NewCreateGatewayExecutor(gatewayFactory)

	if executor.GatewayFactory != gatewayFactory {
		t.Error("GatewayFactory not set correctly")
	}
}

func TestCreateGatewayExecutor_Execute_Success(t *testing.T) {
	mockGateway := &mockGenerationGateway{}
	gatewayFactory := &mockGatewayFactory{gateway: mockGateway}

	executor := NewCreateGatewayExecutor(gatewayFactory)

	params := &Params{
		Profile: createTestProfile(),
	}

	input := ResolvedParams{
		Params: params,
		Config: createTestConfig(),
	}

	result, outcome, err := executor.Execute(context.Background(), input)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if outcome.Type != flows.OutcomeSuccess {
		t.Errorf("Expected success outcome, got: %v", outcome.Type)
	}

	if result.Gateway != mockGateway {
		t.Error("Gateway not set correctly in result")
	}

	if result.Params != params {
		t.Error("Params not set correctly in result")
	}
}

func TestCreateGatewayExecutor_Execute_InvalidInput(t *testing.T) {
	executor := NewCreateGatewayExecutor(&mockGatewayFactory{})

	_, _, err := executor.Execute(context.Background(), "invalid input")

	if err != flows.ErrInvalidInput {
		t.Errorf("Expected ErrInvalidInput, got: %v", err)
	}
}

func TestCreateGatewayExecutor_Execute_GatewayFactoryError(t *testing.T) {
	expectedErr := errors.New("gateway creation failed")
	gatewayFactory := &mockGatewayFactory{err: expectedErr}

	executor := NewCreateGatewayExecutor(gatewayFactory)

	params := &Params{
		Profile: createTestProfile(),
	}

	input := ResolvedParams{
		Params: params,
		Config: createTestConfig(),
	}

	_, _, err := executor.Execute(context.Background(), input)

	if err != expectedErr {
		t.Errorf("Expected gateway factory error, got: %v", err)
	}
}

// Tests for GenerateContentExecutor

func TestGenerateContentExecutor_Execute_Success(t *testing.T) {
	mockGateway := &mockGenerationGateway{content: "generated content"}
	executor := &GenerateContentExecutor{}

	params := &Params{
		Profile:      createTestProfile(),
		SystemPrompt: "system prompt",
		UserPrompt:   "user prompt",
	}

	input := GenerationGateway{
		Gateway: mockGateway,
		Params:  params,
	}

	result, outcome, err := executor.Execute(context.Background(), input)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if outcome.Type != flows.OutcomeSuccess {
		t.Errorf("Expected success outcome, got: %v", outcome.Type)
	}

	if result.Content != "generated content" {
		t.Errorf("Expected content 'generated content', got: %s", result.Content)
	}
}

func TestGenerateContentExecutor_Execute_InvalidInput(t *testing.T) {
	executor := &GenerateContentExecutor{}

	_, _, err := executor.Execute(context.Background(), "invalid input")

	if err != flows.ErrInvalidInput {
		t.Errorf("Expected ErrInvalidInput, got: %v", err)
	}
}

func TestGenerateContentExecutor_Execute_GenerationError(t *testing.T) {
	expectedErr := errors.New("content generation failed")
	mockGateway := &mockGenerationGateway{err: expectedErr}
	executor := &GenerateContentExecutor{}

	params := &Params{
		Profile:      createTestProfile(),
		SystemPrompt: "system prompt",
		UserPrompt:   "user prompt",
	}

	input := GenerationGateway{
		Gateway: mockGateway,
		Params:  params,
	}

	_, _, err := executor.Execute(context.Background(), input)

	if err != expectedErr {
		t.Errorf("Expected content generation error, got: %v", err)
	}
}

func TestGenerateContentExecutor_Execute_ContextTimeout(t *testing.T) {
	// Test with a very short timeout to trigger context cancellation
	profile := createTestProfile()
	profile.Timeout = 1 * time.Millisecond

	// Create a mock gateway that will simulate a delay longer than the timeout
	slowGateway := &slowMockGenerationGateway{
		delay: 100 * time.Millisecond, // Longer than the timeout
	}

	executor := &GenerateContentExecutor{}

	params := &Params{
		Profile:      profile,
		SystemPrompt: "system prompt",
		UserPrompt:   "user prompt",
	}

	input := GenerationGateway{
		Gateway: slowGateway,
		Params:  params,
	}

	ctx := context.Background()
	_, _, err := executor.Execute(ctx, input)

	// The error should be related to context timeout/cancellation
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	// Check that it's actually a context timeout error
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context deadline exceeded error, got: %v", err)
	}
}

// Additional mock for testing timeout scenarios
type slowMockGenerationGateway struct {
	delay time.Duration
}

func (m *slowMockGenerationGateway) GenerateContent(ctx context.Context, request *gateway.GenerateContentRequest) (string, error) {
	select {
	case <-time.After(m.delay):
		return "delayed content", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
