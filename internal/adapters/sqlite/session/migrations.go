// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/retran/meowg1k/internal/adapters/sqlite/migrations"
)

// Migrations defines all database migrations for the session subsystem.
var Migrations = []migrations.Migration{
	{
		Version: 1,
		Up: func(tx *sql.Tx) error {
			ctx := context.Background()

			_, err := tx.ExecContext(ctx, `
				CREATE TABLE sessions (
					id TEXT PRIMARY KEY,
					parent_id TEXT,
					tool_name TEXT NOT NULL,
					status TEXT NOT NULL,
					created_at TIMESTAMP NOT NULL,
					updated_at TIMESTAMP NOT NULL,
					FOREIGN KEY (parent_id) REFERENCES sessions(id) ON DELETE CASCADE
				);

				CREATE TABLE events (
					id TEXT PRIMARY KEY,
					session_id TEXT NOT NULL,
					type TEXT NOT NULL,
					content TEXT NOT NULL,
					tool_call_id TEXT,
					obsolete INTEGER NOT NULL DEFAULT 0,
					created_at TIMESTAMP NOT NULL,
					FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
				);

				CREATE TABLE tool_calls (
					id TEXT PRIMARY KEY,
					event_id TEXT NOT NULL,
					name TEXT NOT NULL,
					params TEXT NOT NULL,
					FOREIGN KEY (event_id) REFERENCES events(id) ON DELETE CASCADE
				);

				CREATE TABLE session_metadata (
					session_id TEXT NOT NULL,
					key TEXT NOT NULL,
					value TEXT NOT NULL,
					PRIMARY KEY (session_id, key),
					FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
				);
			`)
			if err != nil {
				return fmt.Errorf("failed to create session schema tables: %w", err)
			}

			_, err = tx.ExecContext(ctx, `
				CREATE INDEX idx_sessions_parent_id ON sessions (parent_id);
				CREATE INDEX idx_sessions_status ON sessions (status);
				CREATE INDEX idx_sessions_created_at ON sessions (created_at);
				CREATE INDEX idx_events_session_id ON events (session_id);
				CREATE INDEX idx_events_created_at ON events (created_at);
				CREATE INDEX idx_events_obsolete ON events (obsolete);
				CREATE INDEX idx_tool_calls_event_id ON tool_calls (event_id);
				CREATE INDEX idx_session_metadata_session_id ON session_metadata (session_id);
			`)
			if err != nil {
				return fmt.Errorf("failed to create indexes for session schema: %w", err)
			}

			return nil
		},
	},
}
