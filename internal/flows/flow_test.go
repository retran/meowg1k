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
	"errors"
	"testing"
	"time"
)

// TestTask - simple task for tests
type TestTask struct {
	Name   string
	Result string
	Error  error
	Delay  time.Duration
}

func (t *TestTask) Execute(ctx context.Context, input interface{}) (string, Outcome[any], error) {
	if t.Delay > 0 {
		select {
		case <-time.After(t.Delay):
			// Delay completed normally
		case <-ctx.Done():
			return "", Outcome[any]{}, ctx.Err()
		}
	}

	if t.Error != nil {
		return "", Outcome[any]{}, t.Error
	}

	return t.Result, Outcome[any]{Type: OutcomeSuccess}, nil
}

// ConditionalTestTask - task with conditional result
type ConditionalTestTask struct {
	Result     bool
	ReturnData bool
}

func (t *ConditionalTestTask) Execute(ctx context.Context, input interface{}) (bool, Outcome[bool], error) {
	return t.Result, Outcome[bool]{Type: OutcomeConditional, Data: t.ReturnData}, nil
}

// RetryTestTask - task that can request retry
type RetryTestTask struct {
	MaxAttempts int
	attempts    int
	FinalResult string
}

func (t *RetryTestTask) Execute(ctx context.Context, input interface{}) (string, Outcome[any], error) {
	t.attempts++

	if t.attempts < t.MaxAttempts {
		// Return OutcomeRetry to trigger retry
		return "", Outcome[any]{Type: OutcomeRetry}, nil
	}

	return t.FinalResult, Outcome[any]{Type: OutcomeSuccess}, nil
}

// TestTaskWithInput - task that processes input data
type TestTaskWithInput struct {
	Name string
}

func (t *TestTaskWithInput) Execute(ctx context.Context, input interface{}) (string, Outcome[any], error) {
	inputStr, ok := input.(string)
	if !ok {
		return "", Outcome[any]{}, errors.New("invalid input type")
	}
	return t.Name + "_" + inputStr, Outcome[any]{Type: OutcomeSuccess}, nil
}

// ErrorTestTask - task that always returns an error
type ErrorTestTask struct {
	ErrorMessage string
}

func (t *ErrorTestTask) Execute(ctx context.Context, input interface{}) (string, Outcome[any], error) {
	return "", Outcome[any]{}, errors.New(t.ErrorMessage)
}

func TestNewFlow(t *testing.T) {
	t.Run("Basic Flow Creation", func(t *testing.T) {
		flow := NewFlow()

		if flow == nil {
			t.Fatal("NewFlow() returned nil")
		}

		if flow.internal == nil {
			t.Fatal("Flow internal should not be nil")
		}

		if flow.ID() == "" {
			t.Error("Flow ID should not be empty")
		}

		// Check default values
		flow.internal.RLock()
		if flow.internal.tasks == nil {
			t.Error("Tasks map should be initialized")
		}
		if flow.internal.links == nil {
			t.Error("Links map should be initialized")
		}
		if flow.internal.retryPolicy.MaxRetries != 3 {
			t.Error("Default retry policy should have MaxRetries = 3")
		}
		if flow.internal.feedbackHandler == nil {
			t.Error("Feedback handler should be initialized")
		}
		flow.internal.RUnlock()
	})

	t.Run("Flow ID Uniqueness", func(t *testing.T) {
		flow1 := NewFlow()
		flow2 := NewFlow()

		if flow1.ID() == flow2.ID() {
			t.Error("Different flows should have different IDs")
		}
	})

	t.Run("Flow ID Format", func(t *testing.T) {
		flow := NewFlow()
		id := flow.ID()

		if id[:5] != "flow_" {
			t.Error("Flow ID should start with 'flow_'")
		}

		if len(id) < 10 {
			t.Error("Flow ID should be at least 10 characters long")
		}
	})
}

func TestFlowConfiguration(t *testing.T) {
	t.Run("WithRetryPolicy", func(t *testing.T) {
		customPolicy := RetryPolicy{
			InitialDelay: 50 * time.Millisecond,
			MaxDelay:     1 * time.Second,
			Multiplier:   1.5,
			MaxRetries:   5,
		}

		flow := NewFlow().WithRetryPolicy(customPolicy)

		flow.internal.RLock()
		policy := flow.internal.retryPolicy
		flow.internal.RUnlock()

		if policy.InitialDelay != customPolicy.InitialDelay {
			t.Errorf("Expected InitialDelay %v, got %v", customPolicy.InitialDelay, policy.InitialDelay)
		}
		if policy.MaxRetries != customPolicy.MaxRetries {
			t.Errorf("Expected MaxRetries %d, got %d", customPolicy.MaxRetries, policy.MaxRetries)
		}
	})

	t.Run("WithRetryPolicy Validation", func(t *testing.T) {
		// Test with invalid values
		invalidPolicy := RetryPolicy{
			Multiplier: 0.5, // Should be corrected to 1.0
			MaxRetries: -1,  // Should be corrected to 0
		}

		flow := NewFlow().WithRetryPolicy(invalidPolicy)

		flow.internal.RLock()
		policy := flow.internal.retryPolicy
		flow.internal.RUnlock()

		if policy.Multiplier < 1.0 {
			t.Errorf("Multiplier should be at least 1.0, got %f", policy.Multiplier)
		}
		if policy.MaxRetries < 0 {
			t.Errorf("MaxRetries should be at least 0, got %d", policy.MaxRetries)
		}
	})

	t.Run("SetStart", func(t *testing.T) {
		flow := NewFlow().SetStart("start_task")

		flow.internal.RLock()
		startTask := flow.internal.startTask
		flow.internal.RUnlock()

		if startTask != "start_task" {
			t.Errorf("Expected start task 'start_task', got '%s'", startTask)
		}
	})

	t.Run("WithFeedbackHandler", func(t *testing.T) {
		called := false
		handler := func(Feedback) {
			called = true
		}

		flow := NewFlow().WithFeedbackHandler(handler)

		flow.internal.RLock()
		feedbackHandler := flow.internal.feedbackHandler
		flow.internal.RUnlock()

		feedbackHandler(Feedback{})

		if !called {
			t.Error("Custom feedback handler was not called")
		}
	})

	t.Run("WithFeedbackHandler Nil", func(t *testing.T) {
		flow := NewFlow().WithFeedbackHandler(nil)

		flow.internal.RLock()
		feedbackHandler := flow.internal.feedbackHandler
		flow.internal.RUnlock()

		// Should not panic
		feedbackHandler(Feedback{})
	})

	t.Run("Fluent Interface", func(t *testing.T) {
		customPolicy := RetryPolicy{MaxRetries: 5}
		flow := NewFlow().
			WithRetryPolicy(customPolicy).
			SetStart("task1").
			WithFeedbackHandler(func(Feedback) {})

		if flow == nil {
			t.Fatal("Fluent interface should return non-nil flow")
		}
	})
}

func TestAddTask(t *testing.T) {
	t.Run("Add Single Task", func(t *testing.T) {
		flow := NewFlow()
		task := &TestTask{Name: "test", Result: "ok"}

		node := AddTask(flow, "task1", task)

		if node == nil {
			t.Fatal("AddTask should return non-nil node")
		}

		if node.id != "task1" {
			t.Errorf("Expected task ID 'task1', got '%s'", node.id)
		}

		if node.flow != flow {
			t.Error("Node should reference the correct flow")
		}

		// Check task is stored
		flow.internal.RLock()
		_, exists := flow.internal.tasks["task1"]
		flow.internal.RUnlock()

		if !exists {
			t.Error("Task should be stored in flow")
		}
	})

	t.Run("Add Multiple Tasks", func(t *testing.T) {
		flow := NewFlow()

		task1 := &TestTask{Name: "task1", Result: "result1"}
		task2 := &TestTask{Name: "task2", Result: "result2"}

		AddTask(flow, "task1", task1)
		AddTask(flow, "task2", task2)

		flow.internal.RLock()
		tasksCount := len(flow.internal.tasks)
		flow.internal.RUnlock()

		if tasksCount != 2 {
			t.Errorf("Expected 2 tasks, got %d", tasksCount)
		}
	})
}

func TestTaskNode(t *testing.T) {
	t.Run("LinkTo", func(t *testing.T) {
		flow := NewFlow()
		task1 := &TestTask{Name: "task1", Result: "result1"}
		task2 := &TestTask{Name: "task2", Result: "result2"}

		node1 := AddTask(flow, "task1", task1)
		AddTask(flow, "task2", task2)

		returnedNode := node1.LinkToID("task2")

		if returnedNode != node1 {
			t.Error("LinkTo should return the same node for chaining")
		}

		// Check link is created
		flow.internal.RLock()
		links := flow.internal.links["task1"]
		flow.internal.RUnlock()

		if len(links) != 1 {
			t.Errorf("Expected 1 link, got %d", len(links))
		}

		if links[0].to != "task2" {
			t.Errorf("Expected link to 'task2', got '%s'", links[0].to)
		}

		if links[0].on != OutcomeSuccess {
			t.Errorf("Expected link on OutcomeSuccess, got %v", links[0].on)
		}
	})

	t.Run("When Conditional", func(t *testing.T) {
		flow := NewFlow()
		task1 := &ConditionalTestTask{Result: true, ReturnData: true}
		task2 := &TestTask{Name: "task2", Result: "result2"}

		node1 := AddTask(flow, "task1", task1)
		AddTask(flow, "task2", task2)

		condition := func(data bool) bool {
			return data == true
		}

		returnedNode := node1.WhenID(condition, "task2")

		if returnedNode != node1 {
			t.Error("When should return the same node for chaining")
		}

		// Check conditional link is created
		flow.internal.RLock()
		links := flow.internal.links["task1"]
		flow.internal.RUnlock()

		if len(links) != 1 {
			t.Errorf("Expected 1 link, got %d", len(links))
		}

		if links[0].on != OutcomeConditional {
			t.Errorf("Expected link on OutcomeConditional, got %v", links[0].on)
		}

		if links[0].condition == nil {
			t.Error("Conditional link should have condition function")
		}

		// Test condition function
		if !links[0].condition(true) {
			t.Error("Condition should return true for data=true")
		}

		if links[0].condition(false) {
			t.Error("Condition should return false for data=false")
		}
	})

	t.Run("Multiple Links", func(t *testing.T) {
		flow := NewFlow()
		task1 := &ConditionalTestTask{Result: true, ReturnData: true}

		AddTask(flow, "task1", task1).
			LinkToID("task2").
			WhenID(func(data bool) bool { return data }, "task3")

		flow.internal.RLock()
		links := flow.internal.links["task1"]
		flow.internal.RUnlock()

		if len(links) != 2 {
			t.Errorf("Expected 2 links, got %d", len(links))
		}
	})
}

func TestSimpleLinearFlow(t *testing.T) {
	flow := NewFlow()

	task1 := &TestTask{Name: "task1", Result: "result1"}
	task2 := &TestTask{Name: "task2", Result: "result2"}
	task3 := &TestTask{Name: "task3", Result: "final_result"}

	AddTask(flow, "task1", task1).LinkToID("task2")
	AddTask(flow, "task2", task2).LinkToID("task3")
	AddTask(flow, "task3", task3)

	flow.SetStart("task1")

	ctx := context.Background()
	result, err := flow.Run(ctx, "initial_input")

	if err != nil {
		t.Fatalf("Flow execution failed: %v", err)
	}

	if result != "final_result" {
		t.Errorf("Expected 'final_result', got %v", result)
	}
}

func TestFlowWithInput(t *testing.T) {
	flow := NewFlow()

	task1 := &TestTaskWithInput{Name: "task1"}
	task2 := &TestTaskWithInput{Name: "task2"}

	AddTask(flow, "task1", task1).LinkToID("task2")
	AddTask(flow, "task2", task2)

	flow.SetStart("task1")

	ctx := context.Background()
	result, err := flow.Run(ctx, "input")

	if err != nil {
		t.Fatalf("Flow execution failed: %v", err)
	}

	expected := "task2_task1_input"
	if result != expected {
		t.Errorf("Expected '%s', got %v", expected, result)
	}
}

func TestValidationErrors(t *testing.T) {
	t.Run("NoStartTask", func(t *testing.T) {
		flow := NewFlow()
		AddTask(flow, "task1", &TestTask{})

		err := flow.Validate()
		if err == nil {
			t.Error("Expected validation error for missing start task")
		}

		// Check for workflow validation error
		if !errors.Is(err, ErrWorkflowValidation) {
			t.Error("Expected ErrWorkflowValidation error")
		}
	})

	t.Run("StartTaskNotExists", func(t *testing.T) {
		flow := NewFlow().SetStart("nonexistent")
		AddTask(flow, "task1", &TestTask{})

		err := flow.Validate()
		if err == nil {
			t.Error("Expected validation error for nonexistent start task")
		}
	})

	t.Run("ReferencedTaskNotExists", func(t *testing.T) {
		flow := NewFlow().SetStart("task1")

		task1 := &TestTask{Name: "task1", Result: "result1"}
		AddTask(flow, "task1", task1).LinkToID("nonexistent")

		err := flow.Validate()
		if err == nil {
			t.Error("Expected validation error for nonexistent referenced task")
		}

		// Check for workflow validation error
		if !errors.Is(err, ErrWorkflowValidation) {
			t.Error("Expected ErrWorkflowValidation error")
		}
	})
}

func TestReduceTask(t *testing.T) {
	flow := NewFlow()

	AddReduceTask(flow, "reduce", func(acc int, item int) int {
		return acc + item
	}, 0).LinkToID("final")

	AddTask(flow, "final", &TestTask{Result: "reduce_complete"})

	flow.SetStart("reduce")

	ctx := context.Background()
	input := []int{1, 2, 3, 4, 5}
	result, err := flow.Run(ctx, input)

	if err != nil {
		t.Fatalf("Reduce flow execution failed: %v", err)
	}

	if result != "reduce_complete" {
		t.Errorf("Expected 'reduce_complete', got %v", result)
	}
}

func TestFlowCancellation(t *testing.T) {
	flow := NewFlow()

	task := &TestTask{Name: "slow_task", Result: "result", Delay: 100 * time.Millisecond}
	AddTask(flow, "task1", task)
	flow.SetStart("task1")

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := flow.Run(ctx, "input")

	if err == nil {
		t.Error("Expected cancellation error")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestFlowTimeout(t *testing.T) {
	flow := NewFlow()

	task := &TestTask{Name: "slow_task", Result: "result", Delay: 100 * time.Millisecond}
	AddTask(flow, "task1", task)
	flow.SetStart("task1")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := flow.Run(ctx, "input")

	if err == nil {
		t.Error("Expected timeout error")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded error, got %v", err)
	}
}

func TestFlowWithErrors(t *testing.T) {
	flow := NewFlow()

	task := &ErrorTestTask{ErrorMessage: "test error"}
	AddTask(flow, "task1", task)
	flow.SetStart("task1")

	ctx := context.Background()
	_, err := flow.Run(ctx, "input")

	if err == nil {
		t.Error("Expected error from task execution")
	}

	// Check for workflow execution error
	if !errors.Is(err, ErrWorkflowExecution) {
		t.Error("Expected ErrWorkflowExecution error")
	}
}
