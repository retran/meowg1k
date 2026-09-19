// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package vector

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"math"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

// QueryResult represents a single search result from vector search.
type QueryResult struct {
	SnapshotName string
	ChunkID      int64
	Score        float32
}

// Searcher defines the interface for low-level vector search operations.
type Searcher interface {
	// Search performs k-NN search in the vector index for a given snapshot.
	// Returns top-K results sorted by similarity score (higher is better).
	Search(ctx context.Context, snapshotName string, queryEmbedding gateway.Embedding, topK int) ([]QueryResult, error)
}

// SearchService implements Searcher using HNSW indices stored in meta repository.
type SearchService struct {
	metaRepo ports.MetaRepository
}

// NewSearchService creates a new SearchService instance.
func NewSearchService(metaRepo ports.MetaRepository) (*SearchService, error) {
	if metaRepo == nil {
		return nil, fmt.Errorf("vector.NewSearchService: metaRepo cannot be nil")
	}

	return &SearchService{
		metaRepo: metaRepo,
	}, nil
}

// Search performs vector search in the specified snapshot's HNSW index.
func (s *SearchService) Search(
	ctx context.Context,
	snapshotName string,
	queryEmbedding gateway.Embedding,
	topK int,
) ([]QueryResult, error) {
	if s == nil {
		return nil, fmt.Errorf("search service is nil")
	}

	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if snapshotName == "" {
		return nil, fmt.Errorf("snapshot name cannot be empty")
	}

	if len(queryEmbedding) == 0 {
		return nil, fmt.Errorf("searchindex embedding cannot be empty")
	}

	if topK <= 0 {
		return nil, fmt.Errorf("topK must be positive, got %d", topK)
	}

	key := fmt.Sprintf("idx_dump_%s", snapshotName)
	dumpBytes, err := s.metaRepo.GetValue(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get index dump for snapshot %q: %w", snapshotName, err)
	}

	if dumpBytes == nil {
		return nil, fmt.Errorf("no index found for snapshot %q", snapshotName)
	}

	var dump IndexDump
	dumpBuffer := bytes.NewReader(dumpBytes)
	dumpDecoder := gob.NewDecoder(dumpBuffer)
	if err := dumpDecoder.Decode(&dump); err != nil {
		return nil, fmt.Errorf("failed to decode index dump for snapshot %q: %w", snapshotName, err)
	}

	hnswBuffer := bytes.NewReader(dump.HNSWData)

	hnswIndex := NewGraph[int64]()
	if err := hnswIndex.Import(hnswBuffer); err != nil {
		return nil, fmt.Errorf("failed to import HNSW graph for snapshot %q: %w", snapshotName, err)
	}

	queryVec := make([]float32, len(queryEmbedding))
	for i, val := range queryEmbedding {
		queryVec[i] = float32(val)
	}

	searchResults := hnswIndex.Search(queryVec, topK)

	results := make([]QueryResult, 0, len(searchResults))
	for _, node := range searchResults {
		similarity := cosineSimilarity(queryVec, node.Value)

		results = append(results, QueryResult{
			ChunkID:      node.Key,
			Score:        similarity,
			SnapshotName: snapshotName,
		})
	}

	return results, nil
}

// cosineSimilarity computes the cosine similarity between two vectors.
// Returns a value in [-1, 1] where 1 is identical, 0 is orthogonal, and -1 is opposite.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	similarity := dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))

	return similarity
}
