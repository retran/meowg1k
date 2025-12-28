// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package embedall implements a parent activity that computes embeddings for multiple batches sequentially.
package embedall

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/embedbatch"
	"github.com/retran/meowg1k/internal/activities/buildbatches"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the payload for computing embeddings across batches.
type Input struct {
	PreparedBatches *buildbatches.Output
	StateName       string
}

// Output contains the computed embeddings and batch metadata.
type Output struct {
	StateName       string
	PreparedBatches *buildbatches.Output
	Embeddings      []gateway.Embedding
}

// Factory builds embedall activities.
type Factory struct {
	computeBatchFactory executor.ActivityFactory[*embedbatch.Input, *embedbatch.Output]
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a embedall activity factory.
func NewFactory(
	computeBatchFactory executor.ActivityFactory[*embedbatch.Input, *embedbatch.Output],
) (executor.ActivityFactory[*Input, *Output], error) {
	if computeBatchFactory == nil {
		return nil, fmt.Errorf("embedall.NewFactory: computeBatchFactory cannot be nil")
	}

	return &Factory{
		computeBatchFactory: computeBatchFactory,
	}, nil
}

// NewActivity returns the activity implementation.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		numBatches := len(input.PreparedBatches.Batches)
		totalChunks := len(input.PreparedBatches.ChunkResults.AllChunkTexts)

		executorCtx.SendRunningWithDetails(
			"I'm computing embeddings",
			fmt.Sprintf("chunks=%d state=%s", totalChunks, input.StateName),
		)

		if numBatches == 0 {
			executorCtx.SendCompletedWithDetails(
				"I've got no embeddings to compute",
				fmt.Sprintf("chunks=0 state=%s", input.StateName),
			)
			return &Output{
				StateName:       input.StateName,
				PreparedBatches: input.PreparedBatches,
				Embeddings:      []gateway.Embedding{},
			}, nil
		}

		exec := executorCtx.GetExecutor()
		if exec == nil {
			return nil, fmt.Errorf("executor not available in context")
		}

		allEmbeddings := make([]gateway.Embedding, totalChunks)
		for i := 0; i < numBatches; i++ {
			batch := input.PreparedBatches.Batches[i]
			activity := f.computeBatchFactory.NewActivity()
			batchInput := &embedbatch.Input{
				ChunkTexts: batch.Texts,
			}
			result, err := executor.ExecuteActivity(ctx, exec, executorCtx,
				fmt.Sprintf("Batch_%d-%d", batch.StartIndex, batch.EndIndex),
				activity, batchInput)
			if err != nil {
				return nil, fmt.Errorf("failed to compute embeddings batch: %w", err)
			}

			copy(allEmbeddings[batch.StartIndex:batch.EndIndex], result.Embeddings)
		}

		if len(allEmbeddings) != totalChunks {
			return nil, fmt.Errorf("embedding count mismatch: got %d, expected %d", len(allEmbeddings), totalChunks)
		}

		executorCtx.SendCompletedWithDetails(
			fmt.Sprintf("I've computed %d embedding(s)", len(allEmbeddings)),
			fmt.Sprintf("state=%s", input.StateName),
		)
		return &Output{
			StateName:       input.StateName,
			PreparedBatches: input.PreparedBatches,
			Embeddings:      allEmbeddings,
		}, nil
	}
}
