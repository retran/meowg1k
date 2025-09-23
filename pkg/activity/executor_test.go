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

package activity

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/retran/meowg1k/pkg/future"
)

func TestExecutor_Execute(t *testing.T) {
	executor := NewDefaultExecutor()

	// Simple activity that doubles the input
	doubleActivity := func(ctx context.Context, activityCtx *ActivityContext, input int) (int, error) {
		return input * 2, nil
	}

	result, err := Execute(executor, context.Background(), "double", doubleActivity, 5)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := 10
	if result != expected {
		t.Fatalf("Expected %d, got %d", expected, result)
	}
}

func TestExecutor_ExecuteWithRetry(t *testing.T) {
	executor := NewDefaultExecutor()

	// Activity that fails first time, succeeds second time
	attempt := 0
	flakyActivity := func(ctx context.Context, activityCtx *ActivityContext, input string) (string, error) {
		attempt++
		if attempt == 1 {
			return "", fmt.Errorf("first attempt fails")
		}
		return input + " processed", nil
	}

	retryPolicy := RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		Multiplier:   2.0,
		MaxDelay:     100 * time.Millisecond,
	}

	result, err := ExecuteWithCustomRetry(executor, context.Background(), "flaky", flakyActivity, "test", retryPolicy)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "test processed"
	if result != expected {
		t.Fatalf("Expected %s, got %s", expected, result)
	}

	if attempt != 2 {
		t.Fatalf("Expected 2 attempts, got %d", attempt)
	}
}

func TestActivityContext_RunSubActivity(t *testing.T) {
	executor := NewDefaultExecutor()

	// Parent activity that runs a sub-activity
	parentActivity := func(ctx context.Context, activityCtx *ActivityContext, input int) (int, error) {
		// Create sub-activity with interface{} signature
		subActivity := func(ctx context.Context, activityCtx *ActivityContext, input interface{}) (interface{}, error) {
			if num, ok := input.(int); ok {
				return num + 10, nil
			}
			return nil, fmt.Errorf("invalid input type")
		}

		// Run sub-activity using the new asynchronous RunSubActivity method
		future := activityCtx.executor.RunSubActivity(activityCtx, ctx, "sub-add", subActivity, input)
		result, err := future.Get(ctx)
		if err != nil {
			return 0, fmt.Errorf("sub-activity failed: %w", err)
		}

		// Type assert the result
		subResult, ok := result.(int)
		if !ok {
			return 0, fmt.Errorf("sub-activity returned unexpected type")
		}

		// Process sub-activity result
		return subResult * 2, nil
	}

	result, err := Execute(executor, context.Background(), "parent", parentActivity, 5)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Expected: (5 + 10) * 2 = 30
	expected := 30
	if result != expected {
		t.Fatalf("Expected %d, got %d", expected, result)
	}
}

func TestActivityContext_RunSubActivitiesManuallyParallel(t *testing.T) {
	executor := NewDefaultExecutor()

	// Parent activity that runs multiple sub-activities in parallel manually
	parentActivity := func(ctx context.Context, activityCtx *ActivityContext, input int) (int, error) {
		// Create sub-activities
		addActivity := func(ctx context.Context, activityCtx *ActivityContext, input interface{}) (interface{}, error) {
			if num, ok := input.(int); ok {
				return num + 10, nil
			}
			return nil, fmt.Errorf("invalid input type")
		}

		multiplyActivity := func(ctx context.Context, activityCtx *ActivityContext, input interface{}) (interface{}, error) {
			if num, ok := input.(int); ok {
				return num * 3, nil
			}
			return nil, fmt.Errorf("invalid input type")
		}

		// Run sub-activities in parallel using goroutines
		type result struct {
			value interface{}
			err   error
		}

		resultCh1 := make(chan result, 1)
		resultCh2 := make(chan result, 1)

		// Start first sub-activity
		go func() {
			future := activityCtx.executor.RunActivity(ctx, "add", addActivity, input)
			val, err := future.Get(ctx)
			resultCh1 <- result{val, err}
		}()

		// Start second sub-activity
		go func() {
			future := activityCtx.executor.RunActivity(ctx, "multiply", multiplyActivity, input)
			val, err := future.Get(ctx)
			resultCh2 <- result{val, err}
		}()

		// Wait for results
		res1 := <-resultCh1
		res2 := <-resultCh2

		if res1.err != nil {
			return 0, fmt.Errorf("add sub-activity failed: %w", res1.err)
		}
		if res2.err != nil {
			return 0, fmt.Errorf("multiply sub-activity failed: %w", res2.err)
		}

		// Process results
		addResult, ok1 := res1.value.(int)
		multiplyResult, ok2 := res2.value.(int)

		if !ok1 || !ok2 {
			return 0, fmt.Errorf("unexpected result types")
		}

		return addResult + multiplyResult, nil
	}

	result, err := Execute(executor, context.Background(), "parent", parentActivity, 5)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Expected: (5 + 10) + (5 * 3) = 15 + 15 = 30
	expected := 30
	if result != expected {
		t.Fatalf("Expected %d, got %d", expected, result)
	}
}

func TestExecutor_WithFeedbackHandler(t *testing.T) {
	var feedbacks []Feedback

	feedbackHandler := func(feedback Feedback) {
		feedbacks = append(feedbacks, feedback)
	}

	executor := NewDefaultExecutor().WithFeedbackHandler(feedbackHandler)

	simpleActivity := func(ctx context.Context, activityCtx *ActivityContext, input string) (string, error) {
		activityCtx.SendFeedback(StatusRunning, 0.5, "Halfway done")
		return input + " done", nil
	}

	_, err := Execute(executor, context.Background(), "simple", simpleActivity, "test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should have received at least 3 feedbacks: Started, Running (from activity), Completed
	if len(feedbacks) < 3 {
		t.Fatalf("Expected at least 3 feedbacks, got %d", len(feedbacks))
	}

	// Check that we got the expected statuses
	expectedStatuses := []Status{StatusStarted, StatusRunning, StatusCompleted}
	for i, expectedStatus := range expectedStatuses {
		if i >= len(feedbacks) {
			t.Fatalf("Expected feedback %d to have status %v, but not enough feedbacks received", i, expectedStatus)
		}
		if feedbacks[i].Status != expectedStatus {
			t.Fatalf("Expected feedback %d to have status %v, got %v", i, expectedStatus, feedbacks[i].Status)
		}
	}
}

func TestFuture_AsyncExecution(t *testing.T) {
	executor := NewDefaultExecutor()

	// Parent activity that demonstrates async execution with Future
	parentActivity := func(ctx context.Context, activityCtx *ActivityContext, input int) (int, error) {
		// Create two sub-activities
		addActivity := func(ctx context.Context, activityCtx *ActivityContext, input interface{}) (interface{}, error) {
			if num, ok := input.(int); ok {
				// Simulate some work
				time.Sleep(10 * time.Millisecond)
				return num + 10, nil
			}
			return nil, fmt.Errorf("invalid input type")
		}

		multiplyActivity := func(ctx context.Context, activityCtx *ActivityContext, input interface{}) (interface{}, error) {
			if num, ok := input.(int); ok {
				// Simulate some work
				time.Sleep(20 * time.Millisecond)
				return num * 3, nil
			}
			return nil, fmt.Errorf("invalid input type")
		}

		// Start both activities asynchronously
		future1 := activityCtx.executor.RunSubActivity(activityCtx, ctx, "add", addActivity, input)
		future2 := activityCtx.executor.RunSubActivity(activityCtx, ctx, "multiply", multiplyActivity, input)

		// Wait for both to complete
		result1, err1 := future1.Get(ctx)
		if err1 != nil {
			return 0, fmt.Errorf("add activity failed: %w", err1)
		}

		result2, err2 := future2.Get(ctx)
		if err2 != nil {
			return 0, fmt.Errorf("multiply activity failed: %w", err2)
		}

		// Type assert and combine results
		addResult, ok1 := result1.(int)
		multiplyResult, ok2 := result2.(int)

		if !ok1 || !ok2 {
			return 0, fmt.Errorf("unexpected result types")
		}

		return addResult + multiplyResult, nil
	}

	start := time.Now()
	result, err := Execute(executor, context.Background(), "parent", parentActivity, 5)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Expected: (5 + 10) + (5 * 3) = 15 + 15 = 30
	expected := 30
	if result != expected {
		t.Fatalf("Expected %d, got %d", expected, result)
	}

	// Should complete in around 20ms (parallel execution), not 30ms (sequential)
	if duration > 50*time.Millisecond {
		t.Logf("Duration was %v, which seems too long for parallel execution", duration)
	}
}

func TestWaitAll(t *testing.T) {
	executor := NewDefaultExecutor()
	ctx := context.Background()

	// Create multiple futures with different delays
	future1 := executor.RunActivity(ctx, "task1", func(ctx context.Context, activityCtx *ActivityContext, input interface{}) (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		return input.(int) * 2, nil
	}, 5)

	future2 := executor.RunActivity(ctx, "task2", func(ctx context.Context, activityCtx *ActivityContext, input interface{}) (interface{}, error) {
		time.Sleep(20 * time.Millisecond)
		return input.(int) * 3, nil
	}, 10)

	future3 := executor.RunActivity(ctx, "task3", func(ctx context.Context, activityCtx *ActivityContext, input interface{}) (interface{}, error) {
		time.Sleep(5 * time.Millisecond)
		return input.(int) * 4, nil
	}, 15)

	start := time.Now()
	results, errors := future.WaitAll(ctx, future1, future2, future3)
	duration := time.Since(start)

	// Check results
	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}
	if len(errors) != 3 {
		t.Fatalf("Expected 3 errors, got %d", len(errors))
	}

	if results[0].(int) != 10 || errors[0] != nil {
		t.Fatalf("Future 1: expected (10, nil), got (%v, %v)", results[0], errors[0])
	}
	if results[1].(int) != 30 || errors[1] != nil {
		t.Fatalf("Future 2: expected (30, nil), got (%v, %v)", results[1], errors[1])
	}
	if results[2].(int) != 60 || errors[2] != nil {
		t.Fatalf("Future 3: expected (60, nil), got (%v, %v)", results[2], errors[2])
	}

	// Should complete in around 20ms (max delay), not 35ms (sum of delays)
	if duration > 50*time.Millisecond {
		t.Logf("Duration was %v, which seems too long for parallel execution", duration)
	}
}

func TestWaitAny(t *testing.T) {
	executor := NewDefaultExecutor()
	ctx := context.Background()

	// Create futures with different delays
	future1 := executor.RunActivity(ctx, "slow", func(ctx context.Context, activityCtx *ActivityContext, input interface{}) (interface{}, error) {
		time.Sleep(50 * time.Millisecond)
		return input.(int) * 2, nil
	}, 5)

	future2 := executor.RunActivity(ctx, "fast", func(ctx context.Context, activityCtx *ActivityContext, input interface{}) (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		return input.(int) * 3, nil
	}, 10)

	future3 := executor.RunActivity(ctx, "medium", func(ctx context.Context, activityCtx *ActivityContext, input interface{}) (interface{}, error) {
		time.Sleep(30 * time.Millisecond)
		return input.(int) * 4, nil
	}, 15)

	start := time.Now()
	result, index, err := future.WaitAny(ctx, future1, future2, future3)
	duration := time.Since(start)

	// The fast future (index 1) should complete first
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if index != 1 {
		t.Fatalf("Expected fast future (index 1) to complete first, got index %d", index)
	}
	if result.(int) != 30 {
		t.Fatalf("Expected result 30, got %v", result)
	}

	// Should complete in around 10ms (fast task), not 50ms+ (slow task)
	if duration > 40*time.Millisecond {
		t.Fatalf("Duration was %v, which is too long for the fast task", duration)
	}
}

func TestWaitAllMap(t *testing.T) {
	executor := NewDefaultExecutor()
	ctx := context.Background()

	// Create typed activities first
	doubleActivity := func(ctx context.Context, activityCtx *ActivityContext, input interface{}) (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		return input.(int) * 2, nil
	}
	tripleActivity := func(ctx context.Context, activityCtx *ActivityContext, input interface{}) (interface{}, error) {
		time.Sleep(15 * time.Millisecond)
		return input.(int) * 3, nil
	}
	quadrupleActivity := func(ctx context.Context, activityCtx *ActivityContext, input interface{}) (interface{}, error) {
		time.Sleep(5 * time.Millisecond)
		return input.(int) * 4, nil
	}

	futures := map[string]*future.Future[any]{
		"double":    executor.RunActivity(ctx, "double", doubleActivity, 5),
		"triple":    executor.RunActivity(ctx, "triple", tripleActivity, 7),
		"quadruple": executor.RunActivity(ctx, "quadruple", quadrupleActivity, 3),
	}

	start := time.Now()
	results, errors := future.WaitAllMap(ctx, futures)
	duration := time.Since(start)

	// Check results - convert from any to int
	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}
	if len(errors) != 3 {
		t.Fatalf("Expected 3 errors, got %d", len(errors))
	}

	if results["double"].(int) != 10 || errors["double"] != nil {
		t.Fatalf("Double: expected (10, nil), got (%v, %v)", results["double"], errors["double"])
	}
	if results["triple"].(int) != 21 || errors["triple"] != nil {
		t.Fatalf("Triple: expected (21, nil), got (%v, %v)", results["triple"], errors["triple"])
	}
	if results["quadruple"].(int) != 12 || errors["quadruple"] != nil {
		t.Fatalf("Quadruple: expected (12, nil), got (%v, %v)", results["quadruple"], errors["quadruple"])
	}

	// Should complete in around 15ms (max delay), not 30ms (sum of delays)
	if duration > 40*time.Millisecond {
		t.Logf("Duration was %v, which seems too long for parallel execution", duration)
	}
}

func TestWaitAnyEmpty(t *testing.T) {
	ctx := context.Background()

	result, index, err := future.WaitAny[int](ctx)

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
