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

// Package liststaged provides an activity to list staged files from a git repository.
package liststaged

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/services/git"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input structure for the ListStaged activity.
type Input struct{}

// Output defines the output structure for the ListStaged activity.
type Output struct {
	Files []string
}

// Factory creates instances of the ListStaged activity with injected dependencies.
type Factory struct {
	gitService git.Service
}

// NewFactory creates a new ListStaged activity factory with injected services.
func NewFactory(
	gitService git.Service,
) *Factory {
	return &Factory{
		gitService: gitService,
	}
}

// NewActivity creates and returns the ListStaged activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[any, any] {
	return func(ctx context.Context, executorCtx *executor.Context, activityInput any) (any, error) {
		executorCtx.SendRunning("Listing staged files")

		files, err := f.gitService.ReadStagedFiles()
		if err != nil {
			return nil, fmt.Errorf("failed to read staged files: %w", err)
		}

		executorCtx.SendCompleted(fmt.Sprintf("%d files", len(files)))

		return &Output{
			Files: files,
		}, nil
	}
}
