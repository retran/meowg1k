// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package filterfiles implements an activity that filters files based on include/exclude patterns.
package filterfiles

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input structure for the ApplyFilters activity.
type Input struct {
	Files []string
}

// Output defines the output structure for the ApplyFilters activity.
type Output struct {
	Files []string
}

// FileIgnoreChecker checks if a file should be ignored based on filter rules.
type FileIgnoreChecker interface {
	IsIgnoredFile(file string) bool
}

// Factory creates instances of the ApplyFilters activity with injected dependencies.
type Factory struct {
	fileIgnoreChecker FileIgnoreChecker
}

// Compile-time check to ensure Factory implements ActivityFactory interface.
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new ApplyFilters activity factory with the provided file ignore checker.
func NewFactory(fileIgnoreChecker FileIgnoreChecker) (executor.ActivityFactory[*Input, *Output], error) {
	if fileIgnoreChecker == nil {
		return nil, fmt.Errorf("filterfiles.NewFactory: fileIgnoreChecker cannot be nil")
	}

	return &Factory{
		fileIgnoreChecker: fileIgnoreChecker,
	}, nil
}

// NewActivity creates and returns the ApplyFilters activity function.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(_ context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		totalFiles := len(input.Files)
		executorCtx.SendRunningWithDetails(
			"I'm filtering files",
			fmt.Sprintf("total=%d", totalFiles),
		)

		filteredFiles := make([]string, 0, len(input.Files))

		for _, file := range input.Files {
			if !f.fileIgnoreChecker.IsIgnoredFile(file) {
				filteredFiles = append(filteredFiles, file)
			}
		}

		executorCtx.SendCompletedWithDetails(
			"I've filtered the files",
			fmt.Sprintf("kept=%d total=%d", len(filteredFiles), totalFiles),
		)

		return &Output{
			Files: filteredFiles,
		}, nil
	}
}
