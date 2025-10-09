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
	"errors"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/fetchbranchfilediff"
	"github.com/retran/meowg1k/internal/services/git"
	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/future"
)

// ErrBranchFileDiffActivityFactoryIsNil indicates that the branchFileDiffActivityFactory is nil.
var ErrBranchFileDiffActivityFactoryIsNil = errors.New("branchFileDiffActivityFactory is nil")

// Input defines the input structure for the FetchAllBranchDiffs activity.
type Input struct {
	Files        []string
	TargetBranch string
}

// Output defines the output structure for the FetchAllBranchDiffs activity.
type Output struct {
	Changes []*git.FileChange
}

// BranchFileDiffActivityFactory creates activities that fetch branch file diffs.
type BranchFileDiffActivityFactory interface {
	NewActivity() executor.Activity[*fetchbranchfilediff.Input, *git.FileChange]
}

// Factory creates instances of the FetchAllBranchDiffs activity with injected dependencies.
type Factory struct {
	branchFileDiffActivityFactory BranchFileDiffActivityFactory
}

// Compile-time check to ensure Factory implements ActivityFactory interface
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new FetchAllBranchDiffs activity factory with the provided branch file diff activity factory.
func NewFactory(branchFileDiffActivityFactory BranchFileDiffActivityFactory) (*Factory, error) {
	if branchFileDiffActivityFactory == nil {
		return nil, ErrBranchFileDiffActivityFactoryIsNil
	}

	return &Factory{
		branchFileDiffActivityFactory: branchFileDiffActivityFactory,
	}, nil
}

// NewActivity creates and returns the FetchAllBranchDiffs activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		if f == nil {
			// TODO proper error
			return nil, errors.New("factory is nil")
		}

		if input == nil {
			return nil, executor.ErrInputCannotBeNil
		}

		executorCtx.SendRunning(fmt.Sprintf("Fetching branch diffs for %d files", len(input.Files)))

		readChangesFutures := make([]*future.Future[*git.FileChange], 0, len(input.Files))
		for _, file := range input.Files {
			fetchBranchFileDiff := f.branchFileDiffActivityFactory.NewActivity()
			fut := executor.RunActivity[*fetchbranchfilediff.Input, *git.FileChange](
				executorCtx.GetExecutor(),
				ctx,
				executorCtx,
				file,
				fetchBranchFileDiff,
				&fetchbranchfilediff.Input{
					Filename:     file,
					TargetBranch: input.TargetBranch,
				},
			)
			readChangesFutures = append(readChangesFutures, fut)
		}

		changesResults, errs := future.WaitAll(ctx, readChangesFutures...)
		for _, err := range errs {
			if err != nil {
				// TODO proper error joining
				return nil, fmt.Errorf("failed to read branch diffs: %w", err)
			}
		}

		changes := make([]*git.FileChange, 0, len(changesResults))
		changes = append(changes, changesResults...)

		executorCtx.SendCompleted(fmt.Sprintf("Fetched %d diffs", len(changes)))

		return &Output{
			Changes: changes,
		}, nil
	}
}
