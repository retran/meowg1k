// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package ratelimit provides rate limiting services for LLM API calls with token-based and request-based limits.
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/retran/meowg1k/internal/domain/ratelimit"
	"github.com/retran/meowg1k/internal/ports"
)

type Limiter interface {
	Wait(ctx context.Context, tokenCount int) error
	TryAcquire(ctx context.Context, tokenCount int) bool
}

type dbLimiter struct {
	repo    ports.RateLimitRepository
	configs []ratelimit.BucketConfig
}

type Config struct {
	ID                string
	RequestsPerMinute int
	TokensPerMinute   int
	RequestsPerDay    int
}

var Unlimited = Config{
	ID:                "unlimited",
	RequestsPerMinute: 0,
	TokensPerMinute:   0,
	RequestsPerDay:    0,
}

func NewLimiter(ctx context.Context, config Config, repo ports.RateLimitRepository) (Limiter, error) {
	if config.ID == "" {
		return nil, fmt.Errorf("config ID is empty")
	}
	if repo == nil {
		return nil, fmt.Errorf("repository is nil for config %q", config.ID)
	}

	var configs []ratelimit.BucketConfig
	if config.RequestsPerMinute > 0 {
		configs = append(configs, ratelimit.BucketConfig{
			ID:          config.ID + ":rpm",
			Capacity:    config.RequestsPerMinute,
			RefillRate:  config.RequestsPerMinute,
			RefillEvery: time.Minute,
		})
	}
	if config.TokensPerMinute > 0 {
		configs = append(configs, ratelimit.BucketConfig{
			ID:          config.ID + ":tpm",
			Capacity:    config.TokensPerMinute,
			RefillRate:  config.TokensPerMinute,
			RefillEvery: time.Minute,
		})
	}
	if config.RequestsPerDay > 0 {
		configs = append(configs, ratelimit.BucketConfig{
			ID:          config.ID + ":rpd",
			Capacity:    config.RequestsPerDay,
			RefillRate:  config.RequestsPerDay,
			RefillEvery: 24 * time.Hour,
		})
	}

	if len(configs) == 0 {
		return NewNoOpLimiter(), nil
	}

	if err := repo.InitializeBuckets(ctx, configs); err != nil {
		return nil, fmt.Errorf("failed to initialize rate limit buckets for config %q: %w", config.ID, err)
	}

	return &dbLimiter{
		repo:    repo,
		configs: configs,
	}, nil
}

// TryAcquire attempts to acquire the specified number of tokens without blocking.
func (l *dbLimiter) TryAcquire(ctx context.Context, tokenCount int) bool {
	requests := l.buildRequests(tokenCount)
	if len(requests) == 0 {
		return true
	}

	err := l.repo.AcquireTokens(ctx, l.configs, requests)
	return err == nil
}

func (l *dbLimiter) Wait(ctx context.Context, tokenCount int) error {
	requests := l.buildRequests(tokenCount)
	if len(requests) == 0 {
		return nil
	}

	const pollInterval = 100 * time.Millisecond

	for {
		err := l.repo.AcquireTokens(ctx, l.configs, requests)
		if err == nil {
			return nil
		}

		// Check if this is a "not enough tokens" error using type assertion
		var notEnoughTokensErr *ratelimit.NotEnoughTokensError
		if !errors.As(err, &notEnoughTokensErr) {
			// If it's not a token shortage error, return it immediately with context
			return fmt.Errorf("failed to acquire tokens from rate limiter: %w", err)
		}

		// Wait and retry for token shortage errors
		select {
		case <-ctx.Done():
			return fmt.Errorf("rate limiter wait interrupted: %w", ctx.Err())
		case <-time.After(pollInterval):
			// Repeat the attempt
		}
	}
}

// buildRequests constructs the list of AcquisitionRequests based on the token count and configured buckets.
func (l *dbLimiter) buildRequests(tokenCount int) []ratelimit.AcquisitionRequest {
	var requests []ratelimit.AcquisitionRequest
	for _, config := range l.configs {
		switch {
		case config.RefillEvery == time.Minute && config.ID[len(config.ID)-4:] == ":rpm":
			requests = append(requests, ratelimit.AcquisitionRequest{ID: config.ID, Count: 1})
		case config.RefillEvery == time.Minute && config.ID[len(config.ID)-4:] == ":tpm":
			if tokenCount > 0 {
				requests = append(requests, ratelimit.AcquisitionRequest{ID: config.ID, Count: tokenCount})
			}
		case config.RefillEvery == 24*time.Hour:
			requests = append(requests, ratelimit.AcquisitionRequest{ID: config.ID, Count: 1})
		}
	}
	return requests
}

// noOpLimiter is a limiter that does nothing.
type noOpLimiter struct{}

// NewNoOpLimiter creates a new no-op limiter instance.
func NewNoOpLimiter() Limiter {
	return &noOpLimiter{}
}

// Wait is a no-op implementation that always succeeds.
func (n *noOpLimiter) Wait(ctx context.Context, tokenCount int) error {
	return nil
}

// TryAcquire is a no-op implementation that always succeeds.
func (n *noOpLimiter) TryAcquire(ctx context.Context, tokenCount int) bool {
	return true
}
