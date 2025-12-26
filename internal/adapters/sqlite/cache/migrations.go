// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/retran/meowg1k/internal/adapters/sqlite/migrations"
)

// Migrations defines all database migrations for the cache subsystem.
var Migrations = []migrations.Migration{
	{
		Version: 2,
		Up: func(tx *sql.Tx) error {
			ctx := context.Background()

			_, err := tx.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS llm_cache (
					key TEXT PRIMARY KEY,
					value TEXT NOT NULL,
					created_at INTEGER NOT NULL
				);
			`)
			if err != nil {
				return fmt.Errorf("failed to create llm_cache table: %w", err)
			}

			// Create index on created_at to optimize purge operations
			_, err = tx.ExecContext(ctx, `
				CREATE INDEX IF NOT EXISTS idx_llm_cache_created_at
				ON llm_cache(created_at);
			`)
			if err != nil {
				return fmt.Errorf("failed to create index on llm_cache.created_at: %w", err)
			}

			return nil
		},
	},
}
