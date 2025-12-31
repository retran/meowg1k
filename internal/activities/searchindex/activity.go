// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package searchindex implements an activity that performs semantic search across indexed code using vector similarity.
package searchindex

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/core/retrieval"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input contains parameters for the searchindex activity.
type Input struct {
	QueryText        string   // The query text to search for
	SnapshotPriority []string // List of snapshots to search, in priority order (e.g., ["_workdir_", "_stage_", "_head_"])
	TopK             int      // Maximum number of results to return
	MinScore         float32  // Minimum similarity score threshold (0.0 to 1.0)
}

// Output contains the search results.
type Output struct {
	Results []retrieval.SearchResult
}

// Factory creates searchindex activities.
type Factory struct {
	retrievalService retrieval.Retriever
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new searchindex activity factory.
func NewFactory(retrievalService retrieval.Retriever) (executor.ActivityFactory[*Input, *Output], error) {
	if retrievalService == nil {
		return nil, fmt.Errorf("searchindex.NewFactory: retrievalService cannot be nil")
	}

	return &Factory{
		retrievalService: retrievalService,
	}, nil
}

// MustNewFactory creates a new searchindex activity factory or panics.
func MustNewFactory(retrievalService retrieval.Retriever) executor.ActivityFactory[*Input, *Output] {
	f, err := NewFactory(retrievalService)
	if err != nil {
		panic(err)
	}
	return f
}

// NewActivity creates a new searchindex activity instance.
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
			"I'm searching the index",
			fmt.Sprintf(
				"query=%q\nsnapshots=%d\ntop_k=%d\nmin_score=%.2f",
				input.QueryText,
				len(input.SnapshotPriority),
				input.TopK,
				input.MinScore,
			),
		)

		results, err := f.retrievalService.Search(
			ctx,
			input.QueryText,
			input.SnapshotPriority,
			input.TopK,
			input.MinScore,
		)
		if err != nil {
			return nil, fmt.Errorf("search failed: %w", err)
		}

		executorCtx.SendCompletedWithDetails(
			"I've found the results",
			fmt.Sprintf("results=%d", len(results)),
		)

		return &Output{
			Results: results,
		}, nil
	}
}
