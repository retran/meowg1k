// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

var (
	errTest        = errors.New("test error")
	errActivity    = errors.New("activity error")
	errFailed      = errors.New("error")
	errRetry       = errors.New("retry error")
	errFirstRetry  = errors.New("first retry")
	errThirdRetry  = errors.New("third retry")
	errTestFailure = errors.New("test failure")
	errOpFailed    = errors.New("operation failed")
	errNilHandler  = errors.New("error")
	errNilRetry    = errors.New("retry")
)

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()
	if policy.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts 3, got %d", policy.MaxAttempts)
	}
	if policy.InitialDelay != 100*time.Millisecond {
		t.Errorf("expected InitialDelay 100ms, got %v", policy.InitialDelay)
	}
	if policy.MaxDelay != 5*time.Second {
		t.Errorf("expected MaxDelay 5s, got %v", policy.MaxDelay)
	}
	if policy.Multiplier != 2.0 {
		t.Errorf("expected Multiplier 2.0, got %v", policy.Multiplier)
	}
}

func TestNoRetryPolicy(t *testing.T) {
	policy := NoRetryPolicy()
	if policy.MaxAttempts != 1 {
		t.Errorf("expected MaxAttempts 1, got %d", policy.MaxAttempts)
	}
	if policy.InitialDelay != 0 {
		t.Errorf("expected InitialDelay 0, got %v", policy.InitialDelay)
	}
	if policy.MaxDelay != 0 {
		t.Errorf("expected MaxDelay 0, got %v", policy.MaxDelay)
	}
	if policy.Multiplier != 1.0 {
		t.Errorf("expected Multiplier 1.0, got %v", policy.Multiplier)
	}
}

func TestNewExecutor(t *testing.T) {
	exec := NewExecutor(0)
	if exec.RetryPolicy == nil {
		t.Error("expected RetryPolicy to be set")
	}
	if exec.FeedbackHandler == nil {
		t.Error("expected FeedbackHandler to be set")
	}
}

func TestWithRetryPolicy(t *testing.T) {
	exec := NewExecutor(0)
	policy := NoRetryPolicy()
	result := exec.WithRetryPolicy(&policy)
	if result != exec {
		t.Error("expected WithRetryPolicy to return the executor")
	}
	if exec.RetryPolicy.MaxAttempts != 1 {
		t.Error("expected RetryPolicy to be updated")
	}
}

func TestWithFeedbackHandler(t *testing.T) {
	exec := NewExecutor(0)
	handler := func(f *Feedback) {}
	result := exec.WithFeedbackHandler(handler)
	if result != exec {
		t.Error("expected WithFeedbackHandler to return the executor")
	}
	if exec.FeedbackHandler == nil {
		t.Error("expected FeedbackHandler to be updated")
	}
}

func TestRunFlow(t *testing.T) {
	exec := NewExecutor(0)
	ctx := context.Background()

	flow := func(ctx context.Context, activityCtx *Context) error {
		return nil
	}

	err := exec.ExecuteFlow(ctx, "test", flow)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestRunFlowWithError(t *testing.T) {
	exec := NewExecutor(0)
	ctx := context.Background()

	flow := func(ctx context.Context, activityCtx *Context) error {
		return errTest
	}

	err := exec.ExecuteFlow(ctx, "test", flow)
	if err == nil {
		t.Error("expected error")
	}
	if !strings.Contains(err.Error(), "test error") {
		t.Errorf("expected error to contain 'test error', got %v", err)
	}
}

func TestRunActivity(t *testing.T) {
	exec := NewExecutor(0)
	ctx := context.Background()
	parentCtx := NewContext("parent", NoOpFeedbackHandler, exec)

	activity := func(ctx context.Context, activityCtx *Context, input string) (string, error) {
		return "result", nil
	}

	fut := ExecuteActivity(exec, ctx, parentCtx, "test", activity, "input")
	result, err := fut.Get(ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != "result" {
		t.Errorf("expected 'result', got %v", result)
	}
}

func TestRunActivityWithError(t *testing.T) {
	exec := NewExecutor(0)
	ctx := context.Background()
	parentCtx := NewContext("parent", NoOpFeedbackHandler, exec)

	activity := func(ctx context.Context, activityCtx *Context, input string) (string, error) {
		return "", errActivity
	}

	fut := ExecuteActivity(exec, ctx, parentCtx, "test", activity, "input")
	_, err := fut.Get(ctx)
	if err == nil {
		t.Error("expected error")
	}
}

func TestExecutorContext(t *testing.T) {
	exec := NewExecutor(0)
	ctx := NewContext("test", NoOpFeedbackHandler, exec)

	if ctx.GetExecutor() != exec {
		t.Error("expected GetExecutor to return the executor")
	}

	// Test feedback methods don't panic
	ctx.SendRunning("started")
	ctx.SendCompleted("completed")
	ctx.SendFailed(errFailed, "failed")
	ctx.SendRetry(1, errRetry)
}

func TestNoOpFeedbackHandler(t *testing.T) {
	handler := NoOpFeedbackHandler

	// Should not panic when called
	feedback := &Feedback{
		ActivityName: "test-activity",
		Status:       StatusRunning,
		Message:      "test message",
		Progress:     0.5,
		Timestamp:    time.Now(),
	}

	handler(feedback) // Should do nothing and not panic
}

func TestFeedbackString(t *testing.T) {
	tests := []struct {
		name     string
		feedback *Feedback
		expected string
	}{
		{
			name: "error with progress",
			feedback: &Feedback{
				ActivityName: "test-activity",
				Status:       StatusRunning,
				Message:      "running with error",
				Progress:     0.5,
				Error:        errTest,
				Timestamp:    time.Now(),
			},
			expected: "[test-activity] running: running with error (50.0%) (test error)",
		},
		{
			name: "error without progress",
			feedback: &Feedback{
				ActivityName: "test-activity",
				Status:       StatusFailed,
				Message:      "failed",
				Progress:     0.0,
				Error:        errTest,
				Timestamp:    time.Now(),
			},
			expected: "[test-activity] failed: failed (test error)",
		},
		{
			name: "no error with progress",
			feedback: &Feedback{
				ActivityName: "test-activity",
				Status:       StatusRunning,
				Message:      "running",
				Progress:     0.75,
				Error:        nil,
				Timestamp:    time.Now(),
			},
			expected: "[test-activity] running: running (75.0%)",
		},
		{
			name: "no error no progress",
			feedback: &Feedback{
				ActivityName: "test-activity",
				Status:       StatusCompleted,
				Message:      "completed",
				Progress:     0.0,
				Error:        nil,
				Timestamp:    time.Now(),
			},
			expected: "[test-activity] completed: completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.feedback.String()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestContextName(t *testing.T) {
	exec := NewExecutor(0)
	ctx := NewContext("test-activity", NoOpFeedbackHandler, exec)

	if ctx.Name() != "test-activity" {
		t.Errorf("Expected name 'test-activity', got '%s'", ctx.Name())
	}
}

func TestExecutorContextSendFeedbackEdgeCases(t *testing.T) {
	exec := NewExecutor(0)
	ctx := NewContext("test-activity", NoOpFeedbackHandler, exec)

	// Test sending feedback with nil values (should not panic)
	ctx.SendRunning("")
	ctx.SendCompleted("")
	ctx.SendFailed(nil, "")
	ctx.SendRetry(0, nil)
}

func TestExecutorContextSendFeedbackMultipleRetries(t *testing.T) {
	feedbackCalls := []*Feedback{}
	handler := func(feedback *Feedback) {
		feedbackCalls = append(feedbackCalls, feedback)
	}

	exec := NewExecutor(0).WithFeedbackHandler(handler)
	ctx := NewContext("test-activity", handler, exec)

	// Simulate a flow with retries
	ctx.SendRetry(1, errFirstRetry)
	ctx.SendRetry(3, errThirdRetry)

	if len(feedbackCalls) != 2 {
		t.Errorf("Expected 2 retry feedback calls, got %d", len(feedbackCalls))
	}

	// Check first retry
	if feedbackCalls[0].Status != StatusRunning { // SendRetry uses StatusRunning
		t.Errorf("Expected StatusRunning, got %v", feedbackCalls[0].Status)
	}
	if feedbackCalls[0].Error == nil {
		t.Error("Expected error in first retry feedback")
	}
	if feedbackCalls[0].Metadata["retry_attempt"] != 1 {
		t.Errorf("Expected retry attempt 1, got %v", feedbackCalls[0].Metadata["retry_attempt"])
	}

	// Check third retry
	if feedbackCalls[1].Metadata["retry_attempt"] != 3 {
		t.Errorf("Expected retry attempt 3, got %v", feedbackCalls[1].Metadata["retry_attempt"])
	}
}

func TestExecutorContextFailedWithDetails(t *testing.T) {
	feedbackCalls := []*Feedback{}
	handler := func(feedback *Feedback) {
		feedbackCalls = append(feedbackCalls, feedback)
	}

	ctx := NewContext("failed-activity", handler, nil)

	ctx.SendFailed(errTestFailure, "operation failed")

	if len(feedbackCalls) != 1 {
		t.Errorf("Expected 1 failure feedback call, got %d", len(feedbackCalls))
	}

	feedback := feedbackCalls[0]
	if feedback.Status != StatusFailed {
		t.Errorf("Expected StatusFailed, got %v", feedback.Status)
	}
	if !errors.Is(feedback.Error, errTestFailure) {
		t.Errorf("Expected test error, got %v", feedback.Error)
	}
	if feedback.Message != "operation failed" {
		t.Errorf("Expected 'operation failed', got %s", feedback.Message)
	}
}

func TestExecutorWithComplexActivity(t *testing.T) {
	feedbackCalls := []*Feedback{}
	handler := func(feedback *Feedback) {
		feedbackCalls = append(feedbackCalls, feedback)
	}

	executor := NewExecutor(0).WithFeedbackHandler(handler)

	// Define a complex activity that sends multiple feedback updates
	complexActivity := func(ctx context.Context, executorCtx *Context, input any) (any, error) {
		inputStr := input.(string)
		executorCtx.SendRunning("Starting complex operation")

		// Simulate some work
		for i := 1; i <= 3; i++ {
			time.Sleep(10 * time.Millisecond)
		}

		executorCtx.SendCompleted("Complex operation completed")
		return "result-" + inputStr, nil
	}

	// Run the complex activity
	ctx := context.Background()
	parentCtx := NewContext("parent", handler, executor)

	future := ExecuteActivity(executor, ctx, parentCtx, "complex", complexActivity, "test-input")

	result, err := future.Get(context.Background())
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result != "result-test-input" {
		t.Errorf("Expected 'result-test-input', got %v", result)
	}

	// Check that we received feedback updates (started and completed)
	if len(feedbackCalls) < 2 { // At least started and completed
		t.Errorf("Expected at least 2 feedback calls, got %d", len(feedbackCalls))
	}
}

func TestExecutorWithActivityThatFails(t *testing.T) {
	feedbackCalls := []*Feedback{}
	handler := func(feedback *Feedback) {
		feedbackCalls = append(feedbackCalls, feedback)
	}

	executor := NewExecutor(0).WithFeedbackHandler(handler)

	// Define an activity that fails
	failingActivity := func(ctx context.Context, executorCtx *Context, input any) (any, error) {
		executorCtx.SendRunning("Starting operation that will fail")

		time.Sleep(10 * time.Millisecond)

		executorCtx.SendFailed(errOpFailed, "Operation failed as expected")

		return nil, errOpFailed
	}

	ctx := context.Background()
	parentCtx := NewContext("parent", handler, executor)

	future := ExecuteActivity(executor, ctx, parentCtx, "failing", failingActivity, 42)

	_, err := future.Get(context.Background())
	if err == nil {
		t.Fatal("Expected error from failing activity")
	}

	// Check that failure feedback was sent
	hasFailure := false
	for _, feedback := range feedbackCalls {
		if feedback.Status == StatusFailed {
			hasFailure = true
			break
		}
	}
	if !hasFailure {
		t.Error("Expected failure feedback to be sent")
	}
}

func TestExecutorFlowWithSubactivities(t *testing.T) {
	feedbackCalls := []*Feedback{}
	handler := func(feedback *Feedback) {
		feedbackCalls = append(feedbackCalls, feedback)
	}

	executor := NewExecutor(0).WithFeedbackHandler(handler)

	// Define a simple activity
	simpleActivity := func(ctx context.Context, executorCtx *Context, input string) (string, error) {
		executorCtx.SendRunning("Processing " + input)
		time.Sleep(10 * time.Millisecond)
		result := "processed-" + input
		executorCtx.SendCompleted("Finished processing")
		return result, nil
	}

	// Define a flow that runs multiple activities
	testFlow := func(ctx context.Context, executorCtx *Context) error {
		executorCtx.SendRunning("Starting flow")

		// Run first activity
		future1 := ExecuteActivity(executorCtx.GetExecutor(), ctx, executorCtx, "activity1", simpleActivity, "input1")
		result1, err := future1.Get(ctx)
		if err != nil {
			return err
		}

		// Run second activity
		future2 := ExecuteActivity(executorCtx.GetExecutor(), ctx, executorCtx, "activity2", simpleActivity, "input2")
		result2, err := future2.Get(ctx)
		if err != nil {
			return err
		}

		executorCtx.SendCompleted(fmt.Sprintf("Flow completed with results: %s, %s", result1, result2))
		return nil
	}

	// Execute the flow
	ctx := context.Background()
	err := executor.ExecuteFlow(ctx, "test-flow", testFlow)
	if err != nil {
		t.Fatalf("Expected no error from flow, got %v", err)
	}

	// Should have received feedback from both the flow and the activities
	if len(feedbackCalls) == 0 {
		t.Error("Expected feedback calls from flow execution")
	}
}

func TestExecutorContextWithNilHandler(t *testing.T) {
	// Test that executor context works with nil handler (shouldn't panic)
	ctx := NewContext("test-activity", nil, nil)

	// These should not panic
	ctx.SendRunning("started")
	ctx.SendCompleted("completed")
	ctx.SendFailed(errNilHandler, "failed")
	ctx.SendRetry(1, errNilRetry)
}

func TestExecutorWithTimeout(t *testing.T) {
	executor := NewExecutor(0) // Uses NoOpFeedbackHandler by default

	// Define a slow activity
	slowActivity := func(ctx context.Context, executorCtx *Context, input string) (string, error) {
		select {
		case <-time.After(1 * time.Second): // This will timeout
			return "slow-result", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	// Run with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	parentCtx := NewContext("parent", NoOpFeedbackHandler, executor)
	future := ExecuteActivity(executor, ctx, parentCtx, "slow", slowActivity, "test")

	_, err := future.Get(context.Background())
	if err == nil {
		t.Fatal("Expected timeout error")
	}
}
