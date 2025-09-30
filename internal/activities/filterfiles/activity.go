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

// Package filterfiles provides an activity to filter files based on ignore patterns.
package filterfiles

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/services/filter"
	"github.com/retran/meowg1k/pkg/executor"
)

// Factory creates instances of the FilterFiles activity with injected dependencies.
type Factory interface {
	NewActivity() executor.Activity[any, any]
}

// factoryImpl is the concrete implementation of the Factory interface.
type factoryImpl struct {
	filterService filter.Service
}

// NewFactory creates a new instance of the FilterFiles activity factory with injected services.
func NewFactory(filterService filter.Service) Factory {
	return &factoryImpl{
		filterService: filterService,
	}
}

// NewActivity creates and returns the FilterFiles activity function with added progress reporting.
func (f *factoryImpl) NewActivity() executor.Activity[any, any] {
	return func(ctx context.Context, executorCtx *executor.Context, activityInput any) (any, error) {
		executorCtx.SendProgress(0.0, "Preparing to filter files...")

		if activityInput == nil {
			return nil, executor.ErrInputCannotBeNil
		}

		input, ok := activityInput.(*Input)
		if !ok {
			return nil, fmt.Errorf("%w: %T", executor.ErrInvalidInputType, input)
		}

		filteredFiles := make([]string, 0, len(input.Files))

		executorCtx.SendProgress(0.0, "Filtering files...")

		for _, file := range input.Files {
			if !f.filterService.IsIgnoredFile(file) {
				filteredFiles = append(filteredFiles, file)
			}
		}

		executorCtx.SendProgress(1.0, "Files filtered.")

		return &Output{
			Files: filteredFiles,
		}, nil
	}
}
