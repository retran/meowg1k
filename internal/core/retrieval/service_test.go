// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package retrieval

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/retran/meowg1k/internal/core/vector"
	"github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
)

// Mock implementations for testing

type mockEmbeddingsGateway struct {
	ComputeEmbeddingsFunc func(ctx context.Context, request *gateway.ComputeEmbeddingsRequest) ([]gateway.Embedding, error)
	ComputeDistanceFunc   func(emb1, emb2 gateway.Embedding) (float64, error)
}

func (m *mockEmbeddingsGateway) ComputeEmbeddings(ctx context.Context, request *gateway.ComputeEmbeddingsRequest) ([]gateway.Embedding, error) {
	if m.ComputeEmbeddingsFunc != nil {
		return m.ComputeEmbeddingsFunc(ctx, request)
	}
	return []gateway.Embedding{{0.1, 0.2, 0.3}}, nil
}

func (m *mockEmbeddingsGateway) ComputeDistance(emb1, emb2 gateway.Embedding) (float64, error) {
	if m.ComputeDistanceFunc != nil {
		return m.ComputeDistanceFunc(emb1, emb2)
	}
	return 0.5, nil
}

type mockVectorSearchService struct {
	SearchFunc func(ctx context.Context, snapshotName string, queryEmbedding gateway.Embedding, topK int) ([]vector.QueryResult, error)
}

func (m *mockVectorSearchService) Search(ctx context.Context, snapshotName string, queryEmbedding gateway.Embedding, topK int) ([]vector.QueryResult, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, snapshotName, queryEmbedding, topK)
	}
	return []vector.QueryResult{
		{ChunkID: 1, Score: 0.9, SnapshotName: snapshotName},
		{ChunkID: 2, Score: 0.8, SnapshotName: snapshotName},
	}, nil
}

type mockIndexRepository struct {
	GetChunksByIDsFunc   func(ctx context.Context, chunkIDs []int64) ([]domainindex.Chunk, error)
	GetVersionsByIDsFunc func(ctx context.Context, versionIDs []int64) ([]domainindex.DocumentVersion, error)
	AddChunksFunc        func(ctx context.Context, chunks []domainindex.Chunk) error
}

func (m *mockIndexRepository) AddChunks(ctx context.Context, chunks []domainindex.Chunk) error {
	if m.AddChunksFunc != nil {
		return m.AddChunksFunc(ctx, chunks)
	}
	return nil
}

func (m *mockIndexRepository) GetChunksByIDs(ctx context.Context, chunkIDs []int64) ([]domainindex.Chunk, error) {
	if m.GetChunksByIDsFunc != nil {
		return m.GetChunksByIDsFunc(ctx, chunkIDs)
	}
	chunks := make([]domainindex.Chunk, len(chunkIDs))
	for i, id := range chunkIDs {
		chunks[i] = domainindex.Chunk{
			ID:                id,
			DocumentVersionID: 100 + id,
			TextContent:       "Test content for chunk",
			StartLine:         1,
			EndLine:           5,
		}
	}
	return chunks, nil
}

func (m *mockIndexRepository) GetVersionsByIDs(ctx context.Context, versionIDs []int64) ([]domainindex.DocumentVersion, error) {
	if m.GetVersionsByIDsFunc != nil {
		return m.GetVersionsByIDsFunc(ctx, versionIDs)
	}
	versions := make([]domainindex.DocumentVersion, len(versionIDs))
	for i, id := range versionIDs {
		versions[i] = domainindex.DocumentVersion{
			ID:       id,
			FilePath: "test/file.go",
		}
	}
	return versions, nil
}

// Implement other IndexRepository methods as no-ops (not used by retrieval service).
func (m *mockIndexRepository) AddDocumentVersion(ctx context.Context, doc *domainindex.DocumentVersion, content []byte) (int64, error) {
	return 0, nil
}

func (m *mockIndexRepository) AddDocumentVersionWithChunks(ctx context.Context, doc *domainindex.DocumentVersion, content []byte, chunks []domainindex.Chunk) (int64, error) {
	return 0, nil
}

func (m *mockIndexRepository) FindVersionByContentHash(ctx context.Context, filePath, contentHash string) (*domainindex.DocumentVersion, error) {
	return nil, nil
}

func (m *mockIndexRepository) FindVersionsByContentHashes(ctx context.Context, contentHashes []string) (map[string]*domainindex.DocumentVersion, error) {
	return nil, nil
}

func (m *mockIndexRepository) FindContentBlob(ctx context.Context, contentHash string) (bool, error) {
	return false, nil
}

func (m *mockIndexRepository) GetContentBlob(ctx context.Context, contentHash string) ([]byte, error) {
	return nil, nil
}

func (m *mockIndexRepository) FindVersionsByFilePath(ctx context.Context, filePath string) ([]domainindex.DocumentVersion, error) {
	return nil, nil
}

func (m *mockIndexRepository) GetChunksByVersionID(ctx context.Context, versionID int64) ([]domainindex.Chunk, error) {
	return nil, nil
}

func (m *mockIndexRepository) GetAllEmbeddings(ctx context.Context) (map[int64]gateway.Embedding, error) {
	return nil, nil
}

func (m *mockIndexRepository) Checkpoint(ctx context.Context) error {
	return nil
}

func TestNewService(t *testing.T) {
	t.Run("Valid parameters", func(t *testing.T) {
		embeddingsGW := &mockEmbeddingsGateway{}
		vectorSearchSvc := &mockVectorSearchService{}
		indexRepo := &mockIndexRepository{}
		embeddingModel := "test-model"
		taskType := gateway.RetrievalQuery

		service, err := NewService(embeddingsGW, vectorSearchSvc, indexRepo, embeddingModel, taskType)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if service == nil {
			t.Fatal("Expected service to be non-nil")
		}
	})

	t.Run("Nil embeddingsGW", func(t *testing.T) {
		vectorSearchSvc := &mockVectorSearchService{}
		indexRepo := &mockIndexRepository{}

		service, err := NewService(nil, vectorSearchSvc, indexRepo, "model", gateway.RetrievalQuery)
		if err == nil {
			t.Fatal("Expected error for nil embeddingsGW")
		}
		if service != nil {
			t.Fatal("Expected service to be nil when error occurs")
		}
		if !strings.Contains(err.Error(), "embeddingsGW cannot be nil") {
			t.Errorf("Expected embeddingsGW error, got: %v", err)
		}
	})

	t.Run("Nil vectorSearchSvc", func(t *testing.T) {
		embeddingsGW := &mockEmbeddingsGateway{}
		indexRepo := &mockIndexRepository{}

		service, err := NewService(embeddingsGW, nil, indexRepo, "model", gateway.RetrievalQuery)
		if err == nil {
			t.Fatal("Expected error for nil vectorSearchSvc")
		}
		if service != nil {
			t.Fatal("Expected service to be nil when error occurs")
		}
		if !strings.Contains(err.Error(), "vectorSearchSvc cannot be nil") {
			t.Errorf("Expected vectorSearchSvc error, got: %v", err)
		}
	})

	t.Run("Nil indexRepo", func(t *testing.T) {
		embeddingsGW := &mockEmbeddingsGateway{}
		vectorSearchSvc := &mockVectorSearchService{}

		service, err := NewService(embeddingsGW, vectorSearchSvc, nil, "model", gateway.RetrievalQuery)
		if err == nil {
			t.Fatal("Expected error for nil indexRepo")
		}
		if service != nil {
			t.Fatal("Expected service to be nil when error occurs")
		}
		if !strings.Contains(err.Error(), "indexRepo cannot be nil") {
			t.Errorf("Expected indexRepo error, got: %v", err)
		}
	})

	t.Run("Empty embeddingModel", func(t *testing.T) {
		embeddingsGW := &mockEmbeddingsGateway{}
		vectorSearchSvc := &mockVectorSearchService{}
		indexRepo := &mockIndexRepository{}

		service, err := NewService(embeddingsGW, vectorSearchSvc, indexRepo, "", gateway.RetrievalQuery)
		if err == nil {
			t.Fatal("Expected error for empty embeddingModel")
		}
		if service != nil {
			t.Fatal("Expected service to be nil when error occurs")
		}
		if !strings.Contains(err.Error(), "embeddingModel cannot be empty") {
			t.Errorf("Expected embeddingModel error, got: %v", err)
		}
	})
}

func TestService_Search(t *testing.T) {
	t.Run("Successful search", func(t *testing.T) {
		embeddingsGW := &mockEmbeddingsGateway{}
		vectorSearchSvc := &mockVectorSearchService{}
		indexRepo := &mockIndexRepository{}

		service, _ := NewService(embeddingsGW, vectorSearchSvc, indexRepo, "model", gateway.RetrievalQuery)

		ctx := context.Background()
		results, err := service.Search(ctx, "test searchindex", []string{"snapshot1"}, 5, 0.5)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(results) == 0 {
			t.Error("Expected at least one result")
		}
	})

	t.Run("Nil service", func(t *testing.T) {
		var service *Service = nil
		ctx := context.Background()

		_, err := service.Search(ctx, "test", []string{"snap"}, 5, 0.5)
		if err == nil {
			t.Fatal("Expected error for nil service")
		}
		if !strings.Contains(err.Error(), "retrieval service is nil") {
			t.Errorf("Expected service nil error, got: %v", err)
		}
	})

	t.Run("Empty searchindex text", func(t *testing.T) {
		service, _ := NewService(&mockEmbeddingsGateway{}, &mockVectorSearchService{}, &mockIndexRepository{}, "model", gateway.RetrievalQuery)

		ctx := context.Background()
		_, err := service.Search(ctx, "", []string{"snap"}, 5, 0.5)
		if err == nil {
			t.Fatal("Expected error for empty searchindex text")
		}
		if !strings.Contains(err.Error(), "searchindex text cannot be empty") {
			t.Errorf("Expected empty searchindex error, got: %v", err)
		}
	})

	t.Run("Empty snapshot priority", func(t *testing.T) {
		service, _ := NewService(&mockEmbeddingsGateway{}, &mockVectorSearchService{}, &mockIndexRepository{}, "model", gateway.RetrievalQuery)

		ctx := context.Background()
		_, err := service.Search(ctx, "test", []string{}, 5, 0.5)
		if err == nil {
			t.Fatal("Expected error for empty snapshot priority")
		}
		if !strings.Contains(err.Error(), "snapshot priority list cannot be empty") {
			t.Errorf("Expected empty snapshot priority error, got: %v", err)
		}
	})

	t.Run("Invalid topK", func(t *testing.T) {
		service, _ := NewService(&mockEmbeddingsGateway{}, &mockVectorSearchService{}, &mockIndexRepository{}, "model", gateway.RetrievalQuery)

		ctx := context.Background()
		_, err := service.Search(ctx, "test", []string{"snap"}, 0, 0.5)
		if err == nil {
			t.Fatal("Expected error for topK <= 0")
		}
		if !strings.Contains(err.Error(), "topK must be positive") {
			t.Errorf("Expected topK error, got: %v", err)
		}
	})

	t.Run("Negative topK", func(t *testing.T) {
		service, _ := NewService(&mockEmbeddingsGateway{}, &mockVectorSearchService{}, &mockIndexRepository{}, "model", gateway.RetrievalQuery)

		ctx := context.Background()
		_, err := service.Search(ctx, "test", []string{"snap"}, -1, 0.5)
		if err == nil {
			t.Fatal("Expected error for negative topK")
		}
	})

	t.Run("Embeddings computation error", func(t *testing.T) {
		embeddingsGW := &mockEmbeddingsGateway{
			ComputeEmbeddingsFunc: func(ctx context.Context, request *gateway.ComputeEmbeddingsRequest) ([]gateway.Embedding, error) {
				return nil, errors.New("embedding error")
			},
		}
		service, _ := NewService(embeddingsGW, &mockVectorSearchService{}, &mockIndexRepository{}, "model", gateway.RetrievalQuery)

		ctx := context.Background()
		_, err := service.Search(ctx, "test", []string{"snap"}, 5, 0.5)
		if err == nil {
			t.Fatal("Expected error for embeddings computation failure")
		}
		if !strings.Contains(err.Error(), "failed to compute searchindex embedding") {
			t.Errorf("Expected embedding error, got: %v", err)
		}
	})

	t.Run("No embeddings returned", func(t *testing.T) {
		embeddingsGW := &mockEmbeddingsGateway{
			ComputeEmbeddingsFunc: func(ctx context.Context, request *gateway.ComputeEmbeddingsRequest) ([]gateway.Embedding, error) {
				return []gateway.Embedding{}, nil
			},
		}
		service, _ := NewService(embeddingsGW, &mockVectorSearchService{}, &mockIndexRepository{}, "model", gateway.RetrievalQuery)

		ctx := context.Background()
		_, err := service.Search(ctx, "test", []string{"snap"}, 5, 0.5)
		if err == nil {
			t.Fatal("Expected error when no embeddings returned")
		}
		if !strings.Contains(err.Error(), "no embeddings returned") {
			t.Errorf("Expected no embeddings error, got: %v", err)
		}
	})

	t.Run("Multiple snapshots", func(t *testing.T) {
		service, _ := NewService(&mockEmbeddingsGateway{}, &mockVectorSearchService{}, &mockIndexRepository{}, "model", gateway.RetrievalQuery)

		ctx := context.Background()
		results, err := service.Search(ctx, "test", []string{"snap1", "snap2", "snap3"}, 10, 0.5)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(results) == 0 {
			t.Error("Expected results from multiple snapshots")
		}
	})

	t.Run("MinScore filtering", func(t *testing.T) {
		vectorSearchSvc := &mockVectorSearchService{
			SearchFunc: func(ctx context.Context, snapshotName string, queryEmbedding gateway.Embedding, topK int) ([]vector.QueryResult, error) {
				return []vector.QueryResult{
					{ChunkID: 1, Score: 0.9, SnapshotName: snapshotName},
					{ChunkID: 2, Score: 0.4, SnapshotName: snapshotName}, // Below threshold
					{ChunkID: 3, Score: 0.7, SnapshotName: snapshotName},
				}, nil
			},
		}
		service, _ := NewService(&mockEmbeddingsGateway{}, vectorSearchSvc, &mockIndexRepository{}, "model", gateway.RetrievalQuery)

		ctx := context.Background()
		results, err := service.Search(ctx, "test", []string{"snap"}, 10, 0.5)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		// Should have filtered out the 0.4 score result
		if len(results) > 2 {
			t.Error("Expected minScore filtering to remove low-score results")
		}
	})

	t.Run("TopK limiting", func(t *testing.T) {
		vectorSearchSvc := &mockVectorSearchService{
			SearchFunc: func(ctx context.Context, snapshotName string, queryEmbedding gateway.Embedding, topK int) ([]vector.QueryResult, error) {
				return []vector.QueryResult{
					{ChunkID: 1, Score: 0.9, SnapshotName: snapshotName},
					{ChunkID: 2, Score: 0.8, SnapshotName: snapshotName},
					{ChunkID: 3, Score: 0.7, SnapshotName: snapshotName},
					{ChunkID: 4, Score: 0.6, SnapshotName: snapshotName},
					{ChunkID: 5, Score: 0.5, SnapshotName: snapshotName},
				}, nil
			},
		}
		service, _ := NewService(&mockEmbeddingsGateway{}, vectorSearchSvc, &mockIndexRepository{}, "model", gateway.RetrievalQuery)

		ctx := context.Background()
		results, err := service.Search(ctx, "test", []string{"snap"}, 3, 0.0)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(results) != 3 {
			t.Errorf("Expected exactly 3 results (topK), got %d", len(results))
		}
	})
}

func TestService_RetrieveContext(t *testing.T) {
	t.Run("Successful context retrieval", func(t *testing.T) {
		service, _ := NewService(&mockEmbeddingsGateway{}, &mockVectorSearchService{}, &mockIndexRepository{}, "model", gateway.RetrievalQuery)

		ctx := context.Background()
		contextStr, err := service.RetrieveContext(ctx, "test searchindex", []string{"snapshot1"}, 5, 0.5)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if contextStr == "" {
			t.Error("Expected non-empty context string")
		}
		if !strings.Contains(contextStr, "Retrieved Context") {
			t.Error("Expected context to contain header")
		}
		if !strings.Contains(contextStr, "```") {
			t.Error("Expected context to contain code blocks")
		}
	})

	t.Run("No results returns empty string", func(t *testing.T) {
		vectorSearchSvc := &mockVectorSearchService{
			SearchFunc: func(ctx context.Context, snapshotName string, queryEmbedding gateway.Embedding, topK int) ([]vector.QueryResult, error) {
				return []vector.QueryResult{}, nil
			},
		}
		service, _ := NewService(&mockEmbeddingsGateway{}, vectorSearchSvc, &mockIndexRepository{}, "model", gateway.RetrievalQuery)

		ctx := context.Background()
		contextStr, err := service.RetrieveContext(ctx, "test", []string{"snap"}, 5, 0.9)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if contextStr != "" {
			t.Error("Expected empty context string when no results")
		}
	})

	t.Run("Error propagation", func(t *testing.T) {
		embeddingsGW := &mockEmbeddingsGateway{
			ComputeEmbeddingsFunc: func(ctx context.Context, request *gateway.ComputeEmbeddingsRequest) ([]gateway.Embedding, error) {
				return nil, errors.New("test error")
			},
		}
		service, _ := NewService(embeddingsGW, &mockVectorSearchService{}, &mockIndexRepository{}, "model", gateway.RetrievalQuery)

		ctx := context.Background()
		_, err := service.RetrieveContext(ctx, "test", []string{"snap"}, 5, 0.5)
		if err == nil {
			t.Fatal("Expected error propagation from Search")
		}
	})
}

func TestFormatSnapshotName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"_workdir_", "(uncommitted changes)"},
		{"_stage_", "(staged for commit)"},
		{"_head_", "(not changed since last commit)"},
		{"main", "main"},
		{"feature-branch", "feature-branch"},
		{"v1.0.0", "v1.0.0"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := formatSnapshotName(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestService_Search_ScoringAndSorting(t *testing.T) {
	vectorSearchSvc := &mockVectorSearchService{
		SearchFunc: func(ctx context.Context, snapshotName string, queryEmbedding gateway.Embedding, topK int) ([]vector.QueryResult, error) {
			// Return unsorted results
			return []vector.QueryResult{
				{ChunkID: 1, Score: 0.5, SnapshotName: snapshotName},
				{ChunkID: 2, Score: 0.9, SnapshotName: snapshotName},
				{ChunkID: 3, Score: 0.7, SnapshotName: snapshotName},
			}, nil
		},
	}
	service, _ := NewService(&mockEmbeddingsGateway{}, vectorSearchSvc, &mockIndexRepository{}, "model", gateway.RetrievalQuery)

	ctx := context.Background()
	results, err := service.Search(ctx, "test", []string{"snap"}, 10, 0.0)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Results should be sorted by score (descending)
	if len(results) < 2 {
		t.Fatal("Expected at least 2 results")
	}
	if results[0].Score < results[1].Score {
		t.Error("Results should be sorted by score in descending order")
	}
}
