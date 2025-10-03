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

// Package fetchbranchfilediff contains the activity to fetch the diff for a file in branch compared to target branch.
package fetchbranchfilediff

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/services/git"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input structure for the FetchBranchFileDiff activity.
type Input struct {
	Filename     string
	TargetBranch string
}

// Factory creates instances of the FetchBranchFileDiff activity with injected dependencies.
type Factory struct {
	gitService git.Service
}

// NewFactory creates a new FetchBranchFileDiff activity factory with injected services.
func NewFactory(
	gitService git.Service,
) *Factory {
	return &Factory{
		gitService: gitService,
	}
}

// NewActivity creates and returns the FetchBranchFileDiff activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[any, any] {
	return func(ctx context.Context, executorCtx *executor.Context, activityInput any) (any, error) {
		if activityInput == nil {
			return nil, executor.ErrInputCannotBeNil
		}

		input, ok := activityInput.(*Input)
		if !ok {
			return nil, fmt.Errorf("%w: %T", executor.ErrInvalidInputType, activityInput)
		}

		executorCtx.SendRunning("Fetching branch diff")

		change, err := f.gitService.GetBranchDiff(input.Filename, input.TargetBranch)
		if err != nil {
			return nil, fmt.Errorf("failed to read branch diff in %s: %w", input.Filename, err)
		}

		// For branch diff, we get content from target branch (base) and current HEAD
		originalFileContent, err := f.gitService.ReadOriginalFileContent(input.Filename)
		if err != nil {
			if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "not in 'HEAD'") {
				originalFileContent = "" // File is new
			} else {
				return nil, fmt.Errorf("failed to read original file content of %s: %w", input.Filename, err)
			}
		}

		// For branch diff, "staged" content is actually current HEAD content
		stagedFileContent, err := f.gitService.ReadStagedFileContent(input.Filename)
		if err != nil {
			if strings.Contains(err.Error(), "does not exist") {
				// File was deleted - return with empty staged content
				executorCtx.SendCompleted("Deleted")
				return &git.FileChange{
					Filename:            input.Filename,
					Change:              change,
					OriginalFileContent: originalFileContent,
					ChangedFileContent:  "", // Empty for deleted files
				}, nil
			}
			return nil, fmt.Errorf("failed to read current file content of %s: %w", input.Filename, err)
		}

		executorCtx.SendCompleted("")

		return &git.FileChange{
			Filename:            input.Filename,
			Change:              change,
			OriginalFileContent: originalFileContent,
			ChangedFileContent:  stagedFileContent,
		}, nil
	}
}
