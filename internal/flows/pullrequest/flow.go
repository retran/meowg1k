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

// PullRequestConfigProvider provides pull request configuration.
type PullRequestConfigProvider interface {
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
	prConfigProvider           PullRequestConfigProvider
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
	prConfigProvider PullRequestConfigProvider,
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
		if f == nil {
			return fmt.Errorf("factory is nil")
		}

		if ctx == nil {
			return fmt.Errorf("context is nil")
		}

		if flowCtx == nil {
			return fmt.Errorf("flow context is nil")
		}

		flowCtx.SendRunning("Composing Pull Request description")

		// Get the base branch to compare against
		baseBranch, err := f.commandParametersReader.GetBaseBranchFlag()
		if err != nil {
			return fmt.Errorf("failed to get base-branch flag: %w", err)
		}

		if baseBranch == "" {
			return fmt.Errorf("base branch is required for PR command (use --base flag)")
		}

		// Phase 1: List files changed in branch
		listBranchFiles := f.listBranchFilesFactory.NewActivity()
		branchFilesFuture := executor.ExecuteActivity(
			flowCtx.GetExecutor(),
			ctx,
			flowCtx,
			"ListBranchFiles",
			listBranchFiles,
			&listbranchfiles.Input{
				TargetBranch: baseBranch,
			},
		)

		branchFiles, err := branchFilesFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to list branch files: %w", err)
		}

		// Phase 2: Apply filters
		applyFilters := f.applyFiltersFactory.NewActivity()
		filteredFilesFuture := executor.ExecuteActivity(
			flowCtx.GetExecutor(),
			ctx,
			flowCtx,
			"ApplyFilters",
			applyFilters,
			&applyfilters.Input{
				Files: branchFiles.Files,
			},
		)

		filteredFiles, err := filteredFilesFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to apply filters: %w", err)
		}

		// Phase 3: Fetch diffs for all files
		fetchAllBranchDiffs := f.fetchAllBranchDiffsFactory.NewActivity()
		fetchAllBranchDiffsFuture := executor.ExecuteActivity(
			flowCtx.GetExecutor(),
			ctx,
			flowCtx,
			"FetchAllBranchDiffs",
			fetchAllBranchDiffs,
			&fetchallbranchdiffs.Input{
				Files:        filteredFiles.Files,
				TargetBranch: baseBranch,
			},
		)

		fetchAllBranchDiffsOutput, err := fetchAllBranchDiffsFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch branch diffs: %w", err)
		}

		var changes []*git.FileChange
		changes = append(changes, fetchAllBranchDiffsOutput.Changes...)

		// Get configuration
		cfg, err := f.prConfigProvider.Get()
		if err != nil {
			return fmt.Errorf("failed to resolve PR configuration: %w", err)
		}

		intent, err := f.commandParametersReader.GetIntentFlag()
		if err != nil {
			return fmt.Errorf("failed to get intent flag: %w", err)
		}

		if intent == "" {
			stdin, err := f.commandParametersReader.GetStdIn()
			if err != nil {
				return fmt.Errorf("failed to get stdin: %w", err)
			}

			intent = stdin
		}

		var prDescription string

		// Phase 4 & 5: Compose PR description based on strategy
		if cfg.Strategy == "flat" {
			// Flat strategy: send full diff directly to LLM
			composeFlatPR := f.composeFlatPRFactory.NewActivity()
			flatPRFuture := executor.ExecuteActivity(
				flowCtx.GetExecutor(),
				ctx,
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
				return fmt.Errorf("failed to compose PR description using flat strategy: %w", err)
			}

			prDescription = flatPRResult.Description
		} else {
			// Summarize strategy (default): use map-reduce approach
			summarizeAll := f.summarizeAllFactory.NewActivity()
			summarizeAllFuture := executor.ExecuteActivity(
				flowCtx.GetExecutor(),
				ctx,
				flowCtx,
				"SummarizeAll",
				summarizeAll,
				&summarizeall.Input{
					Changes: changes,
				},
			)

			summarizeAllOutput, err := summarizeAllFuture.Get(ctx)
			if err != nil {
				return fmt.Errorf("failed to summarize changes: %w", err)
			}

			composePR := f.composePRFactory.NewActivity()
			prFuture := executor.ExecuteActivity(
				flowCtx.GetExecutor(),
				ctx,
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
				return fmt.Errorf("failed to compose PR description: %w", err)
			}

			prDescription = prResult.PRDescription
		}

		flowCtx.SendCompleted("")

		if err := f.outputWriter.PrintLine(strings.TrimSpace(prDescription)); err != nil {
			return fmt.Errorf("failed to print PR description: %w", err)
		}

		return nil
	}
}
