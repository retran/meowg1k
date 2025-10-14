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

package meta

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/retran/meowg1k/internal/ports"
)

// MetaRepository implements metadata storage using SQLite.
type MetaRepository struct {
	db *sql.DB
}

// Compile-time interface compliance check.
var _ ports.MetaRepository = (*MetaRepository)(nil)

// NewMetaRepository creates a new metadata repository.
func NewMetaRepository(db *sql.DB) *MetaRepository {
	return &MetaRepository{db: db}
}

// SetValue stores a metadata value with the given key.
// If the key already exists, the value is updated.
func (r *MetaRepository) SetValue(ctx context.Context, key string, value []byte) error {
	_, err := r.db.ExecContext(ctx,
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
func (r *MetaRepository) GetValue(ctx context.Context, key string) ([]byte, error) {
	var value []byte
	err := r.db.QueryRowContext(ctx, "SELECT value FROM meta_kv WHERE key = ?", key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get meta value for key '%s': %w", key, err)
	}
	return value, nil
}
