// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package retrieval provides services for semantic search and retrieval of code chunks using vector similarity.
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
	FilePath          string
	TextContent       string
	SnapshotName      string
	ChunkID           int64
	StartLine         int
	EndLine           int
	DocumentVersionID int64
	Score             float32
}

// Retriever defines the interface for high-level RAG operations.
type Retriever interface {
	// RetrieveContext performs vector search across multiple snapshots and assembles context.
	// Returns a formatted context string suitable for LLM input.
	RetrieveContext(ctx context.Context, queryText string, snapshotPriority []string, topK int, minScore float32) (string, error)

	// Search performs vector search and returns detailed results.
	// This is useful when you need access to individual search results.
	Search(ctx context.Context, queryText string, snapshotPriority []string, topK int, minScore float32) ([]SearchResult, error)
}

// Service implements Retriever for RAG operations.
type Service struct {
	embeddingsGW      ports.EmbeddingsGateway
	vectorSearchSvc   vector.Searcher
	indexRepo         ports.IndexRepository
	embeddingModel    string
	embeddingTaskType gateway.TaskType
}

// NewService creates a new retrieval service instance.
func NewService(
	embeddingsGW ports.EmbeddingsGateway,
	vectorSearchSvc vector.Searcher,
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

	if err := validateSearchParams(ctx, queryText, snapshotPriority, topK); err != nil {
		return nil, err
	}

	queryEmbedding, err := s.computeQueryEmbedding(ctx, queryText)
	if err != nil {
		return nil, err
	}

	allResults := s.searchSnapshots(ctx, snapshotPriority, queryEmbedding, topK)
	if len(allResults) == 0 {
		return []SearchResult{}, nil
	}

	filteredResults := filterAndLimitResults(allResults, minScore, topK)

	chunks, err := s.fetchChunks(ctx, filteredResults)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chunks: %w", err)
	}

	versions, err := s.fetchVersions(ctx, chunks)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch document versions: %w", err)
	}

	return assembleSearchResults(filteredResults, chunks, versions), nil
}

func validateSearchParams(
	ctx context.Context,
	queryText string,
	snapshotPriority []string,
	topK int,
) error {
	if ctx == nil {
		return fmt.Errorf("context cannot be nil")
	}
	if queryText == "" {
		return fmt.Errorf("query text cannot be empty")
	}
	if len(snapshotPriority) == 0 {
		return fmt.Errorf("snapshot priority list cannot be empty")
	}
	if topK <= 0 {
		return fmt.Errorf("topK must be positive, got %d", topK)
	}
	return nil
}

func (s *Service) computeQueryEmbedding(ctx context.Context, queryText string) (gateway.Embedding, error) {
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

	return embeddings[0], nil
}

func (s *Service) searchSnapshots(
	ctx context.Context,
	snapshotPriority []string,
	queryEmbedding gateway.Embedding,
	topK int,
) []vector.QueryResult {
	allResults := make([]vector.QueryResult, 0)
	for _, snapshotName := range snapshotPriority {
		results, err := s.vectorSearchSvc.Search(ctx, snapshotName, queryEmbedding, topK)
		if err != nil {
			continue
		}
		allResults = append(allResults, results...)
	}
	return allResults
}

func filterAndLimitResults(
	results []vector.QueryResult,
	minScore float32,
	topK int,
) []vector.QueryResult {
	filtered := make([]vector.QueryResult, 0, len(results))
	for _, result := range results {
		if result.Score >= minScore {
			filtered = append(filtered, result)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Score > filtered[j].Score
	})

	if len(filtered) > topK {
		return filtered[:topK]
	}
	return filtered
}

func (s *Service) fetchChunks(ctx context.Context, results []vector.QueryResult) ([]domainindex.Chunk, error) {
	chunkIDs := make([]int64, len(results))
	for i, result := range results {
		chunkIDs[i] = result.ChunkID
	}

	chunks, err := s.indexRepo.GetChunksByIDs(ctx, chunkIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chunks: %w", err)
	}
	return chunks, nil
}

func (s *Service) fetchVersions(ctx context.Context, chunks []domainindex.Chunk) ([]domainindex.DocumentVersion, error) {
	versionIDs := collectVersionIDs(chunks)
	if len(versionIDs) == 0 {
		return []domainindex.DocumentVersion{}, nil
	}
	versions, err := s.indexRepo.GetVersionsByIDs(ctx, versionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch document versions: %w", err)
	}
	return versions, nil
}

func collectVersionIDs(chunks []domainindex.Chunk) []int64 {
	versionIDs := make([]int64, 0, len(chunks))
	versionIDSet := make(map[int64]bool)
	for _, chunk := range chunks {
		if !versionIDSet[chunk.DocumentVersionID] {
			versionIDs = append(versionIDs, chunk.DocumentVersionID)
			versionIDSet[chunk.DocumentVersionID] = true
		}
	}
	return versionIDs
}

func assembleSearchResults(
	results []vector.QueryResult,
	chunks []domainindex.Chunk,
	versions []domainindex.DocumentVersion,
) []SearchResult {
	chunkMap := make(map[int64]domainindex.Chunk, len(chunks))
	for _, chunk := range chunks {
		chunkMap[chunk.ID] = chunk
	}

	versionMap := make(map[int64]domainindex.DocumentVersion, len(versions))
	for _, version := range versions {
		versionMap[version.ID] = version
	}

	searchResults := make([]SearchResult, 0, len(results))
	for _, queryResult := range results {
		chunk, ok := chunkMap[queryResult.ChunkID]
		if !ok {
			continue
		}

		version, ok := versionMap[chunk.DocumentVersionID]
		if !ok {
			continue
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

	return searchResults
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
