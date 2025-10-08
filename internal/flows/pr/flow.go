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

// Package pr provides a flow to compose a Pull Request description based on branch changes.
package pr

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/applyfilters"
	"github.com/retran/meowg1k/internal/activities/composepr"
	"github.com/retran/meowg1k/internal/activities/fetchallbranchdiffs"
	"github.com/retran/meowg1k/internal/activities/listbranchfiles"
	"github.com/retran/meowg1k/internal/activities/summarizeall"
	"github.com/retran/meowg1k/internal/services/git"
	"github.com/retran/meowg1k/internal/services/prconfig"
	"github.com/retran/meowg1k/pkg/executor"
)

// PRConfigProvider provides pull request configuration.
type PRConfigProvider interface {
	GetPRConfig() (*prconfig.ResolvedPRConfig, error)
}

// CommandParametersReader reads command-line parameters and flags.
type CommandParametersReader interface {
	GetBaseBranchFlag() (string, error)
	GetIntentFlag() (string, error)
	GetStdIn() string
}

// OutputWriter writes output to the user.
type OutputWriter interface {
	PrintLine(line string)
}

// Factory creates instances of the PR flow with injected dependencies.
type Factory struct {
	listBranchFilesFactory     executor.ActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]
	applyFiltersFactory        executor.ActivityFactory[*applyfilters.Input, *applyfilters.Output]
	fetchAllBranchDiffsFactory executor.ActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]
	summarizeAllFactory        executor.ActivityFactory[*summarizeall.Input, *summarizeall.Output]
	composePRFactory           executor.ActivityFactory[*composepr.Input, *composepr.Output]
	prConfigProvider           PRConfigProvider
	commandParametersReader    CommandParametersReader
	outputWriter               OutputWriter
}

// NewFactory creates a new PR flow factory with injected services.
func NewFactory(
	listBranchFilesFactory executor.ActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output],
	applyFiltersFactory executor.ActivityFactory[*applyfilters.Input, *applyfilters.Output],
	fetchAllBranchDiffsFactory executor.ActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output],
	summarizeAllFactory executor.ActivityFactory[*summarizeall.Input, *summarizeall.Output],
	composePRFactory executor.ActivityFactory[*composepr.Input, *composepr.Output],
	prConfigProvider PRConfigProvider,
	commandParametersReader CommandParametersReader,
	outputWriter OutputWriter,
) *Factory {
	return &Factory{
		listBranchFilesFactory:     listBranchFilesFactory,
		applyFiltersFactory:        applyFiltersFactory,
		fetchAllBranchDiffsFactory: fetchAllBranchDiffsFactory,
		summarizeAllFactory:        summarizeAllFactory,
		composePRFactory:           composePRFactory,
		prConfigProvider:           prConfigProvider,
		commandParametersReader:    commandParametersReader,
		outputWriter:               outputWriter,
	}
}

// NewFlow creates and returns the PR composition flow function with added progress reporting.
func (f *Factory) NewFlow() executor.Flow {
	return func(ctx context.Context, flowCtx *executor.Context) error {
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
		branchFilesFuture := executor.RunActivity(
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
		filteredFilesFuture := executor.RunActivity(
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
		fetchAllBranchDiffsFuture := executor.RunActivity(
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

		// Phase 4: Summarize changes for all files
		summarizeAll := f.summarizeAllFactory.NewActivity()
		summarizeAllFuture := executor.RunActivity(
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

		// Phase 5: Compose PR description
		prConfig, err := f.prConfigProvider.GetPRConfig()
		if err != nil {
			return fmt.Errorf("failed to resolve PR configuration: %w", err)
		}

		intent, err := f.commandParametersReader.GetIntentFlag()
		if err != nil {
			return fmt.Errorf("failed to get intent flag: %w", err)
		}

		if intent == "" {
			intent = f.commandParametersReader.GetStdIn()
		}

		composePR := f.composePRFactory.NewActivity()
		prFuture := executor.RunActivity(
			flowCtx.GetExecutor(),
			ctx,
			flowCtx,
			"ComposePR",
			composePR,
			&composepr.Input{
				Profile:      prConfig.Profile,
				SystemPrompt: prConfig.SystemPrompt,
				Summaries:    summarizeAllOutput.Summaries,
				Intent:       intent,
			},
		)

		prResult, err := prFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to compose PR description: %w", err)
		}

		flowCtx.SendCompleted("")

		f.outputWriter.PrintLine(prResult.PRDescription)

		return nil
	}
}
