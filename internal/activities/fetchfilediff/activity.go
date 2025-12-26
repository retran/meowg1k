// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package fetchfilediff implements an activity that fetches the staged diff for a single file.
package fetchfilediff

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/domain/git"
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

// Compile-time check to ensure Factory implements ActivityFactory interface.
var _ executor.ActivityFactory[*Input, *git.FileChange] = (*Factory)(nil)

// NewFactory creates a new FetchFileDiff activity factory with the provided staged changes reader.
func NewFactory(stagedChangesReader StagedChangesReader) (*Factory, error) {
	if stagedChangesReader == nil {
		return nil, fmt.Errorf("staged changes reader cannot be nil")
	}
	return &Factory{
		stagedChangesReader: stagedChangesReader,
	}, nil
}

// NewActivity creates and returns the FetchFileDiff activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[*Input, *git.FileChange] {
	return func(_ context.Context, executorCtx *executor.Context, input *Input) (*git.FileChange, error) {
		if f == nil {
			return nil, fmt.Errorf("fetch file diff factory is nil")
		}

		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		executorCtx.SendRunning("Fetching diff")

		change, err := f.stagedChangesReader.ReadStagedChanges(input.Filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read staged changes in %s: %w", input.Filename, err)
		}

		renameFrom, renameTo := extractRename(change)
		originalFilename := input.Filename
		if renameFrom != "" && renameTo != "" {
			originalFilename = renameFrom
		}

		originalFileContent, err := f.stagedChangesReader.ReadOriginalFileContent(originalFilename)
		if err != nil {
			if isMissingOriginalContent(err) {
				originalFileContent = "" // File is new or was deleted, or this is the initial commit
			} else {
				return nil, fmt.Errorf("failed to read original file content of %s: %w", input.Filename, err)
			}
		}

		stagedFileContent, err := f.stagedChangesReader.ReadStagedFileContent(input.Filename)
		if err != nil {
			if isMissingStagedContent(err) {
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

func isMissingOriginalContent(err error) bool {
	return hasAnySubstring(err, []string{
		"does not exist",
		"not in 'HEAD'",
		"invalid object name 'HEAD'",
	})
}

func isMissingStagedContent(err error) bool {
	return hasAnySubstring(err, []string{
		"does not exist",
		"unknown revision or path not in the working tree",
	})
}

func hasAnySubstring(err error, substrings []string) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	for _, substr := range substrings {
		if strings.Contains(message, substr) {
			return true
		}
	}
	return false
}

func extractRename(diff string) (string, string) {
	var from, to string

	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "rename from ") {
			from = strings.TrimPrefix(line, "rename from ")
		}
		if strings.HasPrefix(line, "rename to ") {
			to = strings.TrimPrefix(line, "rename to ")
		}
	}

	return from, to
}
