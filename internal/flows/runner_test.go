package flows

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// Helper types for testing
type ContextCapturingTask struct {
	CapturedContext *ExecutionContext
}

func (ct *ContextCapturingTask) Execute(ctx context.Context, input interface{}) (string, Outcome[any], error) {
	if ct.CapturedContext != nil {
		*ct.CapturedContext = GetExecutionState(ctx)
	}
	return "result", Outcome[any]{Type: OutcomeSuccess}, nil
}

func TestFlowExecution(t *testing.T) {
	t.Run("Simple Linear Execution", func(t *testing.T) {
		flow := NewFlow()

		task1 := &TestTask{Name: "task1", Result: "result1"}
		task2 := &TestTask{Name: "task2", Result: "result2"}
		task3 := &TestTask{Name: "task3", Result: "final"}

		AddTask(flow, "task1", task1).LinkToID("task2")
		AddTask(flow, "task2", task2).LinkToID("task3")
		AddTask(flow, "task3", task3)

		flow.SetStart("task1")

		ctx := context.Background()
		result, err := flow.Run(ctx, "input")

		if err != nil {
			t.Fatalf("Flow execution failed: %v", err)
		}

		if result != "final" {
			t.Errorf("Expected 'final', got %v", result)
		}
	})

	t.Run("Empty Flow", func(t *testing.T) {
		flow := NewFlow()

		task := &TestTask{Name: "only", Result: "result"}
		AddTask(flow, "only", task)
		flow.SetStart("only")

		ctx := context.Background()
		result, err := flow.Run(ctx, "input")

		if err != nil {
			t.Fatalf("Single task flow failed: %v", err)
		}

		if result != "result" {
			t.Errorf("Expected 'result', got %v", result)
		}
	})

	t.Run("Execution with Context Cancellation", func(t *testing.T) {
		flow := NewFlow()

		slowTask := &TestTask{Name: "slow", Result: "result", Delay: 200 * time.Millisecond}
		AddTask(flow, "slow", slowTask)
		flow.SetStart("slow")

		ctx, cancel := context.WithCancel(context.Background())

		// Cancel after 50ms
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		_, err := flow.Run(ctx, "input")

		if err == nil {
			t.Error("Expected cancellation error")
		}

		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})

	t.Run("Execution with Timeout", func(t *testing.T) {
		flow := NewFlow()

		slowTask := &TestTask{Name: "slow", Result: "result", Delay: 200 * time.Millisecond}
		AddTask(flow, "slow", slowTask)
		flow.SetStart("slow")

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := flow.Run(ctx, "input")

		if err == nil {
			t.Error("Expected timeout error")
		}

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected context.DeadlineExceeded, got %v", err)
		}
	})

	t.Run("Task Error Propagation", func(t *testing.T) {
		flow := NewFlow()

		errorTask := &ErrorTestTask{ErrorMessage: "task failed"}
		AddTask(flow, "error", errorTask)
		flow.SetStart("error")

		ctx := context.Background()
		_, err := flow.Run(ctx, "input")

		if err == nil {
			t.Error("Expected task error to be propagated")
		}

		if !errors.Is(err, ErrWorkflowExecution) {
			t.Errorf("Expected ErrWorkflowExecution, got %T", err)
		}

		if !strings.Contains(err.Error(), "error") {
			t.Errorf("Expected error message to contain task ID 'error', got %s", err.Error())
		}
	})
}

func TestRetryPolicy(t *testing.T) {
	t.Run("No Retries Needed", func(t *testing.T) {
		flow := NewFlow()

		task := &TestTask{Name: "success", Result: "ok"}
		AddTask(flow, "task", task)
		flow.SetStart("task")

		ctx := context.Background()
		result, err := flow.Run(ctx, "input")

		if err != nil {
			t.Fatalf("Expected success: %v", err)
		}

		if result != "ok" {
			t.Errorf("Expected 'ok', got %v", result)
		}
	})

	t.Run("Retry with Success", func(t *testing.T) {
		flow := NewFlow().WithRetryPolicy(RetryPolicy{
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     100 * time.Millisecond,
			Multiplier:   2.0,
			MaxRetries:   3,
		})

		retryTask := &RetryTestTask{MaxAttempts: 2, FinalResult: "success"}
		AddTask(flow, "retry", retryTask)
		flow.SetStart("retry")

		ctx := context.Background()
		result, err := flow.Run(ctx, "input")

		if err != nil {
			t.Fatalf("Expected retry success: %v", err)
		}

		if result != "success" {
			t.Errorf("Expected 'success', got %v", result)
		}

		if retryTask.attempts != 2 {
			t.Errorf("Expected 2 attempts, got %d", retryTask.attempts)
		}
	})

	t.Run("Max Retries Exceeded", func(t *testing.T) {
		flow := NewFlow().WithRetryPolicy(RetryPolicy{
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     100 * time.Millisecond,
			Multiplier:   2.0,
			MaxRetries:   2,
		})

		// This task will retry 3 times but we allow only 2 retries
		retryTask := &RetryTestTask{MaxAttempts: 4, FinalResult: "never reached"}
		AddTask(flow, "error", retryTask)
		flow.SetStart("error")

		ctx := context.Background()
		_, err := flow.Run(ctx, "input")

		if err == nil {
			t.Error("Expected max retries exceeded error")
		}

		if !errors.Is(err, ErrMaxRetriesExceeded) {
			t.Errorf("Expected ErrMaxRetriesExceeded, got %T", err)
		}

		if !strings.Contains(err.Error(), "2") {
			t.Errorf("Expected error to contain MaxRetries 2, got %s", err.Error())
		}

		if !strings.Contains(err.Error(), "error") {
			t.Errorf("Expected error to contain task ID 'error', got %s", err.Error())
		}

		// Verify that we made 3 attempts (initial + 2 retries)
		if retryTask.attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", retryTask.attempts)
		}
	})

	t.Run("Retry During Context Cancellation", func(t *testing.T) {
		flow := NewFlow().WithRetryPolicy(RetryPolicy{
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     500 * time.Millisecond,
			Multiplier:   2.0,
			MaxRetries:   5,
		})

		// Use a task that returns OutcomeRetry
		retryTask := &RetryTestTask{
			MaxAttempts: 3,
			FinalResult: "success",
		}
		AddTask(flow, "retry", retryTask)
		flow.SetStart("retry")

		ctx, cancel := context.WithCancel(context.Background())

		// Cancel during retry delay
		go func() {
			time.Sleep(50 * time.Millisecond) // Cancel after first retry is scheduled
			cancel()
		}()

		_, err := flow.Run(ctx, "input")

		if err == nil {
			t.Error("Expected cancellation error")
		}

		if !errors.Is(err, ErrCancelled) {
			t.Errorf("Expected ErrCancelled, got %T", err)
		}
	})
}

func TestBackoffCalculation(t *testing.T) {
	policy := RetryPolicy{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		MaxRetries:   5,
	}

	testCases := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{4, 1000 * time.Millisecond}, // Capped at MaxDelay
		{5, 1000 * time.Millisecond}, // Still capped
	}

	for _, tc := range testCases {
		delay := calculateBackoff(policy, tc.attempt)
		if delay != tc.expected {
			t.Errorf("Attempt %d: expected %v, got %v", tc.attempt, tc.expected, delay)
		}
	}
}

func TestParallelExecution(t *testing.T) {
	t.Run("Fan-out Pattern", func(t *testing.T) {
		flow := NewFlow()

		// Start task that fans out to multiple parallel tasks
		startTask := &TestTask{Name: "start", Result: "start_result"}
		task1 := &TestTask{Name: "task1", Result: "result1"}
		task2 := &TestTask{Name: "task2", Result: "result2"}
		task3 := &TestTask{Name: "task3", Result: "result3"}

		AddTask(flow, "start", startTask).
			LinkToID("task1").
			LinkToID("task2").
			LinkToID("task3")

		AddTask(flow, "task1", task1)
		AddTask(flow, "task2", task2)
		AddTask(flow, "task3", task3)

		flow.SetStart("start")

		ctx := context.Background()

		start := time.Now()
		_, err := flow.Run(ctx, "input")
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Parallel execution failed: %v", err)
		}

		// Should complete quickly since tasks run in parallel
		if duration > 100*time.Millisecond {
			t.Errorf("Parallel execution took too long: %v", duration)
		}
	})

	t.Run("Concurrent Task Execution", func(t *testing.T) {
		flow := NewFlow()

		// Create tasks with delay to test concurrency
		task1 := &TestTask{Name: "task1", Result: "result1", Delay: 20 * time.Millisecond}
		task2 := &TestTask{Name: "task2", Result: "result2", Delay: 20 * time.Millisecond}
		task3 := &TestTask{Name: "task3", Result: "result3", Delay: 20 * time.Millisecond}

		startTask := &TestTask{Name: "start", Result: "start"}

		AddTask(flow, "start", startTask).
			LinkToID("task1").
			LinkToID("task2").
			LinkToID("task3")

		AddTask(flow, "task1", task1)
		AddTask(flow, "task2", task2)
		AddTask(flow, "task3", task3)

		flow.SetStart("start")

		ctx := context.Background()
		start := time.Now()
		_, err := flow.Run(ctx, "input")
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Concurrent execution failed: %v", err)
		}

		// With parallel execution, should take around 20ms, not 60ms
		if duration > 50*time.Millisecond {
			t.Errorf("Tasks may not be running in parallel, took %v", duration)
		}
	})
}

func TestFeedbackHandler(t *testing.T) {
	t.Run("Feedback Collection", func(t *testing.T) {
		var feedbacks []Feedback

		handler := func(f Feedback) {
			feedbacks = append(feedbacks, f)
		}

		flow := NewFlow().WithFeedbackHandler(handler)

		task := &TestTask{Name: "test", Result: "result"}
		AddTask(flow, "task", task)
		flow.SetStart("task")

		ctx := context.Background()
		_, err := flow.Run(ctx, "input")

		if err != nil {
			t.Fatalf("Flow execution failed: %v", err)
		}

		// Should have at least workflow_started, task_started, task_completed, workflow_completed
		if len(feedbacks) < 4 {
			t.Errorf("Expected at least 4 feedback events, got %d", len(feedbacks))
		}

		// Check for expected feedback types
		statuses := make(map[string]bool)
		for _, f := range feedbacks {
			statuses[string(f.Status)] = true
		}

		expectedStatuses := []string{"workflow_started", "task_started", "task_completed", "workflow_completed"}
		for _, status := range expectedStatuses {
			if !statuses[status] {
				t.Errorf("Missing expected feedback status: %s", status)
			}
		}
	})

	t.Run("Feedback on Error", func(t *testing.T) {
		var feedbacks []Feedback

		handler := func(f Feedback) {
			feedbacks = append(feedbacks, f)
		}

		flow := NewFlow().WithFeedbackHandler(handler)

		errorTask := &ErrorTestTask{ErrorMessage: "test error"}
		AddTask(flow, "error", errorTask)
		flow.SetStart("error")

		ctx := context.Background()
		_, err := flow.Run(ctx, "input")

		if err == nil {
			t.Error("Expected error from task execution")
		}

		// Should have feedback for workflow_started, task_started, task_failed, workflow_failed
		statuses := make(map[string]bool)
		for _, f := range feedbacks {
			statuses[string(f.Status)] = true
		}

		expectedStatuses := []string{"workflow_started", "task_started", "task_failed", "workflow_failed"}
		for _, status := range expectedStatuses {
			if !statuses[status] {
				t.Errorf("Missing expected feedback status: %s", status)
			}
		}
	})
}

func TestExecutionContext(t *testing.T) {
	t.Run("Context Propagation", func(t *testing.T) {
		flow := NewFlow()

		var receivedContext ExecutionContext

		task := &ContextCapturingTask{CapturedContext: &receivedContext}
		AddTask(flow, "context_task", task)
		flow.SetStart("context_task")

		ctx := context.Background()
		_, err := flow.Run(ctx, "input")

		if err != nil {
			t.Fatalf("Flow execution failed: %v", err)
		}

		if receivedContext.TaskID != "context_task" {
			t.Errorf("Expected task ID 'context_task', got %s", receivedContext.TaskID)
		}

		if receivedContext.FlowID != flow.ID() {
			t.Errorf("Expected flow ID %s, got %s", flow.ID(), receivedContext.FlowID)
		}

		if receivedContext.RetryCount != 0 {
			t.Errorf("Expected retry count 0, got %d", receivedContext.RetryCount)
		}
	})
}
