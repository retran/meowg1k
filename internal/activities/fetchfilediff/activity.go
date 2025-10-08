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

// Package fetchfilediff contains the activity to fetch the diff for a staged file from a git repository.
package fetchfilediff

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/services/git"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input structure for the FetchFileDiff activity.
type Input struct {
	Filename string
}

// StagedChangesReader reads staged file changes from git.
type StagedChangesReader interface {
	ReadStagedChanges(filename string) (string, error)
	ReadOriginalFileContent(filename string) (string, error)
	ReadStagedFileContent(filename string) (string, error)
}

// Factory creates instances of the FetchFileDiff activity with injected dependencies.
type Factory struct {
	stagedChangesReader StagedChangesReader
}

// NewFactory creates a new FetchFileDiff activity factory with the provided staged changes reader.
func NewFactory(stagedChangesReader StagedChangesReader) *Factory {
	return &Factory{
		stagedChangesReader: stagedChangesReader,
	}
}

// NewActivity creates and returns the FetchFileDiff activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[any, any] {
	return func(ctx context.Context, executorCtx *executor.Context, activityInput any) (any, error) {
		if activityInput == nil {
			return nil, executor.ErrInputCannotBeNil
		}

		input, ok := activityInput.(*Input)
		if !ok {
			return nil, fmt.Errorf("%w: %T", executor.ErrInvalidInputType, activityInput)
		}

		executorCtx.SendRunning("Fetching diff")

		change, err := f.stagedChangesReader.ReadStagedChanges(input.Filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read staged changes in %s: %w", input.Filename, err)
		}

		originalFileContent, err := f.stagedChangesReader.ReadOriginalFileContent(input.Filename)
		if err != nil {
			if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "not in 'HEAD'") {
				originalFileContent = "" // File is new or was deleted
			} else {
				return nil, fmt.Errorf("failed to read original file content of %s: %w", input.Filename, err)
			}
		}

		stagedFileContent, err := f.stagedChangesReader.ReadStagedFileContent(input.Filename)
		if err != nil {
			if strings.Contains(err.Error(), "does not exist") {
				// File was deleted - return with empty staged content but include original content and diff
				executorCtx.SendCompleted("Deleted")
				return &git.FileChange{
					Filename:            input.Filename,
					Change:              change,
					OriginalFileContent: originalFileContent,
					ChangedFileContent:  "", // Empty for deleted files
				}, nil
			}
			return nil, fmt.Errorf("failed to read staged file content of %s: %w", input.Filename, err)
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
