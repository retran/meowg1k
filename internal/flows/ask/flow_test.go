// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ask

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/retran/meowg1k/internal/activities/generatecontent"
	"github.com/retran/meowg1k/internal/activities/retrievecontext"
	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/profile"
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
	profileErr   error
	question     string
	profile      string
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

func (m *mockCommandParametersReader) GetProfileFlag() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.profile, m.profileErr
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

// Mock profile resolver.
type mockProfileResolver struct {
	err     error
	profile *profile.ResolvedProfile
	mu      sync.Mutex
}

func (m *mockProfileResolver) Get(p profile.Profile) (*profile.ResolvedProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.profile, m.err
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
		retrieveContextFactory executor.ActivityFactory[*retrievecontext.Input, *retrievecontext.Output]
		invokeLLMFactory       executor.ActivityFactory[*generatecontent.Input, *generatecontent.Output]
		parametersReader       CommandParametersReader
		profileResolver        ports.ProfileResolver
		outputWriter           ports.OutputWriter
		configReader           ConfigReader
		name                   string
		expectedErrMsg         string
		wantErr                bool
	}{
		{
			name:                   "nil retrieveContextFactory",
			retrieveContextFactory: nil,
			invokeLLMFactory:       &mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
			parametersReader:       &mockCommandParametersReader{},
			profileResolver:        &mockProfileResolver{},
			outputWriter:           &mockOutputWriter{},
			configReader:           &mockConfigReader{},
			wantErr:                true,
			expectedErrMsg:         "retrieveContextFactory cannot be nil",
		},
		{
			name:                   "nil invokeLLMFactory",
			retrieveContextFactory: &mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{},
			invokeLLMFactory:       nil,
			parametersReader:       &mockCommandParametersReader{},
			profileResolver:        &mockProfileResolver{},
			outputWriter:           &mockOutputWriter{},
			configReader:           &mockConfigReader{},
			wantErr:                true,
			expectedErrMsg:         "invokeLLMFactory cannot be nil",
		},
		{
			name:                   "nil parametersReader",
			retrieveContextFactory: &mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{},
			invokeLLMFactory:       &mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
			parametersReader:       nil,
			profileResolver:        &mockProfileResolver{},
			outputWriter:           &mockOutputWriter{},
			configReader:           &mockConfigReader{},
			wantErr:                true,
			expectedErrMsg:         "parametersReader cannot be nil",
		},
		{
			name:                   "nil profileResolver",
			retrieveContextFactory: &mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{},
			invokeLLMFactory:       &mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
			parametersReader:       &mockCommandParametersReader{},
			profileResolver:        nil,
			outputWriter:           &mockOutputWriter{},
			configReader:           &mockConfigReader{},
			wantErr:                true,
			expectedErrMsg:         "profileResolver cannot be nil",
		},
		{
			name:                   "nil outputWriter",
			retrieveContextFactory: &mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{},
			invokeLLMFactory:       &mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
			parametersReader:       &mockCommandParametersReader{},
			profileResolver:        &mockProfileResolver{},
			outputWriter:           nil,
			configReader:           &mockConfigReader{},
			wantErr:                true,
			expectedErrMsg:         "outputWriter cannot be nil",
		},
		{
			name:                   "nil configReader",
			retrieveContextFactory: &mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{},
			invokeLLMFactory:       &mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
			parametersReader:       &mockCommandParametersReader{},
			profileResolver:        &mockProfileResolver{},
			outputWriter:           &mockOutputWriter{},
			configReader:           nil,
			wantErr:                true,
			expectedErrMsg:         "configReader cannot be nil",
		},
		{
			name:                   "successful factory creation",
			retrieveContextFactory: &mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{},
			invokeLLMFactory:       &mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
			parametersReader:       &mockCommandParametersReader{},
			profileResolver:        &mockProfileResolver{},
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
				tt.profileResolver,
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
					&mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{},
					&mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
					&mockCommandParametersReader{},
					&mockProfileResolver{},
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
			name: "missing ask configuration",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: &config.Config{
						Ask: nil,
					},
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{},
					&mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
					&mockCommandParametersReader{},
					&mockProfileResolver{},
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
			expectedErrMsg: "ask configuration is missing",
		},
		{
			name: "error getting question",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: &config.Config{
						Ask: &config.AskConfig{
							Profile:  "default",
							TopK:     10,
							MinScore: 0.5,
						},
					},
				}
				mockReader := &mockCommandParametersReader{
					questionErr: errors.New("question error"),
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{},
					&mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
					mockReader,
					&mockProfileResolver{},
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
					config: &config.Config{
						Ask: &config.AskConfig{
							Profile:  "default",
							TopK:     10,
							MinScore: 0.5,
						},
					},
				}
				mockReader := &mockCommandParametersReader{
					question: "",
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{},
					&mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
					mockReader,
					&mockProfileResolver{},
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
					config: &config.Config{
						Ask: &config.AskConfig{
							Profile:  "default",
							TopK:     10,
							MinScore: 0.5,
						},
					},
				}
				mockReader := &mockCommandParametersReader{
					question:     "test question",
					snapshotsErr: errors.New("snapshots error"),
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{},
					&mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
					mockReader,
					&mockProfileResolver{},
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
					config: &config.Config{
						Ask: &config.AskConfig{
							Profile:  "default",
							TopK:     10,
							MinScore: 0.5,
						},
					},
				}
				mockReader := &mockCommandParametersReader{
					question:  "test question",
					snapshots: []string{"_head_"},
					topKErr:   errors.New("topK error"),
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{},
					&mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
					mockReader,
					&mockProfileResolver{},
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
					config: &config.Config{
						Ask: &config.AskConfig{
							Profile:  "default",
							TopK:     10,
							MinScore: 0.5,
						},
					},
				}
				mockReader := &mockCommandParametersReader{
					question:    "test question",
					snapshots:   []string{"_head_"},
					topK:        5,
					minScoreErr: errors.New("min score error"),
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{},
					&mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
					mockReader,
					&mockProfileResolver{},
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
			name: "error getting profile",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: &config.Config{
						Ask: &config.AskConfig{
							Profile:  "default",
							TopK:     10,
							MinScore: 0.5,
						},
					},
				}
				mockReader := &mockCommandParametersReader{
					question:   "test question",
					snapshots:  []string{"_head_"},
					topK:       5,
					minScore:   0.7,
					profileErr: errors.New("profile error"),
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{},
					&mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
					mockReader,
					&mockProfileResolver{},
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
			expectedErrMsg: "failed to get profile",
		},
		{
			name: "empty profile in config and flag",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: &config.Config{
						Ask: &config.AskConfig{
							Profile:  "",
							TopK:     10,
							MinScore: 0.5,
						},
					},
				}
				mockReader := &mockCommandParametersReader{
					question:  "test question",
					snapshots: []string{"_head_"},
					topK:      5,
					minScore:  0.7,
					profile:   "",
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{},
					&mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
					mockReader,
					&mockProfileResolver{},
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
			expectedErrMsg: "profile is required",
		},
		{
			name: "error resolving profile",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: &config.Config{
						Ask: &config.AskConfig{
							Profile:  "default",
							TopK:     10,
							MinScore: 0.5,
						},
					},
				}
				mockReader := &mockCommandParametersReader{
					question:  "test question",
					snapshots: []string{"_head_"},
					topK:      5,
					minScore:  0.7,
					profile:   "test-profile",
				}
				mockResolver := &mockProfileResolver{
					err: errors.New("profile resolve error"),
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{},
					&mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
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
			expectedErrMsg: "failed to resolve profile",
		},
		{
			name: "executor not available",
			setupFactory: func() *Factory {
				mockConfigReader := &mockConfigReader{
					config: &config.Config{
						Ask: &config.AskConfig{
							Profile:  "default",
							TopK:     10,
							MinScore: 0.5,
						},
					},
				}
				mockReader := &mockCommandParametersReader{
					question:  "test question",
					snapshots: []string{"_head_"},
					topK:      5,
					minScore:  0.7,
					profile:   "test-profile",
				}
				mockResolver := &mockProfileResolver{
					profile: &profile.ResolvedProfile{},
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{},
					&mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
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
					config: &config.Config{
						Ask: &config.AskConfig{
							Profile:  "default",
							TopK:     10,
							MinScore: 0.5,
						},
					},
				}
				mockReader := &mockCommandParametersReader{
					question:  "test question",
					snapshots: []string{},
					topK:      0,
					minScore:  0,
					profile:   "",
				}
				mockResolver := &mockProfileResolver{
					profile: &profile.ResolvedProfile{},
				}
				mockRetrieveFactory := &mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{
					newActivityFunc: func() executor.Activity[*retrievecontext.Input, *retrievecontext.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *retrievecontext.Input) (*retrievecontext.Output, error) {
							return &retrievecontext.Output{
								Context: "Retrieved context for the question",
							}, nil
						}
					},
				}
				mockLLMFactory := &mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{
					newActivityFunc: func() executor.Activity[*generatecontent.Input, *generatecontent.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *generatecontent.Input) (*generatecontent.Output, error) {
							return &generatecontent.Output{
								Content: "Generated answer",
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
					config: &config.Config{
						Ask: &config.AskConfig{
							Profile:  "default",
							TopK:     10,
							MinScore: 0.5,
						},
					},
				}
				mockReader := &mockCommandParametersReader{
					question:  "test question",
					snapshots: []string{"_head_"},
					topK:      10,
					minScore:  0.5,
					profile:   "default",
				}
				mockResolver := &mockProfileResolver{
					profile: &profile.ResolvedProfile{},
				}
				mockRetrieveFactory := &mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{
					newActivityFunc: func() executor.Activity[*retrievecontext.Input, *retrievecontext.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *retrievecontext.Input) (*retrievecontext.Output, error) {
							return &retrievecontext.Output{
								Context: "",
							}, nil
						}
					},
				}
				factory, _ := NewFactory(
					mockRetrieveFactory,
					&mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
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
					config: &config.Config{
						Ask: &config.AskConfig{
							Profile:  "default",
							TopK:     10,
							MinScore: 0.5,
						},
					},
				}
				mockReader := &mockCommandParametersReader{
					question:  "test question",
					snapshots: []string{"_head_"},
					topK:      10,
					minScore:  0.5,
					profile:   "default",
				}
				mockResolver := &mockProfileResolver{
					profile: &profile.ResolvedProfile{},
				}
				mockRetrieveFactory := &mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{
					newActivityFunc: func() executor.Activity[*retrievecontext.Input, *retrievecontext.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *retrievecontext.Input) (*retrievecontext.Output, error) {
							return nil, errors.New("retrieve error")
						}
					},
				}
				factory, _ := NewFactory(
					mockRetrieveFactory,
					&mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{},
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
					config: &config.Config{
						Ask: &config.AskConfig{
							Profile:  "default",
							TopK:     10,
							MinScore: 0.5,
						},
					},
				}
				mockReader := &mockCommandParametersReader{
					question:  "test question",
					snapshots: []string{"_head_"},
					topK:      10,
					minScore:  0.5,
					profile:   "default",
				}
				mockResolver := &mockProfileResolver{
					profile: &profile.ResolvedProfile{},
				}
				mockRetrieveFactory := &mockActivityFactory[*retrievecontext.Input, *retrievecontext.Output]{
					newActivityFunc: func() executor.Activity[*retrievecontext.Input, *retrievecontext.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *retrievecontext.Input) (*retrievecontext.Output, error) {
							return &retrievecontext.Output{
								Context: "Retrieved context",
							}, nil
						}
					},
				}
				mockLLMFactory := &mockActivityFactory[*generatecontent.Input, *generatecontent.Output]{
					newActivityFunc: func() executor.Activity[*generatecontent.Input, *generatecontent.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *generatecontent.Input) (*generatecontent.Output, error) {
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
