// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package index defines domain types for document indexing including chunks, embeddings, and snapshots.
package index

import (
	"database/sql"
	"time"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/preset"
)

// ResolvedConfig represents the resolved configuration for indexing.
type ResolvedConfig struct {
	Preset              *preset.ResolvedPreset
	ChunkerMaxRunes     int
	ChunkerOverlapRunes int
	BatchSize           int
}

// ContentBlob represents a content blob stored in the database.
type ContentBlob struct {
	ContentHash string `db:"content_hash"`
	Content     []byte `db:"content"`
}

// DocumentVersion represents a version of a document stored in the database.
type DocumentVersion struct {
	IndexedAt              time.Time      `db:"indexed_at"`
	FilePath               string         `db:"file_path"`
	ContentHash            string         `db:"content_hash"`
	GitCommitHashFirstSeen sql.NullString `db:"git_commit_hash_first_seen"`
	ID                     int64          `db:"id"`
}

// Chunk represents a chunk of a document with its embedding.
type Chunk struct {
	ChunkType         string            `db:"chunk_type"`
	TextContent       string            `db:"text_content"`
	Embedding         gateway.Embedding `db:"embedding"`
	ID                int64             `db:"id"`
	DocumentVersionID int64             `db:"document_version_id"`
	StartByte         int               `db:"start_byte"`
	EndByte           int               `db:"end_byte"`
	StartRune         int               `db:"start_rune"`
	EndRune           int               `db:"end_rune"`
	StartLine         int               `db:"start_line"`
	EndLine           int               `db:"end_line"`
}

// ChunkData represents a chunk of text with its position information (before saving to DB).
type ChunkData struct {
	TextContent string
	StartByte   int
	EndByte     int
	StartRune   int
	EndRune     int
	StartLine   int
	EndLine     int
}

// FileState represents the state of a file with its content and hash.
type FileState struct {
	ContentHash string
	Content     []byte
}

// FileToProcess represents a unique file (by content) that needs processing.
// FilePath identifies where the content came from, State holds the file data.
type FileToProcess struct {
	FilePath string
	State    FileState
}

// CommitSnapshot represents a link between a git commit and a document version.
// It tracks which document versions existed at a specific commit.
type CommitSnapshot struct {
	CommitHash        string `db:"commit_hash"`
	DocumentVersionID int64  `db:"document_version_id"`
}
