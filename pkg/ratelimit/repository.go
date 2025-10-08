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
	"errors"
	"fmt"
	"time"

	"github.com/retran/meowg1k/pkg/migrations"
)

var (
	// ErrDatabaseIsNil indicates that the database connection is nil.
	ErrDatabaseIsNil = errors.New("database connection is nil")
	// ErrUpdateFunctionIsNil indicates that the update function is nil.
	ErrUpdateFunctionIsNil = errors.New("update function is nil")
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

	// UpdateBucketStateAtomic performs an atomic read-modify-write operation on bucket state.
	// The updateFn receives the current state and should return the modified state.
	// If updateFn returns an error, the transaction is rolled back.
	UpdateBucketStateAtomic(id string, updateFn func(*BucketState) (*BucketState, error)) error

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
	if r == nil {
		return nil, ErrRepositoryIsNil
	}
	if r.db == nil {
		return nil, ErrDatabaseIsNil
	}
	if id == "" {
		return nil, ErrBucketIDIsEmpty
	}

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
	if r == nil {
		return ErrRepositoryIsNil
	}
	if r.db == nil {
		return ErrDatabaseIsNil
	}
	if state == nil {
		return ErrBucketStateIsNil
	}
	if state.ID == "" {
		return ErrBucketIDIsEmpty
	}

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

// UpdateBucketStateAtomic performs an atomic read-modify-write operation on bucket state.
func (r *repositoryImpl) UpdateBucketStateAtomic(id string, updateFn func(*BucketState) (*BucketState, error)) error {
	if r == nil {
		return ErrRepositoryIsNil
	}
	if r.db == nil {
		return ErrDatabaseIsNil
	}
	if id == "" {
		return ErrBucketIDIsEmpty
	}
	if updateFn == nil {
		return ErrUpdateFunctionIsNil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Will be no-op if transaction is committed

	// Read current state within transaction
	// SQLite uses database-level locking, so this read within a transaction
	// will block other writers until we commit, ensuring atomicity
	var tokens int
	var lastRefillNano int64
	err = tx.QueryRow(`
		SELECT tokens, last_refill FROM rate_limit_buckets WHERE id = ?
	`, id).Scan(&tokens, &lastRefillNano)

	if err == sql.ErrNoRows {
		// Bucket doesn't exist yet
		return fmt.Errorf("bucket %s not found", id)
	}
	if err != nil {
		return fmt.Errorf("failed to read bucket state: %w", err)
	}

	state := &BucketState{
		ID:         id,
		Tokens:     tokens,
		LastRefill: time.Unix(0, lastRefillNano),
	}

	// Apply the update function
	newState, err := updateFn(state)
	if err != nil {
		return fmt.Errorf("update function failed: %w", err)
	}

	// If newState is nil, skip the update (optimization for read-only operations)
	if newState != nil {
		// Write updated state within same transaction
		newLastRefillNano := newState.LastRefill.UnixNano()
		_, err = tx.Exec(`
			UPDATE rate_limit_buckets
			SET tokens = ?, last_refill = ?
			WHERE id = ?
		`, newState.Tokens, newLastRefillNano, id)
		if err != nil {
			return fmt.Errorf("failed to update bucket state: %w", err)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// InitializeBucket creates a new bucket with initial capacity.
func (r *repositoryImpl) InitializeBucket(id string, initialTokens int) error {
	if r == nil {
		return ErrRepositoryIsNil
	}
	if r.db == nil {
		return ErrDatabaseIsNil
	}
	if id == "" {
		return ErrBucketIDIsEmpty
	}
	if initialTokens < 0 {
		return ErrInvalidCapacity
	}

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
	if r == nil {
		return ErrRepositoryIsNil
	}
	if r.db == nil {
		return ErrDatabaseIsNil
	}
	if id == "" {
		return ErrBucketIDIsEmpty
	}

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
