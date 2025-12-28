// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package commitmsg

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/retran/meowg1k/internal/activities/filterfiles"
	"github.com/retran/meowg1k/internal/activities/draftcommit"
	"github.com/retran/meowg1k/internal/activities/draftcommitflat"
	"github.com/retran/meowg1k/internal/activities/fetchbranchdiffs"
	"github.com/retran/meowg1k/internal/activities/fetchstageddiffs"
	"github.com/retran/meowg1k/internal/activities/listbranchchanges"
	"github.com/retran/meowg1k/internal/activities/liststagedfiles"
	"github.com/retran/meowg1k/internal/activities/summarizechanges"
	"github.com/retran/meowg1k/internal/activities/summarizefilechanges"
	"github.com/retran/meowg1k/internal/domain/commit"
	"github.com/retran/meowg1k/internal/domain/git"
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

// Mock config provider.
type mockConfigProvider struct {
	config *commit.ResolvedConfig
	err    error
}

func (m *mockConfigProvider) Get() (*commit.ResolvedConfig, error) {
	return m.config, m.err
}

// Mock command parameters reader.
type mockCommandParametersReader struct {
	diffErr      error
	baseErr      error
	intentErr    error
	stdinErr     error
	diffMode     string
	baseBranch   string
	intent       string
	stdin        string
}

func (m *mockCommandParametersReader) GetDiffFlag() (string, error) {
	return m.diffMode, m.diffErr
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

// Mock output writer.
type mockOutputWriter struct {
	outputs []string
}

func (m *mockOutputWriter) PrintLine(line string) error {
	m.outputs = append(m.outputs, line)
	return nil
}

func TestNewFactory(t *testing.T) {
	tests := []struct {
		summarizeAllFactory        executor.ActivityFactory[*summarizechanges.Input, *summarizechanges.Output]
		commandParametersReader    CommandParametersReader
		listBranchFilesFactory     executor.ActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]
		applyFiltersFactory        executor.ActivityFactory[*filterfiles.Input, *filterfiles.Output]
		fetchAllDiffsFactory       executor.ActivityFactory[*fetchstageddiffs.Input, *fetchstageddiffs.Output]
		fetchAllBranchDiffsFactory executor.ActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]
		composeFlatCommitFactory   executor.ActivityFactory[*draftcommitflat.Input, *draftcommitflat.Output]
		composeCommitFactory       executor.ActivityFactory[*draftcommit.Input, *draftcommit.Output]
		listStagedFactory          executor.ActivityFactory[*liststagedfiles.Input, *liststagedfiles.Output]
		commitConfigProvider       ConfigProvider
		outputWriter               ports.OutputWriter
		name                       string
		expectedErrMsg             string
		wantErr                    bool
	}{
		{
			name:                       "nil listStagedFactory",
			listStagedFactory:          nil,
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchstageddiffs.Input, *fetchstageddiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composeCommitFactory:       &mockActivityFactory[*draftcommit.Input, *draftcommit.Output]{},
			composeFlatCommitFactory:   &mockActivityFactory[*draftcommitflat.Input, *draftcommitflat.Output]{},
			commitConfigProvider:       &mockConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "listStagedFactory is nil",
		},
		{
			name:                       "nil listBranchFilesFactory",
			listStagedFactory:          &mockActivityFactory[*liststagedfiles.Input, *liststagedfiles.Output]{},
			listBranchFilesFactory:     nil,
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchstageddiffs.Input, *fetchstageddiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composeCommitFactory:       &mockActivityFactory[*draftcommit.Input, *draftcommit.Output]{},
			composeFlatCommitFactory:   &mockActivityFactory[*draftcommitflat.Input, *draftcommitflat.Output]{},
			commitConfigProvider:       &mockConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "listBranchFilesFactory is nil",
		},
		{
			name:                       "nil applyFiltersFactory",
			listStagedFactory:          &mockActivityFactory[*liststagedfiles.Input, *liststagedfiles.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        nil,
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchstageddiffs.Input, *fetchstageddiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composeCommitFactory:       &mockActivityFactory[*draftcommit.Input, *draftcommit.Output]{},
			composeFlatCommitFactory:   &mockActivityFactory[*draftcommitflat.Input, *draftcommitflat.Output]{},
			commitConfigProvider:       &mockConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "applyFiltersFactory is nil",
		},
		{
			name:                       "nil fetchAllDiffsFactory",
			listStagedFactory:          &mockActivityFactory[*liststagedfiles.Input, *liststagedfiles.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllDiffsFactory:       nil,
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composeCommitFactory:       &mockActivityFactory[*draftcommit.Input, *draftcommit.Output]{},
			composeFlatCommitFactory:   &mockActivityFactory[*draftcommitflat.Input, *draftcommitflat.Output]{},
			commitConfigProvider:       &mockConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "fetchAllDiffsFactory is nil",
		},
		{
			name:                       "nil fetchAllBranchDiffsFactory",
			listStagedFactory:          &mockActivityFactory[*liststagedfiles.Input, *liststagedfiles.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchstageddiffs.Input, *fetchstageddiffs.Output]{},
			fetchAllBranchDiffsFactory: nil,
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composeCommitFactory:       &mockActivityFactory[*draftcommit.Input, *draftcommit.Output]{},
			composeFlatCommitFactory:   &mockActivityFactory[*draftcommitflat.Input, *draftcommitflat.Output]{},
			commitConfigProvider:       &mockConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "fetchAllBranchDiffsFactory is nil",
		},
		{
			name:                       "nil summarizeAllFactory",
			listStagedFactory:          &mockActivityFactory[*liststagedfiles.Input, *liststagedfiles.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchstageddiffs.Input, *fetchstageddiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        nil,
			composeCommitFactory:       &mockActivityFactory[*draftcommit.Input, *draftcommit.Output]{},
			composeFlatCommitFactory:   &mockActivityFactory[*draftcommitflat.Input, *draftcommitflat.Output]{},
			commitConfigProvider:       &mockConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "summarizeAllFactory is nil",
		},
		{
			name:                       "nil composeCommitFactory",
			listStagedFactory:          &mockActivityFactory[*liststagedfiles.Input, *liststagedfiles.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchstageddiffs.Input, *fetchstageddiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composeCommitFactory:       nil,
			composeFlatCommitFactory:   &mockActivityFactory[*draftcommitflat.Input, *draftcommitflat.Output]{},
			commitConfigProvider:       &mockConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "composeCommitFactory is nil",
		},
		{
			name:                       "nil composeFlatCommitFactory",
			listStagedFactory:          &mockActivityFactory[*liststagedfiles.Input, *liststagedfiles.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchstageddiffs.Input, *fetchstageddiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composeCommitFactory:       &mockActivityFactory[*draftcommit.Input, *draftcommit.Output]{},
			composeFlatCommitFactory:   nil,
			commitConfigProvider:       &mockConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "composeFlatCommitFactory is nil",
		},
		{
			name:                       "nil commitConfigProvider",
			listStagedFactory:          &mockActivityFactory[*liststagedfiles.Input, *liststagedfiles.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchstageddiffs.Input, *fetchstageddiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composeCommitFactory:       &mockActivityFactory[*draftcommit.Input, *draftcommit.Output]{},
			composeFlatCommitFactory:   &mockActivityFactory[*draftcommitflat.Input, *draftcommitflat.Output]{},
			commitConfigProvider:       nil,
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "commitConfigProvider is nil",
		},
		{
			name:                       "nil commandParametersReader",
			listStagedFactory:          &mockActivityFactory[*liststagedfiles.Input, *liststagedfiles.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchstageddiffs.Input, *fetchstageddiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composeCommitFactory:       &mockActivityFactory[*draftcommit.Input, *draftcommit.Output]{},
			composeFlatCommitFactory:   &mockActivityFactory[*draftcommitflat.Input, *draftcommitflat.Output]{},
			commitConfigProvider:       &mockConfigProvider{},
			commandParametersReader:    nil,
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "commandParametersReader is nil",
		},
		{
			name:                       "nil outputWriter",
			listStagedFactory:          &mockActivityFactory[*liststagedfiles.Input, *liststagedfiles.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchstageddiffs.Input, *fetchstageddiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composeCommitFactory:       &mockActivityFactory[*draftcommit.Input, *draftcommit.Output]{},
			composeFlatCommitFactory:   &mockActivityFactory[*draftcommitflat.Input, *draftcommitflat.Output]{},
			commitConfigProvider:       &mockConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               nil,
			wantErr:                    true,
			expectedErrMsg:             "outputWriter is nil",
		},
		{
			name:                       "all valid dependencies",
			listStagedFactory:          &mockActivityFactory[*liststagedfiles.Input, *liststagedfiles.Output]{},
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllDiffsFactory:       &mockActivityFactory[*fetchstageddiffs.Input, *fetchstageddiffs.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composeCommitFactory:       &mockActivityFactory[*draftcommit.Input, *draftcommit.Output]{},
			composeFlatCommitFactory:   &mockActivityFactory[*draftcommitflat.Input, *draftcommitflat.Output]{},
			commitConfigProvider:       &mockConfigProvider{},
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
				tt.composeFlatCommitFactory,
				tt.commitConfigProvider,
				tt.commandParametersReader,
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

func TestFactory_NewFlow(t *testing.T) {
	tests := []struct {
		setupFactory   func() *Factory
		setupContext   func() (context.Context, *executor.Context)
		name           string
		expectedErrMsg string
		wantErr        bool
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
					&mockActivityFactory[*liststagedfiles.Input, *liststagedfiles.Output]{},
					&mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
					&mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
					&mockActivityFactory[*fetchstageddiffs.Input, *fetchstageddiffs.Output]{},
					&mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
					&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
					&mockActivityFactory[*draftcommit.Input, *draftcommit.Output]{},
					&mockActivityFactory[*draftcommitflat.Input, *draftcommitflat.Output]{},
					&mockConfigProvider{},
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
					&mockActivityFactory[*liststagedfiles.Input, *liststagedfiles.Output]{},
					&mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
					&mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
					&mockActivityFactory[*fetchstageddiffs.Input, *fetchstageddiffs.Output]{},
					&mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
					&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
					&mockActivityFactory[*draftcommit.Input, *draftcommit.Output]{},
					&mockActivityFactory[*draftcommitflat.Input, *draftcommitflat.Output]{},
					&mockConfigProvider{},
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
			name: "error getting diff flag",
			setupFactory: func() *Factory {
				mockReader := &mockCommandParametersReader{
					diffErr: errors.New("flag error"),
				}

				factory, _ := NewFactory(
					&mockActivityFactory[*liststagedfiles.Input, *liststagedfiles.Output]{},
					&mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
					&mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
					&mockActivityFactory[*fetchstageddiffs.Input, *fetchstageddiffs.Output]{},
					&mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
					&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
					&mockActivityFactory[*draftcommit.Input, *draftcommit.Output]{},
					&mockActivityFactory[*draftcommitflat.Input, *draftcommitflat.Output]{},
					&mockConfigProvider{},
					mockReader,
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
			expectedErrMsg: "failed to get diff flag",
		},
		{
			name: "successful flow execution - staged mode",
			setupFactory: func() *Factory {
				mockReader := &mockCommandParametersReader{
					diffMode: "staged",
					intent:       "test intent",
				}

				mockConfig := &mockConfigProvider{
					config: &commit.ResolvedConfig{
						Profile:      nil,
						SystemPrompt: "test prompt",
					},
				}

				factory, _ := NewFactory(
					&mockActivityFactory[*liststagedfiles.Input, *liststagedfiles.Output]{
						newActivityFunc: func() executor.Activity[*liststagedfiles.Input, *liststagedfiles.Output] {
							return func(ctx context.Context, activityCtx *executor.Context, input *liststagedfiles.Input) (*liststagedfiles.Output, error) {
								return &liststagedfiles.Output{Files: []string{"file1.go", "file2.go"}}, nil
							}
						},
					},
					&mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
					&mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{
						newActivityFunc: func() executor.Activity[*filterfiles.Input, *filterfiles.Output] {
							return func(ctx context.Context, activityCtx *executor.Context, input *filterfiles.Input) (*filterfiles.Output, error) {
								return &filterfiles.Output{Files: input.Files}, nil
							}
						},
					},
					&mockActivityFactory[*fetchstageddiffs.Input, *fetchstageddiffs.Output]{
						newActivityFunc: func() executor.Activity[*fetchstageddiffs.Input, *fetchstageddiffs.Output] {
							return func(ctx context.Context, activityCtx *executor.Context, input *fetchstageddiffs.Input) (*fetchstageddiffs.Output, error) {
								return &fetchstageddiffs.Output{Changes: []*git.FileChange{}}, nil
							}
						},
					},
					&mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
					&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{
						newActivityFunc: func() executor.Activity[*summarizechanges.Input, *summarizechanges.Output] {
							return func(ctx context.Context, activityCtx *executor.Context, input *summarizechanges.Input) (*summarizechanges.Output, error) {
								return &summarizechanges.Output{
									Summaries: []*summarizefilechanges.Output{
										{Filename: "file1.go", Summary: "summary1", Skipped: false},
										{Filename: "file2.go", Summary: "summary2", Skipped: false},
									},
								}, nil
							}
						},
					},
					&mockActivityFactory[*draftcommit.Input, *draftcommit.Output]{
						newActivityFunc: func() executor.Activity[*draftcommit.Input, *draftcommit.Output] {
							return func(ctx context.Context, activityCtx *executor.Context, input *draftcommit.Input) (*draftcommit.Output, error) {
								return &draftcommit.Output{CommitMessage: "test commit message"}, nil
							}
						},
					},
					&mockActivityFactory[*draftcommitflat.Input, *draftcommitflat.Output]{},
					mockConfig,
					mockReader,
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
			wantErr: false,
		},
		{
			name: "successful flow execution - branch mode",
			setupFactory: func() *Factory {
				mockReader := &mockCommandParametersReader{
					diffMode:     "branch",
					baseBranch:   "main",
					intent:       "",
					stdin:        "stdin intent",
				}

				mockConfig := &mockConfigProvider{
					config: &commit.ResolvedConfig{
						Profile:      nil,
						SystemPrompt: "test prompt",
					},
				}

				factory, _ := NewFactory(
					&mockActivityFactory[*liststagedfiles.Input, *liststagedfiles.Output]{},
					&mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{
						newActivityFunc: func() executor.Activity[*listbranchchanges.Input, *listbranchchanges.Output] {
							return func(ctx context.Context, activityCtx *executor.Context, input *listbranchchanges.Input) (*listbranchchanges.Output, error) {
								return &listbranchchanges.Output{Files: []string{"file1.go"}}, nil
							}
						},
					},
					&mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{
						newActivityFunc: func() executor.Activity[*filterfiles.Input, *filterfiles.Output] {
							return func(ctx context.Context, activityCtx *executor.Context, input *filterfiles.Input) (*filterfiles.Output, error) {
								return &filterfiles.Output{Files: input.Files}, nil
							}
						},
					},
					&mockActivityFactory[*fetchstageddiffs.Input, *fetchstageddiffs.Output]{},
					&mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{
						newActivityFunc: func() executor.Activity[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output] {
							return func(ctx context.Context, activityCtx *executor.Context, input *fetchbranchdiffs.Input) (*fetchbranchdiffs.Output, error) {
								return &fetchbranchdiffs.Output{Changes: []*git.FileChange{}}, nil
							}
						},
					},
					&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{
						newActivityFunc: func() executor.Activity[*summarizechanges.Input, *summarizechanges.Output] {
							return func(ctx context.Context, activityCtx *executor.Context, input *summarizechanges.Input) (*summarizechanges.Output, error) {
								return &summarizechanges.Output{
									Summaries: []*summarizefilechanges.Output{
										{Filename: "file1.go", Summary: "branch summary", Skipped: false},
									},
								}, nil
							}
						},
					},
					&mockActivityFactory[*draftcommit.Input, *draftcommit.Output]{
						newActivityFunc: func() executor.Activity[*draftcommit.Input, *draftcommit.Output] {
							return func(ctx context.Context, activityCtx *executor.Context, input *draftcommit.Input) (*draftcommit.Output, error) {
								return &draftcommit.Output{CommitMessage: "branch commit message"}, nil
							}
						},
					},
					&mockActivityFactory[*draftcommitflat.Input, *draftcommitflat.Output]{},
					mockConfig,
					mockReader,
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
			wantErr: false,
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
