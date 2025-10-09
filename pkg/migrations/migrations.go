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

// Package migrations provides database migration functionalities.
package migrations

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
)

var (
	// ErrDatabaseIsNil indicates that the database connection is nil.
	ErrDatabaseIsNil = errors.New("database connection is nil")
	// ErrMigrationsIsNil indicates that the migrations slice is nil.
	ErrMigrationsIsNil = errors.New("migrations slice is nil")
	// ErrMigrationUpFuncIsNil indicates that a migration's Up function is nil.
	ErrMigrationUpFuncIsNil = errors.New("migration Up function is nil")
)

type Migration struct {
	Version uint
	Up      func(tx *sql.Tx) error
}

func RunMigrations(db *sql.DB, migrations []Migration) error {
	if db == nil {
		return ErrDatabaseIsNil
	}

	if migrations == nil {
		return ErrMigrationsIsNil
	}

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_versions (
			version INTEGER NOT NULL
		);
	`)
	if err != nil {
		// TODO proper error
		return fmt.Errorf("failed to create schema_versions table: %w", err)
	}

	var currentVersion uint
	err = db.QueryRow("SELECT version FROM schema_versions LIMIT 1").Scan(&currentVersion)
	if err != nil {
		if err == sql.ErrNoRows {
			_, err = db.Exec("INSERT INTO schema_versions (version) VALUES (0)")
			if err != nil {
				// TODO proper error
				return fmt.Errorf("failed to initialize schema_versions table: %w", err)
			}
			currentVersion = 0
		} else {
			// TODO proper error
			return fmt.Errorf("failed to get current schema version: %w", err)
		}
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, m := range migrations {
		if m.Version > currentVersion {
			if m.Up == nil {
				// TODO proper error
				return fmt.Errorf("%w for version %d", ErrMigrationUpFuncIsNil, m.Version)
			}
			if err := m.Up(tx); err != nil {
				// TODO proper error
				return fmt.Errorf("failed to apply migration %d: %w", m.Version, err)
			}

			_, err := tx.Exec("UPDATE schema_versions SET version = ?", m.Version)
			if err != nil {
				// TODO proper error
				return fmt.Errorf("failed to update schema version to %d: %w", m.Version, err)
			}
		}
	}

	return tx.Commit()
}
