// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package fetchalldiffs implements a parent activity that fetches staged diffs for multiple files in parallel.
package fetchalldiffs

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/fetchfilediff"
	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/future"
)

// Input defines the input structure for the FetchAllDiffs activity.
type Input struct {
	Files []string
}

// Output defines the output structure for the FetchAllDiffs activity.
type Output struct {
	Changes []*git.FileChange
}

// Factory creates instances of the FetchAllDiffs activity with injected dependencies.
type Factory struct {
	fileDiffActivityFactory executor.ActivityFactory[*fetchfilediff.Input, *git.FileChange]
}

// Compile-time check to ensure Factory implements ActivityFactory interface.
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new FetchAllDiffs activity factory with the provided file diff activity factory.
func NewFactory(fileDiffActivityFactory executor.ActivityFactory[*fetchfilediff.Input, *git.FileChange]) (*Factory, error) {
	if fileDiffActivityFactory == nil {
		return nil, fmt.Errorf("file diff activity factory cannot be nil")
	}

	return &Factory{
		fileDiffActivityFactory: fileDiffActivityFactory,
	}, nil
}

// NewActivity creates and returns the FetchAllDiffs activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		if f == nil {
			return nil, fmt.Errorf("fetch all diffs factory is nil")
		}

		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		executorCtx.SendRunning(fmt.Sprintf("Fetching diffs for %d files", len(input.Files)))

		exec := executorCtx.GetExecutor()
		if exec == nil {
			return nil, fmt.Errorf("executor not available in context")
		}

		readChangesFutures := make([]*future.Future[*git.FileChange], 0, len(input.Files))
		for _, file := range input.Files {
			fetchFileDiff := f.fileDiffActivityFactory.NewActivity()
			fut := executor.ExecuteActivity[*fetchfilediff.Input, *git.FileChange](
				ctx,
				exec,
				executorCtx,
				file,
				fetchFileDiff,
				&fetchfilediff.Input{
					Filename: file,
				},
			)
			readChangesFutures = append(readChangesFutures, fut)
		}

		changesResults, errs := future.WaitAll(ctx, readChangesFutures...)
		for _, err := range errs {
			if err != nil {
				return nil, fmt.Errorf("failed to read staged changes: %w", err)
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
