// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package cache provides a SQLite-based repository for caching LLM responses with TTL support.
package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/retran/meowg1k/internal/ports"
)

// Repository stores cached LLM responses in SQLite.
type Repository struct {
	host ports.Host
}

// NewRepository creates a cache repository backed by SQLite.
func NewRepository(host ports.Host) *Repository {
	return &Repository{host: host}
}

// Get fetches a cache entry by key.
func (r *Repository) Get(ctx context.Context, key string) (value string, found bool, err error) {
	if r == nil {
		return "", false, fmt.Errorf("repository is nil")
	}

	if ctx == nil {
		return "", false, fmt.Errorf("context is nil")
	}

	if key == "" {
		return "", false, fmt.Errorf("key cannot be empty")
	}

	db, err := r.host.GetMainDB()
	if err != nil {
		return "", false, fmt.Errorf("failed to get database: %w", err)
	}

	err = db.QueryRowContext(ctx, `
		SELECT value FROM llm_cache WHERE key = ?
	`, key).Scan(&value)

	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}

	if err != nil {
		return "", false, fmt.Errorf("failed to get cache entry: %w", err)
	}

	return value, true, nil
}

// Set stores a cache entry by key.
func (r *Repository) Set(ctx context.Context, key, value string) error {
	if r == nil {
		return fmt.Errorf("repository is nil")
	}

	if ctx == nil {
		return fmt.Errorf("context is nil")
	}

	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	db, err := r.host.GetMainDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	now := time.Now().Unix()
	_, err = db.ExecContext(ctx, `
		INSERT OR REPLACE INTO llm_cache (key, value, created_at)
		VALUES (?, ?, ?)
	`, key, value, now)
	if err != nil {
		return fmt.Errorf("failed to set cache entry: %w", err)
	}

	return nil
}

// Purge removes cache entries older than the TTL.
func (r *Repository) Purge(ctx context.Context, ttl time.Duration) error {
	if r == nil {
		return fmt.Errorf("repository is nil")
	}

	if ctx == nil {
		return fmt.Errorf("context is nil")
	}

	if ttl <= 0 {
		return fmt.Errorf("TTL must be positive")
	}

	db, err := r.host.GetMainDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	cutoff := time.Now().Add(-ttl).Unix()
	result, err := db.ExecContext(ctx, `
		DELETE FROM llm_cache WHERE created_at < ?
	`, cutoff)
	if err != nil {
		return fmt.Errorf("failed to purge cache entries: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read affected rows: %w", err)
	}

	if rowsAffected > 0 {
		_ = rowsAffected
	}

	return nil
}
