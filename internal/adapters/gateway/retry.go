// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// RetryConfig configures exponential backoff retry behavior.
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// DefaultRetryConfig returns the standard retry configuration:
// 5 retries with exponential backoff (2s, 4s, 8s, 16s, 32s) capped at 60s.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 5,
		BaseDelay:  2 * time.Second,
		MaxDelay:   60 * time.Second,
	}
}

// isHardQuotaError checks if error indicates hard daily/monthly quota exhaustion.
// These errors require billing action and should not be retried.
// NOTE: TPM/RPM errors (429, RESOURCE_EXHAUSTED) are NOT hard quota errors - they're rate limits.
func isHardQuotaError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())

	// If it's a 429 or RESOURCE_EXHAUSTED, it's a rate limit, not a hard quota
	if strings.Contains(errMsg, "429") || strings.Contains(errMsg, "resource_exhausted") {
		return false
	}

	// Hard quota errors explicitly mention insufficient quota or payment required
	return (strings.Contains(errMsg, "quota") && strings.Contains(errMsg, "insufficient")) ||
		strings.Contains(errMsg, "payment required") ||
		strings.Contains(errMsg, "upgrade to paid")
}

// RetryWithBackoff executes fn with exponential backoff on retryable errors.
// Returns error if max retries exceeded or hard quota error encountered.
func RetryWithBackoff[T any](
	ctx context.Context,
	config RetryConfig,
	fn func(ctx context.Context) (T, error),
	errorContext string,
) (T, error) {
	var zero T

	for attempt := 1; attempt <= config.MaxRetries; attempt++ {
		if attempt > 1 {
			if err := waitForRetry(ctx, config, attempt); err != nil {
				return zero, err
			}
		}

		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}

		if isHardQuotaError(err) {
			return zero, fmt.Errorf("%s: hard quota exceeded - check your billing and plan: %w", errorContext, err)
		}

		if attempt >= config.MaxRetries {
			return zero, fmt.Errorf("%s: failed after %d attempts: %w", errorContext, attempt, err)
		}
	}

	return zero, fmt.Errorf("%s: failed after %d retries", errorContext, config.MaxRetries)
}

// waitForRetry waits the appropriate backoff duration before the next retry attempt.
func waitForRetry(ctx context.Context, config RetryConfig, attempt int) error {
	// Exponential backoff: baseDelay * 2^(attempt-2), capped to avoid overflow.
	// attempt is always >= 2 here, so (attempt-2) is non-negative.
	// Cap shift at 62 to prevent overflow of int64.
	const maxShift = 62
	shiftAmount := attempt - 2
	if shiftAmount > maxShift {
		shiftAmount = maxShift
	}
	waitDuration := config.BaseDelay * time.Duration(int64(1)<<shiftAmount)
	if waitDuration > config.MaxDelay {
		waitDuration = config.MaxDelay
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("retry cancelled: %w", ctx.Err())
	case <-time.After(waitDuration):
		return nil
	}
}
