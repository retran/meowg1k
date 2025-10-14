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

// Package chunkfile provides an activity to chunk a single file.
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

// Input contains the file to chunk.
type Input struct {
	FilePath string
	Content  []byte
}

// Output contains the chunks and metadata.
type Output struct {
	FilePath    string
	Content     []byte
	ContentHash string
	Chunks      []domainindex.ChunkData
}

// Factory creates instances of the ChunkFile activity with injected dependencies.
type Factory struct {
	chunkerService ports.ChunkerService
}

// Compile-time check to ensure Factory implements ActivityFactory interface
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new ChunkFile activity factory.
func NewFactory(chunkerService ports.ChunkerService) (executor.ActivityFactory[*Input, *Output], error) {
	if chunkerService == nil {
		return nil, fmt.Errorf("chunkfile.NewFactory: chunkerService cannot be nil")
	}

	return &Factory{
		chunkerService: chunkerService,
	}, nil
}

// NewActivity creates and returns the ChunkFile activity function.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		executorCtx.SendRunning(fmt.Sprintf("Chunking file: %s", input.FilePath))

		// Compute content hash
		contentHash := computeContentHash(input.Content)

		// Chunk the file
		chunks, err := f.chunkerService.Chunk(input.Content, input.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to chunk file %s: %w", input.FilePath, err)
		}

		executorCtx.SendCompleted(fmt.Sprintf("File chunked: %s (%d chunks)", input.FilePath, len(chunks)))
		return &Output{
			FilePath:    input.FilePath,
			Content:     input.Content,
			ContentHash: contentHash,
			Chunks:      chunks,
		}, nil
	}
}

// computeContentHash computes SHA-256 hash of content.
func computeContentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
