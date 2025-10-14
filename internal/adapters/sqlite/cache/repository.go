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
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/retran/meowg1k/internal/ports"
)

// Repository implements the CacheRepository interface using SQLite.
type Repository struct {
	host ports.Host
}

// NewRepository creates a new cache repository.
func NewRepository(host ports.Host) *Repository {
	return &Repository{host: host}
}

// Get retrieves a cached value by key.
// Returns the value, whether it was found, and any error.
func (r *Repository) Get(ctx context.Context, key string) (string, bool, error) {
	if r == nil {
		return "", false, fmt.Errorf("repository is nil")
	}

	if ctx == nil {
		return "", false, fmt.Errorf("context is nil")
	}

	if key == "" {
		return "", false, fmt.Errorf("key cannot be empty")
	}

	db, err := r.host.GetDB()
	if err != nil {
		return "", false, fmt.Errorf("failed to get database: %w", err)
	}

	var value string
	err = db.QueryRowContext(ctx, `
		SELECT value FROM llm_cache WHERE key = ?
	`, key).Scan(&value)

	if err == sql.ErrNoRows {
		return "", false, nil
	}

	if err != nil {
		return "", false, fmt.Errorf("failed to get cache entry: %w", err)
	}

	return value, true, nil
}

// Set stores a value in the cache with the given key.
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

	db, err := r.host.GetDB()
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

// Purge removes cache entries older than the specified TTL.
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

	db, err := r.host.GetDB()
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
		// Log but don't fail on this error
		return nil
	}

	if rowsAffected > 0 {
		// Could add logging here if needed
		_ = rowsAffected
	}

	return nil
}
