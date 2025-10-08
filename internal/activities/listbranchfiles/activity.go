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

// Package listbranchfiles provides an activity to list changed files in a branch compared to target branch.
package listbranchfiles

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
	Files []string
}

// BranchFileListReader reads list of changed files in a branch.
type BranchFileListReader interface {
	GetChangedFilesInBranch(targetBranch string) ([]string, error)
}

// Factory creates instances of the ListBranchFiles activity with injected dependencies.
type Factory struct {
	branchFileListReader BranchFileListReader
}

// NewFactory creates a new ListBranchFiles activity factory with the provided branch file list reader.
func NewFactory(branchFileListReader BranchFileListReader) *Factory {
	return &Factory{
		branchFileListReader: branchFileListReader,
	}
}

// NewActivity creates and returns the ListBranchFiles activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[any, any] {
	return func(ctx context.Context, executorCtx *executor.Context, activityInput any) (any, error) {
		if activityInput == nil {
			return nil, executor.ErrInputCannotBeNil
		}

		input, ok := activityInput.(*Input)
		if !ok {
			return nil, fmt.Errorf("%w: %T", executor.ErrInvalidInputType, activityInput)
		}

		if input.TargetBranch == "" {
			return nil, fmt.Errorf("target branch cannot be empty")
		}

		executorCtx.SendRunning(fmt.Sprintf("Listing changed files compared to %s", input.TargetBranch))

		files, err := f.branchFileListReader.GetChangedFilesInBranch(input.TargetBranch)
		if err != nil {
			return nil, fmt.Errorf("failed to get changed files in branch: %w", err)
		}

		executorCtx.SendCompleted(fmt.Sprintf("%d files", len(files)))

		return &Output{
			Files: files,
		}, nil
	}
}
