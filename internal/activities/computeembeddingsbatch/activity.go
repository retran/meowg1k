// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package computeembeddingsbatch implements an activity that computes embeddings for a batch of text chunks.
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
	modelName   string
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

func NewFactory(embeddingGW ports.EmbeddingsGateway, modelName string) (executor.ActivityFactory[*Input, *Output], error) {
	if embeddingGW == nil {
		return nil, fmt.Errorf("computeembeddingsbatch.NewFactory: embeddingGW cannot be nil")
	}
	if modelName == "" {
		return nil, fmt.Errorf("computeembeddingsbatch.NewFactory: modelName cannot be empty")
	}

	return &Factory{
		embeddingGW: embeddingGW,
		modelName:   modelName,
	}, nil
}

func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		executorCtx.SendRunning(fmt.Sprintf("Computing embeddings: %d chunks", len(input.ChunkTexts)))

		if len(input.ChunkTexts) == 0 {
			executorCtx.SendCompleted("No chunks")
			return &Output{Embeddings: []gateway.Embedding{}}, nil
		}

		embeddingRequest := gateway.NewComputeEmbeddingsRequest(
			f.modelName,
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
