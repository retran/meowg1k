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

// Package commit provides a flow to compose a commit message based on staged changes.
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

// CommitConfigProvider provides commit message configuration.
type CommitConfigProvider interface {
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
	commitConfigProvider       CommitConfigProvider
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
	commitConfigProvider CommitConfigProvider,
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
		if f == nil {
			return fmt.Errorf("factory is nil")
		}

		if ctx == nil {
			return fmt.Errorf("context is nil")
		}

		if flowCtx == nil {
			return fmt.Errorf("flow context is nil")
		}

		flowCtx.SendRunning("Composing commit message")

		// Check if we're in squash mode (branch comparison)
		targetBranch, err := f.commandParametersReader.GetTargetBranchFlag()
		if err != nil {
			return fmt.Errorf("failed to get target-branch flag: %w", err)
		}

		var files []string

		// Phase 1: List files (staged or changed in branch)
		if targetBranch != "" {
			// Squash mode: list files changed in branch
			listBranchFiles := f.listBranchFilesFactory.NewActivity()
			branchFilesFuture := executor.ExecuteActivity(
				flowCtx.GetExecutor(),
				ctx,
				flowCtx,
				"ListBranchFiles",
				listBranchFiles,
				&listbranchfiles.Input{
					TargetBranch: targetBranch,
				},
			)

			branchFiles, err := branchFilesFuture.Get(ctx)
			if err != nil {
				return fmt.Errorf("failed to list branch files: %w", err)
			}
			files = branchFiles.Files
		} else {
			// Normal mode: list staged files
			listStaged := f.listStagedFactory.NewActivity()
			stagedFilesFuture := executor.ExecuteActivity(
				flowCtx.GetExecutor(),
				ctx,
				flowCtx,
				"ListStagedFiles",
				listStaged,
				&liststaged.Input{},
			)

			stagedFiles, err := stagedFilesFuture.Get(ctx)
			if err != nil {
				return fmt.Errorf("failed to list staged files: %w", err)
			}
			files = stagedFiles.Files
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
				Files: files,
			},
		)

		filteredFiles, err := filteredFilesFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to apply filters: %w", err)
		}

		// Phase 3: Fetch diffs for all files
		var changes []*git.FileChange

		if targetBranch != "" {
			// Squash mode: fetch branch diffs
			fetchAllBranchDiffs := f.fetchAllBranchDiffsFactory.NewActivity()
			fetchAllBranchDiffsFuture := executor.ExecuteActivity(
				flowCtx.GetExecutor(),
				ctx,
				flowCtx,
				"FetchAllBranchDiffs",
				fetchAllBranchDiffs,
				&fetchallbranchdiffs.Input{
					Files:        filteredFiles.Files,
					TargetBranch: targetBranch,
				},
			)

			fetchAllBranchDiffsOutput, err := fetchAllBranchDiffsFuture.Get(ctx)
			if err != nil {
				return fmt.Errorf("failed to fetch branch diffs: %w", err)
			}

			changes = append(changes, fetchAllBranchDiffsOutput.Changes...)
		} else {
			// Normal mode: fetch staged diffs
			fetchAllDiffs := f.fetchAllDiffsFactory.NewActivity()
			fetchAllDiffsFuture := executor.ExecuteActivity(
				flowCtx.GetExecutor(),
				ctx,
				flowCtx,
				"FetchAllDiffs",
				fetchAllDiffs,
				&fetchalldiffs.Input{
					Files: filteredFiles.Files,
				},
			)

			fetchAllDiffsOutput, err := fetchAllDiffsFuture.Get(ctx)
			if err != nil {
				return fmt.Errorf("failed to fetch diffs: %w", err)
			}

			changes = append(changes, fetchAllDiffsOutput.Changes...)
		}

		// Get configuration
		cfg, err := f.commitConfigProvider.Get()
		if err != nil {
			return fmt.Errorf("failed to resolve commit configuration: %w", err)
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

		var commitMessage string

		// Phase 4 & 5: Compose commit message based on strategy
		if cfg.Strategy == "flat" {
			// Flat strategy: send full diff directly to LLM
			composeFlatCommit := f.composeFlatCommitFactory.NewActivity()
			flatCommitFuture := executor.ExecuteActivity(
				flowCtx.GetExecutor(),
				ctx,
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

			flatCommitResult, err := flatCommitFuture.Get(ctx)
			if err != nil {
				return fmt.Errorf("failed to compose commit message using flat strategy: %w", err)
			}

			commitMessage = flatCommitResult.CommitMessage
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

			composeCommit := f.composeCommitFactory.NewActivity()
			commitFuture := executor.ExecuteActivity(
				flowCtx.GetExecutor(),
				ctx,
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

			commitResult, err := commitFuture.Get(ctx)
			if err != nil {
				return fmt.Errorf("failed to compose commit message: %w", err)
			}

			commitMessage = commitResult.CommitMessage
		}

		flowCtx.SendCompleted("")

		if err := f.outputWriter.PrintLine(strings.TrimSpace(commitMessage)); err != nil {
			return fmt.Errorf("failed to print commit message: %w", err)
		}

		return nil
	}
}
