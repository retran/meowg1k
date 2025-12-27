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

		executorCtx.SendRunning(fmt.Sprintf("Fetching staged diff: %s", input.Filename))

		change, err := f.stagedChangesReader.ReadStagedChanges(input.Filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read staged changes in %s: %w", input.Filename, err)
		}

		renameFrom, renameTo := extractRename(change)
		originalFilename := resolveOriginalFilename(input.Filename, renameFrom, renameTo)

		originalFileContent, err := readOriginalContent(f.stagedChangesReader, originalFilename)
		if err != nil {
			return nil, err
		}

		stagedFileContent, deleted, err := readStagedContent(f.stagedChangesReader, input.Filename)
		if err != nil {
			return nil, err
		}
		if deleted {
			executorCtx.SendCompleted(fmt.Sprintf("Deleted: %s", input.Filename))
			return buildFileChange(input.Filename, change, originalFileContent, ""), nil
		}

		executorCtx.SendCompleted(fmt.Sprintf("Diff fetched: %s", input.Filename))

		return buildFileChange(input.Filename, change, originalFileContent, stagedFileContent), nil
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

func extractRename(diff string) (from string, to string) {
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

func resolveOriginalFilename(filename, renameFrom, renameTo string) string {
	if renameFrom != "" && renameTo != "" {
		return renameFrom
	}
	return filename
}

func readOriginalContent(reader StagedChangesReader, filename string) (string, error) {
	content, err := reader.ReadOriginalFileContent(filename)
	if err == nil {
		return content, nil
	}
	if isMissingOriginalContent(err) {
		return "", nil
	}
	return "", fmt.Errorf("failed to read original file content of %s: %w", filename, err)
}

func readStagedContent(reader StagedChangesReader, filename string) (content string, found bool, err error) {
	content, err = reader.ReadStagedFileContent(filename)
	if err == nil {
		return content, false, nil
	}
	if isMissingStagedContent(err) {
		return "", true, nil
	}
	return "", false, fmt.Errorf("failed to read staged file content of %s: %w", filename, err)
}

func buildFileChange(filename, change, originalContent, stagedContent string) *git.FileChange {
	return &git.FileChange{
		Filename:            filename,
		Change:              change,
		OriginalFileContent: originalContent,
		ChangedFileContent:  stagedContent,
	}
}
