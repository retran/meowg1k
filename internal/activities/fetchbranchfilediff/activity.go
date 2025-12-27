// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package fetchbranchfilediff implements an activity that fetches the diff of a file between current branch and target branch.
package fetchbranchfilediff

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input structure for the FetchBranchFileDiff activity.
type Input struct {
	Filename     string
	TargetBranch string
}

// BranchDiffReader reads file diffs between branches.
type BranchDiffReader interface {
	GetBranchDiff(filename, targetBranch string) (string, error)
	ReadOriginalFileContent(filename string) (string, error)
	ReadStagedFileContent(filename string) (string, error)
}

// Factory creates instances of the FetchBranchFileDiff activity with injected dependencies.
type Factory struct {
	branchDiffReader BranchDiffReader
}

// Compile-time check to ensure Factory implements ActivityFactory interface.
var _ executor.ActivityFactory[*Input, *git.FileChange] = (*Factory)(nil)

// NewFactory creates a new FetchBranchFileDiff activity factory with the provided branch diff reader.
func NewFactory(branchDiffReader BranchDiffReader) (*Factory, error) {
	if branchDiffReader == nil {
		return nil, fmt.Errorf("branch diff reader cannot be nil")
	}

	return &Factory{
		branchDiffReader: branchDiffReader,
	}, nil
}

// NewActivity creates and returns the FetchBranchFileDiff activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[*Input, *git.FileChange] {
	return func(_ context.Context, executorCtx *executor.Context, input *Input) (*git.FileChange, error) {
		if f == nil {
			return nil, fmt.Errorf("fetch branch file diff factory is nil")
		}

		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		executorCtx.SendRunning(fmt.Sprintf("Fetching diff vs %s: %s", input.TargetBranch, input.Filename))

		change, err := f.branchDiffReader.GetBranchDiff(input.Filename, input.TargetBranch)
		if err != nil {
			return nil, fmt.Errorf("failed to read branch diff in %s: %w", input.Filename, err)
		}

		originalFileContent, err := readBranchOriginalContent(f.branchDiffReader, input.Filename)
		if err != nil {
			return nil, err
		}

		stagedFileContent, deleted, err := readBranchStagedContent(f.branchDiffReader, input.Filename)
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

func readBranchOriginalContent(reader BranchDiffReader, filename string) (string, error) {
	content, err := reader.ReadOriginalFileContent(filename)
	if err == nil {
		return content, nil
	}
	if isMissingOriginalContent(err) {
		return "", nil
	}
	return "", fmt.Errorf("failed to read original file content of %s: %w", filename, err)
}

func readBranchStagedContent(reader BranchDiffReader, filename string) (content string, found bool, err error) {
	content, err = reader.ReadStagedFileContent(filename)
	if err == nil {
		return content, false, nil
	}
	if isMissingStagedContent(err) {
		return "", true, nil
	}
	return "", false, fmt.Errorf("failed to read current file content of %s: %w", filename, err)
}

func buildFileChange(filename, change, originalContent, stagedContent string) *git.FileChange {
	return &git.FileChange{
		Filename:            filename,
		Change:              change,
		OriginalFileContent: originalContent,
		ChangedFileContent:  stagedContent,
	}
}

func isMissingOriginalContent(err error) bool {
	return hasAnySubstring(err, []string{
		"does not exist",
		"not in 'HEAD'",
		"invalid object name 'HEAD'",
		"path not in the working tree",
	})
}

func isMissingStagedContent(err error) bool {
	return hasAnySubstring(err, []string{
		"does not exist",
		"path not in the working tree",
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
