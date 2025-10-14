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

package index

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
)

// Service implements IndexService.
type Service struct {
	indexRepo      ports.IndexRepository
	snapshotRepo   ports.SnapshotRepository
	chunkerService ports.ChunkerService
	embeddingGW    ports.EmbeddingsGateway
}

// NewService creates a new index service.
func NewService(
	indexRepo ports.IndexRepository,
	snapshotRepo ports.SnapshotRepository,
	chunkerService ports.ChunkerService,
	embeddingGW ports.EmbeddingsGateway,
) *Service {
	return &Service{
		indexRepo:      indexRepo,
		snapshotRepo:   snapshotRepo,
		chunkerService: chunkerService,
		embeddingGW:    embeddingGW,
	}
}

// EnsureVersionsExist ensures that document versions exist for the given files.
// Returns a map of file path to document version ID.
func (s *Service) EnsureVersionsExist(files map[string][]byte) (map[string]int64, error) {
	ctx := context.Background()
	result := make(map[string]int64, len(files))

	// Group files into existing and new
	var filesToCreate []string
	var filesToCreateContent [][]byte

	for filePath, content := range files {
		contentHash := computeContentHash(content)

		// Check if version already exists
		existingVersion, err := s.indexRepo.FindVersionByContentHash(ctx, filePath, contentHash)
		if err != nil {
			return nil, fmt.Errorf("failed to find version for %s: %w", filePath, err)
		}

		if existingVersion != nil {
			result[filePath] = existingVersion.ID
		} else {
			filesToCreate = append(filesToCreate, filePath)
			filesToCreateContent = append(filesToCreateContent, content)
		}
	}

	// If no new files, return early
	if len(filesToCreate) == 0 {
		return result, nil
	}

	// Process files that need to be created
	// Step 1: Chunk all files
	type fileChunks struct {
		filePath string
		content  []byte
		chunks   []domainindex.ChunkData
	}

	var allFileChunks []fileChunks
	var allChunkTexts []string

	for i, filePath := range filesToCreate {
		content := filesToCreateContent[i]

		chunks, err := s.chunkerService.Chunk(content, filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to chunk file %s: %w", filePath, err)
		}

		allFileChunks = append(allFileChunks, fileChunks{
			filePath: filePath,
			content:  content,
			chunks:   chunks,
		})

		// Collect all chunk texts for batch embedding
		for _, chunk := range chunks {
			allChunkTexts = append(allChunkTexts, chunk.TextContent)
		}
	}

	// Step 2: Compute embeddings for all chunks in a single batch
	// Note: Model and taskType should be configurable in production
	embeddingRequest := gateway.NewComputeEmbeddingsRequest(
		"text-embedding-3-small", // TODO: Make configurable
		allChunkTexts,
		gateway.RetrievalDocument,
	)

	embeddings, err := s.embeddingGW.ComputeEmbeddings(ctx, embeddingRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to compute embeddings: %w", err)
	}

	// Step 3: Create document versions and chunks
	embeddingIndex := 0
	for _, fc := range allFileChunks {
		contentHash := computeContentHash(fc.content)

		// Create document version
		docVersion := domainindex.DocumentVersion{
			FilePath:               fc.filePath,
			GitCommitHashFirstSeen: sql.NullString{Valid: false}, // Will be set later if needed
			ContentHash:            contentHash,
		}

		versionID, err := s.indexRepo.AddDocumentVersion(ctx, docVersion, fc.content)
		if err != nil {
			return nil, fmt.Errorf("failed to add document version for %s: %w", fc.filePath, err)
		}

		// Create chunks with embeddings
		var chunks []domainindex.Chunk
		for _, chunkData := range fc.chunks {
			if embeddingIndex >= len(embeddings) {
				return nil, fmt.Errorf("not enough embeddings generated")
			}

			chunks = append(chunks, domainindex.Chunk{
				DocumentVersionID: versionID,
				ChunkType:         "text", // Default chunk type
				TextContent:       chunkData.TextContent,
				StartByte:         chunkData.StartByte,
				EndByte:           chunkData.EndByte,
				StartRune:         chunkData.StartRune,
				EndRune:           chunkData.EndRune,
				StartLine:         chunkData.StartLine,
				EndLine:           chunkData.EndLine,
				Embedding:         embeddings[embeddingIndex],
			})

			embeddingIndex++
		}

		// Add chunks to repository
		if err := s.indexRepo.AddChunks(ctx, chunks); err != nil {
			return nil, fmt.Errorf("failed to add chunks for %s: %w", fc.filePath, err)
		}

		result[fc.filePath] = versionID
	}

	return result, nil
}

// BuildSnapshot creates or updates a snapshot with the given versions.
func (s *Service) BuildSnapshot(snapshotName string, versions map[string]int64) error {
	ctx := context.Background()

	// Clear existing snapshot links
	if err := s.snapshotRepo.ClearSnapshotLinks(ctx, snapshotName); err != nil {
		return fmt.Errorf("failed to clear snapshot links: %w", err)
	}

	// Link all versions to the snapshot
	for _, versionID := range versions {
		if err := s.snapshotRepo.LinkVersionToSnapshot(ctx, snapshotName, versionID); err != nil {
			return fmt.Errorf("failed to link version %d to snapshot: %w", versionID, err)
		}
	}

	return nil
}

// computeContentHash computes SHA-256 hash of content.
func computeContentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
