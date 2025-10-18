// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package ratelimit defines domain types for API rate limiting including token budgets and request limits.
package ratelimit

import (
	"fmt"
	"time"
)

// NotEnoughTokensError is returned when there are not enough tokens in a bucket.
type NotEnoughTokensError struct {
	BucketID string
	Need     int
	Have     int
}

func (e *NotEnoughTokensError) Error() string {
	return fmt.Sprintf("not enough tokens in bucket %q: need %d, have %d", e.BucketID, e.Need, e.Have)
}

// BucketConfig defines the configuration for a rate limit bucket.
type BucketConfig struct {
	ID          string
	Capacity    int
	RefillRate  int
	RefillEvery time.Duration
}

// AcquisitionRequest represents a request to acquire tokens from a specific bucket.
type AcquisitionRequest struct {
	ID    string
	Count int
}
