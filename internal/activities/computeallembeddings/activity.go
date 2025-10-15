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

// Package computeallembeddings provides an activity to compute embeddings with controlled parallelism.
package computeallembeddings

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/computeembeddingsbatch"
	"github.com/retran/meowg1k/internal/activities/preparebatches"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/future"
)

type Input struct {
	StateName       string
	PreparedBatches *preparebatches.Output
}

type Output struct {
	StateName       string
	PreparedBatches *preparebatches.Output
	Embeddings      []gateway.Embedding
}

type Factory struct {
	computeBatchFactory executor.ActivityFactory[*computeembeddingsbatch.Input, *computeembeddingsbatch.Output]
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

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

func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		numBatches := len(input.PreparedBatches.Batches)
		totalChunks := len(input.PreparedBatches.ChunkResults.AllChunkTexts)

		executorCtx.SendRunning(fmt.Sprintf("Computing embeddings for %d chunks in %d batches (%s)...", totalChunks, numBatches, input.StateName))

		if numBatches == 0 {
			executorCtx.SendCompleted("No batches to process")
			return &Output{
				StateName:       input.StateName,
				PreparedBatches: input.PreparedBatches,
				Embeddings:      []gateway.Embedding{},
			}, nil
		}

		// Get executor from context
		exec := executorCtx.GetExecutor()
		if exec == nil {
			return nil, fmt.Errorf("executor not available in context")
		}

		// Launch all batches in parallel (rate limiter will control actual concurrency)
		allEmbeddings := make([]gateway.Embedding, totalChunks)
		futures := make([]*future.Future[*computeembeddingsbatch.Output], numBatches)

		for i := 0; i < numBatches; i++ {
			batch := input.PreparedBatches.Batches[i]
			activity := f.computeBatchFactory.NewActivity()
			batchInput := &computeembeddingsbatch.Input{
				ChunkTexts: batch.Texts,
			}
			fut := executor.ExecuteActivity(exec, ctx, executorCtx,
				fmt.Sprintf("Batch_%d-%d", batch.StartIndex, batch.EndIndex),
				activity, batchInput)
			futures[i] = fut
		}

		// Wait for all batches to complete
		for i, fut := range futures {
			result, err := fut.Get(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to compute embeddings batch: %w", err)
			}

			// Place embeddings at correct positions
			batch := input.PreparedBatches.Batches[i]
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
