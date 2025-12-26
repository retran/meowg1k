// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package ratelimit provides a SQLite-based repository for tracking and enforcing API rate limits.
package ratelimit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/retran/meowg1k/internal/domain/ratelimit"
	"github.com/retran/meowg1k/internal/ports"
)

type Repository struct {
	host ports.Host
}

func NewRepository(host ports.Host) *Repository {
	return &Repository{
		host: host,
	}
}

func refill(tokens, capacity, refillRate int, lastRefill time.Time, refillEvery time.Duration) (int, time.Time) {
	now := time.Now()
	elapsed := now.Sub(lastRefill)
	if elapsed < refillEvery {
		return tokens, lastRefill
	}

	intervals := int(elapsed / refillEvery)
	if intervals == 0 {
		return tokens, lastRefill
	}

	tokens += intervals * refillRate
	if tokens > capacity {
		tokens = capacity
	}

	newLastRefill := lastRefill.Add(time.Duration(intervals) * refillEvery)
	return tokens, newLastRefill
}

// AcquireTokens attempts to acquire the specified number of tokens from the given buckets.
func (r *Repository) AcquireTokens(ctx context.Context, configs []ratelimit.BucketConfig, requests []ratelimit.AcquisitionRequest) error {
	db, err := r.host.GetMainDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	// Use IMMEDIATE transaction to prevent database lock contention
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	configMap := make(map[string]ratelimit.BucketConfig)
	for _, config := range configs {
		configMap[config.ID] = config
	}

	type bucketState struct {
		newRefill time.Time
		id        string
		newTokens int
	}
	var statesToUpdate []bucketState

	for _, req := range requests {
		if req.Count <= 0 {
			continue
		}

		config, ok := configMap[req.ID]
		if !ok {
			return fmt.Errorf("config for bucket %s not found", req.ID)
		}

		var currentTokens int
		var lastRefillNano int64
		err := tx.QueryRowContext(ctx, "SELECT tokens, last_refill FROM rate_limit_buckets WHERE id = ?", req.ID).Scan(&currentTokens, &lastRefillNano)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("bucket %q not initialized: bucket not found", req.ID)
			}
			return fmt.Errorf("failed to read bucket %q: %w", req.ID, err)
		}

		refilledTokens, newLastRefill := refill(currentTokens, config.Capacity, config.RefillRate, time.Unix(0, lastRefillNano), config.RefillEvery)

		if refilledTokens < req.Count {
			return &ratelimit.NotEnoughTokensError{
				BucketID: req.ID,
				Need:     req.Count,
				Have:     refilledTokens,
			}
		}

		statesToUpdate = append(statesToUpdate, bucketState{
			id:        req.ID,
			newTokens: refilledTokens - req.Count,
			newRefill: newLastRefill,
		})
	}

	stmt, err := tx.PrepareContext(ctx, "UPDATE rate_limit_buckets SET tokens = ?, last_refill = ? WHERE id = ?")
	if err != nil {
		return fmt.Errorf("failed to prepare update statement: %w", err)
	}
	defer stmt.Close()

	for _, state := range statesToUpdate {
		_, err := stmt.ExecContext(ctx, state.newTokens, state.newRefill.UnixNano(), state.id)
		if err != nil {
			return fmt.Errorf("failed to update bucket %s: %w", state.id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit token acquisition transaction: %w", err)
	}

	// Perform WAL checkpoint to ensure writes are visible to other connections
	_, err = db.ExecContext(ctx, "PRAGMA wal_checkpoint(PASSIVE)")
	if err != nil {
		// Log but don't fail - checkpoint failure is not critical
		return fmt.Errorf("warning: WAL checkpoint failed: %w", err)
	}

	return nil
}

// InitializeBuckets initializes the rate limit buckets in the database.
func (r *Repository) InitializeBuckets(ctx context.Context, configs []ratelimit.BucketConfig) error {
	db, err := r.host.GetMainDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	// Use IMMEDIATE transaction to prevent database lock contention
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("failed to begin transaction for init: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO rate_limit_buckets (id, tokens, last_refill) VALUES (?, ?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement for init: %w", err)
	}
	defer stmt.Close()

	for _, config := range configs {
		_, err := stmt.ExecContext(ctx, config.ID, config.Capacity, time.Now().UnixNano())
		if err != nil {
			return fmt.Errorf("failed to initialize bucket %s: %w", config.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit bucket initialization transaction: %w", err)
	}

	// Perform WAL checkpoint to ensure writes are visible to other connections
	_, err = db.ExecContext(ctx, "PRAGMA wal_checkpoint(PASSIVE)")
	if err != nil {
		// Log but don't fail - checkpoint failure is not critical
		return fmt.Errorf("warning: WAL checkpoint failed: %w", err)
	}

	return nil
}

// ResetBuckets resets the tokens in the specified buckets to their full capacity.
func (r *Repository) ResetBuckets(ctx context.Context, configs []ratelimit.BucketConfig) error {
	db, err := r.host.GetMainDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	// Use IMMEDIATE transaction to prevent database lock contention
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("failed to begin transaction for reset: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "UPDATE rate_limit_buckets SET tokens = ?, last_refill = ? WHERE id = ?")
	if err != nil {
		return fmt.Errorf("failed to prepare update statement for reset: %w", err)
	}
	defer stmt.Close()

	for _, config := range configs {
		_, err := stmt.ExecContext(ctx, config.Capacity, time.Now().UnixNano(), config.ID)
		if err != nil {
			return fmt.Errorf("failed to reset bucket %s: %w", config.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit bucket reset transaction: %w", err)
	}

	// Perform WAL checkpoint to ensure writes are visible to other connections
	_, err = db.ExecContext(ctx, "PRAGMA wal_checkpoint(PASSIVE)")
	if err != nil {
		// Log but don't fail - checkpoint failure is not critical
		return fmt.Errorf("warning: WAL checkpoint failed: %w", err)
	}

	return nil
}
