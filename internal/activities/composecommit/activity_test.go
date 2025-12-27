// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package composecommit

import (
	"context"
	"testing"

	"github.com/retran/meowg1k/internal/activities/generatecontent"
	"github.com/retran/meowg1k/internal/activities/summarizefile"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockContentGenerationActivityFactory is a mock implementation of ContentGenerationActivityFactory for testing.
type mockContentGenerationActivityFactory struct {
	activity executor.Activity[*generatecontent.Input, *generatecontent.Output]
}

func (m *mockContentGenerationActivityFactory) NewActivity() executor.Activity[*generatecontent.Input, *generatecontent.Output] {
	return m.activity
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
	// Create a mock activity that returns predefined content
	mockInvokeLLM := func(ctx context.Context, executorCtx *executor.Context, input *generatecontent.Input) (*generatecontent.Output, error) {
		return &generatecontent.Output{
			Content: "test commit message",
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
		Profile:      &profile.ResolvedProfile{},
		SystemPrompt: "test prompt",
		Summaries: []*summarizefile.Output{
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

	if output.CommitMessage != "test commit message" {
		t.Errorf("Expected commit message 'test commit message', got %s", output.CommitMessage)
	}
}

func TestNewFactory_NilContentGenerationFactory(t *testing.T) {
	_, err := NewFactory(nil)
	if err == nil {
		t.Fatal("expected error for nil content generation factory, got nil")
	}
	expectedMsg := "contentGenerationActivityFactory cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestNewActivity_NilFactory(t *testing.T) {
	var factory *Factory
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{
		Profile:      &profile.ResolvedProfile{},
		SystemPrompt: "test",
		Summaries:    []*summarizefile.Output{},
	}

	_, err := activity(ctx, execCtx, input)
	if err == nil {
		t.Fatal("expected error for nil factory, got nil")
	}
	expectedMsg := "factory is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestActivity_SkippedSummaries(t *testing.T) {
	mockInvokeLLM := func(ctx context.Context, executorCtx *executor.Context, input *generatecontent.Input) (*generatecontent.Output, error) {
		return &generatecontent.Output{Content: "commit message"}, nil
	}

	mockFactory := &mockContentGenerationActivityFactory{activity: mockInvokeLLM}
	factory, _ := NewFactory(mockFactory)
	activity := factory.NewActivity()

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", nil, mockExec)

	input := &Input{
		Profile:      &profile.ResolvedProfile{},
		SystemPrompt: "test",
		Summaries: []*summarizefile.Output{
			{Filename: "file1.go", Summary: "summary", Skipped: false},
			{Filename: "file2.go", Skipped: true},
		},
	}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.CommitMessage != "commit message" {
		t.Errorf("expected commit message, got %q", output.CommitMessage)
	}
}

func TestActivity_EmptySummaries(t *testing.T) {
	mockInvokeLLM := func(ctx context.Context, executorCtx *executor.Context, input *generatecontent.Input) (*generatecontent.Output, error) {
		return &generatecontent.Output{Content: "empty commit"}, nil
	}

	mockFactory := &mockContentGenerationActivityFactory{activity: mockInvokeLLM}
	factory, _ := NewFactory(mockFactory)
	activity := factory.NewActivity()

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", nil, mockExec)

	input := &Input{
		Profile:      &profile.ResolvedProfile{},
		SystemPrompt: "test",
		Summaries:    []*summarizefile.Output{},
		Intent:       "",
	}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.CommitMessage != "empty commit" {
		t.Errorf("expected commit message, got %q", output.CommitMessage)
	}
}
