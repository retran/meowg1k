// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package pullrequest implements the workflow for generating pull request descriptions from branch changes.
package pullrequest

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/applyfilters"
	"github.com/retran/meowg1k/internal/activities/composeflatpr"
	"github.com/retran/meowg1k/internal/activities/composepr"
	"github.com/retran/meowg1k/internal/activities/fetchallbranchdiffs"
	"github.com/retran/meowg1k/internal/activities/listbranchfiles"
	"github.com/retran/meowg1k/internal/activities/summarizeall"
	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/internal/domain/pullrequest"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// ConfigProvider provides pull request configuration.
type ConfigProvider interface {
	Get() (*pullrequest.ResolvedConfig, error)
}

// CommandParametersReader reads command-line parameters and flags.
type CommandParametersReader interface {
	GetBaseBranchFlag() (string, error)
	GetIntentFlag() (string, error)
	GetStdIn() (string, error)
}

// Factory creates instances of the PR flow with injected dependencies.
type Factory struct {
	listBranchFilesFactory     executor.ActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]
	applyFiltersFactory        executor.ActivityFactory[*applyfilters.Input, *applyfilters.Output]
	fetchAllBranchDiffsFactory executor.ActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]
	summarizeAllFactory        executor.ActivityFactory[*summarizeall.Input, *summarizeall.Output]
	composePRFactory           executor.ActivityFactory[*composepr.Input, *composepr.Output]
	composeFlatPRFactory       executor.ActivityFactory[*composeflatpr.Input, *composeflatpr.Output]
	prConfigProvider           ConfigProvider
	commandParametersReader    CommandParametersReader
	outputWriter               ports.OutputWriter
}

// NewFactory creates a new PR flow factory with injected adapters.
func NewFactory(
	listBranchFilesFactory executor.ActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output],
	applyFiltersFactory executor.ActivityFactory[*applyfilters.Input, *applyfilters.Output],
	fetchAllBranchDiffsFactory executor.ActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output],
	summarizeAllFactory executor.ActivityFactory[*summarizeall.Input, *summarizeall.Output],
	composePRFactory executor.ActivityFactory[*composepr.Input, *composepr.Output],
	composeFlatPRFactory executor.ActivityFactory[*composeflatpr.Input, *composeflatpr.Output],
	prConfigProvider ConfigProvider,
	commandParametersReader CommandParametersReader,
	outputWriter ports.OutputWriter,
) (*Factory, error) {
	if listBranchFilesFactory == nil {
		return nil, fmt.Errorf("listBranchFilesFactory is nil")
	}

	if applyFiltersFactory == nil {
		return nil, fmt.Errorf("applyFiltersFactory is nil")
	}

	if fetchAllBranchDiffsFactory == nil {
		return nil, fmt.Errorf("fetchAllBranchDiffsFactory is nil")
	}

	if summarizeAllFactory == nil {
		return nil, fmt.Errorf("summarizeAllFactory is nil")
	}

	if composePRFactory == nil {
		return nil, fmt.Errorf("composePRFactory is nil")
	}

	if composeFlatPRFactory == nil {
		return nil, fmt.Errorf("composeFlatPRFactory is nil")
	}

	if prConfigProvider == nil {
		return nil, fmt.Errorf("prConfigProvider is nil")
	}

	if commandParametersReader == nil {
		return nil, fmt.Errorf("commandParametersReader is nil")
	}

	if outputWriter == nil {
		return nil, fmt.Errorf("outputWriter is nil")
	}

	return &Factory{
		listBranchFilesFactory:     listBranchFilesFactory,
		applyFiltersFactory:        applyFiltersFactory,
		fetchAllBranchDiffsFactory: fetchAllBranchDiffsFactory,
		summarizeAllFactory:        summarizeAllFactory,
		composePRFactory:           composePRFactory,
		composeFlatPRFactory:       composeFlatPRFactory,
		prConfigProvider:           prConfigProvider,
		commandParametersReader:    commandParametersReader,
		outputWriter:               outputWriter,
	}, nil
}

// NewFlow creates and returns the PR composition flow function with added progress reporting.
func (f *Factory) NewFlow() executor.Flow {
	return func(ctx context.Context, flowCtx *executor.Context) error {
		return f.runPullRequestFlow(ctx, flowCtx)
	}
}

func (f *Factory) runPullRequestFlow(ctx context.Context, flowCtx *executor.Context) error {
	exec, err := f.validateFlowContext(ctx, flowCtx)
	if err != nil {
		return err
	}

	flowCtx.SendRunning("Composing Pull Request description")

	baseBranch, err := f.commandParametersReader.GetBaseBranchFlag()
	if err != nil {
		return fmt.Errorf("failed to get base-branch flag: %w", err)
	}

	if baseBranch == "" {
		return fmt.Errorf("base branch is required for PR command (use --base flag)")
	}

	files, err := f.listBranchFiles(ctx, flowCtx, exec, baseBranch)
	if err != nil {
		return err
	}

	filteredFiles, err := f.applyFilters(ctx, flowCtx, exec, files)
	if err != nil {
		return err
	}

	changes, err := f.fetchBranchChanges(ctx, flowCtx, exec, filteredFiles, baseBranch)
	if err != nil {
		return err
	}

	cfg, intent, err := f.resolveConfigAndIntent()
	if err != nil {
		return err
	}

	prDescription, err := f.composePRDescription(ctx, flowCtx, exec, cfg, intent, changes)
	if err != nil {
		return err
	}

	flowCtx.SendCompleted("")

	if err := f.outputWriter.PrintLine(strings.TrimSpace(prDescription)); err != nil {
		return fmt.Errorf("failed to print PR description: %w", err)
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

func (f *Factory) listBranchFiles(
	ctx context.Context,
	flowCtx *executor.Context,
	exec executor.Executor,
	baseBranch string,
) ([]string, error) {
	listBranchFiles := f.listBranchFilesFactory.NewActivity()
	branchFilesFuture := executor.ExecuteActivity(
		ctx,
		exec,
		flowCtx,
		"ListBranchFiles",
		listBranchFiles,
		&listbranchfiles.Input{
			TargetBranch: baseBranch,
		},
	)

	branchFiles, err := branchFilesFuture.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list branch files: %w", err)
	}
	return branchFiles.Files, nil
}

func (f *Factory) applyFilters(
	ctx context.Context,
	flowCtx *executor.Context,
	exec executor.Executor,
	files []string,
) ([]string, error) {
	applyFilters := f.applyFiltersFactory.NewActivity()
	filteredFilesFuture := executor.ExecuteActivity(
		ctx,
		exec,
		flowCtx,
		"ApplyFilters",
		applyFilters,
		&applyfilters.Input{
			Files: files,
		},
	)

	filteredFiles, err := filteredFilesFuture.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to apply filters: %w", err)
	}
	return filteredFiles.Files, nil
}

func (f *Factory) fetchBranchChanges(
	ctx context.Context,
	flowCtx *executor.Context,
	exec executor.Executor,
	files []string,
	baseBranch string,
) ([]*git.FileChange, error) {
	fetchAllBranchDiffs := f.fetchAllBranchDiffsFactory.NewActivity()
	fetchAllBranchDiffsFuture := executor.ExecuteActivity(
		ctx,
		exec,
		flowCtx,
		"FetchAllBranchDiffs",
		fetchAllBranchDiffs,
		&fetchallbranchdiffs.Input{
			Files:        files,
			TargetBranch: baseBranch,
		},
	)

	fetchAllBranchDiffsOutput, err := fetchAllBranchDiffsFuture.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch branch diffs: %w", err)
	}
	return fetchAllBranchDiffsOutput.Changes, nil
}

func (f *Factory) resolveConfigAndIntent() (*pullrequest.ResolvedConfig, string, error) {
	cfg, err := f.prConfigProvider.Get()
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve PR configuration: %w", err)
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

func (f *Factory) composePRDescription(
	ctx context.Context,
	flowCtx *executor.Context,
	exec executor.Executor,
	cfg *pullrequest.ResolvedConfig,
	intent string,
	changes []*git.FileChange,
) (string, error) {
	if cfg.Strategy == "flat" {
		composeFlatPR := f.composeFlatPRFactory.NewActivity()
		flatPRFuture := executor.ExecuteActivity(
			ctx,
			exec,
			flowCtx,
			"ComposeFlatPR",
			composeFlatPR,
			&composeflatpr.Input{
				Profile:      cfg.Profile,
				SystemPrompt: cfg.SystemPrompt,
				Changes:      changes,
				Intent:       intent,
			},
		)

		flatPRResult, err := flatPRFuture.Get(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to compose PR description using flat strategy: %w", err)
		}
		return flatPRResult.Description, nil
	}

	summarizeAll := f.summarizeAllFactory.NewActivity()
	summarizeAllFuture := executor.ExecuteActivity(
		ctx,
		exec,
		flowCtx,
		"SummarizeAll",
		summarizeAll,
		&summarizeall.Input{
			Changes: changes,
		},
	)

	summarizeAllOutput, err := summarizeAllFuture.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to summarize changes: %w", err)
	}

	composePR := f.composePRFactory.NewActivity()
	prFuture := executor.ExecuteActivity(
		ctx,
		exec,
		flowCtx,
		"ComposePR",
		composePR,
		&composepr.Input{
			Profile:      cfg.Profile,
			SystemPrompt: cfg.SystemPrompt,
			Summaries:    summarizeAllOutput.Summaries,
			Intent:       intent,
		},
	)

	prResult, err := prFuture.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to compose PR description: %w", err)
	}

	return prResult.PRDescription, nil
}
