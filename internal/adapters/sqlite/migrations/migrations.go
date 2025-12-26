// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package migrations applies SQLite schema migrations.
package migrations

import (
	"context"
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

	ctx := context.Background()

	if err := ensureSchemaTable(ctx, db); err != nil {
		return err
	}

	currentVersion, err := loadCurrentSchemaVersion(ctx, db)
	if err != nil {
		return err
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }() //nolint:errcheck // Defer rollback errors are not critical

	if err := applyMigrations(ctx, tx, migrations, currentVersion); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migrations: %w", err)
	}
	return nil
}

func ensureSchemaTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_versions (
			version INTEGER NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema_versions table: %w", err)
	}
	return nil
}

func loadCurrentSchemaVersion(ctx context.Context, db *sql.DB) (uint, error) {
	var currentVersion uint
	err := db.QueryRowContext(ctx, "SELECT version FROM schema_versions LIMIT 1").Scan(&currentVersion)
	if err == nil {
		return currentVersion, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("failed to get current schema version: %w", err)
	}

	if _, err = db.ExecContext(ctx, "INSERT INTO schema_versions (version) VALUES (0)"); err != nil {
		return 0, fmt.Errorf("failed to initialize schema_versions table: %w", err)
	}

	return 0, nil
}

func applyMigrations(ctx context.Context, tx *sql.Tx, migrations []Migration, currentVersion uint) error {
	for _, m := range migrations {
		if m.Version <= currentVersion {
			continue
		}

		if m.Up == nil {
			return fmt.Errorf("migration Up function is nil for version %d", m.Version)
		}
		if err := m.Up(tx); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", m.Version, err)
		}

		if _, err := tx.ExecContext(ctx, "UPDATE schema_versions SET version = ?", m.Version); err != nil {
			return fmt.Errorf("failed to update schema version to %d: %w", m.Version, err)
		}
	}
	return nil
}
