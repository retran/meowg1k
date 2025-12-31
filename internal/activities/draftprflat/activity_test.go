// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package draftprflat_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/retran/meowg1k/internal/activities/draftcontent"
	draftprflat "github.com/retran/meowg1k/internal/activities/draftprflat"
	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/domain/provider"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockInvokeLLMFactory is a mock implementation of the invokeLLM factory for testing.
type mockInvokeLLMFactory struct {
	err      error
	response string
}

func (m *mockInvokeLLMFactory) NewActivity() executor.Activity[*draftcontent.Input, *draftcontent.Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *draftcontent.Input) (*draftcontent.Output, error) {
		if m.err != nil {
			return nil, m.err
		}
		return &draftcontent.Output{
			Response: &domainGateway.GenerateContentResponse{
				Blocks: []domainGateway.ContentBlock{{Kind: domainGateway.ContentBlockText, Text: m.response}},
			},
		}, nil
	}
}

func TestComposeFlatPR_Success(t *testing.T) {
	mockLLM := &mockInvokeLLMFactory{
		response: "# Add new feature\n\n## Summary\nAdded support for flat strategy in PR descriptions",
	}

	factory, err := draftprflat.NewFactory(mockLLM)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}

	activity := factory.NewActivity()

	testProfile := &profile.ResolvedProfile{
		Name:            "test",
		Provider:        provider.Gemini,
		Model:           "test-model",
		MaxInputTokens:  10000,
		MaxOutputTokens: 1000,
		Timeout:         5 * time.Minute,
	}

	changes := []*git.FileChange{
		{
			Filename: "test.go",
			Change:   "+func Test() {}\n-func OldTest() {}",
		},
	}

	input := &draftprflat.Input{
		Profile:      testProfile,
		SystemPrompt: "Generate a PR description",
		Changes:      changes,
		Intent:       "",
	}

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	executorCtx := executor.NewContext("test", nil, mockExec)

	output, err := activity(ctx, executorCtx, input)
	if err != nil {
		t.Fatalf("Activity failed: %v", err)
	}

	if output.Description != mockLLM.response {
		t.Errorf("Expected description %q, got %q", mockLLM.response, output.Description)
	}
}

func TestComposeFlatPR_WithIntent(t *testing.T) {
	mockLLM := &mockInvokeLLMFactory{
		response: "# Implement user request\n\n## Summary\nImplemented flat strategy as requested",
	}

	factory, err := draftprflat.NewFactory(mockLLM)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}

	activity := factory.NewActivity()

	testProfile := &profile.ResolvedProfile{
		Name:            "test",
		Provider:        provider.Gemini,
		Model:           "test-model",
		MaxInputTokens:  10000,
		MaxOutputTokens: 1000,
		Timeout:         5 * time.Minute,
	}

	changes := []*git.FileChange{
		{
			Filename: "feature.go",
			Change:   "+func NewFeature() {}",
		},
	}

	input := &draftprflat.Input{
		Profile:      testProfile,
		SystemPrompt: "Generate a PR description",
		Changes:      changes,
		Intent:       "Add support for flat strategy in PR descriptions",
	}

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	executorCtx := executor.NewContext("test", nil, mockExec)

	output, err := activity(ctx, executorCtx, input)
	if err != nil {
		t.Fatalf("Activity failed: %v", err)
	}

	if output.Description != mockLLM.response {
		t.Errorf("Expected description %q, got %q", mockLLM.response, output.Description)
	}
}

func TestComposeFlatPR_DiffTooLarge(t *testing.T) {
	mockLLM := &mockInvokeLLMFactory{
		response: "# Add feature",
	}

	factory, err := draftprflat.NewFactory(mockLLM)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}

	activity := factory.NewActivity()

	testProfile := &profile.ResolvedProfile{
		Name:            "test",
		Provider:        provider.Gemini,
		Model:           "test-model",
		MaxInputTokens:  100, // Very small limit
		MaxOutputTokens: 1000,
		Timeout:         5 * time.Minute,
	}

	// Create a large diff that exceeds the token limit
	largeChange := strings.Repeat("a", 500) // ~125 tokens (500 chars / 4)
	changes := []*git.FileChange{
		{
			Filename: "large.go",
			Change:   largeChange,
		},
	}

	input := &draftprflat.Input{
		Profile:      testProfile,
		SystemPrompt: "Generate a PR description",
		Changes:      changes,
		Intent:       "",
	}

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	executorCtx := executor.NewContext("test", nil, mockExec)

	_, err = activity(ctx, executorCtx, input)
	if err == nil {
		t.Fatal("Expected error for oversized diff, got nil")
	}

	expectedErrSubstring := "too large for the 'flat' strategy"
	if !strings.Contains(err.Error(), expectedErrSubstring) {
		t.Errorf("Expected error to contain %q, got %q", expectedErrSubstring, err.Error())
	}
}

func TestComposeFlatPR_NilFactory(t *testing.T) {
	_, err := draftprflat.NewFactory(nil)
	if err == nil {
		t.Fatal("Expected error for nil factory, got nil")
	}
}

func TestComposeFlatPR_NilInput(t *testing.T) {
	mockLLM := &mockInvokeLLMFactory{
		response: "# Add feature",
	}

	factory, err := draftprflat.NewFactory(mockLLM)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}

	activity := factory.NewActivity()

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	executorCtx := executor.NewContext("test", nil, mockExec)

	_, err = activity(ctx, executorCtx, nil)
	if err == nil {
		t.Fatal("Expected error for nil input, got nil")
	}
}

func TestComposeFlatPR_MultipleFiles(t *testing.T) {
	mockLLM := &mockInvokeLLMFactory{
		response: "# Update multiple files\n\n## Summary\nUpdated three files to implement new feature",
	}

	factory, err := draftprflat.NewFactory(mockLLM)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}

	activity := factory.NewActivity()

	testProfile := &profile.ResolvedProfile{
		Name:            "test",
		Provider:        provider.Gemini,
		Model:           "test-model",
		MaxInputTokens:  10000,
		MaxOutputTokens: 1000,
		Timeout:         5 * time.Minute,
	}

	changes := []*git.FileChange{
		{
			Filename: "file1.go",
			Change:   "+func File1() {}",
		},
		{
			Filename: "file2.go",
			Change:   "+func File2() {}",
		},
		{
			Filename: "file3.go",
			Change:   "+func File3() {}",
		},
	}

	input := &draftprflat.Input{
		Profile:      testProfile,
		SystemPrompt: "Generate a PR description",
		Changes:      changes,
		Intent:       "",
	}

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	executorCtx := executor.NewContext("test", nil, mockExec)

	output, err := activity(ctx, executorCtx, input)
	if err != nil {
		t.Fatalf("Activity failed: %v", err)
	}

	if output.Description != mockLLM.response {
		t.Errorf("Expected description %q, got %q", mockLLM.response, output.Description)
	}
}

func TestComposeFlatPR_EmptyChanges(t *testing.T) {
	mockLLM := &mockInvokeLLMFactory{
		response: "# No changes",
	}

	factory, err := draftprflat.NewFactory(mockLLM)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}

	activity := factory.NewActivity()

	testProfile := &profile.ResolvedProfile{
		Name:            "test",
		Provider:        provider.Gemini,
		Model:           "test-model",
		MaxInputTokens:  10000,
		MaxOutputTokens: 1000,
		Timeout:         5 * time.Minute,
	}

	input := &draftprflat.Input{
		Profile:      testProfile,
		SystemPrompt: "Generate a PR description",
		Changes:      []*git.FileChange{},
		Intent:       "",
	}

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	executorCtx := executor.NewContext("test", nil, mockExec)

	output, err := activity(ctx, executorCtx, input)
	if err != nil {
		t.Fatalf("Activity failed: %v", err)
	}

	if output.Description != mockLLM.response {
		t.Errorf("Expected description %q, got %q", mockLLM.response, output.Description)
	}
}
