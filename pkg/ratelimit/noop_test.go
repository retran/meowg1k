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
