// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ask

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/retran/meowg1k/internal/activities/draftcontent"
	"github.com/retran/meowg1k/internal/activities/fetchcontext"
	"github.com/retran/meowg1k/internal/domain/config"
	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Mock activity factory.
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

// Mock command parameters reader.
type mockCommandParametersReader struct {
	questionErr  error
	snapshotsErr error
	topKErr      error
	minScoreErr  error
	presetErr    error
	question     string
	preset       string
	snapshots    []string
	topK         int
	mu           sync.Mutex
	minScore     float32
}

func (m *mockCommandParametersReader) GetQuestionFlag() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.question, m.questionErr
}

func (m *mockCommandParametersReader) GetSnapshotsFlag() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshots, m.snapshotsErr
}

func (m *mockCommandParametersReader) GetTopKFlag() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.topK, m.topKErr
}

func (m *mockCommandParametersReader) GetMinScoreFlag() (float32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.minScore, m.minScoreErr
}

func (m *mockCommandParametersReader) GetPresetFlag() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.preset, m.presetErr
}

// Mock config reader.
type mockConfigReader struct {
	err    error
	config *config.Config
	mu     sync.Mutex
}

func (m *mockConfigReader) Get() (*config.Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.config, m.err
}

func answerConfig(preset string, topK int, minScore float32) *config.Config {
	return &config.Config{
		Flows: &config.FlowsConfig{
			Answer: &config.AnswerFlowConfig{
				Preset: preset,
				Retrieval: &config.RetrievalConfig{
					TopK:     topK,
					MinScore: minScore,
				},
			},
		},
	}
}

// Mock preset resolver.
type mockPresetResolver struct {
	err    error
	preset *preset.ResolvedPreset
	mu     sync.Mutex
}

func (m *mockPresetResolver) Get(p preset.Preset) (*preset.ResolvedPreset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.preset, m.err
}

// Mock output writer.
type mockOutputWriter struct {
	outputs []string
	mu      sync.Mutex
}

func (m *mockOutputWriter) PrintLine(line string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outputs = append(m.outputs, line)
	return nil
}

func TestNewFactory(t *testing.T) {
	tests := []struct {
		retrieveContextFactory executor.ActivityFactory[*fetchcontext.Input, *fetchcontext.Output]
		invokeLLMFactory       executor.ActivityFactory[*draftcontent.Input, *draftcontent.Output]
		parametersReader       CommandParametersReader
		presetResolver         ports.PresetResolver
		outputWriter           ports.OutputWriter
		configReader           ConfigReader
		name                   string
		expectedErrMsg         string
		wantErr                bool
	}{
		{
			name:                   "nil retrieveContextFactory",
			retrieveContextFactory: nil,
			invokeLLMFactory:       &mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
			parametersReader:       &mockCommandParametersReader{},
			presetResolver:         &mockPresetResolver{},
			outputWriter:           &mockOutputWriter{},
			configReader:           &mockConfigReader{},
			wantErr:                true,
			expectedErrMsg:         "retrieveContextFactory cannot be nil",
		},
		{
			name:                   "nil invokeLLMFactory",
			retrieveContextFactory: &mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{},
			invokeLLMFactory:       nil,
			parametersReader:       &mockCommandParametersReader{},
			presetResolver:         &mockPresetResolver{},
			outputWriter:           &mockOutputWriter{},
			configReader:           &mockConfigReader{},
			wantErr:                true,
			expectedErrMsg:         "invokeLLMFactory cannot be nil",
		},
		{
			name:                   "nil parametersReader",
			retrieveContextFactory: &mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{},
			invokeLLMFactory:       &mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
			parametersReader:       nil,
			presetResolver:         &mockPresetResolver{},
			outputWriter:           &mockOutputWriter{},
			configReader:           &mockConfigReader{},
			wantErr:                true,
			expectedErrMsg:         "parametersReader cannot be nil",
		},
		{
			name:                   "nil presetResolver",
			retrieveContextFactory: &mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{},
			invokeLLMFactory:       &mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
			parametersReader:       &mockCommandParametersReader{},
			presetResolver:         nil,
			outputWriter:           &mockOutputWriter{},
			configReader:           &mockConfigReader{},
			wantErr:                true,
			expectedErrMsg:         "presetResolver cannot be nil",
		},
		{
			name:                   "nil outputWriter",
			retrieveContextFactory: &mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{},
			invokeLLMFactory:       &mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
			parametersReader:       &mockCommandParametersReader{},
			presetResolver:         &mockPresetResolver{},
			outputWriter:           nil,
			configReader:           &mockConfigReader{},
			wantErr:                true,
			expectedErrMsg:         "outputWriter cannot be nil",
		},
		{
			name:                   "nil configReader",
			retrieveContextFactory: &mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{},
			invokeLLMFactory:       &mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
			parametersReader:       &mockCommandParametersReader{},
			presetResolver:         &mockPresetResolver{},
			outputWriter:           &mockOutputWriter{},
			configReader:           nil,
			wantErr:                true,
			expectedErrMsg:         "configReader cannot be nil",
		},
		{
			name:                   "successful factory creation",
			retrieveContextFactory: &mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{},
			invokeLLMFactory:       &mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
			parametersReader:       &mockCommandParametersReader{},
			presetResolver:         &mockPresetResolver{},
			outputWriter:           &mockOutputWriter{},
			configReader:           &mockConfigReader{},
			wantErr:                false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory, err := NewFactory(
				tt.retrieveContextFactory,
				tt.invokeLLMFactory,
				tt.parametersReader,
				tt.presetResolver,
				tt.outputWriter,
				tt.configReader,
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

func TestFactory_NewFlow(t *testing.T) {
	tests := []struct {
		setupFactory   func() *Factory
		setupContext   func() (context.Context, *executor.Context)
		name           string
		expectedErrMsg string
		wantErr        bool
	}{
		{
			name: "error getting config",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					err: errors.New("config error"),
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{},
					&mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
					&mockCommandParametersReader{},
					&mockPresetResolver{},
					&mockOutputWriter{},
					mockConfigReader,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				flowCtx := executor.NewContext("test", nil, nil)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "failed to get config",
		},
		{
			name: "missing answer configuration",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: &config.Config{
						Flows: &config.FlowsConfig{
							Answer: nil,
						},
					},
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{},
					&mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
					&mockCommandParametersReader{},
					&mockPresetResolver{},
					&mockOutputWriter{},
					mockConfigReader,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				flowCtx := executor.NewContext("test", nil, nil)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "answer configuration is missing",
		},
		{
			name: "error getting question",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: answerConfig("default", 10, 0.5),
				}
				mockReader := &mockCommandParametersReader{
					questionErr: errors.New("question error"),
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{},
					&mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
					mockReader,
					&mockPresetResolver{},
					&mockOutputWriter{},
					mockConfigReader,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				flowCtx := executor.NewContext("test", nil, nil)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "failed to get question",
		},
		{
			name: "empty question",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: answerConfig("default", 10, 0.5),
				}
				mockReader := &mockCommandParametersReader{
					question: "",
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{},
					&mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
					mockReader,
					&mockPresetResolver{},
					&mockOutputWriter{},
					mockConfigReader,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				flowCtx := executor.NewContext("test", nil, nil)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "question is required",
		},
		{
			name: "error getting snapshots",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: answerConfig("default", 10, 0.5),
				}
				mockReader := &mockCommandParametersReader{
					question:     "test question",
					snapshotsErr: errors.New("snapshots error"),
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{},
					&mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
					mockReader,
					&mockPresetResolver{},
					&mockOutputWriter{},
					mockConfigReader,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				flowCtx := executor.NewContext("test", nil, nil)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "failed to get snapshots",
		},
		{
			name: "error getting topK",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: answerConfig("default", 10, 0.5),
				}
				mockReader := &mockCommandParametersReader{
					question:  "test question",
					snapshots: []string{"_head_"},
					topKErr:   errors.New("topK error"),
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{},
					&mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
					mockReader,
					&mockPresetResolver{},
					&mockOutputWriter{},
					mockConfigReader,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				flowCtx := executor.NewContext("test", nil, nil)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "failed to get topK",
		},
		{
			name: "error getting min score",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: answerConfig("default", 10, 0.5),
				}
				mockReader := &mockCommandParametersReader{
					question:    "test question",
					snapshots:   []string{"_head_"},
					topK:        5,
					minScoreErr: errors.New("min score error"),
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{},
					&mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
					mockReader,
					&mockPresetResolver{},
					&mockOutputWriter{},
					mockConfigReader,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				flowCtx := executor.NewContext("test", nil, nil)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "failed to get min score",
		},
		{
			name: "error getting preset",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: answerConfig("default", 10, 0.5),
				}
				mockReader := &mockCommandParametersReader{
					question:  "test question",
					snapshots: []string{"_head_"},
					topK:      5,
					minScore:  0.7,
					presetErr: errors.New("preset error"),
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{},
					&mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
					mockReader,
					&mockPresetResolver{},
					&mockOutputWriter{},
					mockConfigReader,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				flowCtx := executor.NewContext("test", nil, nil)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "failed to get preset",
		},
		{
			name: "empty preset in config and flag",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: answerConfig("", 10, 0.5),
				}
				mockReader := &mockCommandParametersReader{
					question:  "test question",
					snapshots: []string{"_head_"},
					topK:      5,
					minScore:  0.7,
					preset:    "",
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{},
					&mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
					mockReader,
					&mockPresetResolver{},
					&mockOutputWriter{},
					mockConfigReader,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				flowCtx := executor.NewContext("test", nil, nil)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "preset is required",
		},
		{
			name: "error resolving preset",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: answerConfig("default", 10, 0.5),
				}
				mockReader := &mockCommandParametersReader{
					question:  "test question",
					snapshots: []string{"_head_"},
					topK:      5,
					minScore:  0.7,
					preset:    "test-preset",
				}
				mockResolver := &mockPresetResolver{
					err: errors.New("preset resolve error"),
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{},
					&mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
					mockReader,
					mockResolver,
					&mockOutputWriter{},
					mockConfigReader,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				flowCtx := executor.NewContext("test", nil, nil)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "failed to resolve preset",
		},
		{
			name: "executor not available",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: answerConfig("default", 10, 0.5),
				}
				mockReader := &mockCommandParametersReader{
					question:  "test question",
					snapshots: []string{"_head_"},
					topK:      5,
					minScore:  0.7,
					preset:    "test-preset",
				}
				mockResolver := &mockPresetResolver{
					preset: &preset.ResolvedPreset{},
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{},
					&mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
					mockReader,
					mockResolver,
					&mockOutputWriter{},
					mockConfigReader,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				flowCtx := executor.NewContext("test", nil, nil)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "executor not available in context",
		},
		{
			name: "successful flow - with context found",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: answerConfig("default", 10, 0.5),
				}
				mockReader := &mockCommandParametersReader{
					question:  "test question",
					snapshots: []string{},
					topK:      0,
					minScore:  0,
					preset:    "",
				}
				mockResolver := &mockPresetResolver{
					preset: &preset.ResolvedPreset{},
				}
				mockRetrieveFactory := &mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{
					newActivityFunc: func() executor.Activity[*fetchcontext.Input, *fetchcontext.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *fetchcontext.Input) (*fetchcontext.Output, error) {
							return &fetchcontext.Output{
								Context: "Retrieved context for the question",
							}, nil
						}
					},
				}
				mockLLMFactory := &mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{
					newActivityFunc: func() executor.Activity[*draftcontent.Input, *draftcontent.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *draftcontent.Input) (*draftcontent.Output, error) {
							return &draftcontent.Output{
								Response: &domainGateway.GenerateContentResponse{
									Blocks: []domainGateway.ContentBlock{{Kind: domainGateway.ContentBlockText, Text: "Generated answer"}},
								},
							}, nil
						}
					},
				}
				factory, _ := NewFactory(
					mockRetrieveFactory,
					mockLLMFactory,
					mockReader,
					mockResolver,
					&mockOutputWriter{},
					mockConfigReader,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				exec := executor.NewExecutor(0)
				flowCtx := executor.NewContext("test", nil, exec)
				return ctx, flowCtx
			},
			wantErr: false,
		},
		{
			name: "successful flow - no context found",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: answerConfig("default", 10, 0.5),
				}
				mockReader := &mockCommandParametersReader{
					question:  "test question",
					snapshots: []string{"_head_"},
					topK:      10,
					minScore:  0.5,
					preset:    "default",
				}
				mockResolver := &mockPresetResolver{
					preset: &preset.ResolvedPreset{},
				}
				mockRetrieveFactory := &mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{
					newActivityFunc: func() executor.Activity[*fetchcontext.Input, *fetchcontext.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *fetchcontext.Input) (*fetchcontext.Output, error) {
							return &fetchcontext.Output{
								Context: "",
							}, nil
						}
					},
				}
				factory, _ := NewFactory(
					mockRetrieveFactory,
					&mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
					mockReader,
					mockResolver,
					&mockOutputWriter{},
					mockConfigReader,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				exec := executor.NewExecutor(0)
				flowCtx := executor.NewContext("test", nil, exec)
				return ctx, flowCtx
			},
			wantErr: false,
		},
		{
			name: "retrieve context activity error",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: answerConfig("default", 10, 0.5),
				}
				mockReader := &mockCommandParametersReader{
					question:  "test question",
					snapshots: []string{"_head_"},
					topK:      10,
					minScore:  0.5,
					preset:    "default",
				}
				mockResolver := &mockPresetResolver{
					preset: &preset.ResolvedPreset{},
				}
				mockRetrieveFactory := &mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{
					newActivityFunc: func() executor.Activity[*fetchcontext.Input, *fetchcontext.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *fetchcontext.Input) (*fetchcontext.Output, error) {
							return nil, errors.New("retrieve error")
						}
					},
				}
				factory, _ := NewFactory(
					mockRetrieveFactory,
					&mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{},
					mockReader,
					mockResolver,
					&mockOutputWriter{},
					mockConfigReader,
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
			expectedErrMsg: "context retrieval failed",
		},
		{
			name: "LLM invocation error",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: answerConfig("default", 10, 0.5),
				}
				mockReader := &mockCommandParametersReader{
					question:  "test question",
					snapshots: []string{"_head_"},
					topK:      10,
					minScore:  0.5,
					preset:    "default",
				}
				mockResolver := &mockPresetResolver{
					preset: &preset.ResolvedPreset{},
				}
				mockRetrieveFactory := &mockActivityFactory[*fetchcontext.Input, *fetchcontext.Output]{
					newActivityFunc: func() executor.Activity[*fetchcontext.Input, *fetchcontext.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *fetchcontext.Input) (*fetchcontext.Output, error) {
							return &fetchcontext.Output{
								Context: "Retrieved context",
							}, nil
						}
					},
				}
				mockLLMFactory := &mockActivityFactory[*draftcontent.Input, *draftcontent.Output]{
					newActivityFunc: func() executor.Activity[*draftcontent.Input, *draftcontent.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *draftcontent.Input) (*draftcontent.Output, error) {
							return nil, errors.New("LLM error")
						}
					},
				}
				factory, _ := NewFactory(
					mockRetrieveFactory,
					mockLLMFactory,
					mockReader,
					mockResolver,
					&mockOutputWriter{},
					mockConfigReader,
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
			expectedErrMsg: "answer generation failed",
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
