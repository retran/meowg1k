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
	"database/sql"
	"fmt"

	"github.com/retran/meowg1k/internal/adapters/sqlite/migrations"
)

// Migrations defines all database migrations for the index subsystem.
var Migrations = []migrations.Migration{
	{
		Version: 1,
		Up: func(tx *sql.Tx) error {
			_, err := tx.Exec(`
				CREATE TABLE content_blobs (
					content_hash TEXT PRIMARY KEY,
					content BLOB NOT NULL
				);

				CREATE TABLE document_versions (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					file_path TEXT NOT NULL,
					git_commit_hash_first_seen TEXT,
					content_hash TEXT NOT NULL,
					indexed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (content_hash) REFERENCES content_blobs(content_hash)
				);

				CREATE TABLE chunks (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					document_version_id INTEGER NOT NULL,
					chunk_type TEXT NOT NULL,
					text_content TEXT NOT NULL,
					start_byte INTEGER NOT NULL,
					end_byte INTEGER NOT NULL,
					start_rune INTEGER NOT NULL,
					end_rune INTEGER NOT NULL,
					start_line INTEGER NOT NULL,
					end_line INTEGER NOT NULL,
					embedding BLOB NOT NULL,
					FOREIGN KEY (document_version_id) REFERENCES document_versions(id) ON DELETE CASCADE
				);

				CREATE TABLE commit_snapshots (
					commit_hash TEXT NOT NULL,
					document_version_id INTEGER NOT NULL,
					PRIMARY KEY (commit_hash, document_version_id),
					FOREIGN KEY (document_version_id) REFERENCES document_versions(id) ON DELETE CASCADE
				);
			`)
			if err != nil {
				return fmt.Errorf("failed to create RAG schema tables: %w", err)
			}

			_, err = tx.Exec(`
				CREATE INDEX idx_document_versions_path ON document_versions (file_path);
				CREATE INDEX idx_document_versions_path_commit ON document_versions (file_path, git_commit_hash_first_seen);
				CREATE INDEX idx_chunks_document_version_id ON chunks (document_version_id);
				CREATE INDEX idx_commit_snapshots_commit_hash ON commit_snapshots (commit_hash);
			`)
			if err != nil {
				return fmt.Errorf("failed to create indexes for RAG schema: %w", err)
			}

			return nil
		},
	},
}
