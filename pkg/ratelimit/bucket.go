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

// Package ratelimit provides rate limiting functionality using the leaky bucket algorithm.
//
// Multi-Process Safety:
// All bucket operations (TryTake, Take, Available, Reset) use atomic database transactions
// to ensure consistency across multiple processes accessing the same database. This makes
// the rate limiter safe to use in distributed systems where multiple processes share the
// same SQLite database file.
//
// The implementation relies on SQLite's database-level locking to serialize concurrent
// write transactions, ensuring that rate limit checks and updates are atomic even when
// performed by different processes simultaneously.
package ratelimit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrBucketIsNil indicates that the bucket is nil.
	ErrBucketIsNil = errors.New("bucket is nil")
	// ErrRepositoryIsNil indicates that the repository is nil.
	ErrRepositoryIsNil = errors.New("repository is nil")
	// ErrBucketIDIsEmpty indicates that the bucket ID is empty.
	ErrBucketIDIsEmpty = errors.New("bucket ID is empty")
	// ErrInvalidCapacity indicates that the capacity is invalid.
	ErrInvalidCapacity = errors.New("capacity must be greater than 0")
	// ErrInvalidRefillRate indicates that the refill rate is invalid.
	ErrInvalidRefillRate = errors.New("refill rate must be greater than 0")
	// ErrInvalidRefillEvery indicates that the refill duration is invalid.
	ErrInvalidRefillEvery = errors.New("refill every must be greater than 0")
	// ErrInvalidTokenCount indicates that the token count is invalid.
	ErrInvalidTokenCount = errors.New("token count must be greater than 0")
	// ErrContextIsNil indicates that the context is nil.
	ErrContextIsNil = errors.New("context is nil")
	// ErrBucketStateIsNil indicates that the bucket state is nil.
	ErrBucketStateIsNil = errors.New("bucket state is nil")
)

// Bucket implements a leaky bucket rate limiter with database persistence.
// All operations are atomic at the database level, making it safe for multi-process use.
type Bucket struct {
	id          string
	capacity    int
	refillRate  int
	refillEvery time.Duration
	repo        Repository
}

// NewBucket creates a new leaky bucket rate limiter with database persistence.
// The bucket state is stored in the database and persists across restarts.
func NewBucket(id string, capacity, refillRate int, refillEvery time.Duration, repo Repository) (*Bucket, error) {
	if id == "" {
		return nil, ErrBucketIDIsEmpty
	}
	if capacity <= 0 {
		return nil, ErrInvalidCapacity
	}
	if refillRate <= 0 {
		return nil, ErrInvalidRefillRate
	}
	if refillEvery <= 0 {
		return nil, ErrInvalidRefillEvery
	}
	if repo == nil {
		return nil, ErrRepositoryIsNil
	}

	bucket := &Bucket{
		id:          id,
		capacity:    capacity,
		refillRate:  refillRate,
		refillEvery: refillEvery,
		repo:        repo,
	}

	// Try to load existing state or initialize new bucket
	_, err := repo.GetBucketState(id)
	if err == sql.ErrNoRows {
		// Bucket doesn't exist, initialize it
		if err := repo.InitializeBucket(id, capacity); err != nil {
			return nil, fmt.Errorf("failed to initialize bucket: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to get bucket state: %w", err)
	}

	return bucket, nil
}

// refill adds tokens based on time elapsed since last refill.
// This is called within atomic database transactions to ensure consistency.
func (b *Bucket) refill(state *BucketState) {
	if b == nil || state == nil {
		return
	}

	now := time.Now()
	elapsed := now.Sub(state.LastRefill)

	if elapsed < b.refillEvery {
		return
	}

	intervals := int(elapsed / b.refillEvery)
	tokensToAdd := intervals * b.refillRate

	state.Tokens += tokensToAdd
	if state.Tokens > b.capacity {
		state.Tokens = b.capacity
	}

	state.LastRefill = state.LastRefill.Add(time.Duration(intervals) * b.refillEvery)
}

// TryTake attempts to take the specified number of tokens from the bucket.
// Returns true if successful, false if not enough tokens available.
// This operation is atomic at the database level, safe for multi-process access.
func (b *Bucket) TryTake(count int) bool {
	if b == nil {
		return false
	}
	if count < 0 {
		return false
	}
	if b.repo == nil {
		return false
	}

	// No in-process lock needed - database transaction provides atomicity
	var success bool
	err := b.repo.UpdateBucketStateAtomic(b.id, func(state *BucketState) (*BucketState, error) {
		b.refill(state)

		if state.Tokens >= count {
			state.Tokens -= count
			success = true
		} else {
			success = false
		}

		return state, nil
	})

	return err == nil && success
}

// Take waits until the specified number of tokens are available and takes them.
// Blocks until tokens are available or context is cancelled.
func (b *Bucket) Take(ctx context.Context, count int) error {
	if b == nil {
		return ErrBucketIsNil
	}
	if ctx == nil {
		return ErrContextIsNil
	}
	if count <= 0 {
		return ErrInvalidTokenCount
	}

	for {
		if b.TryTake(count) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(b.refillEvery / 10): // Check 10 times per refill interval
		}
	}
}

// Available returns the current number of available tokens.
// This operation uses atomic database transaction to ensure consistency across processes.
func (b *Bucket) Available() int {
	if b == nil {
		return 0
	}
	if b.repo == nil {
		return 0
	}

	var tokens int
	err := b.repo.UpdateBucketStateAtomic(b.id, func(state *BucketState) (*BucketState, error) {
		b.refill(state)
		tokens = state.Tokens
		// Always update to persist refill changes
		return state, nil
	})
	if err != nil {
		return 0
	}

	return tokens
}

// Reset resets the bucket to full capacity.
// This operation uses atomic database transaction to ensure consistency across processes.
// Returns an error if the bucket cannot be reset.
func (b *Bucket) Reset() error {
	if b == nil {
		return ErrBucketIsNil
	}
	if b.repo == nil {
		return ErrRepositoryIsNil
	}

	return b.repo.UpdateBucketStateAtomic(b.id, func(state *BucketState) (*BucketState, error) {
		state.Tokens = b.capacity
		state.LastRefill = time.Now()
		return state, nil
	})
}
