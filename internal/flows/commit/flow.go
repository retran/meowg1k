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
	"errors"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/applyfilters"
	"github.com/retran/meowg1k/internal/activities/composecommit"
	"github.com/retran/meowg1k/internal/activities/fetchallbranchdiffs"
	"github.com/retran/meowg1k/internal/activities/fetchalldiffs"
	"github.com/retran/meowg1k/internal/activities/listbranchfiles"
	"github.com/retran/meowg1k/internal/activities/liststaged"
	"github.com/retran/meowg1k/internal/activities/summarizeall"
	"github.com/retran/meowg1k/internal/services/commitconfig"
	"github.com/retran/meowg1k/internal/services/git"
	"github.com/retran/meowg1k/pkg/executor"
)

var (
	// ErrFactoryIsNil indicates that the factory is nil.
	ErrFactoryIsNil = errors.New("factory is nil")
	// ErrContextIsNil indicates that the context is nil.
	ErrContextIsNil = errors.New("context is nil")
	// ErrFlowContextIsNil indicates that the flow context is nil.
	ErrFlowContextIsNil = errors.New("flow context is nil")
	// ErrListStagedFactoryIsNil indicates that the listStagedFactory is nil.
	ErrListStagedFactoryIsNil = errors.New("listStagedFactory is nil")
	// ErrListBranchFilesFactoryIsNil indicates that the listBranchFilesFactory is nil.
	ErrListBranchFilesFactoryIsNil = errors.New("listBranchFilesFactory is nil")
	// ErrApplyFiltersFactoryIsNil indicates that the applyFiltersFactory is nil.
	ErrApplyFiltersFactoryIsNil = errors.New("applyFiltersFactory is nil")
	// ErrFetchAllDiffsFactoryIsNil indicates that the fetchAllDiffsFactory is nil.
	ErrFetchAllDiffsFactoryIsNil = errors.New("fetchAllDiffsFactory is nil")
	// ErrFetchAllBranchDiffsFactoryIsNil indicates that the fetchAllBranchDiffsFactory is nil.
	ErrFetchAllBranchDiffsFactoryIsNil = errors.New("fetchAllBranchDiffsFactory is nil")
	// ErrSummarizeAllFactoryIsNil indicates that the summarizeAllFactory is nil.
	ErrSummarizeAllFactoryIsNil = errors.New("summarizeAllFactory is nil")
	// ErrComposeCommitFactoryIsNil indicates that the composeCommitFactory is nil.
	ErrComposeCommitFactoryIsNil = errors.New("composeCommitFactory is nil")
	// ErrCommitConfigProviderIsNil indicates that the commitConfigProvider is nil.
	ErrCommitConfigProviderIsNil = errors.New("commitConfigProvider is nil")
	// ErrCommandParametersReaderIsNil indicates that the commandParametersReader is nil.
	ErrCommandParametersReaderIsNil = errors.New("commandParametersReader is nil")
	// ErrOutputWriterIsNil indicates that the outputWriter is nil.
	ErrOutputWriterIsNil = errors.New("outputWriter is nil")
)

// CommitConfigProvider provides commit message configuration.
type CommitConfigProvider interface {
	GetCommitConfig() (*commitconfig.ResolvedCommitConfig, error)
}

// CommandParametersReader reads command-line parameters and flags.
type CommandParametersReader interface {
	GetTargetBranchFlag() (string, error)
	GetIntentFlag() (string, error)
	GetStdIn() (string, error)
}

// OutputWriter writes output to the user.
type OutputWriter interface {
	PrintLine(line string) error
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
	commitConfigProvider       CommitConfigProvider
	commandParametersReader    CommandParametersReader
	outputWriter               OutputWriter
}

// NewFactory creates a new commit flow factory with injected services.
func NewFactory(
	listStagedFactory executor.ActivityFactory[*liststaged.Input, *liststaged.Output],
	listBranchFilesFactory executor.ActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output],
	applyFiltersFactory executor.ActivityFactory[*applyfilters.Input, *applyfilters.Output],
	fetchAllDiffsFactory executor.ActivityFactory[*fetchalldiffs.Input, *fetchalldiffs.Output],
	fetchAllBranchDiffsFactory executor.ActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output],
	summarizeAllFactory executor.ActivityFactory[*summarizeall.Input, *summarizeall.Output],
	composeCommitFactory executor.ActivityFactory[*composecommit.Input, *composecommit.Output],
	commitConfigProvider CommitConfigProvider,
	commandParametersReader CommandParametersReader,
	outputWriter OutputWriter,
) (*Factory, error) {
	if listStagedFactory == nil {
		return nil, ErrListStagedFactoryIsNil
	}
	if listBranchFilesFactory == nil {
		return nil, ErrListBranchFilesFactoryIsNil
	}
	if applyFiltersFactory == nil {
		return nil, ErrApplyFiltersFactoryIsNil
	}
	if fetchAllDiffsFactory == nil {
		return nil, ErrFetchAllDiffsFactoryIsNil
	}
	if fetchAllBranchDiffsFactory == nil {
		return nil, ErrFetchAllBranchDiffsFactoryIsNil
	}
	if summarizeAllFactory == nil {
		return nil, ErrSummarizeAllFactoryIsNil
	}
	if composeCommitFactory == nil {
		return nil, ErrComposeCommitFactoryIsNil
	}
	if commitConfigProvider == nil {
		return nil, ErrCommitConfigProviderIsNil
	}
	if commandParametersReader == nil {
		return nil, ErrCommandParametersReaderIsNil
	}
	if outputWriter == nil {
		return nil, ErrOutputWriterIsNil
	}

	return &Factory{
		listStagedFactory:          listStagedFactory,
		listBranchFilesFactory:     listBranchFilesFactory,
		applyFiltersFactory:        applyFiltersFactory,
		fetchAllDiffsFactory:       fetchAllDiffsFactory,
		fetchAllBranchDiffsFactory: fetchAllBranchDiffsFactory,
		summarizeAllFactory:        summarizeAllFactory,
		composeCommitFactory:       composeCommitFactory,
		commitConfigProvider:       commitConfigProvider,
		commandParametersReader:    commandParametersReader,
		outputWriter:               outputWriter,
	}, nil
}

// NewFlow creates and returns the commit composition flow function with added progress reporting.
func (f *Factory) NewFlow() executor.Flow {
	return func(ctx context.Context, flowCtx *executor.Context) error {
		if f == nil {
			return ErrFactoryIsNil
		}
		if ctx == nil {
			return ErrContextIsNil
		}
		if flowCtx == nil {
			return ErrFlowContextIsNil
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
			branchFilesFuture := executor.RunActivity(
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
			stagedFilesFuture := executor.RunActivity(
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
		filteredFilesFuture := executor.RunActivity(
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
			fetchAllBranchDiffsFuture := executor.RunActivity(
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
			// Convert to generic slice
			changes = append(changes, fetchAllBranchDiffsOutput.Changes...)
		} else {
			// Normal mode: fetch staged diffs
			fetchAllDiffs := f.fetchAllDiffsFactory.NewActivity()
			fetchAllDiffsFuture := executor.RunActivity(
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
			// Convert to generic slice
			changes = append(changes, fetchAllDiffsOutput.Changes...)
		}

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

		// Phase 5: Compose commit message
		commitConfig, err := f.commitConfigProvider.GetCommitConfig()
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

		composeCommit := f.composeCommitFactory.NewActivity()
		commitFuture := executor.RunActivity(
			flowCtx.GetExecutor(),
			ctx,
			flowCtx,
			"ComposeCommit",
			composeCommit,
			&composecommit.Input{
				Profile:      commitConfig.Profile,
				SystemPrompt: commitConfig.SystemPrompt,
				Summaries:    summarizeAllOutput.Summaries,
				Intent:       intent,
			},
		)

		commitResult, err := commitFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to compose commit message: %w", err)
		}

		flowCtx.SendCompleted("")

		if err := f.outputWriter.PrintLine(commitResult.CommitMessage); err != nil {
			return fmt.Errorf("failed to print commit message: %w", err)
		}

		return nil
	}
}
