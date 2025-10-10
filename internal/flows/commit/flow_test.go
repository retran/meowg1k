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

package commit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/retran/meowg1k/internal/activities/applyfilters"
	"github.com/retran/meowg1k/internal/activities/composecommit"
	"github.com/retran/meowg1k/internal/activities/fetchallbranchdiffs"
	"github.com/retran/meowg1k/internal/activities/fetchalldiffs"
	"github.com/retran/meowg1k/internal/activities/listbranchfiles"
	"github.com/retran/meowg1k/internal/activities/liststaged"
	"github.com/retran/meowg1k/internal/activities/summarizeall"
	"github.com/retran/meowg1k/internal/domain/commit"
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

// Mock config provider
type mockCommitConfigProvider struct {
	config *commit.ResolvedConfig
	err    error
}

func (m *mockCommitConfigProvider) Get() (*commit.ResolvedConfig, error) {
	return m.config, m.err
}

// Mock command parameters reader
type mockCommandParametersReader struct {
	targetBranch string
	targetErr    error
	intent       string
	intentErr    error
	stdin        string
	stdinErr     error
}

func (m *mockCommandParametersReader) GetTargetBranchFlag() (string, error) {
	return m.targetBranch, m.targetErr
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
		listStagedFactory          executor.ActivityFactory[*liststaged.Input, *liststaged.Output]
		listBranchFilesFactory     executor.ActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]
		applyFiltersFactory        executor.ActivityFactory[*applyfilters.Input, *applyfilters.Output]
		fetchAllDiffsFactory       executor.ActivityFactory[*fetchalldiffs.Input, *fetchalldiffs.Output]
		fetchAllBranchDiffsFactory executor.ActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]
		summarizeAllFactory        executor.ActivityFactory[*summarizeall.Input, *summarizeall.Output]
		composeCommitFactory       executor.ActivityFactory[*composecommit.Input, *composecommit.Output]
		commitConfigProvider       CommitConfigProvider
		commandParametersReader    CommandParametersReader
		outputWriter               ports.OutputWriter
		wantErr                    bool
		expectedErrMsg             string
	}{
		{
			name:                       "nil listStagedFactory",
			listStagedFactory:          nil,
			listBranchFilesFactory:     &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchalldiffs.Input, *fetchalldiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
			composeCommitFactory:       &mockActivityFactory[*composecommit.Input, *composecommit.Output]{},
			commitConfigProvider:       &mockCommitConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "listStagedFactory is nil",
		},
		{
			name:                       "nil listBranchFilesFactory",
			listStagedFactory:          &mockActivityFactory[*liststaged.Input, *liststaged.Output]{},
			listBranchFilesFactory:     nil,
			applyFiltersFactory:        &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchalldiffs.Input, *fetchalldiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
			composeCommitFactory:       &mockActivityFactory[*composecommit.Input, *composecommit.Output]{},
			commitConfigProvider:       &mockCommitConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "listBranchFilesFactory is nil",
		},
		{
			name:                       "nil applyFiltersFactory",
			listStagedFactory:          &mockActivityFactory[*liststaged.Input, *liststaged.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
			applyFiltersFactory:        nil,
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchalldiffs.Input, *fetchalldiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
			composeCommitFactory:       &mockActivityFactory[*composecommit.Input, *composecommit.Output]{},
			commitConfigProvider:       &mockCommitConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "applyFiltersFactory is nil",
		},
		{
			name:                       "nil fetchAllDiffsFactory",
			listStagedFactory:          &mockActivityFactory[*liststaged.Input, *liststaged.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
			fetchAllDiffsFactory:       nil,
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
			composeCommitFactory:       &mockActivityFactory[*composecommit.Input, *composecommit.Output]{},
			commitConfigProvider:       &mockCommitConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "fetchAllDiffsFactory is nil",
		},
		{
			name:                       "nil fetchAllBranchDiffsFactory",
			listStagedFactory:          &mockActivityFactory[*liststaged.Input, *liststaged.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchalldiffs.Input, *fetchalldiffs.Output]{},
			fetchAllBranchDiffsFactory: nil,
			summarizeAllFactory:        &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
			composeCommitFactory:       &mockActivityFactory[*composecommit.Input, *composecommit.Output]{},
			commitConfigProvider:       &mockCommitConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "fetchAllBranchDiffsFactory is nil",
		},
		{
			name:                       "nil summarizeAllFactory",
			listStagedFactory:          &mockActivityFactory[*liststaged.Input, *liststaged.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchalldiffs.Input, *fetchalldiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
			summarizeAllFactory:        nil,
			composeCommitFactory:       &mockActivityFactory[*composecommit.Input, *composecommit.Output]{},
			commitConfigProvider:       &mockCommitConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "summarizeAllFactory is nil",
		},
		{
			name:                       "nil composeCommitFactory",
			listStagedFactory:          &mockActivityFactory[*liststaged.Input, *liststaged.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchalldiffs.Input, *fetchalldiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
			composeCommitFactory:       nil,
			commitConfigProvider:       &mockCommitConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "composeCommitFactory is nil",
		},
		{
			name:                       "nil commitConfigProvider",
			listStagedFactory:          &mockActivityFactory[*liststaged.Input, *liststaged.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchalldiffs.Input, *fetchalldiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
			composeCommitFactory:       &mockActivityFactory[*composecommit.Input, *composecommit.Output]{},
			commitConfigProvider:       nil,
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "commitConfigProvider is nil",
		},
		{
			name:                       "nil commandParametersReader",
			listStagedFactory:          &mockActivityFactory[*liststaged.Input, *liststaged.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchalldiffs.Input, *fetchalldiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
			composeCommitFactory:       &mockActivityFactory[*composecommit.Input, *composecommit.Output]{},
			commitConfigProvider:       &mockCommitConfigProvider{},
			commandParametersReader:    nil,
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "commandParametersReader is nil",
		},
		{
			name:                       "nil outputWriter",
			listStagedFactory:          &mockActivityFactory[*liststaged.Input, *liststaged.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchalldiffs.Input, *fetchalldiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
			composeCommitFactory:       &mockActivityFactory[*composecommit.Input, *composecommit.Output]{},
			commitConfigProvider:       &mockCommitConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               nil,
			wantErr:                    true,
			expectedErrMsg:             "outputWriter is nil",
		},
		{
			name:                       "all valid dependencies",
			listStagedFactory:          &mockActivityFactory[*liststaged.Input, *liststaged.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchalldiffs.Input, *fetchalldiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
			composeCommitFactory:       &mockActivityFactory[*composecommit.Input, *composecommit.Output]{},
			commitConfigProvider:       &mockCommitConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory, err := NewFactory(
				tt.listStagedFactory,
				tt.listBranchFilesFactory,
				tt.applyFiltersFactory,
				tt.fetchAllDiffsFactory,
				tt.fetchAllBranchDiffsFactory,
				tt.summarizeAllFactory,
				tt.composeCommitFactory,
				tt.commitConfigProvider,
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
				return context.Background(), &executor.Context{}
			},
			wantErr:        true,
			expectedErrMsg: "factory is nil",
		},
		{
			name: "nil context",
			setupFactory: func() *Factory {
				factory, _ := NewFactory(
					&mockActivityFactory[*liststaged.Input, *liststaged.Output]{},
					&mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
					&mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
					&mockActivityFactory[*fetchalldiffs.Input, *fetchalldiffs.Output]{},
					&mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
					&mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
					&mockActivityFactory[*composecommit.Input, *composecommit.Output]{},
					&mockCommitConfigProvider{},
					&mockCommandParametersReader{},
					&mockOutputWriter{},
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				return nil, &executor.Context{}
			},
			wantErr:        true,
			expectedErrMsg: "context is nil",
		},
		{
			name: "nil flow context",
			setupFactory: func() *Factory {
				factory, _ := NewFactory(
					&mockActivityFactory[*liststaged.Input, *liststaged.Output]{},
					&mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
					&mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
					&mockActivityFactory[*fetchalldiffs.Input, *fetchalldiffs.Output]{},
					&mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
					&mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
					&mockActivityFactory[*composecommit.Input, *composecommit.Output]{},
					&mockCommitConfigProvider{},
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
			name: "error getting target branch flag",
			setupFactory: func() *Factory {
				mockReader := &mockCommandParametersReader{
					targetErr: errors.New("flag error"),
				}

				factory, _ := NewFactory(
					&mockActivityFactory[*liststaged.Input, *liststaged.Output]{},
					&mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
					&mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
					&mockActivityFactory[*fetchalldiffs.Input, *fetchalldiffs.Output]{},
					&mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
					&mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
					&mockActivityFactory[*composecommit.Input, *composecommit.Output]{},
					&mockCommitConfigProvider{},
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
			expectedErrMsg: "failed to get target-branch flag",
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
