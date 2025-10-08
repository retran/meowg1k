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
	"time"
)

// Limiter defines the interface for rate limiting operations.
type Limiter interface {
	// Wait waits until the request with the specified token count can be processed.
	// Blocks until all limits allow the request or context is cancelled.
	Wait(ctx context.Context, tokenCount int) error

	// TryAcquire attempts to acquire resources for a request with the specified token count.
	// Returns true if successful, false if any limit would be exceeded.
	TryAcquire(tokenCount int) bool
}

// dbLimiter provides multi-dimensional rate limiting with database persistence.
type dbLimiter struct {
	rpm *Bucket // Requests per minute
	tpm *Bucket // Tokens per minute (input)
	rpd *Bucket // Requests per day
	id  string  // Unique identifier for this limiter
}

// Config defines rate limiting configuration.
type Config struct {
	RequestsPerMinute int
	TokensPerMinute   int
	RequestsPerDay    int
}

// Unlimited is a predefined configuration that disables all rate limits.
var Unlimited = Config{
	RequestsPerMinute: 0,
	TokensPerMinute:   0,
	RequestsPerDay:    0,
}

// NewLimiter creates a new multi-dimensional rate limiter with database persistence.
// The id parameter is used to uniquely identify this limiter's buckets in the database.
func NewLimiter(id string, config Config, repo Repository) (Limiter, error) {
	limiter := &dbLimiter{
		id: id,
	}

	var err error

	if config.RequestsPerMinute > 0 {
		limiter.rpm, err = NewBucket(id+":rpm", config.RequestsPerMinute, config.RequestsPerMinute, time.Minute, repo)
		if err != nil {
			return nil, err
		}
	}

	if config.TokensPerMinute > 0 {
		limiter.tpm, err = NewBucket(id+":tpm", config.TokensPerMinute, config.TokensPerMinute, time.Minute, repo)
		if err != nil {
			return nil, err
		}
	}

	if config.RequestsPerDay > 0 {
		limiter.rpd, err = NewBucket(id+":rpd", config.RequestsPerDay, config.RequestsPerDay, 24*time.Hour, repo)
		if err != nil {
			return nil, err
		}
	}

	return limiter, nil
}

// Wait waits until the request with the specified token count can be processed.
// Blocks until all limits allow the request or context is cancelled.
func (l *dbLimiter) Wait(ctx context.Context, tokenCount int) error {
	if l.rpm != nil {
		if err := l.rpm.Take(ctx, 1); err != nil {
			return err
		}
	}

	if l.tpm != nil && tokenCount > 0 {
		if err := l.tpm.Take(ctx, tokenCount); err != nil {
			return err
		}
	}

	if l.rpd != nil {
		if err := l.rpd.Take(ctx, 1); err != nil {
			return err
		}
	}

	return nil
}

// TryAcquire attempts to acquire resources for a request with the specified token count.
// Returns true if successful, false if any limit would be exceeded.
func (l *dbLimiter) TryAcquire(tokenCount int) bool {
	if l.rpm != nil && !l.rpm.TryTake(0) {
		return false
	}

	if l.tpm != nil && tokenCount > 0 && l.tpm.Available() < tokenCount {
		return false
	}

	if l.rpd != nil && !l.rpd.TryTake(0) {
		return false
	}

	if l.rpm != nil {
		l.rpm.TryTake(1)
	}

	if l.tpm != nil && tokenCount > 0 {
		l.tpm.TryTake(tokenCount)
	}

	if l.rpd != nil {
		l.rpd.TryTake(1)
	}

	return true
}

// Reset resets all rate limit buckets to full capacity.
func (l *dbLimiter) Reset() {
	if l.rpm != nil {
		l.rpm.Reset()
	}
	if l.tpm != nil {
		l.tpm.Reset()
	}
	if l.rpd != nil {
		l.rpd.Reset()
	}
}

// Stats returns current statistics for all dimensions.
func (l *dbLimiter) Stats() (rpm, tpm, rpd int) {
	if l.rpm != nil {
		rpm = l.rpm.Available()
	}
	if l.tpm != nil {
		tpm = l.tpm.Available()
	}
	if l.rpd != nil {
		rpd = l.rpd.Available()
	}
	return rpm, tpm, rpd
}

// noOpLimiter is a rate limiter implementation that does nothing.
// It's used when no rate limits are configured for a model.
type noOpLimiter struct{}

// NewNoOpLimiter creates a new no-op rate limiter.
func NewNoOpLimiter() Limiter {
	return &noOpLimiter{}
}

// Wait always returns immediately without error.
func (n *noOpLimiter) Wait(ctx context.Context, tokenCount int) error {
	return nil
}

// TryAcquire always returns true.
func (n *noOpLimiter) TryAcquire(tokenCount int) bool {
	return true
}
