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

// Package computeallembeddings provides an activity to compute embeddings with batching.
package computeallembeddings

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/chunkallfiles"
	"github.com/retran/meowg1k/internal/activities/computeembeddingsbatch"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/future"
)

type Input struct {
	StateName    string
	ChunkResults *chunkallfiles.Output
	BatchSize    int // Maximum chunks per batch (0 = single batch)
}

type Output struct {
	StateName    string
	ChunkResults *chunkallfiles.Output
	Embeddings   []gateway.Embedding
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
		executorCtx.SendRunning(fmt.Sprintf("Computing embeddings for %d chunks (%s)...", len(input.ChunkResults.AllChunkTexts), input.StateName))

		if len(input.ChunkResults.AllChunkTexts) == 0 {
			executorCtx.SendCompleted("No chunks to process")
			return &Output{
				StateName:    input.StateName,
				ChunkResults: input.ChunkResults,
				Embeddings:   []gateway.Embedding{},
			}, nil
		}

		// Get executor from context
		exec := executorCtx.GetExecutor()
		if exec == nil {
			return nil, fmt.Errorf("executor not available in context")
		}

		// Determine batch size (0 or negative means single batch)
		batchSize := input.BatchSize
		if batchSize <= 0 {
			batchSize = len(input.ChunkResults.AllChunkTexts)
		}

		// Split into batches and launch parallel embedding activities
		var batchFutures []*future.Future[*computeembeddingsbatch.Output]
		for i := 0; i < len(input.ChunkResults.AllChunkTexts); i += batchSize {
			end := i + batchSize
			if end > len(input.ChunkResults.AllChunkTexts) {
				end = len(input.ChunkResults.AllChunkTexts)
			}

			batchTexts := input.ChunkResults.AllChunkTexts[i:end]
			activity := f.computeBatchFactory.NewActivity()
			batchInput := &computeembeddingsbatch.Input{
				ChunkTexts: batchTexts,
			}
			fut := executor.ExecuteActivity(exec, ctx, executorCtx, fmt.Sprintf("ComputeBatch_%d-%d", i, end), activity, batchInput)
			batchFutures = append(batchFutures, fut)
		}

		// Collect all embeddings in order
		var allEmbeddings []gateway.Embedding
		for _, fut := range batchFutures {
			result, err := fut.Get(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to compute embeddings batch: %w", err)
			}
			allEmbeddings = append(allEmbeddings, result.Embeddings...)
		}

		if len(allEmbeddings) != len(input.ChunkResults.AllChunkTexts) {
			return nil, fmt.Errorf("embedding count mismatch: got %d, expected %d", len(allEmbeddings), len(input.ChunkResults.AllChunkTexts))
		}

		executorCtx.SendCompleted(fmt.Sprintf("Computed %d embeddings for %s", len(allEmbeddings), input.StateName))
		return &Output{
			StateName:    input.StateName,
			ChunkResults: input.ChunkResults,
			Embeddings:   allEmbeddings,
		}, nil
	}
}
