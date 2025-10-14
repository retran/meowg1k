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
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/retran/meowg1k/internal/domain/ratelimit"
	"github.com/retran/meowg1k/internal/ports"
)

// Repository is a concrete implementation of the RateLimitRepository interface using SQLite.
type Repository struct {
	host ports.Host
}

// NewRepository creates a new Repository with the given database connection.
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
	db, err := r.host.GetDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	configMap := make(map[string]ratelimit.BucketConfig)
	for _, config := range configs {
		configMap[config.ID] = config
	}

	type bucketState struct {
		id        string
		newTokens int
		newRefill time.Time
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
	return nil
}

// InitializeBuckets initializes the rate limit buckets in the database.
func (r *Repository) InitializeBuckets(ctx context.Context, configs []ratelimit.BucketConfig) error {
	db, err := r.host.GetDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
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
	return nil
}

// ResetBuckets resets the tokens in the specified buckets to their full capacity.
func (r *Repository) ResetBuckets(ctx context.Context, configs []ratelimit.BucketConfig) error {
	db, err := r.host.GetDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
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
	return nil
}
