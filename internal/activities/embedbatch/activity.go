// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package embedbatch implements an activity that computes embeddings for a batch of text chunks.
package embedbatch

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the payload for a single embedding batch.
type Input struct {
	ChunkTexts []string
}

// Output contains computed embeddings for the batch.
type Output struct {
	Embeddings []gateway.Embedding
}

// Factory builds embedbatch activities.
type Factory struct {
	embeddingGW ports.EmbeddingsGateway
	modelName   string
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a embedbatch activity factory.
func NewFactory(embeddingGW ports.EmbeddingsGateway, modelName string) (executor.ActivityFactory[*Input, *Output], error) {
	if embeddingGW == nil {
		return nil, fmt.Errorf("embedbatch.NewFactory: embeddingGW cannot be nil")
	}
	if modelName == "" {
		return nil, fmt.Errorf("embedbatch.NewFactory: modelName cannot be empty")
	}

	return &Factory{
		embeddingGW: embeddingGW,
		modelName:   modelName,
	}, nil
}

// NewActivity returns the activity implementation.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		executorCtx.SendRunningWithDetails(
			"I'm computing embeddings",
			fmt.Sprintf("chunks=%d model=%s", len(input.ChunkTexts), f.modelName),
		)

		if len(input.ChunkTexts) == 0 {
			executorCtx.SendCompletedWithDetails(
				"I've got no chunks to embed",
				fmt.Sprintf("chunks=0 model=%s", f.modelName),
			)
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

		executorCtx.SendCompletedWithDetails(
			fmt.Sprintf("I've computed %d embedding(s)", len(embeddings)),
			fmt.Sprintf("model=%s", f.modelName),
		)
		return &Output{Embeddings: embeddings}, nil
	}
}
