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

// Package index provides domain models for the document indexing system.
package index

import (
	"database/sql"
	"time"

	"github.com/retran/meowg1k/internal/domain/gateway"
)

// ContentBlob represents a content blob stored in the database.
type ContentBlob struct {
	ContentHash string `db:"content_hash"`
	Content     []byte `db:"content"`
}

// DocumentVersion represents a version of a document stored in the database.
type DocumentVersion struct {
	ID                     int64          `db:"id"`
	FilePath               string         `db:"file_path"`
	GitCommitHashFirstSeen sql.NullString `db:"git_commit_hash_first_seen"`
	ContentHash            string         `db:"content_hash"`
	IndexedAt              time.Time      `db:"indexed_at"`
}

// Chunk represents a chunk of a document with its embedding.
type Chunk struct {
	ID                int64             `db:"id"`
	DocumentVersionID int64             `db:"document_version_id"`
	ChunkType         string            `db:"chunk_type"`
	TextContent       string            `db:"text_content"`
	StartByte         int               `db:"start_byte"`
	EndByte           int               `db:"end_byte"`
	StartRune         int               `db:"start_rune"`
	EndRune           int               `db:"end_rune"`
	StartLine         int               `db:"start_line"`
	EndLine           int               `db:"end_line"`
	Embedding         gateway.Embedding `db:"embedding"`
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

// CommitSnapshot represents a link between a git commit and a document version.
// It tracks which document versions existed at a specific commit.
type CommitSnapshot struct {
	CommitHash        string `db:"commit_hash"`
	DocumentVersionID int64  `db:"document_version_id"`
}
