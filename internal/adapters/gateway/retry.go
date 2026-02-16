// Copyright © 2025 The meowg1k Authors
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

// isRateLimitError checks if an error is a rate limit error.
// Common indicators: 429 status, "rate limit", "quota", "resource_exhausted", "too many requests"
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "429") ||
		strings.Contains(errMsg, "rate limit") ||
		strings.Contains(errMsg, "quota") ||
		strings.Contains(errMsg, "resource_exhausted") ||
		strings.Contains(errMsg, "too many requests") ||
		strings.Contains(errMsg, "too_many_requests")
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
		// Calculate wait duration with exponential backoff capped at MaxDelay
		var waitDuration time.Duration
		if attempt > 1 {
			// Exponential backoff: baseDelay * 2^(attempt-2)
			waitDuration = config.BaseDelay * time.Duration(1<<uint(attempt-2))
			if waitDuration > config.MaxDelay {
				waitDuration = config.MaxDelay
			}

			// Wait before retry
			select {
			case <-ctx.Done():
				return zero, fmt.Errorf("retry cancelled: %w", ctx.Err())
			case <-time.After(waitDuration):
			}
		}

		// Execute function
		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}

		// Check for hard quota errors (don't retry)
		if isHardQuotaError(err) {
			return zero, fmt.Errorf("%s: hard quota exceeded - check your billing and plan: %w", errorContext, err)
		}

		// Last attempt failed
		if attempt >= config.MaxRetries {
			return zero, fmt.Errorf("%s: failed after %d attempts: %w", errorContext, attempt, err)
		}

		// Continue to next retry
	}

	return zero, fmt.Errorf("%s: failed after %d retries", errorContext, config.MaxRetries)
}
