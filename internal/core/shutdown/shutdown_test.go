// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package shutdown

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

var errTestCallback = errors.New("test callback error")

func TestNewService(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(ctx, logger, timeout)
	if service == nil {
		t.Fatal("NewService should not return nil")
	}

	serviceCtx := service.Context()
	if serviceCtx == nil {
		t.Fatal("Service context should not be nil")
	}

	select {
	case <-serviceCtx.Done():
		t.Error("Service context should not be canceled initially")
	default:
	}
}

func TestRegister(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(ctx, logger, timeout)

	var callbackCalled bool
	callback := func(ctx context.Context) error {
		callbackCalled = true
		return nil
	}

	if err := service.Register(callback); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

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

	service := NewService(ctx, logger, timeout)

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
		if err := service.Register(callback); err != nil {
			t.Fatalf("Register failed: %v", err)
		}
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

	service := NewService(ctx, logger, timeout)

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

	service := NewService(ctx, logger, timeout)
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

	service := NewService(ctx, logger, timeout)

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

	service := NewService(ctx, logger, timeout)

	// Test overlapping registration and context access.
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

	// Shutdown should work after overlapping operations.
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

	service := NewService(ctx, logger, timeout)

	// Start ListenForSignals in the background.
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

	service := NewService(ctx, logger, timeout)

	// Start ListenForSignals in the background.
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

	service := NewService(ctx, logger, timeout)

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

	service := NewService(ctx, logger, timeout)

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

	service := NewService(ctx, logger, timeout)
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

	service := NewService(ctx, logger, timeout)

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

func TestListenForSignals_ContextCanceled(t *testing.T) {
	logger := slog.Default()
	ctx, cancel := context.WithCancel(context.Background())
	timeout := 5 * time.Second

	service := NewService(ctx, logger, timeout)

	// Start listening in the background.
	var result bool
	done := make(chan bool)
	go func() {
		result = service.ListenForSignals()
		done <- true
	}()

	// Give listener time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel the context
	cancel()

	// Wait for ListenForSignals to return
	select {
	case <-done:
		// Expected
	case <-time.After(time.Second):
		t.Fatal("ListenForSignals did not return after context cancellation")
	}

	// Should return false when context is canceled
	if result {
		t.Error("ListenForSignals should return false when context is canceled")
	}
}

func TestNewServiceWithCanceledContext(t *testing.T) {
	logger := slog.Default()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	timeout := 5 * time.Second

	service := NewService(ctx, logger, timeout)

	if service == nil {
		t.Fatal("NewService should not return nil even with canceled context")
	}

	serviceCtx := service.Context()

	// Service context should be separate from input context
	select {
	case <-serviceCtx.Done():
		// This is OK - the service context might inherit cancellation
	default:
		// This is also OK - service creates its own context
	}
}

func TestNewServiceWithZeroTimeout(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 0 * time.Second

	service := NewService(ctx, logger, timeout)

	if service == nil {
		t.Fatal("NewService should not return nil with zero timeout")
	}

	var callbackCalled bool
	service.Register(func(ctx context.Context) error {
		callbackCalled = true
		return nil
	})

	service.Shutdown()
	time.Sleep(100 * time.Millisecond)

	if !callbackCalled {
		t.Error("Callback should be called even with zero timeout")
	}
}

func TestNewServiceWithNilLogger(t *testing.T) {
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(ctx, nil, timeout)

	if service == nil {
		t.Fatal("NewService should not return nil with nil logger")
	}

	var callbackCalled bool
	service.Register(func(ctx context.Context) error {
		callbackCalled = true
		return nil
	})

	service.Shutdown()
	time.Sleep(100 * time.Millisecond)

	if !callbackCalled {
		t.Error("Callback should be called with nil logger (using default)")
	}
}

func TestConcurrentRegisterAndShutdown(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(ctx, logger, timeout)

	var wg sync.WaitGroup
	callbackCount := atomic.Int32{}

	// Start registering callbacks in the background.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			service.Register(func(ctx context.Context) error {
				callbackCount.Add(1)
				return nil
			})
		}()
	}

	// Wait for all registrations
	wg.Wait()

	// Trigger shutdown
	service.Shutdown()
	time.Sleep(200 * time.Millisecond)

	// All callbacks should have been called
	if callbackCount.Load() != 10 {
		t.Errorf("Expected 10 callbacks to be called, got %d", callbackCount.Load())
	}
}

func TestListenForSignals_SIGINT(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(ctx, logger, timeout)

	var callbackCalled bool
	service.Register(func(ctx context.Context) error {
		callbackCalled = true
		return nil
	})

	// Start listening in the background.
	done := make(chan bool)
	go func() {
		result := service.ListenForSignals()
		// Should return true when signal is received
		if !result {
			t.Error("ListenForSignals should return true when signal is received")
		}
		done <- true
	}()

	// Give listener time to start
	time.Sleep(50 * time.Millisecond)

	// Send SIGINT to current process
	pid := os.Getpid()
	process, err := os.FindProcess(pid)
	if err != nil {
		t.Fatalf("Failed to find current process: %v", err)
	}

	err = process.Signal(syscall.SIGINT)
	if err != nil {
		t.Fatalf("Failed to send SIGINT: %v", err)
	}

	// Wait for ListenForSignals to return
	select {
	case <-done:
		// Expected - signal should trigger shutdown
	case <-time.After(time.Second):
		t.Error("ListenForSignals did not return after SIGINT")
	}

	// Give some time for callback to execute
	time.Sleep(100 * time.Millisecond)

	if !callbackCalled {
		t.Error("Callback should be called after SIGINT")
	}
}

func TestListenForSignals_SIGTERM(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(ctx, logger, timeout)

	var callbackCalled bool
	service.Register(func(ctx context.Context) error {
		callbackCalled = true
		return nil
	})

	// Start listening in the background.
	done := make(chan bool)
	go func() {
		result := service.ListenForSignals()
		// Should return true when signal is received
		if !result {
			t.Error("ListenForSignals should return true when signal is received")
		}
		done <- true
	}()

	// Give listener time to start
	time.Sleep(50 * time.Millisecond)

	// Send SIGTERM to current process
	pid := os.Getpid()
	process, err := os.FindProcess(pid)
	if err != nil {
		t.Fatalf("Failed to find current process: %v", err)
	}

	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		t.Fatalf("Failed to send SIGTERM: %v", err)
	}

	// Wait for ListenForSignals to return
	select {
	case <-done:
		// Expected - signal should trigger shutdown
	case <-time.After(time.Second):
		t.Error("ListenForSignals did not return after SIGTERM")
	}

	// Give some time for callback to execute
	time.Sleep(100 * time.Millisecond)

	if !callbackCalled {
		t.Error("Callback should be called after SIGTERM")
	}
}
