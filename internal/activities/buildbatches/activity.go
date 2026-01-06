// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package buildbatches implements an activity that groups chunks into batches for embedding computation.
package buildbatches

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/splitfiles"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the payload for preparing embedding batches.
type Input struct {
	ChunkResults *splitfiles.Output
	StateName    string
	BatchSize    int
}

// Batch represents a slice of chunk texts for embedding requests.
type Batch struct {
	Texts      []string
	StartIndex int
	EndIndex   int
}

// Output contains the prepared batches and original chunk results.
type Output struct {
	StateName    string
	ChunkResults *splitfiles.Output
	Batches      []Batch
}

// Factory builds buildbatches activities.
type Factory struct{}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a buildbatches activity factory.
func NewFactory() (executor.ActivityFactory[*Input, *Output], error) {
	return &Factory{}, nil
}

// NewActivity returns the activity implementation.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(_ context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		totalChunks := len(input.ChunkResults.AllChunkTexts)

		batchSize := input.BatchSize
		if batchSize <= 0 {
			batchSize = totalChunks
		}
		executorCtx.SendRunningWithDetails(
			"I'm batching chunks",
			fmt.Sprintf("count=%d state=%s", totalChunks, input.StateName),
		)

		if totalChunks == 0 {
			executorCtx.SendCompletedWithDetails(
				"I've got no chunks to batch",
				fmt.Sprintf("count=0 state=%s", input.StateName),
			)
			return &Output{
				StateName:    input.StateName,
				ChunkResults: input.ChunkResults,
				Batches:      []Batch{},
			}, nil
		}

		numBatches := (totalChunks + batchSize - 1) / batchSize

		batches := make([]Batch, 0, numBatches)
		for i := 0; i < totalChunks; i += batchSize {
			end := i + batchSize
			if end > totalChunks {
				end = totalChunks
			}

			batches = append(batches, Batch{
				StartIndex: i,
				EndIndex:   end,
				Texts:      input.ChunkResults.AllChunkTexts[i:end],
			})
		}

		executorCtx.SendCompletedWithDetails(
			fmt.Sprintf("I've prepared %d batch(es)", numBatches),
			fmt.Sprintf("batch_size=%d state=%s", batchSize, input.StateName),
		)

		return &Output{
			StateName:    input.StateName,
			ChunkResults: input.ChunkResults,
			Batches:      batches,
		}, nil
	}
}
