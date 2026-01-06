// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package fetchcontext implements an activity that retrieves and formats context for RAG queries.
package fetchcontext

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/core/retrieval"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input contains parameters for the retrieve context activity.
type Input struct {
	QueryText        string   // The query text to search for
	SnapshotPriority []string // List of snapshots to search, in priority order (e.g., ["_workdir_", "_stage_", "_head_"])
	TopK             int      // Maximum number of results to return
	MinScore         float32  // Minimum similarity score threshold (0.0 to 1.0)
}

// Output contains the formatted context string.
type Output struct {
	Context string // Formatted context string ready for LLM consumption
}

// Factory creates retrieve context activities.
type Factory struct {
	retrievalService retrieval.Retriever
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new retrieve context activity factory.
func NewFactory(retrievalService retrieval.Retriever) (executor.ActivityFactory[*Input, *Output], error) {
	if retrievalService == nil {
		return nil, fmt.Errorf("fetchcontext.NewFactory: retrievalService cannot be nil")
	}

	return &Factory{
		retrievalService: retrievalService,
	}, nil
}

// NewActivity creates a new retrieve context activity instance.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		if input.QueryText == "" {
			return nil, fmt.Errorf("query text cannot be empty")
		}

		if len(input.SnapshotPriority) == 0 {
			return nil, fmt.Errorf("snapshot priority list cannot be empty")
		}

		if input.TopK <= 0 {
			return nil, fmt.Errorf("topK must be positive, got %d", input.TopK)
		}

		executorCtx.SendRunningWithDetails(
			"I'm retrieving context",
			fmt.Sprintf(
				"query=%q\nsnapshots=%d\ntop_k=%d\nmin_score=%.2f",
				input.QueryText,
				len(input.SnapshotPriority),
				input.TopK,
				input.MinScore,
			),
		)

		retrievedContext, err := f.retrievalService.RetrieveContext(
			ctx,
			input.QueryText,
			input.SnapshotPriority,
			input.TopK,
			input.MinScore,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve context: %w", err)
		}

		if retrievedContext == "" {
			executorCtx.SendCompletedWithDetails(
				"I couldn't find useful context",
				fmt.Sprintf("top_k=%d results=0", input.TopK),
			)
		} else {
			executorCtx.SendCompletedWithDetails(
				"I've gathered relevant context",
				fmt.Sprintf("top_k=%d", input.TopK),
			)
		}

		return &Output{
			Context: retrievedContext,
		}, nil
	}
}
