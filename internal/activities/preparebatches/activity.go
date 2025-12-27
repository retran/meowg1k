// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package preparebatches implements an activity that groups chunks into batches for embedding computation.
package preparebatches

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/chunkallfiles"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the payload for preparing embedding batches.
type Input struct {
	ChunkResults *chunkallfiles.Output
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
	ChunkResults *chunkallfiles.Output
	Batches      []Batch
}

// Factory builds preparebatches activities.
type Factory struct{}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a preparebatches activity factory.
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
		executorCtx.SendRunning(fmt.Sprintf("I'm batching %d chunk(s) (%s)", totalChunks, input.StateName))

		if totalChunks == 0 {
			executorCtx.SendCompleted(fmt.Sprintf("No chunks to batch (%s)", input.StateName))
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

		executorCtx.SendCompleted(fmt.Sprintf("I prepared %d batch(es) of up to %d (%s)", numBatches, batchSize, input.StateName))

		return &Output{
			StateName:    input.StateName,
			ChunkResults: input.ChunkResults,
			Batches:      batches,
		}, nil
	}
}
