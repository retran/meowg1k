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

// Package summarizeall contains the parent activity to summarize changes for multiple files in parallel.
package summarizeall

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/summarizefile"
	"github.com/retran/meowg1k/internal/services/git"
	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/future"
)

// Input defines the input structure for the SummarizeAll activity.
type Input struct {
	Changes []*git.FileChange
}

// Output defines the output structure for the SummarizeAll activity.
type Output struct {
	Summaries []*summarizefile.Output
}

// Factory creates instances of the SummarizeAll activity with injected dependencies.
type Factory struct {
	summarizeFileFactory *summarizefile.Factory
}

// NewFactory creates a new SummarizeAll activity factory with injected services.
func NewFactory(
	summarizeFileFactory *summarizefile.Factory,
) *Factory {
	return &Factory{
		summarizeFileFactory: summarizeFileFactory,
	}
}

// NewActivity creates and returns the SummarizeAll activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[any, any] {
	return func(ctx context.Context, executorCtx *executor.Context, activityInput any) (any, error) {
		if activityInput == nil {
			return nil, executor.ErrInputCannotBeNil
		}

		input, ok := activityInput.(*Input)
		if !ok {
			return nil, fmt.Errorf("%w: %T", executor.ErrInvalidInputType, activityInput)
		}

		executorCtx.SendRunning(fmt.Sprintf("Summarizing %d files", len(input.Changes)))

		summarizeFutures := make([]*future.Future[any], 0, len(input.Changes))
		for _, change := range input.Changes {
			summarizeFile := f.summarizeFileFactory.NewActivity()
			future := executorCtx.GetExecutor().RunActivity(ctx, executorCtx, change.Filename, summarizeFile, &summarizefile.Input{
				Filename:            change.Filename,
				Change:              change.Change,
				OriginalFileContent: change.OriginalFileContent,
				StagedFileContent:   change.StagedFileContent,
			})
			summarizeFutures = append(summarizeFutures, future)
		}

		summaryResults, errs := future.WaitAll(ctx, summarizeFutures...)
		for _, err := range errs {
			if err != nil {
				return nil, fmt.Errorf("failed to summarize changes: %w", err)
			}
		}

		summaries := make([]*summarizefile.Output, 0, len(summaryResults))
		for _, result := range summaryResults {
			summary, ok := result.(*summarizefile.Output)
			if ok {
				summaries = append(summaries, summary)
			}
		}

		executorCtx.SendCompleted(fmt.Sprintf("Summarized %d files", len(summaries)))

		return &Output{
			Summaries: summaries,
		}, nil
	}
}
