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

package ratelimit

import (
	"database/sql"
	"fmt"

	"github.com/retran/meowg1k/internal/adapters/sqlite/migrations"
)

// Migrations defines the database migrations for the rate limit repository.
var Migrations = []migrations.Migration{
	{
		Version: 1,
		Up: func(tx *sql.Tx) error {
			_, err := tx.Exec(`
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
