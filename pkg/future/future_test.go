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

package future

import (
	"context"
	"testing"
	"time"
)

func TestFuture_CompleteAndGet(t *testing.T) {
	future := NewFuture[int]()
	ctx := context.Background()

	// Complete the future in a goroutine
	go func() {
		time.Sleep(10 * time.Millisecond)
		future.Complete(42)
	}()

	// Get the result
	result, err := future.Get(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result != 42 {
		t.Fatalf("Expected result 42, got %d", result)
	}
}

func TestFuture_CompleteWithError(t *testing.T) {
	future := NewFuture[string]()
	ctx := context.Background()

	// Complete with error in a goroutine
	go func() {
		time.Sleep(10 * time.Millisecond)
		future.CompleteWithError(context.DeadlineExceeded)
	}()

	// Get the result
	result, err := future.Get(ctx)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if result != "" {
		t.Fatalf("Expected empty result, got %s", result)
	}
}

func TestFuture_TryGet(t *testing.T) {
	future := NewFuture[int]()

	// Try to get result before completion
	result, err, ready := future.TryGet()
	if ready {
		t.Fatal("Expected not ready")
	}
	if result != 0 {
		t.Fatalf("Expected zero result, got %d", result)
	}
	if err != nil {
		t.Fatalf("Expected nil error, got %v", err)
	}

	// Complete the future
	future.Complete(123)

	// Try to get result after completion
	result, err, ready = future.TryGet()
	if !ready {
		t.Fatal("Expected ready")
	}
	if result != 123 {
		t.Fatalf("Expected result 123, got %d", result)
	}
	if err != nil {
		t.Fatalf("Expected nil error, got %v", err)
	}
}

func TestFuture_IsDone(t *testing.T) {
	future := NewFuture[bool]()

	if future.IsDone() {
		t.Fatal("Expected not done")
	}

	future.Complete(true)

	if !future.IsDone() {
		t.Fatal("Expected done")
	}
}

func TestWaitAll(t *testing.T) {
	ctx := context.Background()

	future1 := NewFuture[int]()
	future2 := NewFuture[int]()
	future3 := NewFuture[int]()

	// Complete futures in goroutines with different delays
	go func() {
		time.Sleep(10 * time.Millisecond)
		future1.Complete(10)
	}()
	go func() {
		time.Sleep(20 * time.Millisecond)
		future2.Complete(20)
	}()
	go func() {
		time.Sleep(5 * time.Millisecond)
		future3.Complete(30)
	}()

	start := time.Now()
	results, errors := WaitAll(ctx, future1, future2, future3)
	duration := time.Since(start)

	// Check results
	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}
	if len(errors) != 3 {
		t.Fatalf("Expected 3 errors, got %d", len(errors))
	}

	if results[0] != 10 || errors[0] != nil {
		t.Fatalf("Future 1: expected (10, nil), got (%d, %v)", results[0], errors[0])
	}
	if results[1] != 20 || errors[1] != nil {
		t.Fatalf("Future 2: expected (20, nil), got (%d, %v)", results[1], errors[1])
	}
	if results[2] != 30 || errors[2] != nil {
		t.Fatalf("Future 3: expected (30, nil), got (%d, %v)", results[2], errors[2])
	}

	// Should complete in around 20ms (max delay), not 35ms (sum of delays)
	if duration > 40*time.Millisecond {
		t.Logf("Duration was %v, which seems too long for parallel execution", duration)
	}
}

func TestWaitAny(t *testing.T) {
	ctx := context.Background()

	future1 := NewFuture[int]()
	future2 := NewFuture[int]()
	future3 := NewFuture[int]()

	// Complete futures with different delays
	go func() {
		time.Sleep(30 * time.Millisecond)
		future1.Complete(100)
	}()
	go func() {
		time.Sleep(10 * time.Millisecond)
		future2.Complete(200) // This should complete first
	}()
	go func() {
		time.Sleep(20 * time.Millisecond)
		future3.Complete(300)
	}()

	start := time.Now()
	result, index, err := WaitAny(ctx, future1, future2, future3)
	duration := time.Since(start)

	// The fast future (index 1) should complete first
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if index != 1 {
		t.Fatalf("Expected fast future (index 1) to complete first, got index %d", index)
	}
	if result != 200 {
		t.Fatalf("Expected result 200, got %d", result)
	}

	// Should complete in around 10ms (fast task), not 30ms+ (slow task)
	if duration > 25*time.Millisecond {
		t.Fatalf("Duration was %v, which is too long for the fast task", duration)
	}
}

func TestWaitAllMap(t *testing.T) {
	ctx := context.Background()

	futures := map[string]*Future[int]{
		"double": NewFuture[int](),
		"triple": NewFuture[int](),
		"quad":   NewFuture[int](),
	}

	// Complete futures in goroutines
	go func() {
		time.Sleep(10 * time.Millisecond)
		futures["double"].Complete(20)
	}()
	go func() {
		time.Sleep(15 * time.Millisecond)
		futures["triple"].Complete(60)
	}()
	go func() {
		time.Sleep(5 * time.Millisecond)
		futures["quad"].Complete(80)
	}()

	start := time.Now()
	results, errors := WaitAllMap(ctx, futures)
	duration := time.Since(start)

	// Check results
	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}
	if len(errors) != 3 {
		t.Fatalf("Expected 3 errors, got %d", len(errors))
	}

	if results["double"] != 20 || errors["double"] != nil {
		t.Fatalf("Double: expected (20, nil), got (%d, %v)", results["double"], errors["double"])
	}
	if results["triple"] != 60 || errors["triple"] != nil {
		t.Fatalf("Triple: expected (60, nil), got (%d, %v)", results["triple"], errors["triple"])
	}
	if results["quad"] != 80 || errors["quad"] != nil {
		t.Fatalf("Quad: expected (80, nil), got (%d, %v)", results["quad"], errors["quad"])
	}

	// Should complete in around 15ms (max delay), not 30ms (sum of delays)
	if duration > 35*time.Millisecond {
		t.Logf("Duration was %v, which seems too long for parallel execution", duration)
	}
}

func TestWaitAnyEmpty(t *testing.T) {
	ctx := context.Background()

	result, index, err := WaitAny[int](ctx)

	if err == nil {
		t.Fatal("Expected error for empty futures list")
	}
	if index != -1 {
		t.Fatalf("Expected index -1, got %d", index)
	}
	if result != 0 {
		t.Fatalf("Expected zero result, got %d", result)
	}
}
