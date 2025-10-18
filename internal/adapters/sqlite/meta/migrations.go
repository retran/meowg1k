// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package meta

import (
	"database/sql"
	"fmt"

	"github.com/retran/meowg1k/internal/adapters/sqlite/migrations"
)

// Migrations defines all database migrations for the metadata subsystem.
var Migrations = []migrations.Migration{
	{
		Version: 1,
		Up: func(tx *sql.Tx) error {
			_, err := tx.Exec(`
				CREATE TABLE meta_kv (
					key TEXT PRIMARY KEY,
					value BLOB NOT NULL
				);
			`)
			if err != nil {
				return fmt.Errorf("failed to create meta_kv table: %w", err)
			}

			return nil
		},
	},
}
