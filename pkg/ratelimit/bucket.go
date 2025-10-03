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
	"sync"
	"time"
)

// Bucket implements a leaky bucket rate limiter.
type Bucket struct {
	capacity    int
	tokens      int
	refillRate  int
	refillEvery time.Duration
	mu          sync.Mutex
	lastRefill  time.Time
}

// NewBucket creates a new leaky bucket rate limiter.
func NewBucket(capacity, refillRate int, refillEvery time.Duration) *Bucket {
	return &Bucket{
		capacity:    capacity,
		tokens:      capacity,
		refillRate:  refillRate,
		refillEvery: refillEvery,
		lastRefill:  time.Now(),
	}
}

// refill adds tokens based on time elapsed since last refill.
func (b *Bucket) refill() {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill)

	if elapsed < b.refillEvery {
		return
	}

	intervals := int(elapsed / b.refillEvery)
	tokensToAdd := intervals * b.refillRate

	b.tokens += tokensToAdd
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}

	b.lastRefill = b.lastRefill.Add(time.Duration(intervals) * b.refillEvery)
}

// TryTake attempts to take the specified number of tokens from the bucket.
// Returns true if successful, false if not enough tokens available.
func (b *Bucket) TryTake(count int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill()

	if b.tokens >= count {
		b.tokens -= count
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

	b.refill()

	return b.tokens
}

// Reset resets the bucket to full capacity.
func (b *Bucket) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.tokens = b.capacity
	b.lastRefill = time.Now()
}
