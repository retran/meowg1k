// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package pr implements the workflow for generating pull request descriptions from branch changes.
package pr

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/filterfiles"
	"github.com/retran/meowg1k/internal/activities/draftprflat"
	"github.com/retran/meowg1k/internal/activities/draftpr"
	"github.com/retran/meowg1k/internal/activities/fetchbranchdiffs"
	"github.com/retran/meowg1k/internal/activities/listbranchchanges"
	"github.com/retran/meowg1k/internal/activities/summarizechanges"
	"github.com/retran/meowg1k/internal/domain/git"
	domainpullrequest "github.com/retran/meowg1k/internal/domain/pullrequest"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// ConfigProvider provides pull request configuration.
type ConfigProvider interface {
	Get() (*domainpullrequest.ResolvedConfig, error)
}

// CommandParametersReader reads command-line parameters and flags.
type CommandParametersReader interface {
	GetBaseBranchFlag() (string, error)
	GetDiffFlag() (string, error)
	GetIntentFlag() (string, error)
	GetStdIn() (string, error)
}

// Factory creates instances of the PR flow with injected dependencies.
type Factory struct {
	listBranchFilesFactory     executor.ActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]
	applyFiltersFactory        executor.ActivityFactory[*filterfiles.Input, *filterfiles.Output]
	fetchAllBranchDiffsFactory executor.ActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]
	summarizeAllFactory        executor.ActivityFactory[*summarizechanges.Input, *summarizechanges.Output]
	composePRFactory           executor.ActivityFactory[*draftpr.Input, *draftpr.Output]
	composeFlatPRFactory       executor.ActivityFactory[*draftprflat.Input, *draftprflat.Output]
	prConfigProvider           ConfigProvider
	commandParametersReader    CommandParametersReader
	outputWriter               ports.OutputWriter
}

// NewFactory creates a new PR flow factory with injected adapters.
func NewFactory(
	listBranchFilesFactory executor.ActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output],
	applyFiltersFactory executor.ActivityFactory[*filterfiles.Input, *filterfiles.Output],
	fetchAllBranchDiffsFactory executor.ActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output],
	summarizeAllFactory executor.ActivityFactory[*summarizechanges.Input, *summarizechanges.Output],
	composePRFactory executor.ActivityFactory[*draftpr.Input, *draftpr.Output],
	composeFlatPRFactory executor.ActivityFactory[*draftprflat.Input, *draftprflat.Output],
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

	diffMode, err := f.commandParametersReader.GetDiffFlag()
	if err != nil {
		return fmt.Errorf("failed to get diff flag: %w", err)
	}

	baseBranch, err := f.commandParametersReader.GetBaseBranchFlag()
	if err != nil {
		return fmt.Errorf("failed to get base-branch flag: %w", err)
	}

	diffMode = strings.ToLower(strings.TrimSpace(diffMode))
	if diffMode == "" {
		diffMode = "branch"
	}

	if diffMode != "branch" {
		return fmt.Errorf("diff must be 'branch' for pull request drafts")
	}
	if baseBranch == "" {
		return fmt.Errorf("base branch is required when --diff branch")
	}
	flowCtx.SendRunningWithDetails("I'm drafting the pull request description", fmt.Sprintf("diff=branch base=%s", baseBranch))

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

	if err := f.outputWriter.PrintLine(strings.TrimSpace(prDescription)); err != nil {
		return fmt.Errorf("failed to print PR description: %w", err)
	}

	flowCtx.SendCompletedWithDetails("I've drafted the pull request description", fmt.Sprintf("diff=branch base=%s", baseBranch))

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
	branchFiles, err := executor.ExecuteActivity(
		ctx,
		exec,
		flowCtx,
		"ListBranchFiles",
		listBranchFiles,
		&listbranchchanges.Input{
			TargetBranch: baseBranch,
		},
	)
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
	filteredFiles, err := executor.ExecuteActivity(
		ctx,
		exec,
		flowCtx,
		"ApplyFilters",
		applyFilters,
		&filterfiles.Input{
			Files: files,
		},
	)
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
	fetchAllBranchDiffsOutput, err := executor.ExecuteActivity(
		ctx,
		exec,
		flowCtx,
		"FetchAllBranchDiffs",
		fetchAllBranchDiffs,
		&fetchbranchdiffs.Input{
			Files:        files,
			TargetBranch: baseBranch,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch branch diffs: %w", err)
	}
	return fetchAllBranchDiffsOutput.Changes, nil
}

func (f *Factory) resolveConfigAndIntent() (*domainpullrequest.ResolvedConfig, string, error) {
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
	cfg *domainpullrequest.ResolvedConfig,
	intent string,
	changes []*git.FileChange,
) (string, error) {
	if cfg.Strategy == "flat" {
		composeFlatPR := f.composeFlatPRFactory.NewActivity()
		flatPRResult, err := executor.ExecuteActivity(
			ctx,
			exec,
			flowCtx,
			"ComposeFlatPR",
			composeFlatPR,
			&draftprflat.Input{
				Profile:      cfg.Profile,
				SystemPrompt: cfg.SystemPrompt,
				Changes:      changes,
				Intent:       intent,
			},
		)
		if err != nil {
			return "", fmt.Errorf("failed to compose PR description using flat strategy: %w", err)
		}
		return flatPRResult.Description, nil
	}

	summarizeAll := f.summarizeAllFactory.NewActivity()
	summarizeAllOutput, err := executor.ExecuteActivity(
		ctx,
		exec,
		flowCtx,
		"SummarizeAll",
		summarizeAll,
		&summarizechanges.Input{
			Changes: changes,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to summarize changes: %w", err)
	}

	composePR := f.composePRFactory.NewActivity()
	prResult, err := executor.ExecuteActivity(
		ctx,
		exec,
		flowCtx,
		"ComposePR",
		composePR,
		&draftpr.Input{
			Profile:      cfg.Profile,
			SystemPrompt: cfg.SystemPrompt,
			Summaries:    summarizeAllOutput.Summaries,
			Intent:       intent,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to compose PR description: %w", err)
	}

	return prResult.PRDescription, nil
}
