// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package migrations

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
)

// Migration represents a database migration with a version number and upgrade function.
type Migration struct {
	Up      func(tx *sql.Tx) error
	Version uint
}

// RunMigrations applies all pending database migrations in order.
// It creates a schema_versions table if it doesn't exist and tracks applied migrations.
func RunMigrations(db *sql.DB, migrations []Migration) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	if migrations == nil {
		return fmt.Errorf("migrations slice is nil")
	}

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_versions (
			version INTEGER NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema_versions table: %w", err)
	}

	var currentVersion uint
	err = db.QueryRow("SELECT version FROM schema_versions LIMIT 1").Scan(&currentVersion)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_, err = db.Exec("INSERT INTO schema_versions (version) VALUES (0)")
			if err != nil {
				return fmt.Errorf("failed to initialize schema_versions table: %w", err)
			}
			currentVersion = 0
		} else {
			return fmt.Errorf("failed to get current schema version: %w", err)
		}
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, m := range migrations {
		if m.Version > currentVersion {
			if m.Up == nil {
				return fmt.Errorf("migration Up function is nil for version %d", m.Version)
			}
			if err := m.Up(tx); err != nil {
				return fmt.Errorf("failed to apply migration %d: %w", m.Version, err)
			}

			_, err := tx.Exec("UPDATE schema_versions SET version = ?", m.Version)
			if err != nil {
				return fmt.Errorf("failed to update schema version to %d: %w", m.Version, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migrations: %w", err)
	}
	return nil
}
