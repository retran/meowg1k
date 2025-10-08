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
	"time"

	"github.com/retran/meowg1k/pkg/migrations"
)

// BucketState represents the persisted state of a rate limit bucket.
type BucketState struct {
	ID         string
	Tokens     int
	LastRefill time.Time
}

// Repository defines the interface for persisting rate limit bucket state.
type Repository interface {
	// GetBucketState retrieves the current state of a bucket by ID.
	// Returns sql.ErrNoRows if the bucket doesn't exist.
	GetBucketState(id string) (*BucketState, error)

	// SaveBucketState persists or updates the state of a bucket.
	SaveBucketState(state *BucketState) error

	// InitializeBucket creates a new bucket with initial capacity.
	InitializeBucket(id string, initialTokens int) error

	// DeleteBucket removes a bucket's state from the database.
	DeleteBucket(id string) error
}

type repositoryImpl struct {
	db *sql.DB
}

// NewRepository creates a new repository instance.
func NewRepository(db *sql.DB) Repository {
	return &repositoryImpl{
		db: db,
	}
}

// GetBucketState retrieves the current state of a bucket by ID.
func (r *repositoryImpl) GetBucketState(id string) (*BucketState, error) {
	var state BucketState
	var lastRefillNano int64

	err := r.db.QueryRow(
		"SELECT id, tokens, last_refill FROM rate_limit_buckets WHERE id = ?",
		id,
	).Scan(&state.ID, &state.Tokens, &lastRefillNano)
	if err != nil {
		return nil, err
	}

	state.LastRefill = time.Unix(0, lastRefillNano)
	return &state, nil
}

// SaveBucketState persists or updates the state of a bucket.
func (r *repositoryImpl) SaveBucketState(state *BucketState) error {
	lastRefillNano := state.LastRefill.UnixNano()

	_, err := r.db.Exec(`
		INSERT INTO rate_limit_buckets (id, tokens, last_refill)
		VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			tokens = excluded.tokens,
			last_refill = excluded.last_refill
	`, state.ID, state.Tokens, lastRefillNano)
	if err != nil {
		return fmt.Errorf("failed to save bucket state: %w", err)
	}

	return nil
}

// InitializeBucket creates a new bucket with initial capacity.
func (r *repositoryImpl) InitializeBucket(id string, initialTokens int) error {
	lastRefillNano := time.Now().UnixNano()

	_, err := r.db.Exec(
		"INSERT OR IGNORE INTO rate_limit_buckets (id, tokens, last_refill) VALUES (?, ?, ?)",
		id, initialTokens, lastRefillNano,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize bucket: %w", err)
	}

	return nil
}

// DeleteBucket removes a bucket's state from the database.
func (r *repositoryImpl) DeleteBucket(id string) error {
	_, err := r.db.Exec("DELETE FROM rate_limit_buckets WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete bucket: %w", err)
	}

	return nil
}

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
