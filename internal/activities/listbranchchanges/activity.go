// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package listbranchchanges implements an activity that lists files changed between current branch and target branch.
package listbranchchanges

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input structure for the ListBranchFiles activity.
type Input struct {
	TargetBranch string
}

// Output defines the output structure for the ListBranchFiles activity.
type Output struct {
	Files   []string
	Renames map[string]string // Maps new filename -> old filename
}

// BranchFileListReader reads list of changed files in a branch.
type BranchFileListReader interface {
	GetChangedFilesInBranch(targetBranch string) ([]string, error)
	GetChangedFilesInBranchWithRenames(targetBranch string) ([]string, map[string]string, error)
}

// Factory creates instances of the ListBranchFiles activity with injected dependencies.
type Factory struct {
	branchFileListReader BranchFileListReader
}

// Compile-time check to ensure Factory implements ActivityFactory interface.
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new ListBranchFiles activity factory with the provided branch file list reader.
func NewFactory(branchFileListReader BranchFileListReader) (*Factory, error) {
	if branchFileListReader == nil {
		return nil, fmt.Errorf("branch file list reader cannot be nil")
	}

	return &Factory{
		branchFileListReader: branchFileListReader,
	}, nil
}

// NewActivity creates and returns the ListBranchFiles activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(_ context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		if f == nil {
			return nil, fmt.Errorf("list branch files factory is nil")
		}

		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		if input.TargetBranch == "" {
			return nil, fmt.Errorf("target branch cannot be empty")
		}

		executorCtx.SendRunningWithDetails(
			"I'm listing changed files",
			fmt.Sprintf("base=%s", input.TargetBranch),
		)

		files, renames, err := f.branchFileListReader.GetChangedFilesInBranchWithRenames(input.TargetBranch)
		if err != nil {
			return nil, fmt.Errorf("failed to get changed files in branch: %w", err)
		}

		executorCtx.SendCompletedWithDetails(
			"I've found the changed files",
			fmt.Sprintf("count=%d base=%s", len(files), input.TargetBranch),
		)

		return &Output{
			Files:   files,
			Renames: renames,
		}, nil
	}
}
