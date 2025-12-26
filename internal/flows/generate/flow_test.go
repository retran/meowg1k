// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/retran/meowg1k/internal/activities/invokellm"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/domain/task"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Mock factories.
type mockActivityFactory[I, O any] struct {
	newActivityFunc func() executor.Activity[I, O]
}

func (m *mockActivityFactory[I, O]) NewActivity() executor.Activity[I, O] {
	if m.newActivityFunc != nil {
		return m.newActivityFunc()
	}
	return func(ctx context.Context, activityCtx *executor.Context, input I) (O, error) {
		var zero O
		return zero, nil
	}
}

// Mock task config provider.
type mockTaskConfigProvider struct {
	config *task.ResolvedConfig
	err    error
}

func (m *mockTaskConfigProvider) Get() (*task.ResolvedConfig, error) {
	return m.config, m.err
}

// Mock user prompt provider.
type mockUserPromptProvider struct {
	err    error
	prompt string
}

func (m *mockUserPromptProvider) GetUserPrompt() (string, error) {
	return m.prompt, m.err
}

// Mock system prompt provider.
type mockSystemPromptProvider struct {
	err    error
	prompt string
}

func (m *mockSystemPromptProvider) GetSystemPrompt() (string, error) {
	return m.prompt, m.err
}

// Mock output writer.
type mockOutputWriter struct {
	outputs []string
}

func (m *mockOutputWriter) PrintLine(line string) error {
	m.outputs = append(m.outputs, line)
	return nil
}

func TestNewFlowFactory(t *testing.T) {
	tests := []struct {
		taskConfigProvider               TaskConfigProvider
		userPromptProvider               UserPromptProvider
		systemPromptProvider             SystemPromptProvider
		contentGenerationActivityFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output]
		outputWriter                     ports.OutputWriter
		name                             string
		expectedErrMsg                   string
		wantErr                          bool
	}{
		{
			name:                             "nil taskConfigProvider",
			taskConfigProvider:               nil,
			userPromptProvider:               &mockUserPromptProvider{},
			systemPromptProvider:             &mockSystemPromptProvider{},
			contentGenerationActivityFactory: &mockActivityFactory[*invokellm.Input, *invokellm.Output]{},
			outputWriter:                     &mockOutputWriter{},
			wantErr:                          true,
			expectedErrMsg:                   "taskConfigProvider is nil",
		},
		{
			name:                             "nil userPromptProvider",
			taskConfigProvider:               &mockTaskConfigProvider{},
			userPromptProvider:               nil,
			systemPromptProvider:             &mockSystemPromptProvider{},
			contentGenerationActivityFactory: &mockActivityFactory[*invokellm.Input, *invokellm.Output]{},
			outputWriter:                     &mockOutputWriter{},
			wantErr:                          true,
			expectedErrMsg:                   "userPromptProvider is nil",
		},
		{
			name:                             "nil systemPromptProvider",
			taskConfigProvider:               &mockTaskConfigProvider{},
			userPromptProvider:               &mockUserPromptProvider{},
			systemPromptProvider:             nil,
			contentGenerationActivityFactory: &mockActivityFactory[*invokellm.Input, *invokellm.Output]{},
			outputWriter:                     &mockOutputWriter{},
			wantErr:                          true,
			expectedErrMsg:                   "systemPromptProvider is nil",
		},
		{
			name:                             "nil contentGenerationActivityFactory",
			taskConfigProvider:               &mockTaskConfigProvider{},
			userPromptProvider:               &mockUserPromptProvider{},
			systemPromptProvider:             &mockSystemPromptProvider{},
			contentGenerationActivityFactory: nil,
			outputWriter:                     &mockOutputWriter{},
			wantErr:                          true,
			expectedErrMsg:                   "contentGenerationActivityFactory is nil",
		},
		{
			name:                             "nil outputWriter",
			taskConfigProvider:               &mockTaskConfigProvider{},
			userPromptProvider:               &mockUserPromptProvider{},
			systemPromptProvider:             &mockSystemPromptProvider{},
			contentGenerationActivityFactory: &mockActivityFactory[*invokellm.Input, *invokellm.Output]{},
			outputWriter:                     nil,
			wantErr:                          true,
			expectedErrMsg:                   "outputWriter is nil",
		},
		{
			name:                             "all valid dependencies",
			taskConfigProvider:               &mockTaskConfigProvider{},
			userPromptProvider:               &mockUserPromptProvider{},
			systemPromptProvider:             &mockSystemPromptProvider{},
			contentGenerationActivityFactory: &mockActivityFactory[*invokellm.Input, *invokellm.Output]{},
			outputWriter:                     &mockOutputWriter{},
			wantErr:                          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory, err := NewFlowFactory(
				tt.taskConfigProvider,
				tt.userPromptProvider,
				tt.systemPromptProvider,
				tt.contentGenerationActivityFactory,
				tt.outputWriter,
			)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
					return
				}
				if factory != nil {
					t.Errorf("expected nil factory but got %v", factory)
				}
				if tt.expectedErrMsg != "" && !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("expected error message to contain %q, got %q", tt.expectedErrMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
			if factory == nil {
				t.Errorf("expected non-nil factory but got nil")
			}
		})
	}
}

func TestFlowFactory_NewFlow(t *testing.T) {
	tests := []struct {
		setupFactory   func() *FlowFactory
		setupContext   func() (context.Context, *executor.Context)
		name           string
		expectedErrMsg string
		wantErr        bool
	}{
		{
			name: "nil factory",
			setupFactory: func() *FlowFactory {
				return nil
			},
			setupContext: func() (context.Context, *executor.Context) {
				return context.Background(), executor.NewContext("test", nil, nil)
			},
			wantErr:        true,
			expectedErrMsg: "factory is nil",
		},
		{
			name: "nil context",
			setupFactory: func() *FlowFactory {
				factory, _ := NewFlowFactory(
					&mockTaskConfigProvider{},
					&mockUserPromptProvider{},
					&mockSystemPromptProvider{},
					&mockActivityFactory[*invokellm.Input, *invokellm.Output]{},
					&mockOutputWriter{},
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				return nil, executor.NewContext("test", nil, nil)
			},
			wantErr:        true,
			expectedErrMsg: "context is nil",
		},
		{
			name: "nil flow context",
			setupFactory: func() *FlowFactory {
				factory, _ := NewFlowFactory(
					&mockTaskConfigProvider{},
					&mockUserPromptProvider{},
					&mockSystemPromptProvider{},
					&mockActivityFactory[*invokellm.Input, *invokellm.Output]{},
					&mockOutputWriter{},
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				return context.Background(), nil
			},
			wantErr:        true,
			expectedErrMsg: "flow context is nil",
		},
		{
			name: "error getting task config",
			setupFactory: func() *FlowFactory {
				factory, _ := NewFlowFactory(
					&mockTaskConfigProvider{err: errors.New("task config error")},
					&mockUserPromptProvider{},
					&mockSystemPromptProvider{},
					&mockActivityFactory[*invokellm.Input, *invokellm.Output]{},
					&mockOutputWriter{},
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				exec := executor.NewExecutor(0)
				flowCtx := executor.NewContext("test", nil, exec)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "failed to get task config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := tt.setupFactory()
			ctx, flowCtx := tt.setupContext()

			flow := factory.NewFlow()
			err := flow(ctx, flowCtx)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
				}
				if tt.expectedErrMsg != "" && err != nil && !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("expected error message to contain %q, got %q", tt.expectedErrMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}

func TestFlowFactory_NewFlow_UserPromptError(t *testing.T) {
	factory, _ := NewFlowFactory(
		&mockTaskConfigProvider{config: &task.ResolvedConfig{Profile: &profile.ResolvedProfile{}}},
		&mockUserPromptProvider{err: errors.New("user prompt error")},
		&mockSystemPromptProvider{},
		&mockActivityFactory[*invokellm.Input, *invokellm.Output]{},
		&mockOutputWriter{},
	)

	ctx := context.Background()
	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", nil, exec)

	flow := factory.NewFlow()
	err := flow(ctx, flowCtx)

	if err == nil {
		t.Fatal("expected error from user prompt provider")
	}

	if !strings.Contains(err.Error(), "failed to get user prompt") {
		t.Errorf("expected error about user prompt, got: %v", err)
	}
}

func TestFlowFactory_NewFlow_SystemPromptError(t *testing.T) {
	factory, _ := NewFlowFactory(
		&mockTaskConfigProvider{config: &task.ResolvedConfig{Profile: &profile.ResolvedProfile{}}},
		&mockUserPromptProvider{prompt: "test prompt"},
		&mockSystemPromptProvider{err: errors.New("system prompt error")},
		&mockActivityFactory[*invokellm.Input, *invokellm.Output]{},
		&mockOutputWriter{},
	)

	ctx := context.Background()
	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", nil, exec)

	flow := factory.NewFlow()
	err := flow(ctx, flowCtx)

	if err == nil {
		t.Fatal("expected error from system prompt provider")
	}

	if !strings.Contains(err.Error(), "failed to get system prompt") {
		t.Errorf("expected error about system prompt, got: %v", err)
	}
}

func TestFlowFactory_NewFlow_ActivityExecutionError(t *testing.T) {
	mockActivityFactory := &mockActivityFactory[*invokellm.Input, *invokellm.Output]{
		newActivityFunc: func() executor.Activity[*invokellm.Input, *invokellm.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *invokellm.Input) (*invokellm.Output, error) {
				return nil, errors.New("LLM invocation error")
			}
		},
	}

	factory, _ := NewFlowFactory(
		&mockTaskConfigProvider{config: &task.ResolvedConfig{Profile: &profile.ResolvedProfile{}}},
		&mockUserPromptProvider{prompt: "test prompt"},
		&mockSystemPromptProvider{prompt: "test system prompt"},
		mockActivityFactory,
		&mockOutputWriter{},
	)

	ctx := context.Background()
	exec := executor.NewExecutor(1)
	flowCtx := executor.NewContext("test", nil, exec)

	flow := factory.NewFlow()
	err := flow(ctx, flowCtx)

	if err == nil {
		t.Fatal("expected error from activity execution")
	}

	if !strings.Contains(err.Error(), "failed to execute \"InvokeLLM\" activity") {
		t.Errorf("expected error about activity execution, got: %v", err)
	}
}

func TestFlowFactory_NewFlow_OutputWriterError(t *testing.T) {
	mockActivityFactory := &mockActivityFactory[*invokellm.Input, *invokellm.Output]{
		newActivityFunc: func() executor.Activity[*invokellm.Input, *invokellm.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *invokellm.Input) (*invokellm.Output, error) {
				return &invokellm.Output{Content: "Generated content"}, nil
			}
		},
	}

	mockWriter := &mockOutputWriterWithError{}

	factory, _ := NewFlowFactory(
		&mockTaskConfigProvider{config: &task.ResolvedConfig{Profile: &profile.ResolvedProfile{}}},
		&mockUserPromptProvider{prompt: "test prompt"},
		&mockSystemPromptProvider{prompt: "test system prompt"},
		mockActivityFactory,
		mockWriter,
	)

	ctx := context.Background()
	exec := executor.NewExecutor(1)
	flowCtx := executor.NewContext("test", nil, exec)

	flow := factory.NewFlow()
	err := flow(ctx, flowCtx)

	if err == nil {
		t.Fatal("expected error from output writer")
	}

	if !strings.Contains(err.Error(), "failed to print generated content") {
		t.Errorf("expected error about printing content, got: %v", err)
	}
}

func TestFlowFactory_NewFlow_Success(t *testing.T) {
	testProfile := &profile.ResolvedProfile{Name: "test-profile"}

	mockActivityFactory := &mockActivityFactory[*invokellm.Input, *invokellm.Output]{
		newActivityFunc: func() executor.Activity[*invokellm.Input, *invokellm.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *invokellm.Input) (*invokellm.Output, error) {
				// Verify input
				if input.Profile.Name != "test-profile" {
					return nil, errors.New("unexpected profile")
				}
				if input.UserPrompt != "user test prompt" {
					return nil, errors.New("unexpected user prompt")
				}
				if input.SystemPrompt != "system test prompt" {
					return nil, errors.New("unexpected system prompt")
				}
				return &invokellm.Output{Content: "  Generated content with whitespace  "}, nil
			}
		},
	}

	mockWriter := &mockOutputWriter{}

	factory, _ := NewFlowFactory(
		&mockTaskConfigProvider{config: &task.ResolvedConfig{Profile: testProfile}},
		&mockUserPromptProvider{prompt: "user test prompt"},
		&mockSystemPromptProvider{prompt: "system test prompt"},
		mockActivityFactory,
		mockWriter,
	)

	ctx := context.Background()
	exec := executor.NewExecutor(1)
	flowCtx := executor.NewContext("test", nil, exec)

	flow := factory.NewFlow()
	err := flow(ctx, flowCtx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(mockWriter.outputs) != 1 {
		t.Fatalf("expected 1 output, got %d", len(mockWriter.outputs))
	}

	// Verify content is trimmed
	if mockWriter.outputs[0] != "Generated content with whitespace" {
		t.Errorf("expected 'Generated content with whitespace', got %q", mockWriter.outputs[0])
	}
}

// Mock output writer that returns error.
type mockOutputWriterWithError struct{}

func (m *mockOutputWriterWithError) PrintLine(line string) error {
	return errors.New("output writer error")
}
