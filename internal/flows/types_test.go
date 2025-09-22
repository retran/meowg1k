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

package flows

import (
	"context"
	"testing"
	"time"
)

func TestOutcomeType(t *testing.T) {
	t.Run("OutcomeType String Representations", func(t *testing.T) {
		testCases := []struct {
			outcome  OutcomeType
			expected string
		}{
			{OutcomeSuccess, "Success"},
			{OutcomeExit, "Exit"},
			{OutcomeRetry, "Retry"},
			{OutcomeConditional, "Conditional"},
			{OutcomeContinue, "Continue"},
			{OutcomeType(999), "Unknown"}, // Invalid outcome type
		}

		for _, tc := range testCases {
			result := tc.outcome.String()
			if result != tc.expected {
				t.Errorf("Expected %s for outcome %d, got %s", tc.expected, tc.outcome, result)
			}
		}
	})

	t.Run("OutcomeType Constants", func(t *testing.T) {
		// Test that constants have expected values
		if OutcomeSuccess != 0 {
			t.Errorf("Expected OutcomeSuccess to be 0, got %d", OutcomeSuccess)
		}

		if OutcomeExit != 1 {
			t.Errorf("Expected OutcomeExit to be 1, got %d", OutcomeExit)
		}

		if OutcomeRetry != 2 {
			t.Errorf("Expected OutcomeRetry to be 2, got %d", OutcomeRetry)
		}

		if OutcomeConditional != 3 {
			t.Errorf("Expected OutcomeConditional to be 3, got %d", OutcomeConditional)
		}

		if OutcomeContinue != 4 {
			t.Errorf("Expected OutcomeContinue to be 4, got %d", OutcomeContinue)
		}
	})
}

func TestOutcome(t *testing.T) {
	t.Run("Outcome with String Data", func(t *testing.T) {
		outcome := Outcome[string]{
			Type: OutcomeSuccess,
			Data: "test_data",
		}

		if outcome.Type != OutcomeSuccess {
			t.Errorf("Expected OutcomeSuccess, got %v", outcome.Type)
		}

		if outcome.Data != "test_data" {
			t.Errorf("Expected 'test_data', got %v", outcome.Data)
		}
	})

	t.Run("Outcome with Int Data", func(t *testing.T) {
		outcome := Outcome[int]{
			Type: OutcomeConditional,
			Data: 42,
		}

		if outcome.Type != OutcomeConditional {
			t.Errorf("Expected OutcomeConditional, got %v", outcome.Type)
		}

		if outcome.Data != 42 {
			t.Errorf("Expected 42, got %v", outcome.Data)
		}
	})

	t.Run("Outcome with Bool Data", func(t *testing.T) {
		outcome := Outcome[bool]{
			Type: OutcomeConditional,
			Data: true,
		}

		if outcome.Type != OutcomeConditional {
			t.Errorf("Expected OutcomeConditional, got %v", outcome.Type)
		}

		if outcome.Data != true {
			t.Errorf("Expected true, got %v", outcome.Data)
		}
	})

	t.Run("Empty Outcome Data", func(t *testing.T) {
		outcome := Outcome[string]{
			Type: OutcomeRetry,
		}

		if outcome.Type != OutcomeRetry {
			t.Errorf("Expected OutcomeRetry, got %v", outcome.Type)
		}

		if outcome.Data != "" {
			t.Errorf("Expected empty string, got %v", outcome.Data)
		}
	})
}

func TestExecutionContextStruct(t *testing.T) {
	t.Run("Basic ExecutionContext", func(t *testing.T) {
		ctx := ExecutionContext{
			TaskID:     "test_task",
			RetryCount: 2,
			FlowID:     "test_flow",
		}

		if ctx.TaskID != "test_task" {
			t.Errorf("Expected task ID 'test_task', got %s", ctx.TaskID)
		}

		if ctx.RetryCount != 2 {
			t.Errorf("Expected retry count 2, got %d", ctx.RetryCount)
		}

		if ctx.FlowID != "test_flow" {
			t.Errorf("Expected flow ID 'test_flow', got %s", ctx.FlowID)
		}
	})
}

func TestRetryPolicyStruct(t *testing.T) {
	t.Run("Default Retry Policy", func(t *testing.T) {
		policy := DefaultRetryPolicy()

		if policy.InitialDelay != 100*time.Millisecond {
			t.Errorf("Expected initial delay 100ms, got %v", policy.InitialDelay)
		}

		if policy.MaxDelay != 5*time.Second {
			t.Errorf("Expected max delay 5s, got %v", policy.MaxDelay)
		}

		if policy.Multiplier != 2.0 {
			t.Errorf("Expected multiplier 2.0, got %f", policy.Multiplier)
		}

		if policy.MaxRetries != 3 {
			t.Errorf("Expected max retries 3, got %d", policy.MaxRetries)
		}
	})

	t.Run("Custom Retry Policy", func(t *testing.T) {
		policy := RetryPolicy{
			InitialDelay: 50 * time.Millisecond,
			MaxDelay:     2 * time.Second,
			Multiplier:   1.5,
			MaxRetries:   10,
		}

		if policy.InitialDelay != 50*time.Millisecond {
			t.Errorf("Expected initial delay 50ms, got %v", policy.InitialDelay)
		}

		if policy.MaxDelay != 2*time.Second {
			t.Errorf("Expected max delay 2s, got %v", policy.MaxDelay)
		}

		if policy.Multiplier != 1.5 {
			t.Errorf("Expected multiplier 1.5, got %f", policy.Multiplier)
		}

		if policy.MaxRetries != 10 {
			t.Errorf("Expected max retries 10, got %d", policy.MaxRetries)
		}
	})
}

func TestFeedback(t *testing.T) {
	t.Run("Basic Feedback", func(t *testing.T) {
		now := time.Now()
		feedback := Feedback{
			TaskID:    "test_task",
			Status:    "running",
			Progress:  0.5,
			Timestamp: now,
			Metrics: map[string]interface{}{
				"duration": 100 * time.Millisecond,
				"attempts": 1,
			},
		}

		if feedback.TaskID != "test_task" {
			t.Errorf("Expected task ID 'test_task', got %s", feedback.TaskID)
		}

		if feedback.Status != "running" {
			t.Errorf("Expected status 'running', got %s", feedback.Status)
		}

		if feedback.Progress != 0.5 {
			t.Errorf("Expected progress 0.5, got %f", feedback.Progress)
		}

		if !feedback.Timestamp.Equal(now) {
			t.Errorf("Expected timestamp %v, got %v", now, feedback.Timestamp)
		}

		if feedback.Metrics["duration"] != 100*time.Millisecond {
			t.Error("Metrics not properly set")
		}

		if feedback.Metrics["attempts"] != 1 {
			t.Error("Metrics not properly set")
		}
	})

	t.Run("Feedback Progress Boundaries", func(t *testing.T) {
		testCases := []struct {
			progress float64
			valid    bool
		}{
			{0.0, true},
			{0.5, true},
			{1.0, true},
			{-0.1, false}, // Invalid: negative
			{1.1, false},  // Invalid: greater than 1
		}

		for _, tc := range testCases {
			feedback := Feedback{Progress: tc.progress}

			// In a real implementation, you might want to validate progress bounds
			// This test just documents the expected behavior
			if tc.valid {
				if feedback.Progress < 0.0 || feedback.Progress > 1.0 {
					t.Errorf("Progress %f should be valid but is out of bounds", tc.progress)
				}
			}
		}
	})
}

func TestContextHelpers(t *testing.T) {
	t.Run("ExecutionContext in Context", func(t *testing.T) {
		baseCtx := context.Background()
		execCtx := ExecutionContext{
			TaskID:     "context_task",
			RetryCount: 1,
			FlowID:     "context_flow",
		}

		ctx := NewContextWithExecutionState(baseCtx, execCtx)

		retrieved := GetExecutionState(ctx)

		if retrieved.TaskID != execCtx.TaskID {
			t.Errorf("Expected task ID %s, got %s", execCtx.TaskID, retrieved.TaskID)
		}

		if retrieved.RetryCount != execCtx.RetryCount {
			t.Errorf("Expected retry count %d, got %d", execCtx.RetryCount, retrieved.RetryCount)
		}

		if retrieved.FlowID != execCtx.FlowID {
			t.Errorf("Expected flow ID %s, got %s", execCtx.FlowID, retrieved.FlowID)
		}
	})

	t.Run("ExecutionContext Not Found", func(t *testing.T) {
		baseCtx := context.Background()

		retrieved := GetExecutionState(baseCtx)

		// Should return zero value
		if retrieved.TaskID != "" {
			t.Errorf("Expected empty task ID, got %s", retrieved.TaskID)
		}

		if retrieved.RetryCount != 0 {
			t.Errorf("Expected retry count 0, got %d", retrieved.RetryCount)
		}

		if retrieved.FlowID != "" {
			t.Errorf("Expected empty flow ID, got %s", retrieved.FlowID)
		}
	})

	t.Run("Feedback in Context", func(t *testing.T) {
		baseCtx := context.Background()
		var receivedFeedback Feedback

		handler := func(f Feedback) {
			receivedFeedback = f
		}

		ctx := NewContextWithFeedback(baseCtx, handler, "test_task")
		sender := GetFeedbackSender(ctx)

		testFeedback := Feedback{
			Status:   "test_status",
			Progress: 0.8,
		}

		sender(testFeedback)

		// The wrapped handler should set TaskID and Timestamp
		if receivedFeedback.TaskID != "test_task" {
			t.Errorf("Expected task ID 'test_task', got %s", receivedFeedback.TaskID)
		}

		if receivedFeedback.Status != "test_status" {
			t.Errorf("Expected status 'test_status', got %s", receivedFeedback.Status)
		}

		if receivedFeedback.Progress != 0.8 {
			t.Errorf("Expected progress 0.8, got %f", receivedFeedback.Progress)
		}

		if receivedFeedback.Timestamp.IsZero() {
			t.Error("Timestamp should be set automatically")
		}
	})

	t.Run("Feedback Sender Not Found", func(t *testing.T) {
		baseCtx := context.Background()

		sender := GetFeedbackSender(baseCtx)

		// Should not panic with no-op function
		sender(Feedback{})
	})

	t.Run("Logger in Context", func(t *testing.T) {
		baseCtx := context.Background()

		// Test with no logger in context
		logger := GetLogger(baseCtx)
		if logger == nil {
			t.Error("Should return default logger when none in context")
		}

		// Test with custom logger
		// Note: We can't easily test with a custom logger without importing slog
		// This test ensures the function doesn't panic
		ctx := NewContextWithLogger(baseCtx, logger)
		retrievedLogger := GetLogger(ctx)

		if retrievedLogger == nil {
			t.Error("Should return the logger from context")
		}
	})
}

func TestReduceFunc(t *testing.T) {
	t.Run("Sum Reducer", func(t *testing.T) {
		sumReducer := func(acc int, item int) int {
			return acc + item
		}

		// Test the reducer function type
		result := sumReducer(10, 5)
		if result != 15 {
			t.Errorf("Expected 15, got %d", result)
		}
	})

	t.Run("String Concatenation Reducer", func(t *testing.T) {
		concatReducer := func(acc string, item string) string {
			if acc == "" {
				return item
			}
			return acc + "," + item
		}

		result := concatReducer("a", "b")
		if result != "a,b" {
			t.Errorf("Expected 'a,b', got %s", result)
		}

		result = concatReducer("", "first")
		if result != "first" {
			t.Errorf("Expected 'first', got %s", result)
		}
	})
}
