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

package pr

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/retran/meowg1k/internal/activities/applyfilters"
	"github.com/retran/meowg1k/internal/activities/composepr"
	"github.com/retran/meowg1k/internal/activities/fetchallbranchdiffs"
	"github.com/retran/meowg1k/internal/activities/listbranchfiles"
	"github.com/retran/meowg1k/internal/activities/summarizeall"
	"github.com/retran/meowg1k/internal/domain/pullrequest"
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

// Mock PR config provider
type mockPRConfigProvider struct {
	config *pullrequest.ResolvedConfig
	err    error
}

func (m *mockPRConfigProvider) Get() (*pullrequest.ResolvedConfig, error) {
	return m.config, m.err
}

// Mock command parameters reader
type mockCommandParametersReader struct {
	baseBranch string
	baseErr    error
	intent     string
	intentErr  error
	stdin      string
	stdinErr   error
}

func (m *mockCommandParametersReader) GetBaseBranchFlag() (string, error) {
	return m.baseBranch, m.baseErr
}

func (m *mockCommandParametersReader) GetIntentFlag() (string, error) {
	return m.intent, m.intentErr
}

func (m *mockCommandParametersReader) GetStdIn() (string, error) {
	return m.stdin, m.stdinErr
}

// Mock output writer
type mockOutputWriter struct {
	outputs []string
}

func (m *mockOutputWriter) PrintLine(line string) error {
	m.outputs = append(m.outputs, line)
	return nil
}

func TestNewFactory(t *testing.T) {
	tests := []struct {
		name                       string
		listBranchFilesFactory     executor.ActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]
		applyFiltersFactory        executor.ActivityFactory[*applyfilters.Input, *applyfilters.Output]
		fetchAllBranchDiffsFactory executor.ActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]
		summarizeAllFactory        executor.ActivityFactory[*summarizeall.Input, *summarizeall.Output]
		composePRFactory           executor.ActivityFactory[*composepr.Input, *composepr.Output]
		prConfigProvider           PullRequestConfigProvider
		commandParametersReader    CommandParametersReader
		outputWriter               ports.OutputWriter
		wantErr                    bool
		expectedErrMsg             string
	}{
		{
			name:                       "nil listBranchFilesFactory",
			listBranchFilesFactory:     nil,
			applyFiltersFactory:        &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
			composePRFactory:           &mockActivityFactory[*composepr.Input, *composepr.Output]{},
			prConfigProvider:           &mockPRConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "listBranchFilesFactory is nil",
		},
		{
			name:                       "nil applyFiltersFactory",
			listBranchFilesFactory:     &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
			applyFiltersFactory:        nil,
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
			composePRFactory:           &mockActivityFactory[*composepr.Input, *composepr.Output]{},
			prConfigProvider:           &mockPRConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "applyFiltersFactory is nil",
		},
		{
			name:                       "nil fetchAllBranchDiffsFactory",
			listBranchFilesFactory:     &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
			fetchAllBranchDiffsFactory: nil,
			summarizeAllFactory:        &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
			composePRFactory:           &mockActivityFactory[*composepr.Input, *composepr.Output]{},
			prConfigProvider:           &mockPRConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "fetchAllBranchDiffsFactory is nil",
		},
		{
			name:                       "nil summarizeAllFactory",
			listBranchFilesFactory:     &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
			summarizeAllFactory:        nil,
			composePRFactory:           &mockActivityFactory[*composepr.Input, *composepr.Output]{},
			prConfigProvider:           &mockPRConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "summarizeAllFactory is nil",
		},
		{
			name:                       "nil composePRFactory",
			listBranchFilesFactory:     &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
			composePRFactory:           nil,
			prConfigProvider:           &mockPRConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "composePRFactory is nil",
		},
		{
			name:                       "nil prConfigProvider",
			listBranchFilesFactory:     &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
			composePRFactory:           &mockActivityFactory[*composepr.Input, *composepr.Output]{},
			prConfigProvider:           nil,
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "prConfigProvider is nil",
		},
		{
			name:                       "nil commandParametersReader",
			listBranchFilesFactory:     &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
			composePRFactory:           &mockActivityFactory[*composepr.Input, *composepr.Output]{},
			prConfigProvider:           &mockPRConfigProvider{},
			commandParametersReader:    nil,
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "commandParametersReader is nil",
		},
		{
			name:                       "nil outputWriter",
			listBranchFilesFactory:     &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
			composePRFactory:           &mockActivityFactory[*composepr.Input, *composepr.Output]{},
			prConfigProvider:           &mockPRConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               nil,
			wantErr:                    true,
			expectedErrMsg:             "outputWriter is nil",
		},
		{
			name:                       "all valid dependencies",
			listBranchFilesFactory:     &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
			composePRFactory:           &mockActivityFactory[*composepr.Input, *composepr.Output]{},
			prConfigProvider:           &mockPRConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory, err := NewFactory(
				tt.listBranchFilesFactory,
				tt.applyFiltersFactory,
				tt.fetchAllBranchDiffsFactory,
				tt.summarizeAllFactory,
				tt.composePRFactory,
				tt.prConfigProvider,
				tt.commandParametersReader,
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

func TestFactory_NewFlow(t *testing.T) {
	tests := []struct {
		name           string
		setupFactory   func() *Factory
		setupContext   func() (context.Context, *executor.Context)
		wantErr        bool
		expectedErrMsg string
	}{
		{
			name: "nil factory",
			setupFactory: func() *Factory {
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
			setupFactory: func() *Factory {
				factory, _ := NewFactory(
					&mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
					&mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
					&mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
					&mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
					&mockActivityFactory[*composepr.Input, *composepr.Output]{},
					&mockPRConfigProvider{},
					&mockCommandParametersReader{},
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
			setupFactory: func() *Factory {
				factory, _ := NewFactory(
					&mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
					&mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
					&mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
					&mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
					&mockActivityFactory[*composepr.Input, *composepr.Output]{},
					&mockPRConfigProvider{},
					&mockCommandParametersReader{},
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
			name: "error getting base branch flag",
			setupFactory: func() *Factory {
				mockReader := &mockCommandParametersReader{
					baseErr: errors.New("base branch error"),
				}

				factory, _ := NewFactory(
					&mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
					&mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
					&mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
					&mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
					&mockActivityFactory[*composepr.Input, *composepr.Output]{},
					&mockPRConfigProvider{},
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
			expectedErrMsg: "failed to get base-branch flag",
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
