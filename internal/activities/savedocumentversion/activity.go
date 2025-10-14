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

// Package savedocumentversion provides an activity to save a document version with chunks and embeddings.
package savedocumentversion

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input contains the document data with chunks and embeddings.
type Input struct {
	FilePath    string
	Content     []byte
	ContentHash string
	Chunks      []domainindex.ChunkData
	Embeddings  []gateway.Embedding
}

// Output contains the created document version ID.
type Output struct {
	FilePath  string
	VersionID int64
}

// Factory creates instances of the SaveDocumentVersion activity with injected dependencies.
type Factory struct {
	indexRepo ports.IndexRepository
}

// Compile-time check to ensure Factory implements ActivityFactory interface
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new SaveDocumentVersion activity factory.
func NewFactory(indexRepo ports.IndexRepository) (executor.ActivityFactory[*Input, *Output], error) {
	if indexRepo == nil {
		return nil, fmt.Errorf("savedocumentversion.NewFactory: indexRepo cannot be nil")
	}

	return &Factory{
		indexRepo: indexRepo,
	}, nil
}

// NewActivity creates and returns the SaveDocumentVersion activity function.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		executorCtx.SendRunning(fmt.Sprintf("Saving document: %s", input.FilePath))

		// Validate input
		if len(input.Chunks) != len(input.Embeddings) {
			return nil, fmt.Errorf("chunk count (%d) does not match embedding count (%d)", len(input.Chunks), len(input.Embeddings))
		}

		// Check if version already exists
		existingVersion, err := f.indexRepo.FindVersionByContentHash(ctx, input.FilePath, input.ContentHash)
		if err != nil {
			return nil, fmt.Errorf("failed to find version for %s: %w", input.FilePath, err)
		}

		if existingVersion != nil {
			executorCtx.SendCompleted(fmt.Sprintf("Document already indexed: %s", input.FilePath))
			return &Output{
				FilePath:  input.FilePath,
				VersionID: existingVersion.ID,
			}, nil
		}

		// Create document version
		docVersion := domainindex.DocumentVersion{
			FilePath:               input.FilePath,
			GitCommitHashFirstSeen: sql.NullString{Valid: false}, // Will be set later if needed
			ContentHash:            input.ContentHash,
		}

		versionID, err := f.indexRepo.AddDocumentVersion(ctx, docVersion, input.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to add document version for %s: %w", input.FilePath, err)
		}

		// Create chunks with embeddings
		var chunksWithEmbeddings []domainindex.Chunk
		for i, chunkData := range input.Chunks {
			chunksWithEmbeddings = append(chunksWithEmbeddings, domainindex.Chunk{
				DocumentVersionID: versionID,
				ChunkType:         "text", // Default chunk type
				TextContent:       chunkData.TextContent,
				StartByte:         chunkData.StartByte,
				EndByte:           chunkData.EndByte,
				StartRune:         chunkData.StartRune,
				EndRune:           chunkData.EndRune,
				StartLine:         chunkData.StartLine,
				EndLine:           chunkData.EndLine,
				Embedding:         input.Embeddings[i],
			})
		}

		// Add chunks to repository
		if err := f.indexRepo.AddChunks(ctx, chunksWithEmbeddings); err != nil {
			return nil, fmt.Errorf("failed to add chunks for %s: %w", input.FilePath, err)
		}

		executorCtx.SendCompleted(fmt.Sprintf("Document saved: %s (%d chunks)", input.FilePath, len(input.Chunks)))
		return &Output{
			FilePath:  input.FilePath,
			VersionID: versionID,
		}, nil
	}
}
