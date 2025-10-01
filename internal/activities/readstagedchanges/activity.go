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

// Package readstagedchanges contains the activity to read staged changes from a git repository.
package readstagedchanges

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/services/git"
	"github.com/retran/meowg1k/pkg/executor"
)

// Factory creates instances of the generate activity with injected dependencies.
type Factory struct {
	gitService git.Service
}

// NewFactory creates a new generate activity factory with injected services.
func NewFactory(
	gitService git.Service,
) *Factory {
	return &Factory{
		gitService: gitService,
	}
}

// NewActivity creates and returns the generate activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[any, any] {
	return func(ctx context.Context, executorCtx *executor.Context, activityInput any) (any, error) {
		executorCtx.SendProgress(0.0, "Preparing to read staged changes...")

		if activityInput == nil {
			return nil, executor.ErrInputCannotBeNil
		}

		input, ok := activityInput.(*Input)
		if !ok {
			return nil, fmt.Errorf("%w: %T", executor.ErrInvalidInputType, input)
		}

		executorCtx.SendProgress(0.0, fmt.Sprintf("Reading staged changes in %s...", input.Filename))

		change, err := f.gitService.ReadStagedChanges(input.Filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read staged changes in %s: %w", input.Filename, err)
		}

		executorCtx.SendProgress(0.5, "Reading original and staged file contents...")

		originalFileContent, err := f.gitService.ReadOriginalFileContent(input.Filename)
		if err != nil {
			originalFileContent = "" // File might be new, so original content is empty
			// return nil, fmt.Errorf("failed to read original file content of %s: %w", input.Filename, err)
		}

		stagedFileContent, err := f.gitService.ReadStagedFileContent(input.Filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read staged file content of %s: %w", input.Filename, err)
		}

		executorCtx.SendCompleted(fmt.Sprintf("Successfully read staged changes in %s.", input.Filename))

		return &Output{
			Filename:            input.Filename,
			Change:              change,
			OriginalFileContent: originalFileContent,
			StagedFileContent:   stagedFileContent,
		}, nil
	}
}
