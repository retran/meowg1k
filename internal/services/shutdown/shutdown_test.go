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
	"sync/atomic"
	"testing"
	"time"
)

var (
	errTestCallback = errors.New("test callback error")
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

	// Test that context is not canceled initially
	select {
	case <-serviceCtx.Done():
		t.Error("Service context should not be canceled initially")
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
			defer mu.Unlock()
			callbackOrder = append(callbackOrder, idx)
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

	// Register a callback that fails
	service.Register(func(ctx context.Context) error {
		return errTestCallback
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

	// The success callback should still be called
	if !successCallbackCalled {
		t.Error("Success callback should be called even after a previous callback failed")
	}
}

func TestContextCanceledOnShutdown(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(logger, ctx, timeout)
	serviceCtx := service.Context()

	// Context should not be canceled initially
	select {
	case <-serviceCtx.Done():
		t.Error("Context should not be canceled initially")
	default:
		// Expected
	}

	// Trigger shutdown
	service.Shutdown()

	// Give some time for shutdown to process
	time.Sleep(100 * time.Millisecond)

	// Context should be canceled after shutdown
	select {
	case <-serviceCtx.Done():
		// Expected
	default:
		t.Error("Context should be canceled after shutdown")
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
			// Callback should be canceled due to timeout
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
		t.Error("Long-running callback should have been canceled due to timeout")
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
		go func() {
			defer wg.Done()
			service.Register(func(ctx context.Context) error {
				return nil
			})
		}()
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

	// Context should be canceled
	select {
	case <-service.Context().Done():
		// Expected
	default:
		t.Error("Context should be canceled after shutdown")
	}
}

func TestListenForSignalsContextCancellation(t *testing.T) {
	logger := slog.Default()
	ctx, cancel := context.WithCancel(context.Background())
	timeout := 5 * time.Second

	service := NewService(logger, ctx, timeout)

	// Start ListenForSignals in a goroutine
	done := make(chan bool, 1)

	go func() {
		service.ListenForSignals()
		done <- true
	}()

	// Cancel the context after a short delay
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Wait for ListenForSignals to return
	select {
	case <-done:
		// Expected - context cancellation should cause ListenForSignals to return
	case <-time.After(1 * time.Second):
		t.Error("ListenForSignals should have returned after context cancellation")
	}
}

func TestListenForSignalsShutdown(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(logger, ctx, timeout)

	// Start ListenForSignals in a goroutine
	done := make(chan bool, 1)

	go func() {
		service.ListenForSignals()
		done <- true
	}()

	// Trigger shutdown manually after a short delay
	time.Sleep(50 * time.Millisecond)
	service.Shutdown()

	// Wait for ListenForSignals to return
	select {
	case <-done:
		// ListenForSignals should have detected the shutdown and returned
		// The exact return value depends on implementation details
	case <-time.After(1 * time.Second):
		t.Error("ListenForSignals should have returned after manual shutdown")
	}
}

func TestMultipleShutdownCalls(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(logger, ctx, timeout)

	var callbackCount int32
	service.Register(func(ctx context.Context) error {
		atomic.AddInt32(&callbackCount, 1)
		return nil
	})

	// Call shutdown multiple times
	service.Shutdown()
	service.Shutdown()
	service.Shutdown()

	// Give some time for shutdown to complete
	time.Sleep(100 * time.Millisecond)

	// Callback should be called for each shutdown call (if that's the implementation behavior)
	callCount := atomic.LoadInt32(&callbackCount)
	if callCount < 1 {
		t.Errorf("Expected callback to be called at least once, got %d times", callCount)
	}
	// Note: Multiple shutdown calls may execute callbacks multiple times depending on implementation
}

func TestShutdownWithSlowCallback(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 200 * time.Millisecond // Short timeout for test

	service := NewService(logger, ctx, timeout)

	var callbackStarted, callbackCompleted bool
	service.Register(func(ctx context.Context) error {
		callbackStarted = true
		select {
		case <-time.After(500 * time.Millisecond): // Longer than timeout
			callbackCompleted = true
			return nil
		case <-ctx.Done():
			// Context was canceled due to timeout
			return ctx.Err()
		}
	})

	// Trigger shutdown
	start := time.Now()
	service.Shutdown()
	elapsed := time.Since(start)

	// Shutdown should have completed within the timeout period
	if elapsed > 300*time.Millisecond {
		t.Errorf("Shutdown took too long: %v, expected around %v", elapsed, timeout)
	}

	// Callback should have started but not completed normally
	if !callbackStarted {
		t.Error("Callback should have started")
	}

	if callbackCompleted {
		t.Error("Callback should not have completed normally due to timeout")
	}
}

func TestServiceContextAfterShutdown(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(logger, ctx, timeout)
	serviceCtx := service.Context()

	// Context should not be canceled initially
	select {
	case <-serviceCtx.Done():
		t.Error("Context should not be canceled initially")
	default:
		// Expected
	}

	// Trigger shutdown
	service.Shutdown()

	// Give some time for shutdown to process
	time.Sleep(100 * time.Millisecond)

	// Context should be canceled after shutdown
	select {
	case <-serviceCtx.Done():
		// Expected
	default:
		t.Error("Context should be canceled after shutdown")
	}
}

func TestCallbackErrorHandling(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(logger, ctx, timeout)

	var successCallbackCalled bool

	// Register a callback that fails
	service.Register(func(ctx context.Context) error {
		return errTestCallback
	})

	// Register a callback that succeeds (should still be called)
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
