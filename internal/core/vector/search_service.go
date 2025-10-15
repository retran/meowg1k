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

package vector

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"math"

	"github.com/coder/hnsw"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

// QueryResult represents a single search result from vector search.
type QueryResult struct {
	ChunkID int64
	Score   float32
}

// VectorSearchService defines the interface for low-level vector search operations.
type VectorSearchService interface {
	// Search performs k-NN search in the vector index for a given snapshot.
	// Returns top-K results sorted by similarity score (higher is better).
	Search(ctx context.Context, snapshotName string, queryEmbedding gateway.Embedding, topK int) ([]QueryResult, error)
}

// SearchService implements VectorSearchService using HNSW indices stored in meta repository.
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
		return nil, fmt.Errorf("query embedding cannot be empty")
	}

	if topK <= 0 {
		return nil, fmt.Errorf("topK must be positive, got %d", topK)
	}

	// Step 1: Load index dump from meta repository
	key := fmt.Sprintf("idx_dump_%s", snapshotName)
	dumpBytes, err := s.metaRepo.GetValue(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get index dump for snapshot %q: %w", snapshotName, err)
	}

	if dumpBytes == nil {
		return nil, fmt.Errorf("no index found for snapshot %q", snapshotName)
	}

	// Step 2: Deserialize the dump
	var dump IndexDump
	dumpBuffer := bytes.NewReader(dumpBytes)
	dumpDecoder := gob.NewDecoder(dumpBuffer)
	if err := dumpDecoder.Decode(&dump); err != nil {
		return nil, fmt.Errorf("failed to decode index dump for snapshot %q: %w", snapshotName, err)
	}

	// Step 3: Deserialize HNSW index using Import
	hnswBuffer := bytes.NewReader(dump.HNSWData)

	hnswIndex := hnsw.NewGraph[int64]()
	if err := hnswIndex.Import(hnswBuffer); err != nil {
		return nil, fmt.Errorf("failed to import HNSW graph for snapshot %q: %w", snapshotName, err)
	}

	// Step 4: Convert query embedding from float64 to float32
	queryVec := make([]float32, len(queryEmbedding))
	for i, val := range queryEmbedding {
		queryVec[i] = float32(val)
	}

	// Step 5: Perform HNSW search
	// Search returns nodes where Key is the chunk ID
	searchResults := hnswIndex.Search(queryVec, topK)

	// Step 6: Convert results to QueryResult format
	// Note: HNSW returns results by distance (lower is closer), but we need similarity scores
	// For cosine distance, similarity = 1 - distance (assuming distance is normalized to [0,1])
	results := make([]QueryResult, 0, len(searchResults))
	for _, node := range searchResults {
		// Calculate similarity from the embeddings
		// We'll compute cosine similarity: dot(query, node) / (||query|| * ||node||)
		similarity := cosineSimilarity(queryVec, node.Value)

		results = append(results, QueryResult{
			ChunkID: node.Key,
			Score:   similarity,
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

	// Cosine similarity = dot(a, b) / (||a|| * ||b||)
	similarity := dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))

	return similarity
}
