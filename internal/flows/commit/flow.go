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

package commit

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/readstagedchanges"
	"github.com/retran/meowg1k/internal/activities/readstagedfiles"
	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/future"
)

type Factory struct {
	readStagedFilesActivityFactory   executor.ActivityFactory
	readStagedChangesActivityFactory executor.ActivityFactory
}

func NewFactory(
	readStagedFilesActivityFactory executor.ActivityFactory,
	readStagedChangesActivityFactory executor.ActivityFactory,
) *Factory {
	return &Factory{
		readStagedFilesActivityFactory:   readStagedFilesActivityFactory,
		readStagedChangesActivityFactory: readStagedChangesActivityFactory,
	}
}

// NewFlow creates and returns the generate activity function with improved, multi-step status reporting.
func (f *Factory) NewFlow() executor.Flow {
	return func(ctx context.Context, flowCtx *executor.Context) error {

		flowCtx.SendStarted("Reading staged changes...")

		readStagedFiles := f.readStagedFilesActivityFactory.NewActivity()


		filesFuture := flowCtx.GetExecutor().RunActivity(ctx, flowCtx, "ReadStagedFiles", readStagedFiles, nil)
		files, err := filesFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to read staged files: %w", err)
		}

		futures := make([]*future.Future[any], 0)
		for _, file := range files.(*readstagedfiles.Output).Files {
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
