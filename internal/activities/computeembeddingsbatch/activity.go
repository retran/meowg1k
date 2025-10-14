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

// Package computeembeddingsbatch provides an activity to compute embeddings for a batch of chunks.
package computeembeddingsbatch

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

type Input struct {
	ChunkTexts []string
}

type Output struct {
	Embeddings []gateway.Embedding
}

type Factory struct {
	embeddingGW ports.EmbeddingsGateway
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

func NewFactory(embeddingGW ports.EmbeddingsGateway) (executor.ActivityFactory[*Input, *Output], error) {
	if embeddingGW == nil {
		return nil, fmt.Errorf("computeembeddingsbatch.NewFactory: embeddingGW cannot be nil")
	}

	return &Factory{
		embeddingGW: embeddingGW,
	}, nil
}

func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		executorCtx.SendRunning(fmt.Sprintf("Computing embeddings for %d chunks...", len(input.ChunkTexts)))

		if len(input.ChunkTexts) == 0 {
			executorCtx.SendCompleted("No chunks to process")
			return &Output{Embeddings: []gateway.Embedding{}}, nil
		}

		// Compute embeddings in a single batch
		embeddingRequest := gateway.NewComputeEmbeddingsRequest(
			"text-embedding-3-small", // TODO: Make configurable
			input.ChunkTexts,
			gateway.RetrievalDocument,
		)

		embeddings, err := f.embeddingGW.ComputeEmbeddings(ctx, embeddingRequest)
		if err != nil {
			return nil, fmt.Errorf("failed to compute embeddings: %w", err)
		}

		if len(embeddings) != len(input.ChunkTexts) {
			return nil, fmt.Errorf("embedding count mismatch: got %d, expected %d", len(embeddings), len(input.ChunkTexts))
		}

		executorCtx.SendCompleted(fmt.Sprintf("Computed %d embeddings", len(embeddings)))
		return &Output{Embeddings: embeddings}, nil
	}
}
