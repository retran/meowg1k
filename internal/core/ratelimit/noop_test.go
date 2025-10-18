// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ratelimit

import (
	"context"
	"testing"
)

func TestNoOpLimiter_Wait(t *testing.T) {
	limiter := NewNoOpLimiter()
	ctx := context.Background()

	// Should always return nil immediately
	err := limiter.Wait(ctx, 0)
	if err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}

	err = limiter.Wait(ctx, 100)
	if err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}

	err = limiter.Wait(ctx, 1000000)
	if err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
}

func TestNoOpLimiter_TryAcquire(t *testing.T) {
	limiter := NewNoOpLimiter()
	ctx := context.Background()

	// Should always return true
	if !limiter.TryAcquire(ctx, 0) {
		t.Error("TryAcquire(0) = false, want true")
	}

	if !limiter.TryAcquire(ctx, 100) {
		t.Error("TryAcquire(100) = false, want true")
	}

	if !limiter.TryAcquire(ctx, 1000000) {
		t.Error("TryAcquire(1000000) = false, want true")
	}
}

func TestNoOpLimiter_ImplementsInterface(t *testing.T) {
	var _ Limiter = &noOpLimiter{}
}
