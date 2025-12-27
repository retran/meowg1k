// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package fetchallbranchdiffs implements a parent activity that fetches branch diffs for multiple files sequentially.
package fetchallbranchdiffs

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/fetchbranchfilediff"
	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input structure for the FetchAllBranchDiffs activity.
type Input struct {
	TargetBranch string
	Files        []string
}

// Output defines the output structure for the FetchAllBranchDiffs activity.
type Output struct {
	Changes []*git.FileChange
}

// Factory creates instances of the FetchAllBranchDiffs activity with injected dependencies.
type Factory struct {
	branchFileDiffActivityFactory executor.ActivityFactory[*fetchbranchfilediff.Input, *git.FileChange]
}

// Compile-time check to ensure Factory implements ActivityFactory interface.
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new FetchAllBranchDiffs activity factory with the provided branch file diff activity factory.
func NewFactory(branchFileDiffActivityFactory executor.ActivityFactory[*fetchbranchfilediff.Input, *git.FileChange]) (*Factory, error) {
	if branchFileDiffActivityFactory == nil {
		return nil, fmt.Errorf("branch file diff activity factory cannot be nil")
	}

	return &Factory{
		branchFileDiffActivityFactory: branchFileDiffActivityFactory,
	}, nil
}

// NewActivity creates and returns the FetchAllBranchDiffs activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		if f == nil {
			return nil, fmt.Errorf("fetch all branch diffs factory is nil")
		}

		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		executorCtx.SendRunning(fmt.Sprintf("I'm getting %d diff(s) against %s", len(input.Files), input.TargetBranch))

		exec := executorCtx.GetExecutor()
		if exec == nil {
			return nil, fmt.Errorf("executor not available in context")
		}

		changes := make([]*git.FileChange, 0, len(input.Files))
		for _, file := range input.Files {
			fetchBranchFileDiff := f.branchFileDiffActivityFactory.NewActivity()
			change, err := executor.ExecuteActivity[*fetchbranchfilediff.Input, *git.FileChange](
				ctx,
				exec,
				executorCtx,
				file,
				fetchBranchFileDiff,
				&fetchbranchfilediff.Input{
					Filename:     file,
					TargetBranch: input.TargetBranch,
				},
			)
			if err != nil {
				return nil, fmt.Errorf("failed to read branch diffs: %w", err)
			}
			changes = append(changes, change)
		}

		executorCtx.SendCompleted(fmt.Sprintf("I got %d diff(s) against %s", len(changes), input.TargetBranch))

		return &Output{
			Changes: changes,
		}, nil
	}
}
