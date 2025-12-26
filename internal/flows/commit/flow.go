// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package commit implements the workflow for generating commit messages from staged changes or branch diffs.
package commit

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/applyfilters"
	"github.com/retran/meowg1k/internal/activities/composecommit"
	"github.com/retran/meowg1k/internal/activities/composeflatcommit"
	"github.com/retran/meowg1k/internal/activities/fetchallbranchdiffs"
	"github.com/retran/meowg1k/internal/activities/fetchalldiffs"
	"github.com/retran/meowg1k/internal/activities/listbranchfiles"
	"github.com/retran/meowg1k/internal/activities/liststaged"
	"github.com/retran/meowg1k/internal/activities/summarizeall"
	"github.com/retran/meowg1k/internal/domain/commit"
	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// ConfigProvider provides commit message configuration.
type ConfigProvider interface {
	Get() (*commit.ResolvedConfig, error)
}

// CommandParametersReader reads command-line parameters and flags.
type CommandParametersReader interface {
	GetTargetBranchFlag() (string, error)
	GetIntentFlag() (string, error)
	GetStdIn() (string, error)
}

// Factory creates instances of the commit flow with injected dependencies.
type Factory struct {
	listStagedFactory          executor.ActivityFactory[*liststaged.Input, *liststaged.Output]
	listBranchFilesFactory     executor.ActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]
	applyFiltersFactory        executor.ActivityFactory[*applyfilters.Input, *applyfilters.Output]
	fetchAllDiffsFactory       executor.ActivityFactory[*fetchalldiffs.Input, *fetchalldiffs.Output]
	fetchAllBranchDiffsFactory executor.ActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]
	summarizeAllFactory        executor.ActivityFactory[*summarizeall.Input, *summarizeall.Output]
	composeCommitFactory       executor.ActivityFactory[*composecommit.Input, *composecommit.Output]
	composeFlatCommitFactory   executor.ActivityFactory[*composeflatcommit.Input, *composeflatcommit.Output]
	commitConfigProvider       ConfigProvider
	commandParametersReader    CommandParametersReader
	outputWriter               ports.OutputWriter
}

// NewFactory creates a new commit flow factory with injected adapters.
func NewFactory(
	listStagedFactory executor.ActivityFactory[*liststaged.Input, *liststaged.Output],
	listBranchFilesFactory executor.ActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output],
	applyFiltersFactory executor.ActivityFactory[*applyfilters.Input, *applyfilters.Output],
	fetchAllDiffsFactory executor.ActivityFactory[*fetchalldiffs.Input, *fetchalldiffs.Output],
	fetchAllBranchDiffsFactory executor.ActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output],
	summarizeAllFactory executor.ActivityFactory[*summarizeall.Input, *summarizeall.Output],
	composeCommitFactory executor.ActivityFactory[*composecommit.Input, *composecommit.Output],
	composeFlatCommitFactory executor.ActivityFactory[*composeflatcommit.Input, *composeflatcommit.Output],
	commitConfigProvider ConfigProvider,
	commandParametersReader CommandParametersReader,
	outputWriter ports.OutputWriter,
) (*Factory, error) {
	if listStagedFactory == nil {
		return nil, fmt.Errorf("listStagedFactory is nil")
	}

	if listBranchFilesFactory == nil {
		return nil, fmt.Errorf("listBranchFilesFactory is nil")
	}

	if applyFiltersFactory == nil {
		return nil, fmt.Errorf("applyFiltersFactory is nil")
	}

	if fetchAllDiffsFactory == nil {
		return nil, fmt.Errorf("fetchAllDiffsFactory is nil")
	}

	if fetchAllBranchDiffsFactory == nil {
		return nil, fmt.Errorf("fetchAllBranchDiffsFactory is nil")
	}

	if summarizeAllFactory == nil {
		return nil, fmt.Errorf("summarizeAllFactory is nil")
	}

	if composeCommitFactory == nil {
		return nil, fmt.Errorf("composeCommitFactory is nil")
	}

	if composeFlatCommitFactory == nil {
		return nil, fmt.Errorf("composeFlatCommitFactory is nil")
	}

	if commitConfigProvider == nil {
		return nil, fmt.Errorf("commitConfigProvider is nil")
	}

	if commandParametersReader == nil {
		return nil, fmt.Errorf("commandParametersReader is nil")
	}

	if outputWriter == nil {
		return nil, fmt.Errorf("outputWriter is nil")
	}

	return &Factory{
		listStagedFactory:          listStagedFactory,
		listBranchFilesFactory:     listBranchFilesFactory,
		applyFiltersFactory:        applyFiltersFactory,
		fetchAllDiffsFactory:       fetchAllDiffsFactory,
		fetchAllBranchDiffsFactory: fetchAllBranchDiffsFactory,
		summarizeAllFactory:        summarizeAllFactory,
		composeCommitFactory:       composeCommitFactory,
		composeFlatCommitFactory:   composeFlatCommitFactory,
		commitConfigProvider:       commitConfigProvider,
		commandParametersReader:    commandParametersReader,
		outputWriter:               outputWriter,
	}, nil
}

// NewFlow creates and returns the commit composition flow function with added progress reporting.
func (f *Factory) NewFlow() executor.Flow {
	return func(ctx context.Context, flowCtx *executor.Context) error {
		return f.runCommitFlow(ctx, flowCtx)
	}
}

func (f *Factory) runCommitFlow(ctx context.Context, flowCtx *executor.Context) error {
	exec, err := f.validateFlowContext(ctx, flowCtx)
	if err != nil {
		return err
	}

	flowCtx.SendRunning("Commit Flow")

	targetBranch, err := f.commandParametersReader.GetTargetBranchFlag()
	if err != nil {
		return fmt.Errorf("failed to get target-branch flag: %w", err)
	}

	files, err := f.listFiles(ctx, flowCtx, exec, targetBranch)
	if err != nil {
		return err
	}

	filteredFiles, err := f.applyFilters(ctx, flowCtx, exec, files)
	if err != nil {
		return err
	}

	changes, err := f.fetchChanges(ctx, flowCtx, exec, filteredFiles, targetBranch)
	if err != nil {
		return err
	}

	cfg, intent, err := f.resolveConfigAndIntent()
	if err != nil {
		return err
	}

	commitMessage, err := f.composeCommitMessage(ctx, flowCtx, exec, cfg, intent, changes)
	if err != nil {
		return err
	}

	flowCtx.SendCompleted("")

	if err := f.outputWriter.PrintLine(strings.TrimSpace(commitMessage)); err != nil {
		return fmt.Errorf("failed to print commit message: %w", err)
	}

	return nil
}

func (f *Factory) validateFlowContext(ctx context.Context, flowCtx *executor.Context) (executor.Executor, error) {
	if f == nil {
		return nil, fmt.Errorf("factory is nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}
	if flowCtx == nil {
		return nil, fmt.Errorf("flow context is nil")
	}
	exec := flowCtx.GetExecutor()
	if exec == nil {
		return nil, fmt.Errorf("executor not available in context")
	}
	return exec, nil
}

func (f *Factory) listFiles(
	ctx context.Context,
	flowCtx *executor.Context,
	exec executor.Executor,
	targetBranch string,
) ([]string, error) {
	if targetBranch != "" {
		listBranchFiles := f.listBranchFilesFactory.NewActivity()
		branchFiles, err := executor.ExecuteActivity(
			ctx,
			exec,
			flowCtx,
			"ListBranchFiles",
			listBranchFiles,
			&listbranchfiles.Input{
				TargetBranch: targetBranch,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list branch files: %w", err)
		}
		return branchFiles.Files, nil
	}

	listStaged := f.listStagedFactory.NewActivity()
	stagedFiles, err := executor.ExecuteActivity(
		ctx,
		exec,
		flowCtx,
		"ListStagedFiles",
		listStaged,
		&liststaged.Input{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list staged files: %w", err)
	}
	return stagedFiles.Files, nil
}

func (f *Factory) applyFilters(
	ctx context.Context,
	flowCtx *executor.Context,
	exec executor.Executor,
	files []string,
) ([]string, error) {
	applyFilters := f.applyFiltersFactory.NewActivity()
	filteredFiles, err := executor.ExecuteActivity(
		ctx,
		exec,
		flowCtx,
		"ApplyFilters",
		applyFilters,
		&applyfilters.Input{
			Files: files,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to apply filters: %w", err)
	}
	return filteredFiles.Files, nil
}

func (f *Factory) fetchChanges(
	ctx context.Context,
	flowCtx *executor.Context,
	exec executor.Executor,
	files []string,
	targetBranch string,
) ([]*git.FileChange, error) {
	if targetBranch != "" {
		fetchAllBranchDiffs := f.fetchAllBranchDiffsFactory.NewActivity()
		fetchAllBranchDiffsOutput, err := executor.ExecuteActivity(
			ctx,
			exec,
			flowCtx,
			"FetchAllBranchDiffs",
			fetchAllBranchDiffs,
			&fetchallbranchdiffs.Input{
				Files:        files,
				TargetBranch: targetBranch,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch branch diffs: %w", err)
		}
		return fetchAllBranchDiffsOutput.Changes, nil
	}

	fetchAllDiffs := f.fetchAllDiffsFactory.NewActivity()
	fetchAllDiffsOutput, err := executor.ExecuteActivity(
		ctx,
		exec,
		flowCtx,
		"FetchAllDiffs",
		fetchAllDiffs,
		&fetchalldiffs.Input{
			Files: files,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch diffs: %w", err)
	}
	return fetchAllDiffsOutput.Changes, nil
}

func (f *Factory) resolveConfigAndIntent() (*commit.ResolvedConfig, string, error) {
	cfg, err := f.commitConfigProvider.Get()
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve commit configuration: %w", err)
	}

	intent, err := f.commandParametersReader.GetIntentFlag()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get intent flag: %w", err)
	}

	if intent == "" {
		stdin, err := f.commandParametersReader.GetStdIn()
		if err != nil {
			return nil, "", fmt.Errorf("failed to get stdin: %w", err)
		}
		intent = stdin
	}

	return cfg, intent, nil
}

func (f *Factory) composeCommitMessage(
	ctx context.Context,
	flowCtx *executor.Context,
	exec executor.Executor,
	cfg *commit.ResolvedConfig,
	intent string,
	changes []*git.FileChange,
) (string, error) {
	if cfg.Strategy == "flat" {
		composeFlatCommit := f.composeFlatCommitFactory.NewActivity()
		flatCommitResult, err := executor.ExecuteActivity(
			ctx,
			exec,
			flowCtx,
			"ComposeFlatCommit",
			composeFlatCommit,
			&composeflatcommit.Input{
				Profile:      cfg.Profile,
				SystemPrompt: cfg.SystemPrompt,
				Changes:      changes,
				Intent:       intent,
			},
		)
		if err != nil {
			return "", fmt.Errorf("failed to compose commit message using flat strategy: %w", err)
		}
		return flatCommitResult.CommitMessage, nil
	}

	summarizeAll := f.summarizeAllFactory.NewActivity()
	summarizeAllOutput, err := executor.ExecuteActivity(
		ctx,
		exec,
		flowCtx,
		"SummarizeAll",
		summarizeAll,
		&summarizeall.Input{
			Changes: changes,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to summarize changes: %w", err)
	}

	composeCommit := f.composeCommitFactory.NewActivity()
	commitResult, err := executor.ExecuteActivity(
		ctx,
		exec,
		flowCtx,
		"ComposeCommit",
		composeCommit,
		&composecommit.Input{
			Profile:      cfg.Profile,
			SystemPrompt: cfg.SystemPrompt,
			Summaries:    summarizeAllOutput.Summaries,
			Intent:       intent,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to compose commit message: %w", err)
	}

	return commitResult.CommitMessage, nil
}
