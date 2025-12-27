// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package chunkfile implements an activity that chunks a single file into smaller pieces for embedding.
package chunkfile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the request payload for chunking a file.
type Input struct {
	FilePath string
	Content  []byte
}

// Output contains the chunked data for a file.
type Output struct {
	FilePath    string
	Content     []byte
	ContentHash string
	Chunks      []domainindex.ChunkData
}

// Factory builds chunkfile activities.
type Factory struct {
	chunkerService ports.ChunkerService
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a chunkfile activity factory.
func NewFactory(chunkerService ports.ChunkerService) (executor.ActivityFactory[*Input, *Output], error) {
	if chunkerService == nil {
		return nil, fmt.Errorf("chunkfile.NewFactory: chunkerService cannot be nil")
	}

	return &Factory{
		chunkerService: chunkerService,
	}, nil
}

// NewActivity returns the activity implementation.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(_ context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		executorCtx.SendRunning(fmt.Sprintf("Chunking: %s", input.FilePath))

		contentHash := computeContentHash(input.Content)

		chunks, err := f.chunkerService.Chunk(input.Content, input.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to chunk file %s: %w", input.FilePath, err)
		}

		executorCtx.SendCompleted(fmt.Sprintf("Chunked %s: %d chunks", input.FilePath, len(chunks)))
		return &Output{
			FilePath:    input.FilePath,
			Content:     input.Content,
			ContentHash: contentHash,
			Chunks:      chunks,
		}, nil
	}
}

func computeContentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
