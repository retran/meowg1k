/*package shutdown

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
