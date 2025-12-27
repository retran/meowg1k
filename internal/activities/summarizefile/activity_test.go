// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package summarizefile

import (
	"context"
	"fmt"
	"testing"

	"github.com/retran/meowg1k/internal/activities/generatecontent"
	"github.com/retran/meowg1k/internal/domain/summarize"
	"github.com/retran/meowg1k/pkg/executor"
)

type mockContentGenerationFactory struct {
	newActivityFunc func() executor.Activity[*generatecontent.Input, *generatecontent.Output]
}

func (m *mockContentGenerationFactory) NewActivity() executor.Activity[*generatecontent.Input, *generatecontent.Output] {
	if m.newActivityFunc != nil {
		return m.newActivityFunc()
	}
	return nil
}

type mockSummarizationConfigProvider struct {
	getFunc func(filename string) (*summarize.ResolvedConfig, error)
}

func (m *mockSummarizationConfigProvider) Get(filename string) (*summarize.ResolvedConfig, error) {
	if m.getFunc != nil {
		return m.getFunc(filename)
	}
	return nil, nil
}

func TestNewFactory_NilContentGenerationFactory(t *testing.T) {
	_, err := NewFactory(nil, &mockSummarizationConfigProvider{})
	if err == nil {
		t.Fatal("expected error for nil content generation factory, got nil")
	}
	expectedMsg := "content generation activity factory is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestNewFactory_NilSummarizationConfigProvider(t *testing.T) {
	_, err := NewFactory(&mockContentGenerationFactory{}, nil)
	if err == nil {
		t.Fatal("expected error for nil summarization config provider, got nil")
	}
	expectedMsg := "file summarization config provider is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestNewFactory_Success(t *testing.T) {
	factory, err := NewFactory(&mockContentGenerationFactory{}, &mockSummarizationConfigProvider{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if factory == nil {
		t.Fatal("expected non-nil factory, got nil")
	}
}

func TestNewActivity_NilFactory(t *testing.T) {
	var factory *Factory
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{Filename: "test.go"}

	_, err := activity(ctx, execCtx, input)
	if err == nil {
		t.Fatal("expected error for nil factory, got nil")
	}
	expectedMsg := "factory is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestActivity_NilInput(t *testing.T) {
	factory, _ := NewFactory(&mockContentGenerationFactory{}, &mockSummarizationConfigProvider{})
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)

	_, err := activity(ctx, execCtx, nil)
	if err == nil {
		t.Fatal("expected error for nil input, got nil")
	}
	expectedMsg := "input cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestActivity_ConfigProviderError(t *testing.T) {
	mockConfigProvider := &mockSummarizationConfigProvider{
		getFunc: func(filename string) (*summarize.ResolvedConfig, error) {
			return nil, fmt.Errorf("config error")
		},
	}

	factory, _ := NewFactory(&mockContentGenerationFactory{}, mockConfigProvider)
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{Filename: "test.go"}

	_, err := activity(ctx, execCtx, input)
	if err == nil {
		t.Fatal("expected error when config provider fails, got nil")
	}
}

func TestActivity_SkippedFile(t *testing.T) {
	mockConfigProvider := &mockSummarizationConfigProvider{
		getFunc: func(filename string) (*summarize.ResolvedConfig, error) {
			return &summarize.ResolvedConfig{Skip: true}, nil
		},
	}

	factory, _ := NewFactory(&mockContentGenerationFactory{}, mockConfigProvider)
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{Filename: "test.go"}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !output.Skipped {
		t.Error("expected file to be skipped")
	}
	if output.Summary != "" {
		t.Errorf("expected empty summary for skipped file, got %q", output.Summary)
	}
	if output.Filename != "test.go" {
		t.Errorf("expected filename %q, got %q", "test.go", output.Filename)
	}
}

func TestActivity_NilConfig(t *testing.T) {
	mockConfigProvider := &mockSummarizationConfigProvider{
		getFunc: func(filename string) (*summarize.ResolvedConfig, error) {
			return nil, nil
		},
	}

	factory, _ := NewFactory(&mockContentGenerationFactory{}, mockConfigProvider)
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{Filename: "test.go"}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !output.Skipped {
		t.Error("expected file to be skipped when config is nil")
	}
}

func TestActivity_SuccessWithAllContent(t *testing.T) {
	mockConfigProvider := &mockSummarizationConfigProvider{
		getFunc: func(filename string) (*summarize.ResolvedConfig, error) {
			return &summarize.ResolvedConfig{
				Skip:                false,
				IncludeOriginalFile: true,
				IncludeChangedFile:  true,
				Profile:             nil,
				SystemPrompt:        "test prompt",
			}, nil
		},
	}

	mockLLM := func(ctx context.Context, executorCtx *executor.Context, input *generatecontent.Input) (*generatecontent.Output, error) {
		return &generatecontent.Output{Content: "test summary"}, nil
	}

	mockFactory := &mockContentGenerationFactory{
		newActivityFunc: func() executor.Activity[*generatecontent.Input, *generatecontent.Output] {
			return mockLLM
		},
	}

	factory, _ := NewFactory(mockFactory, mockConfigProvider)
	activity := factory.NewActivity()

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", nil, mockExec)

	input := &Input{
		Filename:            "test.go",
		Change:              "diff content",
		OriginalFileContent: "original",
		StagedFileContent:   "staged",
	}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.Skipped {
		t.Error("expected file not to be skipped")
	}
	if output.Summary != "test summary" {
		t.Errorf("expected summary %q, got %q", "test summary", output.Summary)
	}
	if output.Filename != "test.go" {
		t.Errorf("expected filename %q, got %q", "test.go", output.Filename)
	}
}

func TestActivity_SuccessWithoutOriginalFile(t *testing.T) {
	mockConfigProvider := &mockSummarizationConfigProvider{
		getFunc: func(filename string) (*summarize.ResolvedConfig, error) {
			return &summarize.ResolvedConfig{
				Skip:                false,
				IncludeOriginalFile: false,
				IncludeChangedFile:  true,
				Profile:             nil,
				SystemPrompt:        "test prompt",
			}, nil
		},
	}

	mockLLM := func(ctx context.Context, executorCtx *executor.Context, input *generatecontent.Input) (*generatecontent.Output, error) {
		return &generatecontent.Output{Content: "summary without original"}, nil
	}

	mockFactory := &mockContentGenerationFactory{
		newActivityFunc: func() executor.Activity[*generatecontent.Input, *generatecontent.Output] {
			return mockLLM
		},
	}

	factory, _ := NewFactory(mockFactory, mockConfigProvider)
	activity := factory.NewActivity()

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", nil, mockExec)

	input := &Input{
		Filename:            "test.go",
		Change:              "diff",
		OriginalFileContent: "original",
		StagedFileContent:   "staged",
	}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.Summary != "summary without original" {
		t.Errorf("expected summary, got %q", output.Summary)
	}
}

func TestActivity_SuccessWithoutChangedFile(t *testing.T) {
	mockConfigProvider := &mockSummarizationConfigProvider{
		getFunc: func(filename string) (*summarize.ResolvedConfig, error) {
			return &summarize.ResolvedConfig{
				Skip:                false,
				IncludeOriginalFile: true,
				IncludeChangedFile:  false,
				Profile:             nil,
				SystemPrompt:        "test prompt",
			}, nil
		},
	}

	mockLLM := func(ctx context.Context, executorCtx *executor.Context, input *generatecontent.Input) (*generatecontent.Output, error) {
		return &generatecontent.Output{Content: "summary without changed"}, nil
	}

	mockFactory := &mockContentGenerationFactory{
		newActivityFunc: func() executor.Activity[*generatecontent.Input, *generatecontent.Output] {
			return mockLLM
		},
	}

	factory, _ := NewFactory(mockFactory, mockConfigProvider)
	activity := factory.NewActivity()

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", nil, mockExec)

	input := &Input{
		Filename:            "test.go",
		Change:              "diff",
		OriginalFileContent: "original",
		StagedFileContent:   "staged",
	}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.Summary != "summary without changed" {
		t.Errorf("expected summary, got %q", output.Summary)
	}
}

func TestActivity_GenerationError(t *testing.T) {
	mockConfigProvider := &mockSummarizationConfigProvider{
		getFunc: func(filename string) (*summarize.ResolvedConfig, error) {
			return &summarize.ResolvedConfig{
				Skip:         false,
				Profile:      nil,
				SystemPrompt: "test",
			}, nil
		},
	}

	mockLLM := func(ctx context.Context, executorCtx *executor.Context, input *generatecontent.Input) (*generatecontent.Output, error) {
		return nil, fmt.Errorf("generation failed")
	}

	mockFactory := &mockContentGenerationFactory{
		newActivityFunc: func() executor.Activity[*generatecontent.Input, *generatecontent.Output] {
			return mockLLM
		},
	}

	factory, _ := NewFactory(mockFactory, mockConfigProvider)
	activity := factory.NewActivity()

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", nil, mockExec)

	input := &Input{
		Filename: "test.go",
		Change:   "diff",
	}

	_, err := activity(ctx, execCtx, input)
	if err == nil {
		t.Fatal("expected error when generation fails, got nil")
	}
}
