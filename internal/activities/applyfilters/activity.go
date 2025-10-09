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

// Package applyfilters provides an activity to filter files based on ignore patterns.
package applyfilters

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/core/ports"
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

// Factory creates instances of the ApplyFilters activity with injected dependencies.
type Factory struct {
	fileIgnoreChecker ports.FileIgnoreChecker
}

// Compile-time check to ensure Factory implements ActivityFactory interface
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new ApplyFilters activity factory with the provided file ignore checker.
func NewFactory(fileIgnoreChecker ports.FileIgnoreChecker) (executor.ActivityFactory[*Input, *Output], error) {
	if fileIgnoreChecker == nil {
		return nil, fmt.Errorf("applyfilters.NewFactory: fileIgnoreChecker cannot be nil")
	}

	return &Factory{
		fileIgnoreChecker: fileIgnoreChecker,
	}, nil
}

// NewActivity creates and returns the ApplyFilters activity function.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		executorCtx.SendRunning("Applying filters")

		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		filteredFiles := make([]string, 0, len(input.Files))

		for _, file := range input.Files {
			if !f.fileIgnoreChecker.IsIgnoredFile(file) {
				filteredFiles = append(filteredFiles, file)
			}
		}

		executorCtx.SendCompleted(fmt.Sprintf("%d files", len(filteredFiles)))

		return &Output{
			Files: filteredFiles,
		}, nil
	}
}
