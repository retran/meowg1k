// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package index

import (
	"bytes"
	"context"
	"database/sql"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
)

// mockHost is a simple mock implementation of ports.Host for testing.
type mockHost struct {
	db *sql.DB
}

func newMockHost(db *sql.DB) ports.Host {
	return &mockHost{db: db}
}

func (m *mockHost) GetMainDB() (*sql.DB, error) {
	return m.db, nil
}

func (m *mockHost) GetProjectDB() (*sql.DB, error) {
	return m.db, nil
}

func (m *mockHost) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// setupTestDB creates an in-memory SQLite database for testing.
func setupTestDB(t *testing.T) (*sql.DB, ports.Host) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Run migrations
	for _, migration := range Migrations {
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatalf("failed to begin transaction for migration %d: %v", migration.Version, err)
		}

		if err := migration.Up(tx); err != nil {
			tx.Rollback()
			t.Fatalf("failed to run migration %d: %v", migration.Version, err)
		}

		if err := tx.Commit(); err != nil {
			t.Fatalf("failed to commit migration %d: %v", migration.Version, err)
		}
	}

	host := newMockHost(db)
	return db, host
}

func TestRepository_AddDocumentVersion(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	doc := domainindex.DocumentVersion{
		FilePath:               "main.go",
		GitCommitHashFirstSeen: sql.NullString{String: "abc123", Valid: true},
		ContentHash:            "hash1",
	}
	content := []byte("package main")

	id, err := repo.AddDocumentVersion(ctx, &doc, content)
	if err != nil {
		t.Fatalf("AddDocumentVersion() error = %v", err)
	}

	if id <= 0 {
		t.Errorf("AddDocumentVersion() returned invalid ID: %d", id)
	}

	// Verify content blob was created
	exists, err := repo.FindContentBlob(ctx, "hash1")
	if err != nil {
		t.Fatalf("FindContentBlob() error = %v", err)
	}
	if !exists {
		t.Error("Content blob was not created")
	}

	// Verify content can be retrieved
	retrievedContent, err := repo.GetContentBlob(ctx, "hash1")
	if err != nil {
		t.Fatalf("GetContentBlob() error = %v", err)
	}
	if !bytes.Equal(retrievedContent, content) {
		t.Errorf("GetContentBlob() = %q, want %q", retrievedContent, content)
	}
}

func TestRepository_AddDocumentVersion_DuplicateContent(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	content := []byte("package main")

	// Add first version
	doc1 := domainindex.DocumentVersion{
		FilePath:               "main.go",
		GitCommitHashFirstSeen: sql.NullString{String: "abc123", Valid: true},
		ContentHash:            "hash1",
	}
	_, err := repo.AddDocumentVersion(ctx, &doc1, content)
	if err != nil {
		t.Fatalf("AddDocumentVersion() first call error = %v", err)
	}

	// Add second version with same content hash (should not fail)
	doc2 := domainindex.DocumentVersion{
		FilePath:               "main2.go",
		GitCommitHashFirstSeen: sql.NullString{String: "def456", Valid: true},
		ContentHash:            "hash1",
	}
	_, err = repo.AddDocumentVersion(ctx, &doc2, content)
	if err != nil {
		t.Fatalf("AddDocumentVersion() second call error = %v", err)
	}
}

func TestRepository_AddChunks(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// First add a document version
	doc := domainindex.DocumentVersion{
		FilePath:               "test.go",
		GitCommitHashFirstSeen: sql.NullString{String: "abc", Valid: true},
		ContentHash:            "hash1",
	}
	versionID, err := repo.AddDocumentVersion(ctx, &doc, []byte("content"))
	if err != nil {
		t.Fatalf("AddDocumentVersion() error = %v", err)
	}

	// Add chunks
	chunks := []domainindex.Chunk{
		{
			DocumentVersionID: versionID,
			ChunkType:         "function",
			TextContent:       "func main() {}",
			StartByte:         0,
			EndByte:           14,
			StartRune:         0,
			EndRune:           14,
			StartLine:         1,
			EndLine:           1,
			Embedding:         gateway.Embedding{0.1, 0.2, 0.3},
		},
		{
			DocumentVersionID: versionID,
			ChunkType:         "comment",
			TextContent:       "// Comment",
			StartByte:         15,
			EndByte:           25,
			StartRune:         15,
			EndRune:           25,
			StartLine:         2,
			EndLine:           2,
			Embedding:         gateway.Embedding{0.4, 0.5, 0.6},
		},
	}

	err = repo.AddChunks(ctx, chunks)
	if err != nil {
		t.Fatalf("AddChunks() error = %v", err)
	}

	// Verify chunks were added
	retrieved, err := repo.GetChunksByVersionID(ctx, versionID)
	if err != nil {
		t.Fatalf("GetChunksByVersionID() error = %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("GetChunksByVersionID() returned %d chunks, want 2", len(retrieved))
	}

	// Verify first chunk
	if retrieved[0].TextContent != "func main() {}" {
		t.Errorf("chunk[0].TextContent = %q, want %q", retrieved[0].TextContent, "func main() {}")
	}
	if len(retrieved[0].Embedding) != 3 {
		t.Errorf("chunk[0].Embedding length = %d, want 3", len(retrieved[0].Embedding))
	}
}

func TestRepository_FindVersionByContentHash(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	doc := domainindex.DocumentVersion{
		FilePath:               "test.go",
		GitCommitHashFirstSeen: sql.NullString{String: "abc", Valid: true},
		ContentHash:            "hash1",
	}
	id, err := repo.AddDocumentVersion(ctx, &doc, []byte("content"))
	if err != nil {
		t.Fatalf("AddDocumentVersion() error = %v", err)
	}

	// Find existing version
	found, err := repo.FindVersionByContentHash(ctx, "test.go", "hash1")
	if err != nil {
		t.Fatalf("FindVersionByContentHash() error = %v", err)
	}
	if found == nil {
		t.Fatal("FindVersionByContentHash() returned nil")
	}
	if found.ID != id {
		t.Errorf("FindVersionByContentHash().ID = %d, want %d", found.ID, id)
	}

	// Find non-existing version
	notFound, err := repo.FindVersionByContentHash(ctx, "test.go", "nonexistent")
	if err != nil {
		t.Fatalf("FindVersionByContentHash() error = %v", err)
	}
	if notFound != nil {
		t.Error("FindVersionByContentHash() should return nil for non-existing version")
	}
}

func TestRepository_FindVersionsByFilePath(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Add multiple versions of the same file
	versions := []domainindex.DocumentVersion{
		{
			FilePath:               "test.go",
			GitCommitHashFirstSeen: sql.NullString{String: "v1", Valid: true},
			ContentHash:            "hash1",
		},
		{
			FilePath:               "test.go",
			GitCommitHashFirstSeen: sql.NullString{String: "v2", Valid: true},
			ContentHash:            "hash2",
		},
		{
			FilePath:               "other.go",
			GitCommitHashFirstSeen: sql.NullString{String: "v1", Valid: true},
			ContentHash:            "hash3",
		},
	}

	for i, v := range versions {
		_, err := repo.AddDocumentVersion(ctx, &v, []byte("content"+string(rune(i))))
		if err != nil {
			t.Fatalf("AddDocumentVersion() error = %v", err)
		}
	}

	// Find versions for test.go
	found, err := repo.FindVersionsByFilePath(ctx, "test.go")
	if err != nil {
		t.Fatalf("FindVersionsByFilePath() error = %v", err)
	}

	if len(found) != 2 {
		t.Errorf("FindVersionsByFilePath() returned %d versions, want 2", len(found))
	}

	// Verify that both expected hashes are present
	hashSet := make(map[string]bool)
	for _, v := range found {
		hashSet[v.ContentHash] = true
	}
	if !hashSet["hash1"] || !hashSet["hash2"] {
		t.Errorf("FindVersionsByFilePath() missing expected hashes, got: %v", hashSet)
	}

	// Verify they're all for test.go
	for _, v := range found {
		if v.FilePath != "test.go" {
			t.Errorf("FindVersionsByFilePath() returned version with wrong file path: %s", v.FilePath)
		}
	}
}

func TestRepository_GetAllEmbeddings(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Add document and chunks
	doc := domainindex.DocumentVersion{
		FilePath:    "test.go",
		ContentHash: "hash1",
	}
	versionID, err := repo.AddDocumentVersion(ctx, &doc, []byte("content"))
	if err != nil {
		t.Fatalf("AddDocumentVersion() error = %v", err)
	}

	chunks := []domainindex.Chunk{
		{
			DocumentVersionID: versionID,
			ChunkType:         "function",
			TextContent:       "func1",
			Embedding:         gateway.Embedding{0.1, 0.2},
		},
		{
			DocumentVersionID: versionID,
			ChunkType:         "function",
			TextContent:       "func2",
			Embedding:         gateway.Embedding{0.3, 0.4},
		},
	}

	err = repo.AddChunks(ctx, chunks)
	if err != nil {
		t.Fatalf("AddChunks() error = %v", err)
	}

	// Get all embeddings
	embeddings, err := repo.GetAllEmbeddings(ctx)
	if err != nil {
		t.Fatalf("GetAllEmbeddings() error = %v", err)
	}

	if len(embeddings) != 2 {
		t.Errorf("GetAllEmbeddings() returned %d embeddings, want 2", len(embeddings))
	}

	// Verify embeddings are correct
	for _, emb := range embeddings {
		if len(emb) != 2 {
			t.Errorf("embedding length = %d, want 2", len(emb))
		}
	}
}

func TestRepository_LinkVersionToSnapshot(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Add document version
	doc := domainindex.DocumentVersion{
		FilePath:    "test.go",
		ContentHash: "hash1",
	}
	versionID, err := repo.AddDocumentVersion(ctx, &doc, []byte("content"))
	if err != nil {
		t.Fatalf("AddDocumentVersion() error = %v", err)
	}

	// Link to snapshot
	err = repo.LinkVersionToSnapshot(ctx, "commit123", versionID)
	if err != nil {
		t.Fatalf("LinkVersionToSnapshot() error = %v", err)
	}

	// Verify link exists
	ids, err := repo.GetVersionIDsForSnapshot(ctx, "commit123")
	if err != nil {
		t.Fatalf("GetVersionIDsForSnapshot() error = %v", err)
	}

	if len(ids) != 1 {
		t.Errorf("GetVersionIDsForSnapshot() returned %d IDs, want 1", len(ids))
	}
	if len(ids) > 0 && ids[0] != versionID {
		t.Errorf("GetVersionIDsForSnapshot() returned ID %d, want %d", ids[0], versionID)
	}

	// Link to same snapshot again (should not error due to ON CONFLICT)
	err = repo.LinkVersionToSnapshot(ctx, "commit123", versionID)
	if err != nil {
		t.Errorf("LinkVersionToSnapshot() duplicate link error = %v", err)
	}
}

func TestRepository_UnlinkVersionFromSnapshot(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Add document version and link
	doc := domainindex.DocumentVersion{
		FilePath:    "test.go",
		ContentHash: "hash1",
	}
	versionID, err := repo.AddDocumentVersion(ctx, &doc, []byte("content"))
	if err != nil {
		t.Fatalf("AddDocumentVersion() error = %v", err)
	}

	err = repo.LinkVersionToSnapshot(ctx, "commit123", versionID)
	if err != nil {
		t.Fatalf("LinkVersionToSnapshot() error = %v", err)
	}

	// Unlink
	err = repo.UnlinkVersionFromSnapshot(ctx, "commit123", versionID)
	if err != nil {
		t.Fatalf("UnlinkVersionFromSnapshot() error = %v", err)
	}

	// Verify link is removed
	ids, err := repo.GetVersionIDsForSnapshot(ctx, "commit123")
	if err != nil {
		t.Fatalf("GetVersionIDsForSnapshot() error = %v", err)
	}

	if len(ids) != 0 {
		t.Errorf("GetVersionIDsForSnapshot() returned %d IDs, want 0", len(ids))
	}
}

func TestRepository_ClearSnapshotLinks(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Add multiple document versions
	for i := 0; i < 3; i++ {
		doc := domainindex.DocumentVersion{
			FilePath:    "test.go",
			ContentHash: "hash" + string(rune('1'+i)),
		}
		id, err := repo.AddDocumentVersion(ctx, &doc, []byte("content"))
		if err != nil {
			t.Fatalf("AddDocumentVersion() error = %v", err)
		}

		// Link all to same snapshot
		err = repo.LinkVersionToSnapshot(ctx, "commit123", id)
		if err != nil {
			t.Fatalf("LinkVersionToSnapshot() error = %v", err)
		}
	}

	// Clear all links
	err := repo.ClearSnapshotLinks(ctx, "commit123")
	if err != nil {
		t.Fatalf("ClearSnapshotLinks() error = %v", err)
	}

	// Verify all links are removed
	ids, err := repo.GetVersionIDsForSnapshot(ctx, "commit123")
	if err != nil {
		t.Fatalf("GetVersionIDsForSnapshot() error = %v", err)
	}

	if len(ids) != 0 {
		t.Errorf("GetVersionIDsForSnapshot() returned %d IDs, want 0", len(ids))
	}
}

func TestRepository_GetVersionsByIDs(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Add multiple versions
	var expectedIDs []int64
	for i := 0; i < 3; i++ {
		doc := domainindex.DocumentVersion{
			FilePath:    "test" + string(rune('0'+i)) + ".go",
			ContentHash: "hash" + string(rune('1'+i)),
		}
		id, err := repo.AddDocumentVersion(ctx, &doc, []byte("content"))
		if err != nil {
			t.Fatalf("AddDocumentVersion() error = %v", err)
		}
		expectedIDs = append(expectedIDs, id)
	}

	// Get versions by IDs
	versions, err := repo.GetVersionsByIDs(ctx, expectedIDs)
	if err != nil {
		t.Fatalf("GetVersionsByIDs() error = %v", err)
	}

	if len(versions) != 3 {
		t.Errorf("GetVersionsByIDs() returned %d versions, want 3", len(versions))
	}

	// Test with empty slice
	emptyVersions, err := repo.GetVersionsByIDs(ctx, []int64{})
	if err != nil {
		t.Fatalf("GetVersionsByIDs() with empty slice error = %v", err)
	}

	if len(emptyVersions) != 0 {
		t.Errorf("GetVersionsByIDs() with empty slice returned %d versions, want 0", len(emptyVersions))
	}
}

func TestRepository_AddDocumentVersionWithChunks(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	doc := domainindex.DocumentVersion{
		FilePath:               "main.go",
		GitCommitHashFirstSeen: sql.NullString{String: "abc123", Valid: true},
		ContentHash:            "hash1",
	}
	content := []byte("package main\n\nfunc main() {}")

	chunks := []domainindex.Chunk{
		{
			StartLine: 1,
			EndLine:   1,
			Embedding: gateway.Embedding{0.1, 0.2, 0.3},
		},
		{
			StartLine: 3,
			EndLine:   3,
			Embedding: gateway.Embedding{0.4, 0.5, 0.6},
		},
	}

	// Test with chunks
	id, err := repo.AddDocumentVersionWithChunks(ctx, &doc, content, chunks)
	if err != nil {
		t.Fatalf("AddDocumentVersionWithChunks() error = %v", err)
	}

	if id <= 0 {
		t.Errorf("AddDocumentVersionWithChunks() returned invalid ID: %d", id)
	}

	// Verify content blob was created
	exists, err := repo.FindContentBlob(ctx, "hash1")
	if err != nil {
		t.Fatalf("FindContentBlob() error = %v", err)
	}
	if !exists {
		t.Error("Content blob was not created")
	}

	// Verify chunks were created
	retrievedChunks, err := repo.GetChunksByVersionID(ctx, id)
	if err != nil {
		t.Fatalf("GetChunksByVersionID() error = %v", err)
	}
	if len(retrievedChunks) != 2 {
		t.Errorf("Expected 2 chunks, got %d", len(retrievedChunks))
	}

	// Test with empty chunks
	doc2 := domainindex.DocumentVersion{
		FilePath:    "empty.go",
		ContentHash: "hash2",
	}
	id2, err := repo.AddDocumentVersionWithChunks(ctx, &doc2, []byte("empty"), []domainindex.Chunk{})
	if err != nil {
		t.Fatalf("AddDocumentVersionWithChunks() with no chunks error = %v", err)
	}
	if id2 <= 0 {
		t.Errorf("AddDocumentVersionWithChunks() with no chunks returned invalid ID: %d", id2)
	}

	// Test with nil doc
	_, err = repo.AddDocumentVersionWithChunks(ctx, nil, content, chunks)
	if err == nil {
		t.Error("AddDocumentVersionWithChunks() with nil doc should return error")
	}
}

func TestRepository_FindVersionsByContentHashes(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Add multiple versions with different content hashes
	doc1 := domainindex.DocumentVersion{
		FilePath:    "file1.go",
		ContentHash: "hash1",
	}
	doc2 := domainindex.DocumentVersion{
		FilePath:    "file2.go",
		ContentHash: "hash2",
	}
	doc3 := domainindex.DocumentVersion{
		FilePath:    "file3.go",
		ContentHash: "hash3",
	}

	_, err := repo.AddDocumentVersion(ctx, &doc1, []byte("content1"))
	if err != nil {
		t.Fatalf("AddDocumentVersion() error = %v", err)
	}
	_, err = repo.AddDocumentVersion(ctx, &doc2, []byte("content2"))
	if err != nil {
		t.Fatalf("AddDocumentVersion() error = %v", err)
	}
	_, err = repo.AddDocumentVersion(ctx, &doc3, []byte("content3"))
	if err != nil {
		t.Fatalf("AddDocumentVersion() error = %v", err)
	}

	// Test finding by multiple hashes
	hashes := []string{"hash1", "hash2", "nonexistent"}
	result, err := repo.FindVersionsByContentHashes(ctx, hashes)
	if err != nil {
		t.Fatalf("FindVersionsByContentHashes() error = %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 versions, got %d", len(result))
	}

	if result["hash1"] == nil {
		t.Error("Expected to find version with hash1")
	}
	if result["hash2"] == nil {
		t.Error("Expected to find version with hash2")
	}
	if result["nonexistent"] != nil {
		t.Error("Should not find version with nonexistent hash")
	}

	// Test with empty slice
	emptyResult, err := repo.FindVersionsByContentHashes(ctx, []string{})
	if err != nil {
		t.Fatalf("FindVersionsByContentHashes() with empty slice error = %v", err)
	}
	if len(emptyResult) != 0 {
		t.Errorf("Expected empty result, got %d entries", len(emptyResult))
	}
}

func TestRepository_GetChunksByIDs(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Add a version with chunks
	doc := domainindex.DocumentVersion{
		FilePath:    "test.go",
		ContentHash: "hash1",
	}
	versionID, err := repo.AddDocumentVersion(ctx, &doc, []byte("content"))
	if err != nil {
		t.Fatalf("AddDocumentVersion() error = %v", err)
	}

	chunks := []domainindex.Chunk{
		{
			DocumentVersionID: versionID,
			StartLine:         1,
			EndLine:           1,
			Embedding:         gateway.Embedding{0.1, 0.2, 0.3},
		},
		{
			DocumentVersionID: versionID,
			StartLine:         2,
			EndLine:           2,
			Embedding:         gateway.Embedding{0.4, 0.5, 0.6},
		},
		{
			DocumentVersionID: versionID,
			StartLine:         3,
			EndLine:           3,
			Embedding:         gateway.Embedding{0.7, 0.8, 0.9},
		},
	}

	err = repo.AddChunks(ctx, chunks)
	if err != nil {
		t.Fatalf("AddChunks() error = %v", err)
	}

	// Get all chunks to get their IDs
	allChunks, err := repo.GetChunksByVersionID(ctx, versionID)
	if err != nil {
		t.Fatalf("GetChunksByVersionID() error = %v", err)
	}
	if len(allChunks) != 3 {
		t.Fatalf("Expected 3 chunks, got %d", len(allChunks))
	}

	// Test getting chunks by IDs
	chunkIDs := []int64{allChunks[0].ID, allChunks[2].ID}
	retrievedChunks, err := repo.GetChunksByIDs(ctx, chunkIDs)
	if err != nil {
		t.Fatalf("GetChunksByIDs() error = %v", err)
	}

	if len(retrievedChunks) != 2 {
		t.Errorf("Expected 2 chunks, got %d", len(retrievedChunks))
	}

	// Test with empty slice
	emptyChunks, err := repo.GetChunksByIDs(ctx, []int64{})
	if err != nil {
		t.Fatalf("GetChunksByIDs() with empty slice error = %v", err)
	}
	if len(emptyChunks) != 0 {
		t.Errorf("Expected empty result, got %d chunks", len(emptyChunks))
	}

	// Test with nonexistent ID
	nonexistentChunks, err := repo.GetChunksByIDs(ctx, []int64{999999})
	if err != nil {
		t.Fatalf("GetChunksByIDs() with nonexistent ID error = %v", err)
	}
	if len(nonexistentChunks) != 0 {
		t.Errorf("Expected no chunks for nonexistent ID, got %d", len(nonexistentChunks))
	}
}

func TestRepository_Checkpoint(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Add some data
	doc := domainindex.DocumentVersion{
		FilePath:    "test.go",
		ContentHash: "hash1",
	}
	_, err := repo.AddDocumentVersion(ctx, &doc, []byte("content"))
	if err != nil {
		t.Fatalf("AddDocumentVersion() error = %v", err)
	}

	// Test checkpoint
	err = repo.Checkpoint(ctx)
	if err != nil {
		t.Fatalf("Checkpoint() error = %v", err)
	}
}
