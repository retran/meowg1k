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

// Package preparebatches implements an activity that groups chunks into batches for embedding computation.
package preparebatches

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/chunkallfiles"
	"github.com/retran/meowg1k/pkg/executor"
)

type Input struct {
	StateName    string
	ChunkResults *chunkallfiles.Output
	BatchSize    int // Maximum chunks per batch (0 = single batch)
}

type Batch struct {
	StartIndex int
	EndIndex   int
	Texts      []string
}

type Output struct {
	StateName    string
	ChunkResults *chunkallfiles.Output
	Batches      []Batch
}

type Factory struct{}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

func NewFactory() (executor.ActivityFactory[*Input, *Output], error) {
	return &Factory{}, nil
}

func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		totalChunks := len(input.ChunkResults.AllChunkTexts)

		executorCtx.SendRunning(fmt.Sprintf("Preparing batches: %d chunks (%s)", totalChunks, input.StateName))

		if totalChunks == 0 {
			executorCtx.SendCompleted("No chunks")
			return &Output{
				StateName:    input.StateName,
				ChunkResults: input.ChunkResults,
				Batches:      []Batch{},
			}, nil
		}

		batchSize := input.BatchSize
		if batchSize <= 0 {
			batchSize = totalChunks
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

		executorCtx.SendCompleted(fmt.Sprintf("Prepared %d batches (%s)", numBatches, input.StateName))

		return &Output{
			StateName:    input.StateName,
			ChunkResults: input.ChunkResults,
			Batches:      batches,
		}, nil
	}
}
