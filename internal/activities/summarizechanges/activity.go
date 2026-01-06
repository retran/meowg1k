// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package summarizechanges implements a parent activity that summarizes changes for multiple files sequentially.
package summarizechanges

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/summarizefilechanges"
	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input structure for the SummarizeAll activity.
type Input struct {
	Changes []*git.FileChange
}

// Output defines the output structure for the SummarizeAll activity.
type Output struct {
	Summaries []*summarizefilechanges.Output
}

// Factory creates instances of the SummarizeAll activity with injected dependencies.
type Factory struct {
	fileSummarizationActivityFactory executor.ActivityFactory[*summarizefilechanges.Input, *summarizefilechanges.Output]
}

// Compile-time check to ensure Factory implements ActivityFactory interface.
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new SummarizeAll activity factory with the provided file summarization activity factory.
func NewFactory(fileSummarizationActivityFactory executor.ActivityFactory[*summarizefilechanges.Input, *summarizefilechanges.Output]) (*Factory, error) {
	if fileSummarizationActivityFactory == nil {
		return nil, fmt.Errorf("file summarization activity factory cannot be nil")
	}

	return &Factory{
		fileSummarizationActivityFactory: fileSummarizationActivityFactory,
	}, nil
}

// NewActivity creates and returns the SummarizeAll activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		if f == nil {
			return nil, fmt.Errorf("summarize all factory is nil")
		}

		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		totalChanges := len(input.Changes)
		executorCtx.SendRunningWithDetails("I'm summarizing changes", fmt.Sprintf("files=%d", totalChanges))

		if totalChanges == 0 {
			executorCtx.SendCompletedWithDetails("I've got no changes to summarize", "files=0")
			return &Output{Summaries: []*summarizefilechanges.Output{}}, nil
		}

		exec := executorCtx.GetExecutor()
		if exec == nil {
			return nil, fmt.Errorf("executor not available in context")
		}

		summaries := make([]*summarizefilechanges.Output, 0, len(input.Changes))
		for _, change := range input.Changes {
			summarizeFile := f.fileSummarizationActivityFactory.NewActivity()
			summary, err := executor.ExecuteActivity[*summarizefilechanges.Input, *summarizefilechanges.Output](
				ctx,
				exec,
				executorCtx,
				change.Filename,
				summarizeFile,
				&summarizefilechanges.Input{
					Filename:            change.Filename,
					Change:              change.Change,
					OriginalFileContent: change.OriginalFileContent,
					StagedFileContent:   change.ChangedFileContent,
					RenamedFrom:         change.RenamedFrom,
				},
			)
			if err != nil {
				return nil, fmt.Errorf("failed to summarize changes: %w", err)
			}
			summaries = append(summaries, summary)
		}

		executorCtx.SendCompletedWithDetails("I've finished the change summaries", fmt.Sprintf("files=%d", len(summaries)))

		return &Output{
			Summaries: summaries,
		}, nil
	}
}
