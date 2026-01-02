// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package draftpr

import (
	"context"
	"testing"

	"github.com/retran/meowg1k/internal/activities/draftcontent"
	"github.com/retran/meowg1k/internal/activities/summarizefilechanges"
	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockContentGenerationActivityFactory is a mock implementation of ContentGenerationActivityFactory for testing.
type mockContentGenerationActivityFactory struct {
	activity executor.Activity[*draftcontent.Input, *draftcontent.Output]
}

func (m *mockContentGenerationActivityFactory) NewActivity() executor.Activity[*draftcontent.Input, *draftcontent.Output] {
	if m.activity != nil {
		return m.activity
	}
	return func(ctx context.Context, executorCtx *executor.Context, input *draftcontent.Input) (*draftcontent.Output, error) {
		return &draftcontent.Output{
			Response: &domainGateway.GenerateContentResponse{
				Blocks: []domainGateway.ContentBlock{{Kind: domainGateway.ContentBlockText, Text: "test content"}},
			},
		}, nil
	}
}

func TestNewFactory(t *testing.T) {
	mockFactory := &mockContentGenerationActivityFactory{}
	factory, err := NewFactory(mockFactory)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}
	if factory == nil {
		t.Error("NewFactory returned nil")
	}
}

func TestNewFactoryNil(t *testing.T) {
	factory, err := NewFactory(nil)
	if err == nil {
		t.Error("Expected error when NewFactory called with nil")
	}
	if factory != nil {
		t.Error("Expected nil factory when error returned")
	}
}

func TestActivityNilInput(t *testing.T) {
	mockFactory := &mockContentGenerationActivityFactory{}
	factory, err := NewFactory(mockFactory)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}

	activity := factory.NewActivity()
	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)

	_, err = activity(ctx, execCtx, nil)
	if err == nil {
		t.Error("Expected error for nil input, got nil")
	}
}

func TestActivitySuccess(t *testing.T) {
	mockInvokeLLM := func(ctx context.Context, executorCtx *executor.Context, input *draftcontent.Input) (*draftcontent.Output, error) {
		return &draftcontent.Output{
			Response: &domainGateway.GenerateContentResponse{
				Blocks: []domainGateway.ContentBlock{{Kind: domainGateway.ContentBlockText, Text: "test PR description"}},
			},
		}, nil
	}

	mockFactory := &mockContentGenerationActivityFactory{
		activity: mockInvokeLLM,
	}

	factory, err := NewFactory(mockFactory)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}
	activity := factory.NewActivity()

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", nil, mockExec)

	input := &Input{
		Preset:       &preset.ResolvedPreset{},
		SystemPrompt: "test prompt",
		Summaries: []*summarizefilechanges.Output{
			{
				Filename: "test.go",
				Summary:  "test summary",
				Skipped:  false,
			},
		},
		Intent: "test intent",
	}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if output.PRDescription != "test PR description" {
		t.Errorf("Expected PR description 'test PR description', got %s", output.PRDescription)
	}
}

func TestNewActivity_NilFactory(t *testing.T) {
	var factory *Factory
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{
		Preset:       &preset.ResolvedPreset{},
		SystemPrompt: "test",
		Summaries:    []*summarizefilechanges.Output{},
	}

	_, err := activity(ctx, execCtx, input)
	if err == nil {
		t.Fatal("expected error for nil factory, got nil")
	}
	expectedMsg := "compose PR factory is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestActivity_SkippedSummaries(t *testing.T) {
	mockInvokeLLM := func(ctx context.Context, executorCtx *executor.Context, input *draftcontent.Input) (*draftcontent.Output, error) {
		return &draftcontent.Output{
			Response: &domainGateway.GenerateContentResponse{
				Blocks: []domainGateway.ContentBlock{{Kind: domainGateway.ContentBlockText, Text: "PR description"}},
			},
		}, nil
	}

	mockFactory := &mockContentGenerationActivityFactory{activity: mockInvokeLLM}
	factory, _ := NewFactory(mockFactory)
	activity := factory.NewActivity()

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", nil, mockExec)

	input := &Input{
		Preset:       &preset.ResolvedPreset{},
		SystemPrompt: "test",
		Summaries: []*summarizefilechanges.Output{
			{Filename: "file1.go", Summary: "summary", Skipped: false},
			{Filename: "file2.go", Skipped: true},
		},
	}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.PRDescription != "PR description" {
		t.Errorf("expected PR description, got %q", output.PRDescription)
	}
}

func TestActivity_EmptySummaries(t *testing.T) {
	mockInvokeLLM := func(ctx context.Context, executorCtx *executor.Context, input *draftcontent.Input) (*draftcontent.Output, error) {
		return &draftcontent.Output{
			Response: &domainGateway.GenerateContentResponse{
				Blocks: []domainGateway.ContentBlock{{Kind: domainGateway.ContentBlockText, Text: "empty PR"}},
			},
		}, nil
	}

	mockFactory := &mockContentGenerationActivityFactory{activity: mockInvokeLLM}
	factory, _ := NewFactory(mockFactory)
	activity := factory.NewActivity()

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", nil, mockExec)

	input := &Input{
		Preset:       &preset.ResolvedPreset{},
		SystemPrompt: "test",
		Summaries:    []*summarizefilechanges.Output{},
		Intent:       "",
	}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.PRDescription != "empty PR" {
		t.Errorf("expected PR description, got %q", output.PRDescription)
	}
}
