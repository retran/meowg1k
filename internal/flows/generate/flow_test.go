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

	"github.com/retran/meowg1k/internal/activities/invokellm"
	"github.com/retran/meowg1k/internal/domain/task"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Mock factories
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

// Mock task config provider
type mockTaskConfigProvider struct {
	config *task.ResolvedConfig
	err    error
}

func (m *mockTaskConfigProvider) Get() (*task.ResolvedConfig, error) {
	return m.config, m.err
}

// Mock user prompt provider
type mockUserPromptProvider struct {
	prompt string
	err    error
}

func (m *mockUserPromptProvider) GetUserPrompt() (string, error) {
	return m.prompt, m.err
}

// Mock system prompt provider
type mockSystemPromptProvider struct {
	prompt string
	err    error
}

func (m *mockSystemPromptProvider) GetSystemPrompt() (string, error) {
	return m.prompt, m.err
}

// Mock output writer
type mockOutputWriter struct {
	outputs []string
}

func (m *mockOutputWriter) PrintLine(line string) error {
	m.outputs = append(m.outputs, line)
	return nil
}

func TestNewFlowFactory(t *testing.T) {
	tests := []struct {
		name                             string
		taskConfigProvider               TaskConfigProvider
		userPromptProvider               UserPromptProvider
		systemPromptProvider             SystemPromptProvider
		contentGenerationActivityFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output]
		outputWriter                     ports.OutputWriter
		wantErr                          bool
		expectedErrMsg                   string
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
				}
				if factory != nil {
					t.Errorf("expected nil factory but got %v", factory)
				}
				if tt.expectedErrMsg != "" && err != nil && !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("expected error message to contain %q, got %q", tt.expectedErrMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
				if factory == nil {
					t.Errorf("expected non-nil factory but got nil")
				}
			}
		})
	}
}

func TestFlowFactory_NewFlow(t *testing.T) {
	tests := []struct {
		name           string
		setupFactory   func() *FlowFactory
		setupContext   func() (context.Context, *executor.Context)
		wantErr        bool
		expectedErrMsg string
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
				flowCtx := executor.NewContext("test", nil, nil)
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
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}
