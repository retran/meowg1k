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

package shutdown

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestNewService(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(logger, ctx, timeout)
	if service == nil {
		t.Fatal("NewService should not return nil")
	}

	// Test that context is available
	serviceCtx := service.Context()
	if serviceCtx == nil {
		t.Fatal("Service context should not be nil")
	}

	// Test that context is not cancelled initially
	select {
	case <-serviceCtx.Done():
		t.Error("Service context should not be cancelled initially")
	default:
		// Expected
	}
}

func TestRegister(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(logger, ctx, timeout)

	var callbackCalled bool
	callback := func(ctx context.Context) error {
		callbackCalled = true
		return nil
	}

	// Register callback should not panic or error
	service.Register(callback)

	// Manually trigger shutdown to test callback execution
	service.Shutdown()

	// Give some time for shutdown to complete
	time.Sleep(100 * time.Millisecond)

	if !callbackCalled {
		t.Error("Registered callback was not called during shutdown")
	}
}

func TestMultipleCallbacks(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(logger, ctx, timeout)

	var callbackOrder []int
	var mu sync.Mutex

	// Register multiple callbacks
	for i := 0; i < 3; i++ {
		idx := i
		callback := func(ctx context.Context) error {
			mu.Lock()
			callbackOrder = append(callbackOrder, idx)
			mu.Unlock()
			return nil
		}
		service.Register(callback)
	}

	// Trigger shutdown
	service.Shutdown()

	// Give some time for shutdown to complete
	time.Sleep(100 * time.Millisecond)

	// Check that callbacks were called in order
	mu.Lock()
	defer mu.Unlock()

	if len(callbackOrder) != 3 {
		t.Errorf("Expected 3 callbacks to be called, got %d", len(callbackOrder))
	}

	for i, order := range callbackOrder {
		if order != i {
			t.Errorf("Callback %d was called out of order, expected %d", order, i)
		}
	}
}

func TestCallbackError(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(logger, ctx, timeout)

	var successCallbackCalled bool
	testError := errors.New("test callback error")

	// Register a callback that fails
	service.Register(func(ctx context.Context) error {
		return testError
	})

	// Register a callback that succeeds (should still be called even after error)
	service.Register(func(ctx context.Context) error {
		successCallbackCalled = true
		return nil
	})

	// Trigger shutdown
	service.Shutdown()

	// Give some time for shutdown to complete
	time.Sleep(100 * time.Millisecond)

	// Success callback should still be called even after error
	if !successCallbackCalled {
		t.Error("Success callback should be called even after error in previous callback")
	}
}

func TestContextCancelledOnShutdown(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(logger, ctx, timeout)
	serviceCtx := service.Context()

	// Context should not be cancelled initially
	select {
	case <-serviceCtx.Done():
		t.Error("Context should not be cancelled initially")
	default:
		// Expected
	}

	// Trigger shutdown
	service.Shutdown()

	// Give some time for shutdown to process
	time.Sleep(100 * time.Millisecond)

	// Context should be cancelled after shutdown
	select {
	case <-serviceCtx.Done():
		// Expected
	default:
		t.Error("Context should be cancelled after shutdown")
	}
}

func TestShutdownTimeout(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 50 * time.Millisecond // Very short timeout

	service := NewService(logger, ctx, timeout)

	var callbackCompleted bool

	// Register a callback that takes longer than the timeout
	service.Register(func(ctx context.Context) error {
		select {
		case <-time.After(200 * time.Millisecond):
			callbackCompleted = true
			return nil
		case <-ctx.Done():
			// Callback should be cancelled due to timeout
			return ctx.Err()
		}
	})

	// Trigger shutdown
	start := time.Now()
	service.Shutdown()
	elapsed := time.Since(start)

	// Shutdown should complete within reasonable time due to timeout
	if elapsed > 150*time.Millisecond {
		t.Errorf("Shutdown took too long: %v, expected around %v", elapsed, timeout)
	}

	// The long-running callback should not have completed normally
	if callbackCompleted {
		t.Error("Long-running callback should have been cancelled due to timeout")
	}
}

func TestConcurrentAccess(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(logger, ctx, timeout)

	// Test concurrent registration and context access
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			service.Register(func(ctx context.Context) error {
				return nil
			})
		}(i)
		go func() {
			defer wg.Done()
			_ = service.Context()
		}()
	}

	wg.Wait()

	// Shutdown should work after concurrent operations
	service.Shutdown()

	// Give some time for shutdown to complete
	time.Sleep(100 * time.Millisecond)

	// Context should be cancelled
	select {
	case <-service.Context().Done():
		// Expected
	default:
		t.Error("Context should be cancelled after shutdown")
	}
}