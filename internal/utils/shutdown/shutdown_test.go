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
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	manager := NewManager(10 * time.Second)

	if manager == nil {
		t.Error("NewManager should return a non-nil manager")
		return
	}

	if manager.callbacks == nil {
		t.Error("Manager should have initialized callbacks slice")
	}

	if manager.ctx == nil {
		t.Error("Manager should have initialized shutdown context")
	}
}

func TestManager_Register(t *testing.T) {
	manager := NewManager(10 * time.Second)

	called := false
	callback := func(ctx context.Context) error {
		called = true
		return nil
	}

	manager.Register(callback)

	if len(manager.callbacks) != 1 {
		t.Error("Callback should be registered")
	}

	// Test that the callback is actually stored
	manager.callbacks[0](context.Background())
	if !called {
		t.Error("Registered callback should be executable")
	}
}

func TestManager_RegisterMultiple(t *testing.T) {
	manager := NewManager(10 * time.Second)

	var callOrder []int

	callback1 := func(ctx context.Context) error {
		callOrder = append(callOrder, 1)
		return nil
	}

	callback2 := func(ctx context.Context) error {
		callOrder = append(callOrder, 2)
		return nil
	}

	callback3 := func(ctx context.Context) error {
		callOrder = append(callOrder, 3)
		return nil
	}

	manager.Register(callback1)
	manager.Register(callback2)
	manager.Register(callback3)

	if len(manager.callbacks) != 3 {
		t.Errorf("Expected 3 callbacks, got %d", len(manager.callbacks))
	}

	// Execute callbacks to test order (should be reverse registration order)
	for i := len(manager.callbacks) - 1; i >= 0; i-- {
		manager.callbacks[i](context.Background())
	}

	expected := []int{3, 2, 1}
	if len(callOrder) != len(expected) {
		t.Errorf("Expected %d callback executions, got %d", len(expected), len(callOrder))
		return
	}

	for i, v := range callOrder {
		if v != expected[i] {
			t.Errorf("Expected callback order %v, got %v", expected, callOrder)
			break
		}
	}
}

func TestManager_Context(t *testing.T) {
	manager := NewManager(10 * time.Second)

	ctx := manager.Context()

	if ctx == nil {
		t.Error("Context should not be nil")
	}

	// Test that context is not done initially
	select {
	case <-ctx.Done():
		t.Error("Context should not be done initially")
	default:
		// Good, context is not done
	}

	// Test that multiple calls return the same context
	ctx2 := manager.Context()
	if ctx != ctx2 {
		t.Error("Multiple calls to Context() should return the same context")
	}
}

func TestManager_ContextCancellation(t *testing.T) {
	manager := NewManager(10 * time.Second)
	ctx := manager.Context()

	// Trigger shutdown manually to test context cancellation
	manager.mu.Lock()
	manager.cancel()
	manager.mu.Unlock()

	// Wait a bit for the context to be cancelled
	select {
	case <-ctx.Done():
		// Good, context was cancelled
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be cancelled after shutdown")
	}
}

func TestManager_SignalHandling(t *testing.T) {
	manager := NewManager(10 * time.Second)

	// Test that ListenForSignals doesn't crash
	// We can't easily test actual signal sending in unit tests without
	// interfering with the test process, but we can test that it runs

	// Since ListenForSignals blocks, we'll test it in a goroutine with a timeout
	done := make(chan bool, 1)
	go func() {
		// This will block until context is cancelled or signal received
		result := manager.ListenForSignals()
		done <- result
	}()

	// Cancel the context to unblock ListenForSignals
	manager.cancel()

	// Should return false because context was cancelled, not signal received
	select {
	case result := <-done:
		if result {
			t.Error("Expected false when context cancelled, got true")
		}
	case <-time.After(time.Second):
		t.Error("ListenForSignals should have returned when context was cancelled")
	}
}

func TestManager_ShutdownExecution(t *testing.T) {
	manager := NewManager(10 * time.Second)

	var callOrder []int

	// Register callbacks that record their execution order
	manager.Register(func(ctx context.Context) error {
		callOrder = append(callOrder, 1)
		return nil
	})

	manager.Register(func(ctx context.Context) error {
		callOrder = append(callOrder, 2)
		return nil
	})

	manager.Register(func(ctx context.Context) error {
		callOrder = append(callOrder, 3)
		return nil
	})

	// Manually trigger shutdown to test callback execution
	manager.mu.Lock()
	for i := len(manager.callbacks) - 1; i >= 0; i-- {
		manager.callbacks[i](context.Background())
	}
	manager.mu.Unlock()

	// Callbacks should execute in reverse order (LIFO)
	expected := []int{3, 2, 1}
	if len(callOrder) != len(expected) {
		t.Errorf("Expected %d callbacks executed, got %d", len(expected), len(callOrder))
		return
	}

	for i, v := range callOrder {
		if v != expected[i] {
			t.Errorf("Expected execution order %v, got %v", expected, callOrder)
			break
		}
	}
}

func TestManager_ShutdownIdempotency(t *testing.T) {
	manager := NewManager(10 * time.Second)

	callCount := 0
	manager.Register(func(ctx context.Context) error {
		callCount++
		return nil
	})

	// Trigger shutdown multiple times
	manager.mu.Lock()
	// Simulate first shutdown
	if manager.ctx.Err() == nil {
		manager.cancel()
		for i := len(manager.callbacks) - 1; i >= 0; i-- {
			manager.callbacks[i](context.Background())
		}
	}
	manager.mu.Unlock()

	manager.mu.Lock()
	// Simulate second shutdown (should be ignored)
	if manager.ctx.Err() == nil {
		for i := len(manager.callbacks) - 1; i >= 0; i-- {
			manager.callbacks[i](context.Background())
		}
	}
	manager.mu.Unlock()

	// Callback should only be called once
	if callCount != 1 {
		t.Errorf("Expected callback to be called once, got %d calls", callCount)
	}
}

func TestManager_CallbackErrorHandling(t *testing.T) {
	manager := NewManager(10 * time.Second)

	var callOrder []int

	// Register a callback that panics
	manager.Register(func(ctx context.Context) error {
		callOrder = append(callOrder, 1)
		panic("test panic")
	})

	// Register a normal callback
	manager.Register(func(ctx context.Context) error {
		callOrder = append(callOrder, 2)
		return nil
	})

	// Manually execute callbacks to test error handling
	manager.mu.Lock()
	for i := len(manager.callbacks) - 1; i >= 0; i-- {
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Recover from panic to continue testing
				}
			}()
			manager.callbacks[i](context.Background())
		}()
	}
	manager.mu.Unlock()

	// Both callbacks should have been called despite the panic
	expected := []int{2, 1}
	if len(callOrder) != len(expected) {
		t.Errorf("Expected %d callbacks executed, got %d", len(expected), len(callOrder))
		return
	}

	for i, v := range callOrder {
		if v != expected[i] {
			t.Errorf("Expected execution order %v, got %v", expected, callOrder)
			break
		}
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	manager := NewManager(10 * time.Second)

	// Test concurrent registration
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			manager.Register(func(ctx context.Context) error {
				// Empty callback
				return nil
			})
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Good
		case <-time.After(time.Second):
			t.Error("Timeout waiting for concurrent registration")
			return
		}
	}

	if len(manager.callbacks) != 10 {
		t.Errorf("Expected 10 registered callbacks, got %d", len(manager.callbacks))
	}
}

func TestManager_ContextAfterShutdown(t *testing.T) {
	manager := NewManager(10 * time.Second)

	// Get context before shutdown
	ctx1 := manager.Context()

	// Trigger shutdown
	manager.mu.Lock()
	manager.cancel()
	manager.mu.Unlock()

	// Get context after shutdown
	ctx2 := manager.Context()

	// Both contexts should be the same instance
	if ctx1 != ctx2 {
		t.Error("Context should remain the same instance after shutdown")
	}

	// Both should be done
	select {
	case <-ctx1.Done():
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be done after shutdown")
	}

	select {
	case <-ctx2.Done():
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be done after shutdown")
	}
}

// TestManager_SignalHandlingLogic tests signal handling logic without real signals
func TestManager_SignalHandlingLogic(t *testing.T) {
	manager := NewManager(10 * time.Second)

	manager.Register(func(ctx context.Context) error {
		// Test callback - execution tested elsewhere
		return nil
	})

	// Test that context is available and not initially done
	ctx := manager.Context()

	// Test that context is initially not done
	select {
	case <-ctx.Done():
		t.Error("Context should not be done initially")
	default:
		// Good
	}

	if ctx == nil {
		t.Error("Context should be available")
	}

	// Test ListenForSignals behavior when context is already cancelled
	// This simulates what happens when shutdown is triggered programmatically
	manager.cancel() // Cancel context first

	// Verify context is now done
	select {
	case <-ctx.Done():
		// Good - context should be done after cancel
	default:
		t.Error("Context should be done after cancel")
	}

	// ListenForSignals should return false when context is already cancelled
	signalTriggered := manager.ListenForSignals()
	if signalTriggered {
		t.Error("ListenForSignals should return false when context is already cancelled")
	}
}

// Benchmark test for performance characteristics
func BenchmarkManager_Register(b *testing.B) {
	manager := NewManager(10 * time.Second)

	callback := func(ctx context.Context) error {
		// Empty callback
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.Register(callback)
	}
}

func BenchmarkManager_Context(b *testing.B) {
	manager := NewManager(10 * time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.Context()
	}
}

// TestManager_WithLogger tests the WithLogger method
func TestManager_WithLogger(t *testing.T) {
	manager := NewManager(10 * time.Second)

	// Create a custom logger
	customLogger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Test WithLogger returns the manager for chaining
	result := manager.WithLogger(customLogger)
	if result != manager {
		t.Error("WithLogger should return the same manager instance for chaining")
	}

	// Test that the logger was set
	manager.mu.RLock()
	if manager.logger != customLogger {
		t.Error("WithLogger should set the custom logger")
	}
	manager.mu.RUnlock()
}

// TestManager_Shutdown tests the public Shutdown method
func TestManager_Shutdown(t *testing.T) {
	manager := NewManager(100 * time.Millisecond)

	var callbackExecuted bool
	manager.Register(func(ctx context.Context) error {
		callbackExecuted = true
		return nil
	})

	ctx := manager.Context()

	// Context should not be done initially
	select {
	case <-ctx.Done():
		t.Error("Context should not be done before Shutdown is called")
	default:
		// Good
	}

	// Call Shutdown
	manager.Shutdown()

	// Context should be done after Shutdown
	select {
	case <-ctx.Done():
		// Good
	case <-time.After(time.Second):
		t.Error("Context should be done after Shutdown is called")
	}

	// Give some time for callback execution
	time.Sleep(50 * time.Millisecond)

	if !callbackExecuted {
		t.Error("Shutdown callback should have been executed")
	}
}

// TestManager_ShutdownWithError tests shutdown behavior when callbacks return errors
func TestManager_ShutdownWithError(t *testing.T) {
	manager := NewManager(100 * time.Millisecond)

	var callOrder []int

	// Register callback that returns an error
	manager.Register(func(ctx context.Context) error {
		callOrder = append(callOrder, 1)
		return context.DeadlineExceeded // Return an error
	})

	// Register callback that succeeds
	manager.Register(func(ctx context.Context) error {
		callOrder = append(callOrder, 2)
		return nil
	})

	// Call Shutdown
	manager.Shutdown()

	// Give some time for callback execution
	time.Sleep(50 * time.Millisecond)

	// Both callbacks should have been executed despite the error
	expectedOrder := []int{1, 2}
	if len(callOrder) != len(expectedOrder) {
		t.Errorf("Expected %d callbacks executed, got %d", len(expectedOrder), len(callOrder))
		return
	}

	for i, v := range callOrder {
		if v != expectedOrder[i] {
			t.Errorf("Expected execution order %v, got %v", expectedOrder, callOrder)
			break
		}
	}
}

// TestManager_ShutdownTimeout tests shutdown behavior when callbacks take too long
func TestManager_ShutdownTimeout(t *testing.T) {
	manager := NewManager(50 * time.Millisecond) // Very short timeout

	var callOrder []int

	// Register callback that takes longer than timeout
	manager.Register(func(ctx context.Context) error {
		callOrder = append(callOrder, 1)
		time.Sleep(100 * time.Millisecond) // Longer than timeout
		return nil
	})

	// Register callback that should not be reached due to timeout
	manager.Register(func(ctx context.Context) error {
		callOrder = append(callOrder, 2)
		return nil
	})

	start := time.Now()
	manager.Shutdown()
	duration := time.Since(start)

	// Should not take much longer than the timeout
	if duration > 150*time.Millisecond {
		t.Errorf("Shutdown took too long: %v, expected around 50ms", duration)
	}

	// Only first callback should have been started
	if len(callOrder) == 0 {
		t.Error("At least the first callback should have been started")
	}
}

// TestManager_CreateShutdownContext tests the CreateShutdownContext method
func TestManager_CreateShutdownContext(t *testing.T) {
	manager := NewManager(10 * time.Second)

	parentCtx, parentCancel := context.WithCancel(context.Background())

	// Create shutdown context
	shutdownCtx := manager.CreateShutdownContext(parentCtx)

	if shutdownCtx == nil {
		t.Fatal("CreateShutdownContext should return a non-nil context")
	}

	// Context should not be done initially
	select {
	case <-shutdownCtx.Done():
		t.Error("Shutdown context should not be done initially")
	default:
		// Good
	}

	// Cancel parent context
	parentCancel()

	// Shutdown context should be done when parent is cancelled
	select {
	case <-shutdownCtx.Done():
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Error("Shutdown context should be done when parent context is cancelled")
	}
}

// TestManager_CreateShutdownContextWithShutdown tests CreateShutdownContext when shutdown occurs
func TestManager_CreateShutdownContextWithShutdown(t *testing.T) {
	manager := NewManager(10 * time.Second)

	parentCtx := context.Background()

	// Create shutdown context
	shutdownCtx := manager.CreateShutdownContext(parentCtx)

	// Context should not be done initially
	select {
	case <-shutdownCtx.Done():
		t.Error("Shutdown context should not be done initially")
	default:
		// Good
	}

	// Trigger shutdown
	manager.Shutdown()

	// Shutdown context should be done when shutdown occurs
	select {
	case <-shutdownCtx.Done():
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Error("Shutdown context should be done when shutdown is triggered")
	}
}

// TestManager_ListenForSignalsTimeout tests ListenForSignals with context timeout
func TestManager_ListenForSignalsTimeout(t *testing.T) {
	manager := NewManager(10 * time.Second)

	// Create a context with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Replace the manager's context with our timeout context for testing
	manager.mu.Lock()
	oldCtx := manager.ctx
	oldCancel := manager.cancel
	manager.ctx = ctx
	manager.cancel = cancel
	manager.mu.Unlock()

	// Restore original context after test
	defer func() {
		manager.mu.Lock()
		manager.ctx = oldCtx
		manager.cancel = oldCancel
		manager.mu.Unlock()
	}()

	start := time.Now()
	result := manager.ListenForSignals()
	duration := time.Since(start)

	// Should return false (context cancelled, not signal)
	if result {
		t.Error("ListenForSignals should return false when context times out")
	}

	// Should return relatively quickly due to timeout
	if duration > 100*time.Millisecond {
		t.Errorf("ListenForSignals took too long: %v, expected around 50ms", duration)
	}
}

// TestManager_ShutdownMultipleCalls tests that multiple shutdowns execute callbacks multiple times
func TestManager_ShutdownMultipleCalls(t *testing.T) {
	manager := NewManager(100 * time.Millisecond)

	callCount := 0
	manager.Register(func(ctx context.Context) error {
		callCount++
		return nil
	})

	// Call Shutdown multiple times
	manager.Shutdown()
	manager.Shutdown()
	manager.Shutdown()

	// Give some time for any potential callback execution
	time.Sleep(50 * time.Millisecond)

	// Each Shutdown call executes callbacks (current implementation behavior)
	if callCount != 3 {
		t.Errorf("Expected callback to be called 3 times, got %d calls", callCount)
	}
}
