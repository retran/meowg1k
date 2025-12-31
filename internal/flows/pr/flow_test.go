// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package pr

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/retran/meowg1k/internal/activities/draftpr"
	"github.com/retran/meowg1k/internal/activities/draftprflat"
	"github.com/retran/meowg1k/internal/activities/fetchbranchdiffs"
	"github.com/retran/meowg1k/internal/activities/filterfiles"
	"github.com/retran/meowg1k/internal/activities/listbranchchanges"
	"github.com/retran/meowg1k/internal/activities/summarizechanges"
	domainpullrequest "github.com/retran/meowg1k/internal/domain/pullrequest"
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

// Mock PR config provider.
type mockPRConfigProvider struct {
	config *domainpullrequest.ResolvedConfig
	err    error
}

func (m *mockPRConfigProvider) Get() (*domainpullrequest.ResolvedConfig, error) {
	return m.config, m.err
}

// Mock command parameters reader.
type mockCommandParametersReader struct {
	diffErr    error
	baseErr    error
	intentErr  error
	stdinErr   error
	diffMode   string
	baseBranch string
	intent     string
	stdin      string
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
		listBranchFilesFactory     executor.ActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]
		applyFiltersFactory        executor.ActivityFactory[*filterfiles.Input, *filterfiles.Output]
		fetchAllBranchDiffsFactory executor.ActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]
		summarizeAllFactory        executor.ActivityFactory[*summarizechanges.Input, *summarizechanges.Output]
		composePRFactory           executor.ActivityFactory[*draftpr.Input, *draftpr.Output]
		composeFlatPRFactory       executor.ActivityFactory[*draftprflat.Input, *draftprflat.Output]
		prConfigProvider           ConfigProvider
		commandParametersReader    CommandParametersReader
		outputWriter               ports.OutputWriter
		name                       string
		expectedErrMsg             string
		wantErr                    bool
	}{
		{
			name:                       "nil listBranchFilesFactory",
			listBranchFilesFactory:     nil,
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composePRFactory:           &mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
			composeFlatPRFactory:       &mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
			prConfigProvider:           &mockPRConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "listBranchFilesFactory is nil",
		},
		{
			name:                       "nil applyFiltersFactory",
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        nil,
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composePRFactory:           &mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
			composeFlatPRFactory:       &mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
			prConfigProvider:           &mockPRConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "applyFiltersFactory is nil",
		},
		{
			name:                       "nil fetchAllBranchDiffsFactory",
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllBranchDiffsFactory: nil,
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composePRFactory:           &mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
			composeFlatPRFactory:       &mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
			prConfigProvider:           &mockPRConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "fetchAllBranchDiffsFactory is nil",
		},
		{
			name:                       "nil summarizeAllFactory",
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        nil,
			composePRFactory:           &mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
			composeFlatPRFactory:       &mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
			prConfigProvider:           &mockPRConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "summarizeAllFactory is nil",
		},
		{
			name:                       "nil composePRFactory",
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composePRFactory:           nil,
			composeFlatPRFactory:       &mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
			prConfigProvider:           &mockPRConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "composePRFactory is nil",
		},
		{
			name:                       "nil composeFlatPRFactory",
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composePRFactory:           &mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
			composeFlatPRFactory:       nil,
			prConfigProvider:           &mockPRConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "composeFlatPRFactory is nil",
		},
		{
			name:                       "nil prConfigProvider",
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composePRFactory:           &mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
			composeFlatPRFactory:       &mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
			prConfigProvider:           nil,
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "prConfigProvider is nil",
		},
		{
			name:                       "nil commandParametersReader",
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composePRFactory:           &mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
			composeFlatPRFactory:       &mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
			prConfigProvider:           &mockPRConfigProvider{},
			commandParametersReader:    nil,
			outputWriter:               &mockOutputWriter{},
			wantErr:                    true,
			expectedErrMsg:             "commandParametersReader is nil",
		},
		{
			name:                       "nil outputWriter",
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composePRFactory:           &mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
			composeFlatPRFactory:       &mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
			prConfigProvider:           &mockPRConfigProvider{},
			commandParametersReader:    &mockCommandParametersReader{},
			outputWriter:               nil,
			wantErr:                    true,
			expectedErrMsg:             "outputWriter is nil",
		},
		{
			name:                       "all factories provided",
			listBranchFilesFactory:     &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
			applyFiltersFactory:        &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
			fetchAllBranchDiffsFactory: &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
			summarizeAllFactory:        &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
			composePRFactory:           &mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
			composeFlatPRFactory:       &mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
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
				tt.composeFlatPRFactory,
				tt.prConfigProvider,
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
				return context.Background(), executor.NewContext("test", nil, nil)
			},
			wantErr:        true,
			expectedErrMsg: "factory is nil",
		},
		{
			name: "nil context",
			setupFactory: func() *Factory {
				factory, _ := NewFactory(
					&mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
					&mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
					&mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
					&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
					&mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
					&mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
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
					&mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
					&mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
					&mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
					&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
					&mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
					&mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
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
			name: "error getting diff flag",
			setupFactory: func() *Factory {
				mockReader := &mockCommandParametersReader{
					diffErr: errors.New("diff error"),
				}

				factory, _ := NewFactory(
					&mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
					&mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
					&mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
					&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
					&mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
					&mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
					&mockPRConfigProvider{},
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
			name: "error getting base branch flag",
			setupFactory: func() *Factory {
				mockReader := &mockCommandParametersReader{
					diffMode: "branch",
					baseErr:  errors.New("base branch error"),
				}

				factory, _ := NewFactory(
					&mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
					&mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
					&mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
					&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
					&mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
					&mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
					&mockPRConfigProvider{},
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
			} else if err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}
