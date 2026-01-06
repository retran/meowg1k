// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package search

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	queryactivity "github.com/retran/meowg1k/internal/activities/searchindex"
	"github.com/retran/meowg1k/internal/core/retrieval"
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
	queryTextErr error
	snapshotsErr error
	topKErr      error
	minScoreErr  error
	jsonErr      error
	queryText    string
	snapshots    []string
	topK         int
	mu           sync.Mutex
	minScore     float32
	useJSON      bool
}

func (m *mockCommandParametersReader) GetQueryTextFlag() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.queryText, m.queryTextErr
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

func (m *mockCommandParametersReader) GetJSONFlag() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.useJSON, m.jsonErr
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
		searchFactory    executor.ActivityFactory[*queryactivity.Input, *queryactivity.Output]
		parametersReader CommandParametersReader
		outputWriter     ports.OutputWriter
		name             string
		expectedErrMsg   string
		wantErr          bool
	}{
		{
			name:             "nil searchFactory",
			searchFactory:    nil,
			parametersReader: &mockCommandParametersReader{},
			outputWriter:     &mockOutputWriter{},
			wantErr:          true,
			expectedErrMsg:   "searchFactory cannot be nil",
		},
		{
			name:             "nil parametersReader",
			searchFactory:    &mockActivityFactory[*queryactivity.Input, *queryactivity.Output]{},
			parametersReader: nil,
			outputWriter:     &mockOutputWriter{},
			wantErr:          true,
			expectedErrMsg:   "parametersReader cannot be nil",
		},
		{
			name:             "nil outputWriter",
			searchFactory:    &mockActivityFactory[*queryactivity.Input, *queryactivity.Output]{},
			parametersReader: &mockCommandParametersReader{},
			outputWriter:     nil,
			wantErr:          true,
			expectedErrMsg:   "outputWriter cannot be nil",
		},
		{
			name:             "successful factory creation",
			searchFactory:    &mockActivityFactory[*queryactivity.Input, *queryactivity.Output]{},
			parametersReader: &mockCommandParametersReader{},
			outputWriter:     &mockOutputWriter{},
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory, err := NewFactory(tt.searchFactory, tt.parametersReader, tt.outputWriter)

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
			name: "error getting search text",
			setupFactory: func() *Factory {
				mockReader := &mockCommandParametersReader{
					queryTextErr: errors.New("search text error"),
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*queryactivity.Input, *queryactivity.Output]{},
					mockReader,
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
			expectedErrMsg: "failed to get search text",
		},
		{
			name: "empty search text",
			setupFactory: func() *Factory {
				mockReader := &mockCommandParametersReader{
					queryText: "",
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*queryactivity.Input, *queryactivity.Output]{},
					mockReader,
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
			expectedErrMsg: "search text is required",
		},
		{
			name: "error getting snapshots",
			setupFactory: func() *Factory {
				mockReader := &mockCommandParametersReader{
					queryText:    "test search",
					snapshotsErr: errors.New("snapshots error"),
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*queryactivity.Input, *queryactivity.Output]{},
					mockReader,
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
			expectedErrMsg: "failed to get snapshots",
		},
		{
			name: "error getting topK",
			setupFactory: func() *Factory {
				mockReader := &mockCommandParametersReader{
					queryText: "test search",
					snapshots: []string{"_head_"},
					topKErr:   errors.New("topK error"),
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*queryactivity.Input, *queryactivity.Output]{},
					mockReader,
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
			expectedErrMsg: "failed to get topK",
		},
		{
			name: "error getting min score",
			setupFactory: func() *Factory {
				mockReader := &mockCommandParametersReader{
					queryText:   "test search",
					snapshots:   []string{"_head_"},
					topK:        10,
					minScoreErr: errors.New("min score error"),
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*queryactivity.Input, *queryactivity.Output]{},
					mockReader,
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
			expectedErrMsg: "failed to get min score",
		},
		{
			name: "error getting json flag",
			setupFactory: func() *Factory {
				mockReader := &mockCommandParametersReader{
					queryText: "test search",
					snapshots: []string{"_head_"},
					topK:      10,
					minScore:  0.5,
					jsonErr:   errors.New("json error"),
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*queryactivity.Input, *queryactivity.Output]{},
					mockReader,
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
			expectedErrMsg: "failed to get json flag",
		},
		{
			name: "executor not available",
			setupFactory: func() *Factory {
				mockReader := &mockCommandParametersReader{
					queryText: "test search",
					snapshots: []string{"_head_"},
					topK:      10,
					minScore:  0.5,
					useJSON:   false,
				}
				factory, _ := NewFactory(
					&mockActivityFactory[*queryactivity.Input, *queryactivity.Output]{},
					mockReader,
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
			expectedErrMsg: "executor not available in context",
		},
		{
			name: "successful flow - default snapshots, human-readable output with results",
			setupFactory: func() *Factory {
				mockReader := &mockCommandParametersReader{
					queryText: "test search",
					snapshots: []string{},
					topK:      0,
					minScore:  -1,
					useJSON:   false,
				}
				mockSearchFactory := &mockActivityFactory[*queryactivity.Input, *queryactivity.Output]{
					newActivityFunc: func() executor.Activity[*queryactivity.Input, *queryactivity.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *queryactivity.Input) (*queryactivity.Output, error) {
							return &queryactivity.Output{
								Results: []retrieval.SearchResult{
									{
										Score:       0.95,
										FilePath:    "test.go",
										StartLine:   1,
										EndLine:     10,
										TextContent: "func main() {}",
									},
								},
							}, nil
						}
					},
				}
				factory, _ := NewFactory(mockSearchFactory, mockReader, &mockOutputWriter{})
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
			name: "successful flow - no results",
			setupFactory: func() *Factory {
				mockReader := &mockCommandParametersReader{
					queryText: "test search",
					snapshots: []string{"_head_"},
					topK:      5,
					minScore:  0.8,
					useJSON:   false,
				}
				mockSearchFactory := &mockActivityFactory[*queryactivity.Input, *queryactivity.Output]{
					newActivityFunc: func() executor.Activity[*queryactivity.Input, *queryactivity.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *queryactivity.Input) (*queryactivity.Output, error) {
							return &queryactivity.Output{
								Results: []retrieval.SearchResult{},
							}, nil
						}
					},
				}
				factory, _ := NewFactory(mockSearchFactory, mockReader, &mockOutputWriter{})
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
			name: "successful flow - JSON output",
			setupFactory: func() *Factory {
				mockReader := &mockCommandParametersReader{
					queryText: "test search",
					snapshots: []string{"_workdir_", "_stage_", "_head_"},
					topK:      10,
					minScore:  0.5,
					useJSON:   true,
				}
				mockSearchFactory := &mockActivityFactory[*queryactivity.Input, *queryactivity.Output]{
					newActivityFunc: func() executor.Activity[*queryactivity.Input, *queryactivity.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *queryactivity.Input) (*queryactivity.Output, error) {
							return &queryactivity.Output{
								Results: []retrieval.SearchResult{
									{
										Score:       0.85,
										FilePath:    "main.go",
										StartLine:   5,
										EndLine:     15,
										TextContent: "package main",
									},
								},
							}, nil
						}
					},
				}
				factory, _ := NewFactory(mockSearchFactory, mockReader, &mockOutputWriter{})
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
			name: "search activity error",
			setupFactory: func() *Factory {
				mockReader := &mockCommandParametersReader{
					queryText: "test search",
					snapshots: []string{"_head_"},
					topK:      10,
					minScore:  0.5,
					useJSON:   false,
				}
				mockSearchFactory := &mockActivityFactory[*queryactivity.Input, *queryactivity.Output]{
					newActivityFunc: func() executor.Activity[*queryactivity.Input, *queryactivity.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *queryactivity.Input) (*queryactivity.Output, error) {
							return nil, errors.New("search activity error")
						}
					},
				}
				factory, _ := NewFactory(mockSearchFactory, mockReader, &mockOutputWriter{})
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				exec := executor.NewExecutor(0)
				flowCtx := executor.NewContext("test", nil, exec)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "search failed",
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
