// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ratelimit

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/retran/meowg1k/internal/adapters/sqlite/migrations"
)

// Migrations defines the database migrations for the rate limit repository.
var Migrations = []migrations.Migration{
	{
		Version: 1,
		Up: func(tx *sql.Tx) error {
			ctx := context.Background()

			_, err := tx.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS rate_limit_buckets (
					id TEXT PRIMARY KEY,
					tokens INTEGER NOT NULL,
					last_refill INTEGER NOT NULL
				)
			`)
			if err != nil {
				return fmt.Errorf("failed to create rate_limit_buckets table: %w", err)
			}
			return nil
		},
	},
}
