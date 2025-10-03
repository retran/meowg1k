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

// Package fetchallbranchdiffs contains the parent activity to fetch branch diffs for multiple files in parallel.
package fetchallbranchdiffs

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/fetchbranchfilediff"
	"github.com/retran/meowg1k/internal/services/git"
	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/future"
)

// Input defines the input structure for the FetchAllBranchDiffs activity.
type Input struct {
	Files        []string
	TargetBranch string
}

// Output defines the output structure for the FetchAllBranchDiffs activity.
type Output struct {
	Changes []*git.FileChange
}

// Factory creates instances of the FetchAllBranchDiffs activity with injected dependencies.
type Factory struct {
	fetchBranchFileDiffActivityFactory executor.ActivityFactory
}

// NewFactory creates a new FetchAllBranchDiffs activity factory with injected services.
func NewFactory(
	fetchBranchFileDiffActivityFactory executor.ActivityFactory,
) *Factory {
	return &Factory{
		fetchBranchFileDiffActivityFactory: fetchBranchFileDiffActivityFactory,
	}
}

// NewActivity creates and returns the FetchAllBranchDiffs activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[any, any] {
	return func(ctx context.Context, executorCtx *executor.Context, activityInput any) (any, error) {
		if activityInput == nil {
			return nil, executor.ErrInputCannotBeNil
		}

		input, ok := activityInput.(*Input)
		if !ok {
			return nil, fmt.Errorf("%w: %T", executor.ErrInvalidInputType, activityInput)
		}

		executorCtx.SendRunning(fmt.Sprintf("Fetching branch diffs for %d files", len(input.Files)))

		readChangesFutures := make([]*future.Future[any], 0, len(input.Files))
		for _, file := range input.Files {
			fetchBranchFileDiff := f.fetchBranchFileDiffActivityFactory.NewActivity()
			future := executorCtx.GetExecutor().RunActivity(ctx, executorCtx, file, fetchBranchFileDiff, &fetchbranchfilediff.Input{
				Filename:     file,
				TargetBranch: input.TargetBranch,
			})
			readChangesFutures = append(readChangesFutures, future)
		}

		changesResults, errs := future.WaitAll(ctx, readChangesFutures...)
		for _, err := range errs {
			if err != nil {
				return nil, fmt.Errorf("failed to read branch diffs: %w", err)
			}
		}

		changes := make([]*git.FileChange, 0, len(changesResults))
		for _, result := range changesResults {
			changeOutput, ok := result.(*git.FileChange)
			if !ok {
				continue
			}
			changes = append(changes, changeOutput)
		}

		executorCtx.SendCompleted(fmt.Sprintf("Fetched %d diffs", len(changes)))

		return &Output{
			Changes: changes,
		}, nil
	}
}
