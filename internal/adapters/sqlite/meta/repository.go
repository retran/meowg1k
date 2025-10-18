// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package meta provides a SQLite-based repository for storing and retrieving metadata key-value pairs.
package meta

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/retran/meowg1k/internal/ports"
)

// Repository implements metadata storage using SQLite.
type Repository struct {
	host ports.Host
}

// Compile-time interface compliance check.
var _ ports.MetaRepository = (*Repository)(nil)

// NewRepository creates a new metadata repository.
func NewRepository(host ports.Host) *Repository {
	return &Repository{host: host}
}

// SetValue stores a metadata value with the given key.
// If the key already exists, the value is updated.
func (r *Repository) SetValue(ctx context.Context, key string, value []byte) error {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO meta_kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("failed to set meta value for key '%s': %w", key, err)
	}
	return nil
}

// GetValue retrieves a metadata value by key.
// Returns nil if the key does not exist.
func (r *Repository) GetValue(ctx context.Context, key string) ([]byte, error) {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	var value []byte
	err = db.QueryRowContext(ctx, "SELECT value FROM meta_kv WHERE key = ?", key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get meta value for key '%s': %w", key, err)
	}
	return value, nil
}

// DeleteValue deletes a metadata value by key.
// Does not return an error if the key does not exist.
func (r *Repository) DeleteValue(ctx context.Context, key string) error {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	_, err = db.ExecContext(ctx, "DELETE FROM meta_kv WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("failed to delete meta value for key '%s': %w", key, err)
	}
	return nil
}
