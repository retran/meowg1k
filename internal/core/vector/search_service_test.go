// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package vector

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"strings"
	"testing"

	"github.com/coder/hnsw"

	"github.com/retran/meowg1k/internal/domain/gateway"
)

// Mock implementations for testing

type mockMetaRepository struct {
	GetValueFunc    func(ctx context.Context, key string) ([]byte, error)
	SetValueFunc    func(ctx context.Context, key string, value []byte) error
	DeleteValueFunc func(ctx context.Context, key string) error
}

func (m *mockMetaRepository) GetValue(ctx context.Context, key string) ([]byte, error) {
	if m.GetValueFunc != nil {
		return m.GetValueFunc(ctx, key)
	}
	return nil, nil
}

func (m *mockMetaRepository) SetValue(ctx context.Context, key string, value []byte) error {
	if m.SetValueFunc != nil {
		return m.SetValueFunc(ctx, key, value)
	}
	return nil
}

func (m *mockMetaRepository) DeleteValue(ctx context.Context, key string) error {
	if m.DeleteValueFunc != nil {
		return m.DeleteValueFunc(ctx, key)
	}
	return nil
}

func TestNewSearchService(t *testing.T) {
	t.Run("Valid parameters", func(t *testing.T) {
		metaRepo := &mockMetaRepository{}

		service, err := NewSearchService(metaRepo)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if service == nil {
			t.Fatal("Expected service to be non-nil")
		}
	})

	t.Run("Nil metaRepo", func(t *testing.T) {
		service, err := NewSearchService(nil)
		if err == nil {
			t.Fatal("Expected error for nil metaRepo")
		}
		if service != nil {
			t.Fatal("Expected service to be nil when error occurs")
		}
		if !strings.Contains(err.Error(), "metaRepo cannot be nil") {
			t.Errorf("Expected metaRepo error, got: %v", err)
		}
	})
}

func TestSearchService_Search(t *testing.T) {
	// Helper to create a valid index dump
	createTestIndexDump := func() []byte {
		// Create a simple HNSW index
		hnswIndex := hnsw.NewGraph[int64]()

		// Add a couple of test vectors
		vec1 := []float32{0.1, 0.2, 0.3}
		vec2 := []float32{0.4, 0.5, 0.6}

		node1 := hnsw.MakeNode(int64(1), vec1)
		node2 := hnsw.MakeNode(int64(2), vec2)

		hnswIndex.Add(node1)
		hnswIndex.Add(node2)

		// Export HNSW index
		var hnswBuffer bytes.Buffer
		if err := hnswIndex.Export(&hnswBuffer); err != nil {
			panic(err)
		}

		// Create dump
		dump := IndexDump{
			HNSWData: hnswBuffer.Bytes(),
			IDToChunkID: map[uint32]int64{
				0: 1,
				1: 2,
			},
			ChunkIDToID: map[int64]uint32{
				1: 0,
				2: 1,
			},
		}

		// Encode dump
		var dumpBuffer bytes.Buffer
		encoder := gob.NewEncoder(&dumpBuffer)
		if err := encoder.Encode(dump); err != nil {
			panic(err)
		}

		return dumpBuffer.Bytes()
	}

	t.Run("Successful search", func(t *testing.T) {
		dumpBytes := createTestIndexDump()

		metaRepo := &mockMetaRepository{
			GetValueFunc: func(ctx context.Context, key string) ([]byte, error) {
				if key != "idx_dump_test-snapshot" {
					t.Errorf("Expected key 'idx_dump_test-snapshot', got %s", key)
				}
				return dumpBytes, nil
			},
		}

		service, _ := NewSearchService(metaRepo)

		queryEmbedding := gateway.Embedding{0.1, 0.2, 0.3}
		results, err := service.Search(context.Background(), "test-snapshot", queryEmbedding, 5)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(results) == 0 {
			t.Error("Expected at least one result")
		}
	})

	t.Run("Nil service", func(t *testing.T) {
		var service *SearchService = nil

		_, err := service.Search(context.Background(), "snap", gateway.Embedding{0.1}, 5)
		if err == nil {
			t.Fatal("Expected error for nil service")
		}
		if !strings.Contains(err.Error(), "search service is nil") {
			t.Errorf("Expected service nil error, got: %v", err)
		}
	})

	t.Run("Nil context", func(t *testing.T) {
		service, _ := NewSearchService(&mockMetaRepository{})

		//nolint:staticcheck // intentionally testing nil context handling
		_, err := service.Search(nil, "snap", gateway.Embedding{0.1}, 5)
		if err == nil {
			t.Fatal("Expected error for nil context")
		}
		if !strings.Contains(err.Error(), "context cannot be nil") {
			t.Errorf("Expected context error, got: %v", err)
		}
	})

	t.Run("Empty snapshot name", func(t *testing.T) {
		service, _ := NewSearchService(&mockMetaRepository{})

		_, err := service.Search(context.Background(), "", gateway.Embedding{0.1}, 5)
		if err == nil {
			t.Fatal("Expected error for empty snapshot name")
		}
		if !strings.Contains(err.Error(), "snapshot name cannot be empty") {
			t.Errorf("Expected snapshot name error, got: %v", err)
		}
	})

	t.Run("Empty searchindex embedding", func(t *testing.T) {
		service, _ := NewSearchService(&mockMetaRepository{})

		_, err := service.Search(context.Background(), "snap", gateway.Embedding{}, 5)
		if err == nil {
			t.Fatal("Expected error for empty searchindex embedding")
		}
		if !strings.Contains(err.Error(), "searchindex embedding cannot be empty") {
			t.Errorf("Expected embedding error, got: %v", err)
		}
	})

	t.Run("Invalid topK zero", func(t *testing.T) {
		service, _ := NewSearchService(&mockMetaRepository{})

		_, err := service.Search(context.Background(), "snap", gateway.Embedding{0.1}, 0)
		if err == nil {
			t.Fatal("Expected error for topK <= 0")
		}
		if !strings.Contains(err.Error(), "topK must be positive") {
			t.Errorf("Expected topK error, got: %v", err)
		}
	})

	t.Run("Invalid topK negative", func(t *testing.T) {
		service, _ := NewSearchService(&mockMetaRepository{})

		_, err := service.Search(context.Background(), "snap", gateway.Embedding{0.1}, -1)
		if err == nil {
			t.Fatal("Expected error for negative topK")
		}
	})

	t.Run("MetaRepo error propagates", func(t *testing.T) {
		metaRepo := &mockMetaRepository{
			GetValueFunc: func(ctx context.Context, key string) ([]byte, error) {
				return nil, errors.New("database error")
			},
		}
		service, _ := NewSearchService(metaRepo)

		_, err := service.Search(context.Background(), "snap", gateway.Embedding{0.1}, 5)
		if err == nil {
			t.Fatal("Expected error from metaRepo")
		}
		if !strings.Contains(err.Error(), "failed to get index dump") {
			t.Errorf("Expected index dump error, got: %v", err)
		}
	})

	t.Run("No index found", func(t *testing.T) {
		metaRepo := &mockMetaRepository{
			GetValueFunc: func(ctx context.Context, key string) ([]byte, error) {
				return nil, nil
			},
		}
		service, _ := NewSearchService(metaRepo)

		_, err := service.Search(context.Background(), "nonexistent", gateway.Embedding{0.1}, 5)
		if err == nil {
			t.Fatal("Expected error when no index found")
		}
		if !strings.Contains(err.Error(), "no index found") {
			t.Errorf("Expected no index error, got: %v", err)
		}
	})

	t.Run("Invalid dump format", func(t *testing.T) {
		metaRepo := &mockMetaRepository{
			GetValueFunc: func(ctx context.Context, key string) ([]byte, error) {
				return []byte("invalid data"), nil
			},
		}
		service, _ := NewSearchService(metaRepo)

		_, err := service.Search(context.Background(), "snap", gateway.Embedding{0.1}, 5)
		if err == nil {
			t.Fatal("Expected error for invalid dump format")
		}
		if !strings.Contains(err.Error(), "failed to decode index dump") {
			t.Errorf("Expected decode error, got: %v", err)
		}
	})
}

func TestCosineSimilarity(t *testing.T) {
	t.Run("Identical vectors", func(t *testing.T) {
		a := []float32{1.0, 0.0, 0.0}
		b := []float32{1.0, 0.0, 0.0}

		similarity := cosineSimilarity(a, b)
		if similarity < 0.999 || similarity > 1.001 {
			t.Errorf("Expected similarity ~1.0 for identical vectors, got %f", similarity)
		}
	})

	t.Run("Orthogonal vectors", func(t *testing.T) {
		a := []float32{1.0, 0.0, 0.0}
		b := []float32{0.0, 1.0, 0.0}

		similarity := cosineSimilarity(a, b)
		if similarity < -0.001 || similarity > 0.001 {
			t.Errorf("Expected similarity ~0.0 for orthogonal vectors, got %f", similarity)
		}
	})

	t.Run("Opposite vectors", func(t *testing.T) {
		a := []float32{1.0, 0.0, 0.0}
		b := []float32{-1.0, 0.0, 0.0}

		similarity := cosineSimilarity(a, b)
		if similarity < -1.001 || similarity > -0.999 {
			t.Errorf("Expected similarity ~-1.0 for opposite vectors, got %f", similarity)
		}
	})

	t.Run("Different length vectors", func(t *testing.T) {
		a := []float32{1.0, 0.0}
		b := []float32{1.0, 0.0, 0.0}

		similarity := cosineSimilarity(a, b)
		if similarity != 0 {
			t.Errorf("Expected similarity 0 for different length vectors, got %f", similarity)
		}
	})

	t.Run("Zero vectors", func(t *testing.T) {
		a := []float32{0.0, 0.0, 0.0}
		b := []float32{1.0, 0.0, 0.0}

		similarity := cosineSimilarity(a, b)
		if similarity != 0 {
			t.Errorf("Expected similarity 0 for zero vector, got %f", similarity)
		}
	})

	t.Run("Similar vectors", func(t *testing.T) {
		a := []float32{0.8, 0.6, 0.0}
		b := []float32{0.6, 0.8, 0.0}

		similarity := cosineSimilarity(a, b)
		// These should have positive similarity
		if similarity <= 0 {
			t.Errorf("Expected positive similarity for similar vectors, got %f", similarity)
		}
	})

	t.Run("Normalized vectors", func(t *testing.T) {
		// Unit vectors
		a := []float32{0.6, 0.8, 0.0}
		b := []float32{0.6, 0.8, 0.0}

		similarity := cosineSimilarity(a, b)
		if similarity < 0.999 || similarity > 1.001 {
			t.Errorf("Expected similarity ~1.0, got %f", similarity)
		}
	})
}

func TestSearchService_Search_Integration(t *testing.T) {
	t.Run("Returns results with correct structure", func(t *testing.T) {
		dumpBytes := func() []byte {
			hnswIndex := hnsw.NewGraph[int64]()
			vec := []float32{0.5, 0.5, 0.5}
			node := hnsw.MakeNode(int64(123), vec)
			hnswIndex.Add(node)

			var hnswBuffer bytes.Buffer
			hnswIndex.Export(&hnswBuffer)

			dump := IndexDump{
				HNSWData:    hnswBuffer.Bytes(),
				IDToChunkID: map[uint32]int64{0: 123},
				ChunkIDToID: map[int64]uint32{123: 0},
			}

			var dumpBuffer bytes.Buffer
			gob.NewEncoder(&dumpBuffer).Encode(dump)
			return dumpBuffer.Bytes()
		}()

		metaRepo := &mockMetaRepository{
			GetValueFunc: func(ctx context.Context, key string) ([]byte, error) {
				return dumpBytes, nil
			},
		}

		service, _ := NewSearchService(metaRepo)
		results, err := service.Search(context.Background(), "snap", gateway.Embedding{0.5, 0.5, 0.5}, 10)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(results) == 0 {
			t.Fatal("Expected results")
		}

		result := results[0]
		if result.ChunkID != 123 {
			t.Errorf("Expected chunk ID 123, got %d", result.ChunkID)
		}
		if result.SnapshotName != "snap" {
			t.Errorf("Expected snapshot 'snap', got %s", result.SnapshotName)
		}
		if result.Score < 0 || result.Score > 1 {
			t.Errorf("Expected score in [0, 1], got %f", result.Score)
		}
	})
}
