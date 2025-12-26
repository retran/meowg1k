// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package computeallembeddings implements a parent activity that computes embeddings for multiple batches sequentially.
package computeallembeddings

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/computeembeddingsbatch"
	"github.com/retran/meowg1k/internal/activities/preparebatches"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the payload for computing embeddings across batches.
type Input struct {
	PreparedBatches *preparebatches.Output
	StateName       string
}

// Output contains the computed embeddings and batch metadata.
type Output struct {
	StateName       string
	PreparedBatches *preparebatches.Output
	Embeddings      []gateway.Embedding
}

// Factory builds computeallembeddings activities.
type Factory struct {
	computeBatchFactory executor.ActivityFactory[*computeembeddingsbatch.Input, *computeembeddingsbatch.Output]
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a computeallembeddings activity factory.
func NewFactory(
	computeBatchFactory executor.ActivityFactory[*computeembeddingsbatch.Input, *computeembeddingsbatch.Output],
) (executor.ActivityFactory[*Input, *Output], error) {
	if computeBatchFactory == nil {
		return nil, fmt.Errorf("computeallembeddings.NewFactory: computeBatchFactory cannot be nil")
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

		executorCtx.SendRunning(fmt.Sprintf("Computing %d embeddings (%s)", totalChunks, input.StateName))

		if numBatches == 0 {
			executorCtx.SendCompleted(fmt.Sprintf("No batches for %s", input.StateName))
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
			batchInput := &computeembeddingsbatch.Input{
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

		executorCtx.SendCompleted(fmt.Sprintf("Computed %d embeddings for %s", len(allEmbeddings), input.StateName))
		return &Output{
			StateName:       input.StateName,
			PreparedBatches: input.PreparedBatches,
			Embeddings:      allEmbeddings,
		}, nil
	}
}
