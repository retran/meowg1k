// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package summarizeall implements a parent activity that summarizes changes for multiple files sequentially.
package summarizeall

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/summarizefile"
	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/pkg/executor"
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
	fileSummarizationActivityFactory executor.ActivityFactory[*summarizefile.Input, *summarizefile.Output]
}

// Compile-time check to ensure Factory implements ActivityFactory interface.
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new SummarizeAll activity factory with the provided file summarization activity factory.
func NewFactory(fileSummarizationActivityFactory executor.ActivityFactory[*summarizefile.Input, *summarizefile.Output]) (*Factory, error) {
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
		executorCtx.SendRunning(fmt.Sprintf("Summarizing %d files", totalChanges))

		if totalChanges == 0 {
			executorCtx.SendCompleted("No files to summarize")
			return &Output{Summaries: []*summarizefile.Output{}}, nil
		}

		exec := executorCtx.GetExecutor()
		if exec == nil {
			return nil, fmt.Errorf("executor not available in context")
		}

		summaries := make([]*summarizefile.Output, 0, len(input.Changes))
		for _, change := range input.Changes {
			summarizeFile := f.fileSummarizationActivityFactory.NewActivity()
			summary, err := executor.ExecuteActivity[*summarizefile.Input, *summarizefile.Output](
				ctx,
				exec,
				executorCtx,
				change.Filename,
				summarizeFile,
				&summarizefile.Input{
					Filename:            change.Filename,
					Change:              change.Change,
					OriginalFileContent: change.OriginalFileContent,
					StagedFileContent:   change.ChangedFileContent,
				},
			)
			if err != nil {
				return nil, fmt.Errorf("failed to summarize changes: %w", err)
			}
			summaries = append(summaries, summary)
		}

		executorCtx.SendCompleted(fmt.Sprintf("%d summaries", len(summaries)))

		return &Output{
			Summaries: summaries,
		}, nil
	}
}
