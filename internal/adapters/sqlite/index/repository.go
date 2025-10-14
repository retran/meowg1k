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

	"github.com/retran/meowg1k/internal/domain/gateway"
)

// Repository implements index storage using SQLite.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new index repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// AddDocumentVersion adds a new document version to the index.
// Returns the ID of the newly created document version.
func (r *Repository) AddDocumentVersion(ctx context.Context, doc DocumentVersion, content []byte) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
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
		`INSERT INTO document_versions (file_path, git_commit_hash, content_hash) VALUES (?, ?, ?)`,
		doc.FilePath, doc.GitCommitHash, doc.ContentHash,
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

// AddChunks adds multiple chunks to the index in a single transaction.
func (r *Repository) AddChunks(ctx context.Context, chunks []Chunk) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO chunks (
			document_version_id, chunk_type, text_content,
			start_byte, end_byte, start_line, end_line, embedding
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
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
			chunk.Start, chunk.End, chunk.StartLine, chunk.EndLine,
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
	rows, err := r.db.QueryContext(ctx, "SELECT id, embedding FROM chunks")
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
