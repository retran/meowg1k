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
	"errors"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/services/git"
	"github.com/retran/meowg1k/pkg/executor"
)

// ErrStagedChangesReaderIsNil indicates that the stagedChangesReader is nil.
var ErrStagedChangesReaderIsNil = errors.New("stagedChangesReader is nil")

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

// Compile-time check to ensure Factory implements ActivityFactory interface
var _ executor.ActivityFactory[*Input, *git.FileChange] = (*Factory)(nil)

// NewFactory creates a new FetchFileDiff activity factory with the provided staged changes reader.
func NewFactory(stagedChangesReader StagedChangesReader) (*Factory, error) {
	if stagedChangesReader == nil {
		return nil, ErrStagedChangesReaderIsNil
	}
	return &Factory{
		stagedChangesReader: stagedChangesReader,
	}, nil
}

// NewActivity creates and returns the FetchFileDiff activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[*Input, *git.FileChange] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*git.FileChange, error) {
		if f == nil {
			return nil, errors.New("factory is nil")
		}
		if input == nil {
			return nil, executor.ErrInputCannotBeNil
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
