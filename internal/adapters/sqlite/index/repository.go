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
	"database/sql"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
)

type Repository struct {
	host ports.Host
}

var (
	_ ports.IndexRepository    = (*Repository)(nil)
	_ ports.SnapshotRepository = (*Repository)(nil)
)

func NewRepository(host ports.Host) *Repository {
	return &Repository{host: host}
}

func (r *Repository) AddDocumentVersion(ctx context.Context, doc domainindex.DocumentVersion, content []byte) (int64, error) {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return 0, fmt.Errorf("failed to get database: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("could not begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO content_blobs (content_hash, content) VALUES (?, ?) ON CONFLICT(content_hash) DO NOTHING`,
		doc.ContentHash, content,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert content blob: %w", err)
	}

	res, err := tx.ExecContext(ctx,
		`INSERT INTO document_versions (file_path, git_commit_hash_first_seen, content_hash) VALUES (?, ?, ?)`,
		doc.FilePath, doc.GitCommitHashFirstSeen, doc.ContentHash,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert document version: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID for document version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return id, nil
}

func (r *Repository) AddChunks(ctx context.Context, chunks []domainindex.Chunk) error {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO chunks (
			document_version_id, chunk_type, text_content,
			start_byte, end_byte, start_rune, end_rune, start_line, end_line, embedding
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare chunk insert statement: %w", err)
	}
	defer stmt.Close()

	for _, chunk := range chunks {
		embeddingBytes, err := encodeEmbedding(chunk.Embedding)
		if err != nil {
			return fmt.Errorf("could not encode embedding for chunk: %w", err)
		}

		_, err = stmt.ExecContext(ctx,
			chunk.DocumentVersionID, chunk.ChunkType, chunk.TextContent,
			chunk.StartByte, chunk.EndByte, chunk.StartRune, chunk.EndRune,
			chunk.StartLine, chunk.EndLine,
			embeddingBytes,
		)
		if err != nil {
			return fmt.Errorf("failed to execute chunk insert: %w", err)
		}
	}

	return tx.Commit()
}

// GetAllEmbeddings retrieves all embeddings from the index.
// Returns a map of chunk ID to embedding vector.
func (r *Repository) GetAllEmbeddings(ctx context.Context) (map[int64]gateway.Embedding, error) {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	rows, err := db.QueryContext(ctx, "SELECT id, embedding FROM chunks")
	if err != nil {
		return nil, fmt.Errorf("failed to query embeddings: %w", err)
	}
	defer rows.Close()

	embeddings := make(map[int64]gateway.Embedding)
	for rows.Next() {
		var id int64
		var embeddingBytes []byte
		if err := rows.Scan(&id, &embeddingBytes); err != nil {
			return nil, fmt.Errorf("failed to scan embedding row: %w", err)
		}

		embedding, err := decodeEmbedding(embeddingBytes)
		if err != nil {
			return nil, fmt.Errorf("could not decode embedding for chunk id %d: %w", id, err)
		}
		embeddings[id] = embedding
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return embeddings, nil
}

// FindVersionByContentHash finds a document version by content hash and file path.
// Returns nil if no matching version is found.
func (r *Repository) FindVersionByContentHash(ctx context.Context, filePath, contentHash string) (*domainindex.DocumentVersion, error) {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	var doc domainindex.DocumentVersion
	err = db.QueryRowContext(ctx,
		`SELECT id, file_path, git_commit_hash_first_seen, content_hash, indexed_at
		FROM document_versions
		WHERE file_path = ? AND content_hash = ?`,
		filePath, contentHash,
	).Scan(&doc.ID, &doc.FilePath, &doc.GitCommitHashFirstSeen, &doc.ContentHash, &doc.IndexedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find version by content hash: %w", err)
	}

	return &doc, nil
}

// FindVersionsByContentHashes finds document versions for multiple content hashes.
// Returns a map of contentHash to document version.
// Only returns entries for versions that exist in the database.
// Note: Returns any document version that matches the content hash (picks arbitrarily if multiple exist).
func (r *Repository) FindVersionsByContentHashes(ctx context.Context, contentHashes []string) (map[string]*domainindex.DocumentVersion, error) {
	if len(contentHashes) == 0 {
		return make(map[string]*domainindex.DocumentVersion), nil
	}

	db, err := r.host.GetProjectDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	// Build query with placeholders for batch lookup using IN clause
	placeholders := make([]string, len(contentHashes))
	args := make([]interface{}, len(contentHashes))
	for i, contentHash := range contentHashes {
		placeholders[i] = "?"
		args[i] = contentHash
	}

	query := fmt.Sprintf(`
		SELECT id, file_path, git_commit_hash_first_seen, content_hash, indexed_at
		FROM document_versions
		WHERE content_hash IN (%s)
		GROUP BY content_hash
	`, strings.Join(placeholders, ", "))

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query versions by content hashes: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*domainindex.DocumentVersion)
	for rows.Next() {
		var doc domainindex.DocumentVersion
		if err := rows.Scan(&doc.ID, &doc.FilePath, &doc.GitCommitHashFirstSeen, &doc.ContentHash, &doc.IndexedAt); err != nil {
			return nil, fmt.Errorf("failed to scan document version: %w", err)
		}
		result[doc.ContentHash] = &doc
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return result, nil
}

// FindContentBlob checks if a content blob exists by its hash.
// Returns true if the blob exists, false otherwise.
func (r *Repository) FindContentBlob(ctx context.Context, contentHash string) (bool, error) {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return false, fmt.Errorf("failed to get database: %w", err)
	}

	var exists bool
	err = db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM content_blobs WHERE content_hash = ?)`,
		contentHash,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check content blob existence: %w", err)
	}

	return exists, nil
}

// GetContentBlob retrieves the content of a blob by its hash.
// Returns nil if the blob does not exist.
func (r *Repository) GetContentBlob(ctx context.Context, contentHash string) ([]byte, error) {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	var content []byte
	err = db.QueryRowContext(ctx,
		`SELECT content FROM content_blobs WHERE content_hash = ?`,
		contentHash,
	).Scan(&content)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get content blob: %w", err)
	}

	return content, nil
}

// FindVersionsByFilePath finds all versions of a document by file path.
func (r *Repository) FindVersionsByFilePath(ctx context.Context, filePath string) ([]domainindex.DocumentVersion, error) {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	rows, err := db.QueryContext(ctx,
		`SELECT id, file_path, git_commit_hash_first_seen, content_hash, indexed_at
		FROM document_versions
		WHERE file_path = ?
		ORDER BY indexed_at DESC`,
		filePath,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query versions by file path: %w", err)
	}
	defer rows.Close()

	var versions []domainindex.DocumentVersion
	for rows.Next() {
		var doc domainindex.DocumentVersion
		if err := rows.Scan(&doc.ID, &doc.FilePath, &doc.GitCommitHashFirstSeen, &doc.ContentHash, &doc.IndexedAt); err != nil {
			return nil, fmt.Errorf("failed to scan document version: %w", err)
		}
		versions = append(versions, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return versions, nil
}

// GetChunksByVersionID retrieves all chunks for a given document version.
func (r *Repository) GetChunksByVersionID(ctx context.Context, versionID int64) ([]domainindex.Chunk, error) {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	rows, err := db.QueryContext(ctx,
		`SELECT id, document_version_id, chunk_type, text_content,
		start_byte, end_byte, start_rune, end_rune, start_line, end_line, embedding
		FROM chunks
		WHERE document_version_id = ?`,
		versionID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunks by version ID: %w", err)
	}
	defer rows.Close()

	var chunks []domainindex.Chunk
	for rows.Next() {
		var chunk domainindex.Chunk
		var embeddingBytes []byte
		if err := rows.Scan(
			&chunk.ID, &chunk.DocumentVersionID, &chunk.ChunkType, &chunk.TextContent,
			&chunk.StartByte, &chunk.EndByte, &chunk.StartRune, &chunk.EndRune,
			&chunk.StartLine, &chunk.EndLine, &embeddingBytes,
		); err != nil {
			return nil, fmt.Errorf("failed to scan chunk: %w", err)
		}

		embedding, err := decodeEmbedding(embeddingBytes)
		if err != nil {
			return nil, fmt.Errorf("could not decode embedding for chunk id %d: %w", chunk.ID, err)
		}
		chunk.Embedding = embedding

		chunks = append(chunks, chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return chunks, nil
}

// GetChunksByIDs retrieves chunks by their IDs.
// Returns chunks in the same order as they appear in the result set (not necessarily input order).
func (r *Repository) GetChunksByIDs(ctx context.Context, chunkIDs []int64) ([]domainindex.Chunk, error) {
	if len(chunkIDs) == 0 {
		return []domainindex.Chunk{}, nil
	}

	db, err := r.host.GetProjectDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	// Build query with placeholders for IN clause
	placeholders := make([]string, len(chunkIDs))
	args := make([]interface{}, len(chunkIDs))
	for i, id := range chunkIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, document_version_id, chunk_type, text_content,
		start_byte, end_byte, start_rune, end_rune, start_line, end_line, embedding
		FROM chunks
		WHERE id IN (%s)
	`, strings.Join(placeholders, ", "))

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunks by IDs: %w", err)
	}
	defer rows.Close()

	var chunks []domainindex.Chunk
	for rows.Next() {
		var chunk domainindex.Chunk
		var embeddingBytes []byte
		if err := rows.Scan(
			&chunk.ID, &chunk.DocumentVersionID, &chunk.ChunkType, &chunk.TextContent,
			&chunk.StartByte, &chunk.EndByte, &chunk.StartRune, &chunk.EndRune,
			&chunk.StartLine, &chunk.EndLine, &embeddingBytes,
		); err != nil {
			return nil, fmt.Errorf("failed to scan chunk: %w", err)
		}

		embedding, err := decodeEmbedding(embeddingBytes)
		if err != nil {
			return nil, fmt.Errorf("could not decode embedding for chunk id %d: %w", chunk.ID, err)
		}
		chunk.Embedding = embedding

		chunks = append(chunks, chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return chunks, nil
}

// LinkVersionToSnapshot links a document version to a commit snapshot.
func (r *Repository) LinkVersionToSnapshot(ctx context.Context, commitHash string, versionID int64) error {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO commit_snapshots (commit_hash, document_version_id)
		VALUES (?, ?)
		ON CONFLICT(commit_hash, document_version_id) DO NOTHING`,
		commitHash, versionID,
	)
	if err != nil {
		return fmt.Errorf("failed to link version %d to snapshot %s: %w", versionID, commitHash, err)
	}
	return nil
}

// UnlinkVersionFromSnapshot removes a link between a document version and a snapshot.
func (r *Repository) UnlinkVersionFromSnapshot(ctx context.Context, commitHash string, versionID int64) error {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	_, err = db.ExecContext(ctx,
		`DELETE FROM commit_snapshots
		WHERE commit_hash = ? AND document_version_id = ?`,
		commitHash, versionID,
	)
	if err != nil {
		return fmt.Errorf("failed to unlink version %d from snapshot %s: %w", versionID, commitHash, err)
	}
	return nil
}

// GetVersionIDsForSnapshot retrieves all document version IDs for a given snapshot.
func (r *Repository) GetVersionIDsForSnapshot(ctx context.Context, commitHash string) ([]int64, error) {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	rows, err := db.QueryContext(ctx,
		`SELECT document_version_id FROM commit_snapshots WHERE commit_hash = ?`,
		commitHash,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query version IDs for snapshot %s: %w", commitHash, err)
	}
	defer rows.Close()

	var versionIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan version ID: %w", err)
		}
		versionIDs = append(versionIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return versionIDs, nil
}

// ClearSnapshotLinks removes all links for a given snapshot.
func (r *Repository) ClearSnapshotLinks(ctx context.Context, commitHash string) error {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	_, err = db.ExecContext(ctx,
		`DELETE FROM commit_snapshots WHERE commit_hash = ?`,
		commitHash,
	)
	if err != nil {
		return fmt.Errorf("failed to clear snapshot links for %s: %w", commitHash, err)
	}
	return nil
}

// GetVersionsByIDs retrieves document versions by their IDs.
func (r *Repository) GetVersionsByIDs(ctx context.Context, versionIDs []int64) ([]domainindex.DocumentVersion, error) {
	if len(versionIDs) == 0 {
		return []domainindex.DocumentVersion{}, nil
	}

	db, err := r.host.GetProjectDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(versionIDs))
	for i := range versionIDs {
		placeholders[i] = "?"
	}

	query := fmt.Sprintf(
		`SELECT id, file_path, git_commit_hash_first_seen, content_hash, indexed_at
		FROM document_versions
		WHERE id IN (%s)`,
		strings.Join(placeholders, ", "),
	)

	// Convert []int64 to []interface{} for QueryContext
	args := make([]interface{}, len(versionIDs))
	for i, id := range versionIDs {
		args[i] = id
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query versions by IDs: %w", err)
	}
	defer rows.Close()

	var versions []domainindex.DocumentVersion
	for rows.Next() {
		var doc domainindex.DocumentVersion
		if err := rows.Scan(&doc.ID, &doc.FilePath, &doc.GitCommitHashFirstSeen, &doc.ContentHash, &doc.IndexedAt); err != nil {
			return nil, fmt.Errorf("failed to scan document version: %w", err)
		}
		versions = append(versions, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return versions, nil
}
