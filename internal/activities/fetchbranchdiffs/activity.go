// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package fetchbranchdiffs implements a parent activity that fetches branch diffs for multiple files sequentially.
package fetchbranchdiffs

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/fetchbranchdiff"
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
	branchFileDiffActivityFactory executor.ActivityFactory[*fetchbranchdiff.Input, *git.FileChange]
}

// Compile-time check to ensure Factory implements ActivityFactory interface.
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new FetchAllBranchDiffs activity factory with the provided branch file diff activity factory.
func NewFactory(branchFileDiffActivityFactory executor.ActivityFactory[*fetchbranchdiff.Input, *git.FileChange]) (*Factory, error) {
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

		executorCtx.SendRunningWithDetails(
			"I'm fetching diffs",
			fmt.Sprintf("files=%d base=%s", len(input.Files), input.TargetBranch),
		)

		exec := executorCtx.GetExecutor()
		if exec == nil {
			return nil, fmt.Errorf("executor not available in context")
		}

		changes := make([]*git.FileChange, 0, len(input.Files))
		for _, file := range input.Files {
			fetchBranchFileDiff := f.branchFileDiffActivityFactory.NewActivity()
			change, err := executor.ExecuteActivity[*fetchbranchdiff.Input, *git.FileChange](
				ctx,
				exec,
				executorCtx,
				file,
				fetchBranchFileDiff,
				&fetchbranchdiff.Input{
					Filename:     file,
					TargetBranch: input.TargetBranch,
				},
			)
			if err != nil {
				return nil, fmt.Errorf("failed to read branch diffs: %w", err)
			}
			changes = append(changes, change)
		}

		executorCtx.SendCompletedWithDetails(
			"I've fetched the diffs",
			fmt.Sprintf("diffs=%d base=%s", len(changes), input.TargetBranch),
		)

		return &Output{
			Changes: changes,
		}, nil
	}
}
