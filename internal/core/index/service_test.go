// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package index

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/retran/meowg1k/internal/activities/scanworkspacestate"
	"github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
)

// Mock implementations for testing

type mockIndexRepository struct {
	FindVersionsByContentHashesFunc  func(ctx context.Context, contentHashes []string) (map[string]*domainindex.DocumentVersion, error)
	AddDocumentVersionWithChunksFunc func(ctx context.Context, doc domainindex.DocumentVersion, content []byte, chunks []domainindex.Chunk) (int64, error)
}

func (m *mockIndexRepository) FindVersionsByContentHashes(ctx context.Context, contentHashes []string) (map[string]*domainindex.DocumentVersion, error) {
	if m.FindVersionsByContentHashesFunc != nil {
		return m.FindVersionsByContentHashesFunc(ctx, contentHashes)
	}
	return make(map[string]*domainindex.DocumentVersion), nil
}

func (m *mockIndexRepository) AddDocumentVersionWithChunks(ctx context.Context, doc domainindex.DocumentVersion, content []byte, chunks []domainindex.Chunk) (int64, error) {
	if m.AddDocumentVersionWithChunksFunc != nil {
		return m.AddDocumentVersionWithChunksFunc(ctx, doc, content, chunks)
	}
	return 1, nil
}

// Implement other IndexRepository methods as no-ops
func (m *mockIndexRepository) AddDocumentVersion(ctx context.Context, doc domainindex.DocumentVersion, content []byte) (int64, error) {
	return 0, nil
}

func (m *mockIndexRepository) AddChunks(ctx context.Context, chunks []domainindex.Chunk) error {
	return nil
}

func (m *mockIndexRepository) FindVersionByContentHash(ctx context.Context, filePath, contentHash string) (*domainindex.DocumentVersion, error) {
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

func (m *mockIndexRepository) GetChunksByIDs(ctx context.Context, chunkIDs []int64) ([]domainindex.Chunk, error) {
	return nil, nil
}

func (m *mockIndexRepository) GetAllEmbeddings(ctx context.Context) (map[int64]gateway.Embedding, error) {
	return nil, nil
}

func (m *mockIndexRepository) GetVersionsByIDs(ctx context.Context, versionIDs []int64) ([]domainindex.DocumentVersion, error) {
	return nil, nil
}

func (m *mockIndexRepository) Checkpoint(ctx context.Context) error {
	return nil
}

type mockSnapshotRepository struct {
	ClearSnapshotLinksFunc       func(ctx context.Context, snapshotName string) error
	LinkVersionToSnapshotFunc    func(ctx context.Context, snapshotName string, versionID int64) error
	GetVersionIDsForSnapshotFunc func(ctx context.Context, commitHash string) ([]int64, error)
}

func (m *mockSnapshotRepository) ClearSnapshotLinks(ctx context.Context, snapshotName string) error {
	if m.ClearSnapshotLinksFunc != nil {
		return m.ClearSnapshotLinksFunc(ctx, snapshotName)
	}
	return nil
}

func (m *mockSnapshotRepository) LinkVersionToSnapshot(ctx context.Context, snapshotName string, versionID int64) error {
	if m.LinkVersionToSnapshotFunc != nil {
		return m.LinkVersionToSnapshotFunc(ctx, snapshotName, versionID)
	}
	return nil
}

func (m *mockSnapshotRepository) GetVersionIDsForSnapshot(ctx context.Context, commitHash string) ([]int64, error) {
	if m.GetVersionIDsForSnapshotFunc != nil {
		return m.GetVersionIDsForSnapshotFunc(ctx, commitHash)
	}
	return []int64{}, nil
}

func (m *mockSnapshotRepository) UnlinkVersionFromSnapshot(ctx context.Context, commitHash string, versionID int64) error {
	return nil
}

func TestNewService(t *testing.T) {
	t.Run("Valid parameters", func(t *testing.T) {
		indexRepo := &mockIndexRepository{}
		snapshotRepo := &mockSnapshotRepository{}

		service, err := NewService(indexRepo, snapshotRepo)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if service == nil {
			t.Fatal("Expected service to be non-nil")
		}
	})

	t.Run("Nil indexRepo", func(t *testing.T) {
		snapshotRepo := &mockSnapshotRepository{}

		service, err := NewService(nil, snapshotRepo)
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

	t.Run("Nil snapshotRepo", func(t *testing.T) {
		indexRepo := &mockIndexRepository{}

		service, err := NewService(indexRepo, nil)
		if err == nil {
			t.Fatal("Expected error for nil snapshotRepo")
		}
		if service != nil {
			t.Fatal("Expected service to be nil when error occurs")
		}
		if !strings.Contains(err.Error(), "snapshotRepo cannot be nil") {
			t.Errorf("Expected snapshotRepo error, got: %v", err)
		}
	})
}

func TestService_PrepareForProcessing(t *testing.T) {
	t.Run("Successful preparation with new files", func(t *testing.T) {
		indexRepo := &mockIndexRepository{
			FindVersionsByContentHashesFunc: func(ctx context.Context, contentHashes []string) (map[string]*domainindex.DocumentVersion, error) {
				return make(map[string]*domainindex.DocumentVersion), nil
			},
		}
		service, _ := NewService(indexRepo, &mockSnapshotRepository{})

		workspaceState := &scanworkspacestate.Output{
			HeadState: map[string]domainindex.FileState{
				"file1.go": {
					Content:     []byte("package main"),
					ContentHash: "hash1",
				},
			},
			StageState:   map[string]domainindex.FileState{},
			WorkdirState: map[string]domainindex.FileState{},
		}

		result, err := service.PrepareForProcessing(context.Background(), workspaceState)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		output, ok := result.(*PrepareOutput)
		if !ok {
			t.Fatalf("Expected *PrepareOutput, got %T", result)
		}

		if len(output.FilesToProcess) != 1 {
			t.Errorf("Expected 1 file to process, got %d", len(output.FilesToProcess))
		}
		if len(output.ExistingVersions) != 0 {
			t.Errorf("Expected 0 existing versions, got %d", len(output.ExistingVersions))
		}
	})

	t.Run("Invalid workspace state type", func(t *testing.T) {
		service, _ := NewService(&mockIndexRepository{}, &mockSnapshotRepository{})

		_, err := service.PrepareForProcessing(context.Background(), "invalid")
		if err == nil {
			t.Fatal("Expected error for invalid workspaceState type")
		}
		if !strings.Contains(err.Error(), "invalid workspaceState type") {
			t.Errorf("Expected type error, got: %v", err)
		}
	})

	t.Run("Nil workspace state", func(t *testing.T) {
		service, _ := NewService(&mockIndexRepository{}, &mockSnapshotRepository{})

		_, err := service.PrepareForProcessing(context.Background(), (*scanworkspacestate.Output)(nil))
		if err == nil {
			t.Fatal("Expected error for nil workspaceState")
		}
		if !strings.Contains(err.Error(), "workspaceState cannot be nil") {
			t.Errorf("Expected nil error, got: %v", err)
		}
	})

	t.Run("Deduplicates files with same content hash", func(t *testing.T) {
		indexRepo := &mockIndexRepository{}
		service, _ := NewService(indexRepo, &mockSnapshotRepository{})

		workspaceState := &scanworkspacestate.Output{
			HeadState: map[string]domainindex.FileState{
				"file1.go": {
					Content:     []byte("same content"),
					ContentHash: "hash1",
				},
				"file2.go": {
					Content:     []byte("same content"),
					ContentHash: "hash1",
				},
			},
			StageState:   map[string]domainindex.FileState{},
			WorkdirState: map[string]domainindex.FileState{},
		}

		result, err := service.PrepareForProcessing(context.Background(), workspaceState)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		output := result.(*PrepareOutput)
		if len(output.FilesToProcess) != 1 {
			t.Errorf("Expected 1 file to process (deduplicated), got %d", len(output.FilesToProcess))
		}
	})

	t.Run("Filters existing versions", func(t *testing.T) {
		indexRepo := &mockIndexRepository{
			FindVersionsByContentHashesFunc: func(ctx context.Context, contentHashes []string) (map[string]*domainindex.DocumentVersion, error) {
				return map[string]*domainindex.DocumentVersion{
					"hash1": {ID: 123, ContentHash: "hash1"},
				}, nil
			},
		}
		service, _ := NewService(indexRepo, &mockSnapshotRepository{})

		workspaceState := &scanworkspacestate.Output{
			HeadState: map[string]domainindex.FileState{
				"file1.go": {
					Content:     []byte("content"),
					ContentHash: "hash1",
				},
			},
			StageState:   map[string]domainindex.FileState{},
			WorkdirState: map[string]domainindex.FileState{},
		}

		result, err := service.PrepareForProcessing(context.Background(), workspaceState)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		output := result.(*PrepareOutput)
		if len(output.FilesToProcess) != 0 {
			t.Errorf("Expected 0 files to process (already exists), got %d", len(output.FilesToProcess))
		}
		if len(output.ExistingVersions) != 1 {
			t.Errorf("Expected 1 existing version, got %d", len(output.ExistingVersions))
		}
		if output.ExistingVersions["hash1"] != 123 {
			t.Errorf("Expected version ID 123, got %d", output.ExistingVersions["hash1"])
		}
	})

	t.Run("Handles multiple snapshots", func(t *testing.T) {
		indexRepo := &mockIndexRepository{}
		service, _ := NewService(indexRepo, &mockSnapshotRepository{})

		workspaceState := &scanworkspacestate.Output{
			HeadState: map[string]domainindex.FileState{
				"head.go": {
					Content:     []byte("head content"),
					ContentHash: "hash_head",
				},
			},
			StageState: map[string]domainindex.FileState{
				"stage.go": {
					Content:     []byte("stage content"),
					ContentHash: "hash_stage",
				},
			},
			WorkdirState: map[string]domainindex.FileState{
				"workdir.go": {
					Content:     []byte("workdir content"),
					ContentHash: "hash_workdir",
				},
			},
		}

		result, err := service.PrepareForProcessing(context.Background(), workspaceState)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		output := result.(*PrepareOutput)
		if len(output.FilesToProcess) != 3 {
			t.Errorf("Expected 3 files to process, got %d", len(output.FilesToProcess))
		}
		if len(output.ContentHashMap) != 3 {
			t.Errorf("Expected 3 content hash mappings, got %d", len(output.ContentHashMap))
		}
	})

	t.Run("Repository error propagates", func(t *testing.T) {
		indexRepo := &mockIndexRepository{
			FindVersionsByContentHashesFunc: func(ctx context.Context, contentHashes []string) (map[string]*domainindex.DocumentVersion, error) {
				return nil, errors.New("database error")
			},
		}
		service, _ := NewService(indexRepo, &mockSnapshotRepository{})

		workspaceState := &scanworkspacestate.Output{
			HeadState: map[string]domainindex.FileState{
				"file1.go": {
					Content:     []byte("content"),
					ContentHash: "hash1",
				},
			},
			StageState:   map[string]domainindex.FileState{},
			WorkdirState: map[string]domainindex.FileState{},
		}

		_, err := service.PrepareForProcessing(context.Background(), workspaceState)
		if err == nil {
			t.Fatal("Expected error from repository")
		}
		if !strings.Contains(err.Error(), "failed to find existing versions") {
			t.Errorf("Expected repository error, got: %v", err)
		}
	})
}

func TestService_SaveNewVersion(t *testing.T) {
	t.Run("Successful save", func(t *testing.T) {
		indexRepo := &mockIndexRepository{
			AddDocumentVersionWithChunksFunc: func(ctx context.Context, doc domainindex.DocumentVersion, content []byte, chunks []domainindex.Chunk) (int64, error) {
				return 42, nil
			},
		}
		service, _ := NewService(indexRepo, &mockSnapshotRepository{})

		input := &SaveVersionInput{
			FilePath:    "test.go",
			Content:     []byte("package main"),
			ContentHash: "hash123",
			Chunks: []domainindex.ChunkData{
				{
					StartLine:   1,
					EndLine:     5,
					TextContent: "package main",
				},
			},
			Embeddings: []gateway.Embedding{
				{0.1, 0.2, 0.3},
			},
		}

		result, err := service.SaveNewVersion(context.Background(), input)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		output, ok := result.(*SaveVersionOutput)
		if !ok {
			t.Fatalf("Expected *SaveVersionOutput, got %T", result)
		}

		if output.VersionID != 42 {
			t.Errorf("Expected version ID 42, got %d", output.VersionID)
		}
		if output.FilePath != "test.go" {
			t.Errorf("Expected file path 'test.go', got %s", output.FilePath)
		}
	})

	t.Run("Invalid input type", func(t *testing.T) {
		service, _ := NewService(&mockIndexRepository{}, &mockSnapshotRepository{})

		_, err := service.SaveNewVersion(context.Background(), "invalid")
		if err == nil {
			t.Fatal("Expected error for invalid input type")
		}
		if !strings.Contains(err.Error(), "invalid input type") {
			t.Errorf("Expected type error, got: %v", err)
		}
	})

	t.Run("Nil input", func(t *testing.T) {
		service, _ := NewService(&mockIndexRepository{}, &mockSnapshotRepository{})

		_, err := service.saveNewVersionImpl(context.Background(), nil)
		if err == nil {
			t.Fatal("Expected error for nil input")
		}
		if !strings.Contains(err.Error(), "input cannot be nil") {
			t.Errorf("Expected nil input error, got: %v", err)
		}
	})

	t.Run("Chunk and embedding count mismatch", func(t *testing.T) {
		service, _ := NewService(&mockIndexRepository{}, &mockSnapshotRepository{})

		input := &SaveVersionInput{
			FilePath:    "test.go",
			Content:     []byte("content"),
			ContentHash: "hash",
			Chunks: []domainindex.ChunkData{
				{TextContent: "chunk1"},
				{TextContent: "chunk2"},
			},
			Embeddings: []gateway.Embedding{
				{0.1, 0.2},
			},
		}

		_, err := service.saveNewVersionImpl(context.Background(), input)
		if err == nil {
			t.Fatal("Expected error for chunk/embedding mismatch")
		}
		if !strings.Contains(err.Error(), "does not match embedding count") {
			t.Errorf("Expected mismatch error, got: %v", err)
		}
	})

	t.Run("Saves with empty chunks", func(t *testing.T) {
		indexRepo := &mockIndexRepository{
			AddDocumentVersionWithChunksFunc: func(ctx context.Context, doc domainindex.DocumentVersion, content []byte, chunks []domainindex.Chunk) (int64, error) {
				if len(chunks) != 0 {
					return 0, errors.New("expected no chunks")
				}
				return 1, nil
			},
		}
		service, _ := NewService(indexRepo, &mockSnapshotRepository{})

		input := &SaveVersionInput{
			FilePath:    "test.go",
			Content:     []byte("content"),
			ContentHash: "hash",
			Chunks:      []domainindex.ChunkData{},
			Embeddings:  []gateway.Embedding{},
		}

		result, err := service.saveNewVersionImpl(context.Background(), input)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if result == nil {
			t.Fatal("Expected result to be non-nil")
		}
	})

	t.Run("Repository error propagates", func(t *testing.T) {
		indexRepo := &mockIndexRepository{
			AddDocumentVersionWithChunksFunc: func(ctx context.Context, doc domainindex.DocumentVersion, content []byte, chunks []domainindex.Chunk) (int64, error) {
				return 0, errors.New("database error")
			},
		}
		service, _ := NewService(indexRepo, &mockSnapshotRepository{})

		input := &SaveVersionInput{
			FilePath:    "test.go",
			Content:     []byte("content"),
			ContentHash: "hash",
			Chunks:      []domainindex.ChunkData{},
			Embeddings:  []gateway.Embedding{},
		}

		_, err := service.saveNewVersionImpl(context.Background(), input)
		if err == nil {
			t.Fatal("Expected error from repository")
		}
		if !strings.Contains(err.Error(), "failed to add document version") {
			t.Errorf("Expected repository error, got: %v", err)
		}
	})

	t.Run("Creates correct chunk structure", func(t *testing.T) {
		var savedChunks []domainindex.Chunk
		indexRepo := &mockIndexRepository{
			AddDocumentVersionWithChunksFunc: func(ctx context.Context, doc domainindex.DocumentVersion, content []byte, chunks []domainindex.Chunk) (int64, error) {
				savedChunks = chunks
				return 1, nil
			},
		}
		service, _ := NewService(indexRepo, &mockSnapshotRepository{})

		input := &SaveVersionInput{
			FilePath:    "test.go",
			Content:     []byte("content"),
			ContentHash: "hash",
			Chunks: []domainindex.ChunkData{
				{
					StartLine:   1,
					EndLine:     10,
					StartByte:   0,
					EndByte:     100,
					StartRune:   0,
					EndRune:     100,
					TextContent: "chunk text",
				},
			},
			Embeddings: []gateway.Embedding{
				{0.1, 0.2, 0.3},
			},
		}

		_, err := service.saveNewVersionImpl(context.Background(), input)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(savedChunks) != 1 {
			t.Fatalf("Expected 1 chunk, got %d", len(savedChunks))
		}

		chunk := savedChunks[0]
		if chunk.ChunkType != "plain_text" {
			t.Errorf("Expected chunk type 'plain_text', got %s", chunk.ChunkType)
		}
		if chunk.StartLine != 1 {
			t.Errorf("Expected start line 1, got %d", chunk.StartLine)
		}
		if chunk.EndLine != 10 {
			t.Errorf("Expected end line 10, got %d", chunk.EndLine)
		}
		if chunk.TextContent != "chunk text" {
			t.Errorf("Expected text 'chunk text', got %s", chunk.TextContent)
		}
		if len(chunk.Embedding) != 3 {
			t.Errorf("Expected embedding length 3, got %d", len(chunk.Embedding))
		}
	})
}

func TestService_FinalizeLiveSnapshots(t *testing.T) {
	t.Run("Successful finalization", func(t *testing.T) {
		clearCalls := 0
		linkCalls := 0
		snapshotRepo := &mockSnapshotRepository{
			ClearSnapshotLinksFunc: func(ctx context.Context, snapshotName string) error {
				clearCalls++
				return nil
			},
			LinkVersionToSnapshotFunc: func(ctx context.Context, snapshotName string, versionID int64) error {
				linkCalls++
				return nil
			},
		}
		service, _ := NewService(&mockIndexRepository{}, snapshotRepo)

		input := &FinalizeInput{
			ScanResult: &scanworkspacestate.Output{
				HeadState: map[string]domainindex.FileState{
					"file.go": {Content: []byte("content"), ContentHash: "hash1"},
				},
				StageState:   map[string]domainindex.FileState{},
				WorkdirState: map[string]domainindex.FileState{},
			},
			ExistingVersions: map[string]int64{"hash1": 123},
			NewVersions:      map[string]int64{},
		}

		err := service.FinalizeLiveSnapshots(context.Background(), input)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if clearCalls != 3 {
			t.Errorf("Expected 3 clear calls (_head_, _stage_, _workdir_), got %d", clearCalls)
		}
		if linkCalls != 1 {
			t.Errorf("Expected 1 link call, got %d", linkCalls)
		}
	})

	t.Run("Invalid input type", func(t *testing.T) {
		service, _ := NewService(&mockIndexRepository{}, &mockSnapshotRepository{})

		err := service.FinalizeLiveSnapshots(context.Background(), "invalid")
		if err == nil {
			t.Fatal("Expected error for invalid input type")
		}
		if !strings.Contains(err.Error(), "invalid input type") {
			t.Errorf("Expected type error, got: %v", err)
		}
	})

	t.Run("Nil input", func(t *testing.T) {
		service, _ := NewService(&mockIndexRepository{}, &mockSnapshotRepository{})

		err := service.finalizeLiveSnapshotsImpl(context.Background(), nil)
		if err == nil {
			t.Fatal("Expected error for nil input")
		}
		if !strings.Contains(err.Error(), "input cannot be nil") {
			t.Errorf("Expected nil input error, got: %v", err)
		}
	})

	t.Run("Nil scan result", func(t *testing.T) {
		service, _ := NewService(&mockIndexRepository{}, &mockSnapshotRepository{})

		input := &FinalizeInput{
			ScanResult:       nil,
			ExistingVersions: map[string]int64{},
			NewVersions:      map[string]int64{},
		}

		err := service.finalizeLiveSnapshotsImpl(context.Background(), input)
		if err == nil {
			t.Fatal("Expected error for nil scanResult")
		}
		if !strings.Contains(err.Error(), "scanResult cannot be nil") {
			t.Errorf("Expected nil scanResult error, got: %v", err)
		}
	})

	t.Run("Merges existing and new versions", func(t *testing.T) {
		linkedVersions := make(map[int64]bool)
		snapshotRepo := &mockSnapshotRepository{
			LinkVersionToSnapshotFunc: func(ctx context.Context, snapshotName string, versionID int64) error {
				linkedVersions[versionID] = true
				return nil
			},
		}
		service, _ := NewService(&mockIndexRepository{}, snapshotRepo)

		input := &FinalizeInput{
			ScanResult: &scanworkspacestate.Output{
				HeadState: map[string]domainindex.FileState{
					"file1.go": {Content: []byte("c1"), ContentHash: "hash1"},
					"file2.go": {Content: []byte("c2"), ContentHash: "hash2"},
				},
				StageState:   map[string]domainindex.FileState{},
				WorkdirState: map[string]domainindex.FileState{},
			},
			ExistingVersions: map[string]int64{"hash1": 100},
			NewVersions:      map[string]int64{"hash2": 200},
		}

		err := service.finalizeLiveSnapshotsImpl(context.Background(), input)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if !linkedVersions[100] {
			t.Error("Expected existing version 100 to be linked")
		}
		if !linkedVersions[200] {
			t.Error("Expected new version 200 to be linked")
		}
	})

	t.Run("Clear error propagates", func(t *testing.T) {
		snapshotRepo := &mockSnapshotRepository{
			ClearSnapshotLinksFunc: func(ctx context.Context, snapshotName string) error {
				return errors.New("clear error")
			},
		}
		service, _ := NewService(&mockIndexRepository{}, snapshotRepo)

		input := &FinalizeInput{
			ScanResult: &scanworkspacestate.Output{
				HeadState:    map[string]domainindex.FileState{},
				StageState:   map[string]domainindex.FileState{},
				WorkdirState: map[string]domainindex.FileState{},
			},
			ExistingVersions: map[string]int64{},
			NewVersions:      map[string]int64{},
		}

		err := service.finalizeLiveSnapshotsImpl(context.Background(), input)
		if err == nil {
			t.Fatal("Expected error from clear")
		}
		if !strings.Contains(err.Error(), "failed to finalize _head_ snapshot") {
			t.Errorf("Expected clear error, got: %v", err)
		}
	})

	t.Run("Link error propagates", func(t *testing.T) {
		snapshotRepo := &mockSnapshotRepository{
			LinkVersionToSnapshotFunc: func(ctx context.Context, snapshotName string, versionID int64) error {
				return errors.New("link error")
			},
		}
		service, _ := NewService(&mockIndexRepository{}, snapshotRepo)

		input := &FinalizeInput{
			ScanResult: &scanworkspacestate.Output{
				HeadState: map[string]domainindex.FileState{
					"file.go": {Content: []byte("content"), ContentHash: "hash1"},
				},
				StageState:   map[string]domainindex.FileState{},
				WorkdirState: map[string]domainindex.FileState{},
			},
			ExistingVersions: map[string]int64{"hash1": 123},
			NewVersions:      map[string]int64{},
		}

		err := service.finalizeLiveSnapshotsImpl(context.Background(), input)
		if err == nil {
			t.Fatal("Expected error from link")
		}
		if !strings.Contains(err.Error(), "failed to finalize _head_ snapshot") {
			t.Errorf("Expected link error, got: %v", err)
		}
	})

	t.Run("Processes all three snapshots", func(t *testing.T) {
		snapshotNames := make(map[string]int)
		snapshotRepo := &mockSnapshotRepository{
			ClearSnapshotLinksFunc: func(ctx context.Context, snapshotName string) error {
				snapshotNames[snapshotName]++
				return nil
			},
		}
		service, _ := NewService(&mockIndexRepository{}, snapshotRepo)

		input := &FinalizeInput{
			ScanResult: &scanworkspacestate.Output{
				HeadState: map[string]domainindex.FileState{
					"head.go": {Content: []byte("h"), ContentHash: "hash_h"},
				},
				StageState: map[string]domainindex.FileState{
					"stage.go": {Content: []byte("s"), ContentHash: "hash_s"},
				},
				WorkdirState: map[string]domainindex.FileState{
					"work.go": {Content: []byte("w"), ContentHash: "hash_w"},
				},
			},
			ExistingVersions: map[string]int64{
				"hash_h": 1,
				"hash_s": 2,
				"hash_w": 3,
			},
			NewVersions: map[string]int64{},
		}

		err := service.finalizeLiveSnapshotsImpl(context.Background(), input)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if snapshotNames["_head_"] != 1 {
			t.Errorf("Expected 1 clear for _head_, got %d", snapshotNames["_head_"])
		}
		if snapshotNames["_stage_"] != 1 {
			t.Errorf("Expected 1 clear for _stage_, got %d", snapshotNames["_stage_"])
		}
		if snapshotNames["_workdir_"] != 1 {
			t.Errorf("Expected 1 clear for _workdir_, got %d", snapshotNames["_workdir_"])
		}
	})
}

func TestService_FinalizeSnapshot(t *testing.T) {
	t.Run("Missing version for content hash", func(t *testing.T) {
		service, _ := NewService(&mockIndexRepository{}, &mockSnapshotRepository{})

		fileStates := map[string]domainindex.FileState{
			"file.go": {Content: []byte("content"), ContentHash: "hash_missing"},
		}
		versionMap := map[string]int64{
			"hash_other": 123,
		}

		err := service.finalizeSnapshot(context.Background(), "_head_", fileStates, versionMap)
		if err == nil {
			t.Fatal("Expected error for missing version")
		}
		if !strings.Contains(err.Error(), "inconsistency detected") {
			t.Errorf("Expected inconsistency error, got: %v", err)
		}
	})
}

func TestSaveVersionInput_DocumentVersion(t *testing.T) {
	t.Run("Creates document version with correct fields", func(t *testing.T) {
		var savedDoc domainindex.DocumentVersion
		indexRepo := &mockIndexRepository{
			AddDocumentVersionWithChunksFunc: func(ctx context.Context, doc domainindex.DocumentVersion, content []byte, chunks []domainindex.Chunk) (int64, error) {
				savedDoc = doc
				return 1, nil
			},
		}
		service, _ := NewService(indexRepo, &mockSnapshotRepository{})

		input := &SaveVersionInput{
			FilePath:    "path/to/file.go",
			Content:     []byte("file content"),
			ContentHash: "abc123",
			Chunks:      []domainindex.ChunkData{},
			Embeddings:  []gateway.Embedding{},
		}

		_, err := service.saveNewVersionImpl(context.Background(), input)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if savedDoc.FilePath != "path/to/file.go" {
			t.Errorf("Expected file path 'path/to/file.go', got %s", savedDoc.FilePath)
		}
		if savedDoc.ContentHash != "abc123" {
			t.Errorf("Expected content hash 'abc123', got %s", savedDoc.ContentHash)
		}
		if savedDoc.GitCommitHashFirstSeen.Valid {
			t.Error("Expected GitCommitHashFirstSeen to be invalid/null")
		}
	})
}
