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

// Package retrieval provides high-level RAG (Retrieval-Augmented Generation) services.
package retrieval

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/retran/meowg1k/internal/core/vector"
	"github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
)

// SearchResult represents a complete search result with chunk metadata.
type SearchResult struct {
	ChunkID           int64
	Score             float32
	FilePath          string
	TextContent       string
	StartLine         int
	EndLine           int
	DocumentVersionID int64
	SnapshotName      string
}

// RetrievalService defines the interface for high-level RAG operations.
type RetrievalService interface {
	// RetrieveContext performs vector search across multiple snapshots and assembles context.
	// Returns a formatted context string suitable for LLM input.
	RetrieveContext(ctx context.Context, queryText string, snapshotPriority []string, topK int, minScore float32) (string, error)

	// Search performs vector search and returns detailed results.
	// This is useful when you need access to individual search results.
	Search(ctx context.Context, queryText string, snapshotPriority []string, topK int, minScore float32) ([]SearchResult, error)
}

// Service implements RetrievalService for RAG operations.
type Service struct {
	embeddingsGW      ports.EmbeddingsGateway
	vectorSearchSvc   vector.VectorSearchService
	indexRepo         ports.IndexRepository
	embeddingModel    string
	embeddingTaskType gateway.TaskType
}

// NewService creates a new retrieval service instance.
func NewService(
	embeddingsGW ports.EmbeddingsGateway,
	vectorSearchSvc vector.VectorSearchService,
	indexRepo ports.IndexRepository,
	embeddingModel string,
	embeddingTaskType gateway.TaskType,
) (*Service, error) {
	if embeddingsGW == nil {
		return nil, fmt.Errorf("retrieval.NewService: embeddingsGW cannot be nil")
	}
	if vectorSearchSvc == nil {
		return nil, fmt.Errorf("retrieval.NewService: vectorSearchSvc cannot be nil")
	}
	if indexRepo == nil {
		return nil, fmt.Errorf("retrieval.NewService: indexRepo cannot be nil")
	}
	if embeddingModel == "" {
		return nil, fmt.Errorf("retrieval.NewService: embeddingModel cannot be empty")
	}

	return &Service{
		embeddingsGW:      embeddingsGW,
		vectorSearchSvc:   vectorSearchSvc,
		indexRepo:         indexRepo,
		embeddingModel:    embeddingModel,
		embeddingTaskType: embeddingTaskType,
	}, nil
}

// Search performs vector search across snapshots and returns detailed results.
func (s *Service) Search(
	ctx context.Context,
	queryText string,
	snapshotPriority []string,
	topK int,
	minScore float32,
) ([]SearchResult, error) {
	if s == nil {
		return nil, fmt.Errorf("retrieval service is nil")
	}

	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if queryText == "" {
		return nil, fmt.Errorf("query text cannot be empty")
	}

	if len(snapshotPriority) == 0 {
		return nil, fmt.Errorf("snapshot priority list cannot be empty")
	}

	if topK <= 0 {
		return nil, fmt.Errorf("topK must be positive, got %d", topK)
	}

	// Step 1: Compute embedding for query text
	request := gateway.NewComputeEmbeddingsRequest(
		s.embeddingModel,
		[]string{queryText},
		s.embeddingTaskType,
	)

	embeddings, err := s.embeddingsGW.ComputeEmbeddings(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to compute query embedding: %w", err)
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned for query")
	}

	queryEmbedding := embeddings[0]

	// Step 2: Search across all snapshots
	allResults := make([]vector.QueryResult, 0)
	for _, snapshotName := range snapshotPriority {
		results, err := s.vectorSearchSvc.Search(ctx, snapshotName, queryEmbedding, topK)
		if err != nil {
			// Log error but continue with other snapshots
			continue
		}
		allResults = append(allResults, results...)
	}

	if len(allResults) == 0 {
		return []SearchResult{}, nil
	}

	// Step 3: Filter by minimum score and sort by score (descending)
	filteredResults := make([]vector.QueryResult, 0)
	for _, result := range allResults {
		if result.Score >= minScore {
			filteredResults = append(filteredResults, result)
		}
	}

	// Sort by score descending (higher scores first)
	sort.Slice(filteredResults, func(i, j int) bool {
		return filteredResults[i].Score > filteredResults[j].Score
	})

	// Step 4: Take top-K results
	if len(filteredResults) > topK {
		filteredResults = filteredResults[:topK]
	}

	// Step 5: Fetch chunk details from database
	chunkIDs := make([]int64, len(filteredResults))
	for i, result := range filteredResults {
		chunkIDs[i] = result.ChunkID
	}

	chunks, err := s.indexRepo.GetChunksByIDs(ctx, chunkIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chunks: %w", err)
	}

	// Create a map for quick lookup
	chunkMap := make(map[int64]domainindex.Chunk)
	for _, chunk := range chunks {
		chunkMap[chunk.ID] = chunk
	}

	// Get document versions to retrieve file paths
	versionIDs := make([]int64, 0, len(chunks))
	versionIDSet := make(map[int64]bool)
	for _, chunk := range chunks {
		if !versionIDSet[chunk.DocumentVersionID] {
			versionIDs = append(versionIDs, chunk.DocumentVersionID)
			versionIDSet[chunk.DocumentVersionID] = true
		}
	}

	versions, err := s.indexRepo.GetVersionsByIDs(ctx, versionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch document versions: %w", err)
	}

	// Create version map
	versionMap := make(map[int64]domainindex.DocumentVersion)
	for _, version := range versions {
		versionMap[version.ID] = version
	}

	// Step 6: Assemble final results
	searchResults := make([]SearchResult, 0, len(filteredResults))
	for _, queryResult := range filteredResults {
		chunk, ok := chunkMap[queryResult.ChunkID]
		if !ok {
			continue // Skip if chunk not found
		}

		version, ok := versionMap[chunk.DocumentVersionID]
		if !ok {
			continue // Skip if version not found
		}

		searchResults = append(searchResults, SearchResult{
			ChunkID:           chunk.ID,
			Score:             queryResult.Score,
			FilePath:          version.FilePath,
			TextContent:       chunk.TextContent,
			StartLine:         chunk.StartLine,
			EndLine:           chunk.EndLine,
			DocumentVersionID: chunk.DocumentVersionID,
			SnapshotName:      queryResult.SnapshotName,
		})
	}

	return searchResults, nil
}

// formatSnapshotName converts internal snapshot names to user-friendly descriptions.
func formatSnapshotName(snapshotName string) string {
	switch snapshotName {
	case "_workdir_":
		return "(uncommitted changes)"
	case "_stage_":
		return "(staged for commit)"
	case "_head_":
		return "(not changed since last commit)"
	default:
		return snapshotName
	}
}

// RetrieveContext performs search and assembles a formatted context string.
func (s *Service) RetrieveContext(
	ctx context.Context,
	queryText string,
	snapshotPriority []string,
	topK int,
	minScore float32,
) (string, error) {
	results, err := s.Search(ctx, queryText, snapshotPriority, topK, minScore)
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return "", nil
	}

	// Format context as a structured string
	var builder strings.Builder
	builder.WriteString("# Retrieved Context\n\n")

	for _, result := range results {
		builder.WriteString(fmt.Sprintf("%s (Lines %d-%d)\n", result.FilePath, result.StartLine, result.EndLine))
		builder.WriteString(fmt.Sprintf("%s\n\n", formatSnapshotName(result.SnapshotName)))
		builder.WriteString("```\n")
		builder.WriteString(result.TextContent)
		builder.WriteString("\n```\n\n")
	}

	return builder.String(), nil
}
