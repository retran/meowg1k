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
package ratelimit

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// Bucket implements a leaky bucket rate limiter with database persistence.
type Bucket struct {
	id          string
	capacity    int
	refillRate  int
	refillEvery time.Duration
	repo        Repository
	mu          sync.Mutex
}

// NewBucket creates a new leaky bucket rate limiter with database persistence.
// The bucket state is stored in the database and persists across restarts.
func NewBucket(id string, capacity, refillRate int, refillEvery time.Duration, repo Repository) (*Bucket, error) {
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

// loadState loads the current bucket state from the database.
func (b *Bucket) loadState() (*BucketState, error) {
	state, err := b.repo.GetBucketState(b.id)
	if err != nil {
		return nil, fmt.Errorf("failed to load bucket state: %w", err)
	}
	return state, nil
}

// saveState saves the current bucket state to the database.
func (b *Bucket) saveState(state *BucketState) error {
	if err := b.repo.SaveBucketState(state); err != nil {
		return fmt.Errorf("failed to save bucket state: %w", err)
	}
	return nil
}

// refill adds tokens based on time elapsed since last refill.
func (b *Bucket) refill(state *BucketState) {
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
func (b *Bucket) TryTake(count int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	state, err := b.loadState()
	if err != nil {
		return false
	}

	b.refill(state)

	if state.Tokens >= count {
		state.Tokens -= count
		if err := b.saveState(state); err != nil {
			return false
		}
		return true
	}

	return false
}

// Take waits until the specified number of tokens are available and takes them.
// Blocks until tokens are available or context is cancelled.
func (b *Bucket) Take(ctx context.Context, count int) error {
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
func (b *Bucket) Available() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	state, err := b.loadState()
	if err != nil {
		return 0
	}

	oldTokens := state.Tokens
	b.refill(state)

	// Only save if tokens were refilled
	if state.Tokens != oldTokens {
		if err := b.saveState(state); err != nil {
			return 0
		}
	}

	return state.Tokens
}

// Reset resets the bucket to full capacity.
func (b *Bucket) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	state := &BucketState{
		ID:         b.id,
		Tokens:     b.capacity,
		LastRefill: time.Now(),
	}

	_ = b.saveState(state)
}
