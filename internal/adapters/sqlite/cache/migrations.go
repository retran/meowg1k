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

package cache

import (
	"database/sql"
	"fmt"

	"github.com/retran/meowg1k/internal/adapters/sqlite/migrations"
)

// Migrations defines all database migrations for the cache subsystem.
var Migrations = []migrations.Migration{
	{
		Version: 2,
		Up: func(tx *sql.Tx) error {
			_, err := tx.Exec(`
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
			_, err = tx.Exec(`
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
