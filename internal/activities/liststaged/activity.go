// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package liststaged implements an activity that lists all files currently staged in git.
package liststaged

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input structure for the ListStaged activity.
type Input struct{}

// Output defines the output structure for the ListStaged activity.
type Output struct {
	Files []string
}

// StagedFileListReader reads list of staged files from git.
type StagedFileListReader interface {
	ReadStagedFiles() ([]string, error)
}

// Factory creates instances of the ListStaged activity with injected dependencies.
type Factory struct {
	stagedFileListReader StagedFileListReader
}

// Compile-time check to ensure Factory implements ActivityFactory interface.
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new ListStaged activity factory with the provided staged file list reader.
func NewFactory(stagedFileListReader StagedFileListReader) (*Factory, error) {
	if stagedFileListReader == nil {
		return nil, fmt.Errorf("staged file list reader cannot be nil")
	}

	return &Factory{
		stagedFileListReader: stagedFileListReader,
	}, nil
}

// NewActivity creates and returns the ListStaged activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(_ context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		if f == nil {
			return nil, fmt.Errorf("list staged factory is nil")
		}

		executorCtx.SendRunning("Checking staged files")

		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		files, err := f.stagedFileListReader.ReadStagedFiles()
		if err != nil {
			return nil, fmt.Errorf("failed to read staged files: %w", err)
		}

		executorCtx.SendCompleted(fmt.Sprintf("Found %d staged file(s)", len(files)))

		return &Output{
			Files: files,
		}, nil
	}
}
