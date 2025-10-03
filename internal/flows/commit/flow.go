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

	"github.com/retran/meowg1k/internal/activities/applyfilters"
	"github.com/retran/meowg1k/internal/activities/composecommit"
	"github.com/retran/meowg1k/internal/activities/fetchallbranchdiffs"
	"github.com/retran/meowg1k/internal/activities/fetchalldiffs"
	"github.com/retran/meowg1k/internal/activities/listbranchfiles"
	"github.com/retran/meowg1k/internal/activities/liststaged"
	"github.com/retran/meowg1k/internal/activities/summarizeall"
	"github.com/retran/meowg1k/internal/services/command"
	"github.com/retran/meowg1k/internal/services/commitconfig"
	"github.com/retran/meowg1k/internal/services/git"
	"github.com/retran/meowg1k/internal/services/output"
	"github.com/retran/meowg1k/pkg/executor"
)

// Factory creates instances of the commit flow with injected dependencies.
type Factory struct {
	listStagedFactory          executor.ActivityFactory
	listBranchFilesFactory     executor.ActivityFactory
	applyFiltersFactory        executor.ActivityFactory
	fetchAllDiffsFactory       executor.ActivityFactory
	fetchAllBranchDiffsFactory executor.ActivityFactory
	summarizeAllFactory        executor.ActivityFactory
	composeCommitFactory       executor.ActivityFactory
	commitConfigService        commitconfig.Service
	commandService             command.Service
	outputService              output.Service
}

// NewFactory creates a new commit flow factory with injected services.
func NewFactory(
	listStagedFactory executor.ActivityFactory,
	listBranchFilesFactory executor.ActivityFactory,
	applyFiltersFactory executor.ActivityFactory,
	fetchAllDiffsFactory executor.ActivityFactory,
	fetchAllBranchDiffsFactory executor.ActivityFactory,
	summarizeAllFactory executor.ActivityFactory,
	composeCommitFactory executor.ActivityFactory,
	commitConfigService commitconfig.Service,
	commandService command.Service,
	outputService output.Service,
) *Factory {
	return &Factory{
		listStagedFactory:          listStagedFactory,
		listBranchFilesFactory:     listBranchFilesFactory,
		applyFiltersFactory:        applyFiltersFactory,
		fetchAllDiffsFactory:       fetchAllDiffsFactory,
		fetchAllBranchDiffsFactory: fetchAllBranchDiffsFactory,
		summarizeAllFactory:        summarizeAllFactory,
		composeCommitFactory:       composeCommitFactory,
		commitConfigService:        commitConfigService,
		commandService:             commandService,
		outputService:              outputService,
	}
}

// NewFlow creates and returns the commit composition flow function with added progress reporting.
func (f *Factory) NewFlow() executor.Flow {
	return func(ctx context.Context, flowCtx *executor.Context) error {
		flowCtx.SendRunning("Composing commit message")

		// Check if we're in squash mode (branch comparison)
		targetBranch, err := f.commandService.GetTargetBranchFlag()
		if err != nil {
			return fmt.Errorf("failed to get target-branch flag: %w", err)
		}

		var files []string

		// Phase 1: List files (staged or changed in branch)
		if targetBranch != "" {
			// Squash mode: list files changed in branch
			listBranchFiles := f.listBranchFilesFactory.NewActivity()
			branchFilesFuture := flowCtx.GetExecutor().RunActivity(ctx, flowCtx, "ListBranchFiles", listBranchFiles, &listbranchfiles.Input{
				TargetBranch: targetBranch,
			})
			branchFilesRaw, err := branchFilesFuture.Get(ctx)
			if err != nil {
				return fmt.Errorf("failed to list branch files: %w", err)
			}
			branchFiles, ok := branchFilesRaw.(*listbranchfiles.Output)
			if !ok {
				return fmt.Errorf("%w: %T", executor.ErrInvalidInputType, branchFilesRaw)
			}
			files = branchFiles.Files
		} else {
			// Normal mode: list staged files
			listStaged := f.listStagedFactory.NewActivity()
			stagedFilesFuture := flowCtx.GetExecutor().RunActivity(ctx, flowCtx, "ListStagedFiles", listStaged, &liststaged.Input{})
			stagedFilesRaw, err := stagedFilesFuture.Get(ctx)
			if err != nil {
				return fmt.Errorf("failed to list staged files: %w", err)
			}
			stagedFiles, ok := stagedFilesRaw.(*liststaged.Output)
			if !ok {
				return fmt.Errorf("%w: %T", executor.ErrInvalidInputType, stagedFilesRaw)
			}
			files = stagedFiles.Files
		}

		// Phase 2: Apply filters
		applyFilters := f.applyFiltersFactory.NewActivity()
		filteredFilesFuture := flowCtx.GetExecutor().RunActivity(ctx, flowCtx, "ApplyFilters", applyFilters, &applyfilters.Input{
			Files: files,
		})
		filteredFilesRaw, err := filteredFilesFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to apply filters: %w", err)
		}
		filteredFiles, ok := filteredFilesRaw.(*applyfilters.Output)
		if !ok {
			return fmt.Errorf("%w: %T", executor.ErrInvalidInputType, filteredFilesRaw)
		}

		// Phase 3: Fetch diffs for all files
		var changes []*git.FileChange

		if targetBranch != "" {
			// Squash mode: fetch branch diffs
			fetchAllBranchDiffs := f.fetchAllBranchDiffsFactory.NewActivity()
			fetchAllBranchDiffsFuture := flowCtx.GetExecutor().RunActivity(ctx, flowCtx, "FetchAllBranchDiffs", fetchAllBranchDiffs, &fetchallbranchdiffs.Input{
				Files:        filteredFiles.Files,
				TargetBranch: targetBranch,
			})
			fetchAllBranchDiffsRaw, err := fetchAllBranchDiffsFuture.Get(ctx)
			if err != nil {
				return fmt.Errorf("failed to fetch branch diffs: %w", err)
			}
			fetchAllBranchDiffsOutput, ok := fetchAllBranchDiffsRaw.(*fetchallbranchdiffs.Output)
			if !ok {
				return fmt.Errorf("%w: %T", executor.ErrInvalidInputType, fetchAllBranchDiffsRaw)
			}
			// Convert to generic slice
			changes = append(changes, fetchAllBranchDiffsOutput.Changes...)
		} else {
			// Normal mode: fetch staged diffs
			fetchAllDiffs := f.fetchAllDiffsFactory.NewActivity()
			fetchAllDiffsFuture := flowCtx.GetExecutor().RunActivity(ctx, flowCtx, "FetchAllDiffs", fetchAllDiffs, &fetchalldiffs.Input{
				Files: filteredFiles.Files,
			})
			fetchAllDiffsRaw, err := fetchAllDiffsFuture.Get(ctx)
			if err != nil {
				return fmt.Errorf("failed to fetch diffs: %w", err)
			}
			fetchAllDiffsOutput, ok := fetchAllDiffsRaw.(*fetchalldiffs.Output)
			if !ok {
				return fmt.Errorf("%w: %T", executor.ErrInvalidInputType, fetchAllDiffsRaw)
			}
			// Convert to generic slice
			changes = append(changes, fetchAllDiffsOutput.Changes...)
		}

		// Phase 4: Summarize changes for all files
		summarizeAll := f.summarizeAllFactory.NewActivity()
		summarizeAllFuture := flowCtx.GetExecutor().RunActivity(ctx, flowCtx, "SummarizeAll", summarizeAll, &summarizeall.Input{
			Changes: changes,
		})
		summarizeAllRaw, err := summarizeAllFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to summarize changes: %w", err)
		}
		summarizeAllOutput, ok := summarizeAllRaw.(*summarizeall.Output)
		if !ok {
			return fmt.Errorf("%w: %T", executor.ErrInvalidInputType, summarizeAllRaw)
		}

		// Phase 5: Compose commit message
		commitConfig, err := f.commitConfigService.GetCommitConfig()
		if err != nil {
			return fmt.Errorf("failed to resolve commit configuration: %w", err)
		}

		intent, err := f.commandService.GetIntentFlag()
		if err != nil {
			return fmt.Errorf("failed to get intent flag: %w", err)
		}

		if intent == "" {
			intent = f.commandService.GetStdIn()
		}

		composeCommit := f.composeCommitFactory.NewActivity()
		commitFuture := flowCtx.GetExecutor().RunActivity(ctx, flowCtx, "ComposeCommit", composeCommit, &composecommit.Input{
			Profile:      commitConfig.Profile,
			SystemPrompt: commitConfig.SystemPrompt,
			Summaries:    summarizeAllOutput.Summaries,
			Intent:       intent,
		})

		commitResultRaw, err := commitFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to compose commit message: %w", err)
		}

		commitResult, ok := commitResultRaw.(*composecommit.Output)
		if !ok {
			return fmt.Errorf("%w: %T", executor.ErrInvalidInputType, commitResultRaw)
		}

		flowCtx.SendCompleted("")

		f.outputService.PrintLine(commitResult.CommitMessage)

		return nil
	}
}
