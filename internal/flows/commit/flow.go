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

// Package commit provides a flow to generate a commit message based on staged changes.
package commit

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/filterfiles"
	"github.com/retran/meowg1k/internal/activities/readstagedchanges"
	"github.com/retran/meowg1k/internal/activities/readstagedfiles"
	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/future"
)

// Factory creates instances of the commit flow with injected dependencies.
type Factory struct {
	readStagedFilesActivityFactory   executor.ActivityFactory
	filterFilesFactory               filterfiles.Factory
	readStagedChangesActivityFactory executor.ActivityFactory
}

// NewFactory creates a new commit flow factory with injected services.
func NewFactory(
	readStagedFilesActivityFactory executor.ActivityFactory,
	filterFilesFactory filterfiles.Factory,
	readStagedChangesActivityFactory executor.ActivityFactory,
) *Factory {
	return &Factory{
		readStagedFilesActivityFactory:   readStagedFilesActivityFactory,
		filterFilesFactory:               filterFilesFactory,
		readStagedChangesActivityFactory: readStagedChangesActivityFactory,
	}
}

// NewFlow creates and returns the commit generation flow function with added progress reporting.
func (f *Factory) NewFlow() executor.Flow {
	return func(ctx context.Context, flowCtx *executor.Context) error {
		flowCtx.SendStarted("Reading staged changes...")

		readStagedFiles := f.readStagedFilesActivityFactory.NewActivity()

		stagedFilesFuture := flowCtx.GetExecutor().RunActivity(ctx, flowCtx, "ReadStagedFiles", readStagedFiles, nil)
		stagedFilesRaw, err := stagedFilesFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to read staged files: %w", err)
		}
		stagedFiles, ok := stagedFilesRaw.(*readstagedfiles.Output)
		if !ok {
			return fmt.Errorf("%w: %T", executor.ErrInvalidInputType, stagedFilesRaw)
		}

		flowCtx.SendProgress(0.3, "Filtering files...")

		filterFiles := f.filterFilesFactory.NewActivity()
		filteredFilesFuture := flowCtx.GetExecutor().RunActivity(ctx, flowCtx, "FilterFiles", filterFiles, &filterfiles.Input{
			Files: stagedFiles.Files,
		})
		filteredFilesRaw, err := filteredFilesFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to filter files: %w", err)
		}
		filteredFiles, ok := filteredFilesRaw.(*filterfiles.Output)
		if !ok {
			return fmt.Errorf("%w: %T", executor.ErrInvalidInputType, filteredFilesRaw)
		}

		flowCtx.SendProgress(0.6, "Reading staged changes from filtered files...")

		futures := make([]*future.Future[any], 0)
		for _, file := range filteredFiles.Files {
			readStagedChanges := f.readStagedChangesActivityFactory.NewActivity()
			future := flowCtx.GetExecutor().RunActivity(ctx, flowCtx, "ReadStagedChanges", readStagedChanges, &readstagedchanges.Input{
				Filename: file,
			})
			futures = append(futures, future)
		}

		results, errs := future.WaitAll(ctx, futures...)
		for _, err := range errs {
			if err != nil {
				return fmt.Errorf("failed to read staged changes: %w", err)
			}
		}

		flowCtx.SendCompleted("Successfully read staged changes.")

		for _, result := range results {
			output, ok := result.(*readstagedchanges.Output)
			if ok {
				fmt.Println("File:", output.Filename)
				fmt.Println("Change:", output.Change)
				fmt.Println("-----")
			}
		}

		return nil
	}
}
