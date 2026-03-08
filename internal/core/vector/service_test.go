// Copyright © 2025 The meowg1k Authors.
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
	domainindex "github.com/retran/meowg1k/internal/domain/index"
)

// Mock repositories for BuildAndSave testing.
type mockIndexRepositoryForBuild struct {
	chunks map[int64][]domainindex.Chunk
	err    error
}

func (m *mockIndexRepositoryForBuild) GetChunksByVersionID(ctx context.Context, versionID int64) ([]domainindex.Chunk, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.chunks[versionID], nil
}

// Implement other required methods (not used in BuildAndSave).
func (m *mockIndexRepositoryForBuild) AddDocumentVersion(ctx context.Context, doc *domainindex.DocumentVersion, content []byte) (int64, error) {
	return 0, errors.New("not implemented")
}

func (m *mockIndexRepositoryForBuild) AddDocumentVersionWithChunks(ctx context.Context, doc *domainindex.DocumentVersion, content []byte, chunks []domainindex.Chunk) (int64, error) {
	return 0, errors.New("not implemented")
}

func (m *mockIndexRepositoryForBuild) AddChunks(ctx context.Context, chunks []domainindex.Chunk) error {
	return errors.New("not implemented")
}

func (m *mockIndexRepositoryForBuild) FindVersionByContentHash(ctx context.Context, filePath, contentHash string) (*domainindex.DocumentVersion, error) {
	return nil, errors.New("not implemented")
}

func (m *mockIndexRepositoryForBuild) FindVersionsByContentHashes(ctx context.Context, contentHashes []string) (map[string]*domainindex.DocumentVersion, error) {
	return nil, errors.New("not implemented")
}

func (m *mockIndexRepositoryForBuild) FindContentBlob(ctx context.Context, contentHash string) (bool, error) {
	return false, errors.New("not implemented")
}

func (m *mockIndexRepositoryForBuild) GetContentBlob(ctx context.Context, contentHash string) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func (m *mockIndexRepositoryForBuild) FindVersionsByFilePath(ctx context.Context, filePath string) ([]domainindex.DocumentVersion, error) {
	return nil, errors.New("not implemented")
}

func (m *mockIndexRepositoryForBuild) GetChunksByIDs(ctx context.Context, chunkIDs []int64) ([]domainindex.Chunk, error) {
	return nil, errors.New("not implemented")
}

func (m *mockIndexRepositoryForBuild) GetAllEmbeddings(ctx context.Context) (map[int64]gateway.Embedding, error) {
	return nil, errors.New("not implemented")
}

func (m *mockIndexRepositoryForBuild) GetVersionsByIDs(ctx context.Context, versionIDs []int64) ([]domainindex.DocumentVersion, error) {
	return nil, errors.New("not implemented")
}

func (m *mockIndexRepositoryForBuild) Checkpoint(ctx context.Context) error {
	return errors.New("not implemented")
}

type mockSnapshotRepositoryForBuild struct {
	err        error
	versionIDs []int64
}

func (m *mockSnapshotRepositoryForBuild) GetVersionIDsForSnapshot(ctx context.Context, snapshotName string) ([]int64, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.versionIDs, nil
}

// Implement other required methods (not used in BuildAndSave).
func (m *mockSnapshotRepositoryForBuild) ClearSnapshotLinks(ctx context.Context, commitHash string) error {
	return errors.New("not implemented")
}

func (m *mockSnapshotRepositoryForBuild) LinkVersionToSnapshot(ctx context.Context, commitHash string, versionID int64) error {
	return errors.New("not implemented")
}

func (m *mockSnapshotRepositoryForBuild) UnlinkVersionFromSnapshot(ctx context.Context, commitHash string, versionID int64) error {
	return errors.New("not implemented")
}

func (m *mockSnapshotRepositoryForBuild) GetVersionsForSnapshot(ctx context.Context, snapshotName string) ([]domainindex.DocumentVersion, error) {
	return nil, errors.New("not implemented")
}

type mockMetaRepositoryForBuild struct {
	values map[string][]byte
	setErr error
	delErr error
}

func (m *mockMetaRepositoryForBuild) SetValue(ctx context.Context, key string, value []byte) error {
	if m.setErr != nil {
		return m.setErr
	}
	if m.values == nil {
		m.values = make(map[string][]byte)
	}
	m.values[key] = value
	return nil
}

func (m *mockMetaRepositoryForBuild) GetValue(ctx context.Context, key string) ([]byte, error) {
	if m.values == nil {
		return nil, errors.New("not found")
	}
	val, ok := m.values[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return val, nil
}

func (m *mockMetaRepositoryForBuild) DeleteValue(ctx context.Context, key string) error {
	if m.delErr != nil {
		return m.delErr
	}
	if m.values != nil {
		delete(m.values, key)
	}
	return nil
}

func TestNewService(t *testing.T) {
	indexRepo := &mockIndexRepositoryForBuild{}
	snapshotRepo := &mockSnapshotRepositoryForBuild{}
	metaRepo := &mockMetaRepositoryForBuild{}

	service := NewService(indexRepo, snapshotRepo, metaRepo)

	if service == nil {
		t.Fatal("NewService returned nil")
	}
}

func TestService_BuildAndSave_EmptySnapshot(t *testing.T) {
	indexRepo := &mockIndexRepositoryForBuild{}
	snapshotRepo := &mockSnapshotRepositoryForBuild{
		versionIDs: []int64{}, // Empty snapshot
	}
	metaRepo := &mockMetaRepositoryForBuild{
		values: map[string][]byte{
			"idx_dump_test": []byte("old data"), // Should be deleted
		},
	}

	service := NewService(indexRepo, snapshotRepo, metaRepo)
	err := service.BuildAndSave("test")
	if err != nil {
		t.Fatalf("expected no error for empty snapshot, got: %v", err)
	}

	// Verify old index was deleted
	_, exists := metaRepo.values["idx_dump_test"]
	if exists {
		t.Error("expected old index dump to be deleted for empty snapshot")
	}
}

func TestService_BuildAndSave_GetVersionIDsError(t *testing.T) {
	indexRepo := &mockIndexRepositoryForBuild{}
	snapshotRepo := &mockSnapshotRepositoryForBuild{
		err: errors.New("snapshot error"),
	}
	metaRepo := &mockMetaRepositoryForBuild{}

	service := NewService(indexRepo, snapshotRepo, metaRepo)
	err := service.BuildAndSave("test")

	if err == nil {
		t.Fatal("expected error from snapshot repository")
	}

	if !strings.Contains(err.Error(), "failed to get version IDs") {
		t.Errorf("expected error about version IDs, got: %v", err)
	}
}

func TestService_BuildAndSave_GetChunksError(t *testing.T) {
	indexRepo := &mockIndexRepositoryForBuild{
		err: errors.New("chunks error"),
	}
	snapshotRepo := &mockSnapshotRepositoryForBuild{
		versionIDs: []int64{1, 2},
	}
	metaRepo := &mockMetaRepositoryForBuild{}

	service := NewService(indexRepo, snapshotRepo, metaRepo)
	err := service.BuildAndSave("test")

	if err == nil {
		t.Fatal("expected error from index repository")
	}

	if !strings.Contains(err.Error(), "failed to get chunks for version") {
		t.Errorf("expected error about getting chunks, got: %v", err)
	}
}

func TestService_BuildAndSave_NoChunks(t *testing.T) {
	indexRepo := &mockIndexRepositoryForBuild{
		chunks: map[int64][]domainindex.Chunk{
			1: {}, // Empty chunks for version 1
			2: {}, // Empty chunks for version 2
		},
	}
	snapshotRepo := &mockSnapshotRepositoryForBuild{
		versionIDs: []int64{1, 2},
	}
	metaRepo := &mockMetaRepositoryForBuild{}

	service := NewService(indexRepo, snapshotRepo, metaRepo)
	err := service.BuildAndSave("test")

	if err == nil {
		t.Fatal("expected error for no chunks")
	}

	if !strings.Contains(err.Error(), "no chunks found") {
		t.Errorf("expected error about no chunks, got: %v", err)
	}
}

func TestService_BuildAndSave_Success(t *testing.T) {
	chunk1 := domainindex.Chunk{
		ID:        101,
		Embedding: gateway.Embedding{0.1, 0.2, 0.3},
	}
	chunk2 := domainindex.Chunk{
		ID:        102,
		Embedding: gateway.Embedding{0.4, 0.5, 0.6},
	}
	chunk3 := domainindex.Chunk{
		ID:        103,
		Embedding: gateway.Embedding{0.7, 0.8, 0.9},
	}

	indexRepo := &mockIndexRepositoryForBuild{
		chunks: map[int64][]domainindex.Chunk{
			1: {chunk1, chunk2},
			2: {chunk3},
		},
	}
	snapshotRepo := &mockSnapshotRepositoryForBuild{
		versionIDs: []int64{1, 2},
	}
	metaRepo := &mockMetaRepositoryForBuild{
		values: make(map[string][]byte),
	}

	service := NewService(indexRepo, snapshotRepo, metaRepo)
	err := service.BuildAndSave("test-snapshot")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify dump was saved
	dumpData, ok := metaRepo.values["idx_dump_test-snapshot"]
	if !ok {
		t.Fatal("expected index dump to be saved")
	}

	// Verify dump can be deserialized
	var dump IndexDump
	dumpReader := bytes.NewReader(dumpData)
	dumpDecoder := gob.NewDecoder(dumpReader)
	if err := dumpDecoder.Decode(&dump); err != nil {
		t.Fatalf("failed to decode dump: %v", err)
	}

	// Verify mapping tables
	if len(dump.IDToChunkID) != 3 {
		t.Errorf("expected 3 entries in IDToChunkID, got %d", len(dump.IDToChunkID))
	}
	if len(dump.ChunkIDToID) != 3 {
		t.Errorf("expected 3 entries in ChunkIDToID, got %d", len(dump.ChunkIDToID))
	}

	// Verify HNSW data exists
	if len(dump.HNSWData) == 0 {
		t.Error("expected HNSW data to be present")
	}

	// Verify HNSW index can be imported
	hnswReader := bytes.NewReader(dump.HNSWData)
	hnswIndex := hnsw.NewGraph[int64]()
	if err := hnswIndex.Import(hnswReader); err != nil {
		t.Fatalf("failed to import HNSW index: %v", err)
	}

	// Verify chunk IDs are in mapping
	for chunkID := range dump.ChunkIDToID {
		if chunkID != 101 && chunkID != 102 && chunkID != 103 {
			t.Errorf("unexpected chunk ID in mapping: %d", chunkID)
		}
	}
}

func TestService_BuildAndSave_SaveError(t *testing.T) {
	chunk1 := domainindex.Chunk{
		ID:        101,
		Embedding: gateway.Embedding{0.1, 0.2, 0.3},
	}

	indexRepo := &mockIndexRepositoryForBuild{
		chunks: map[int64][]domainindex.Chunk{
			1: {chunk1},
		},
	}
	snapshotRepo := &mockSnapshotRepositoryForBuild{
		versionIDs: []int64{1},
	}
	metaRepo := &mockMetaRepositoryForBuild{
		setErr: errors.New("save error"),
	}

	service := NewService(indexRepo, snapshotRepo, metaRepo)
	err := service.BuildAndSave("test")

	if err == nil {
		t.Fatal("expected error from meta repository")
	}

	if !strings.Contains(err.Error(), "failed to save index dump") {
		t.Errorf("expected error about saving dump, got: %v", err)
	}
}

func TestService_BuildAndSave_LargeNumberOfChunks(t *testing.T) {
	// Create a large number of chunks
	chunks := make([]domainindex.Chunk, 1000)
	for i := 0; i < 1000; i++ {
		chunks[i] = domainindex.Chunk{
			ID:        int64(i + 1),
			Embedding: gateway.Embedding{float64(i) * 0.1, float64(i) * 0.2, float64(i) * 0.3},
		}
	}

	indexRepo := &mockIndexRepositoryForBuild{
		chunks: map[int64][]domainindex.Chunk{
			1: chunks,
		},
	}
	snapshotRepo := &mockSnapshotRepositoryForBuild{
		versionIDs: []int64{1},
	}
	metaRepo := &mockMetaRepositoryForBuild{
		values: make(map[string][]byte),
	}

	service := NewService(indexRepo, snapshotRepo, metaRepo)
	err := service.BuildAndSave("large-test")
	if err != nil {
		t.Fatalf("expected no error for large chunk set, got: %v", err)
	}

	// Verify dump was saved
	dumpData, ok := metaRepo.values["idx_dump_large-test"]
	if !ok {
		t.Fatal("expected index dump to be saved")
	}

	// Verify dump can be deserialized
	var dump IndexDump
	dumpReader := bytes.NewReader(dumpData)
	dumpDecoder := gob.NewDecoder(dumpReader)
	if err := dumpDecoder.Decode(&dump); err != nil {
		t.Fatalf("failed to decode dump: %v", err)
	}

	// Verify all chunks are in mapping
	if len(dump.IDToChunkID) != 1000 {
		t.Errorf("expected 1000 entries in IDToChunkID, got %d", len(dump.IDToChunkID))
	}
	if len(dump.ChunkIDToID) != 1000 {
		t.Errorf("expected 1000 entries in ChunkIDToID, got %d", len(dump.ChunkIDToID))
	}
}

func TestService_BuildAndSave_MultipleVersions(t *testing.T) {
	chunks1 := []domainindex.Chunk{
		{ID: 1, Embedding: gateway.Embedding{0.1, 0.2}},
		{ID: 2, Embedding: gateway.Embedding{0.3, 0.4}},
	}
	chunks2 := []domainindex.Chunk{
		{ID: 3, Embedding: gateway.Embedding{0.5, 0.6}},
	}
	chunks3 := []domainindex.Chunk{
		{ID: 4, Embedding: gateway.Embedding{0.7, 0.8}},
		{ID: 5, Embedding: gateway.Embedding{0.9, 1.0}},
	}

	indexRepo := &mockIndexRepositoryForBuild{
		chunks: map[int64][]domainindex.Chunk{
			10: chunks1,
			20: chunks2,
			30: chunks3,
		},
	}
	snapshotRepo := &mockSnapshotRepositoryForBuild{
		versionIDs: []int64{10, 20, 30},
	}
	metaRepo := &mockMetaRepositoryForBuild{
		values: make(map[string][]byte),
	}

	service := NewService(indexRepo, snapshotRepo, metaRepo)
	err := service.BuildAndSave("multi-version")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify dump was saved
	dumpData, ok := metaRepo.values["idx_dump_multi-version"]
	if !ok {
		t.Fatal("expected index dump to be saved")
	}

	// Verify all 5 chunks are present
	var dump IndexDump
	dumpReader := bytes.NewReader(dumpData)
	dumpDecoder := gob.NewDecoder(dumpReader)
	if err := dumpDecoder.Decode(&dump); err != nil {
		t.Fatalf("failed to decode dump: %v", err)
	}

	if len(dump.ChunkIDToID) != 5 {
		t.Errorf("expected 5 chunks total, got %d", len(dump.ChunkIDToID))
	}
}

func TestService_BuildAndSave_EmbeddingConversion(t *testing.T) {
	// Test that float64 embeddings are correctly converted to float32
	chunk := domainindex.Chunk{
		ID:        1,
		Embedding: gateway.Embedding{0.123456789, 0.987654321, 0.555555555},
	}

	indexRepo := &mockIndexRepositoryForBuild{
		chunks: map[int64][]domainindex.Chunk{
			1: {chunk},
		},
	}
	snapshotRepo := &mockSnapshotRepositoryForBuild{
		versionIDs: []int64{1},
	}
	metaRepo := &mockMetaRepositoryForBuild{
		values: make(map[string][]byte),
	}

	service := NewService(indexRepo, snapshotRepo, metaRepo)
	err := service.BuildAndSave("conversion-test")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify dump was saved
	_, ok := metaRepo.values["idx_dump_conversion-test"]
	if !ok {
		t.Fatal("expected index dump to be saved")
	}
}
