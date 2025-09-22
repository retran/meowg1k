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
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidateWorkflow(t *testing.T) {
	t.Run("Valid Workflow", func(t *testing.T) {
		flow := NewFlow()
		task := &TestTask{Name: "test", Result: "ok"}

		AddTask(flow, "start", task)
		flow.SetStart("start")

		err := flow.Validate()
		if err != nil {
			t.Errorf("Valid workflow should not have validation errors: %v", err)
		}
	})

	t.Run("No Start Task Set", func(t *testing.T) {
		flow := NewFlow()
		task := &TestTask{Name: "test", Result: "ok"}

		AddTask(flow, "task1", task)

		err := flow.Validate()
		if err == nil {
			t.Error("Expected validation error for missing start task")
		}

		// Check for workflow validation error
		if !errors.Is(err, ErrWorkflowValidation) {
			t.Error("Expected ErrWorkflowValidation error")
		}

		if !strings.Contains(err.Error(), "start task is not set") {
			t.Errorf("Expected error to contain 'start task is not set', got: %s", err.Error())
		}
	})

	t.Run("Start Task Does Not Exist", func(t *testing.T) {
		flow := NewFlow()
		task := &TestTask{Name: "test", Result: "ok"}

		AddTask(flow, "task1", task)
		flow.SetStart("nonexistent")

		err := flow.Validate()
		if err == nil {
			t.Error("Expected validation error for nonexistent start task")
		}

		// Check for workflow validation error
		if !errors.Is(err, ErrWorkflowValidation) {
			t.Error("Expected ErrWorkflowValidation error")
		}

		if !strings.Contains(err.Error(), "start task does not exist") {
			t.Errorf("Expected error to contain 'start task does not exist', got: %s", err.Error())
		}

		if !strings.Contains(err.Error(), "nonexistent") {
			t.Error("Error details should contain start task name")
		}
	})

	t.Run("Referenced Task Does Not Exist", func(t *testing.T) {
		flow := NewFlow()
		task := &TestTask{Name: "test", Result: "ok"}

		AddTask(flow, "task1", task).LinkToID("nonexistent")
		flow.SetStart("task1")

		err := flow.Validate()
		if err == nil {
			t.Error("Expected validation error for nonexistent referenced task")
		}

		// Check for workflow validation error
		if !errors.Is(err, ErrWorkflowValidation) {
			t.Error("Expected ErrWorkflowValidation error")
		}

		if !strings.Contains(err.Error(), "referenced task does not exist") {
			t.Errorf("Expected error to contain 'referenced task does not exist', got: %s", err.Error())
		}

		if !strings.Contains(err.Error(), "task1") {
			t.Error("Error details should contain source task name")
		}

		if !strings.Contains(err.Error(), "nonexistent") {
			t.Error("Error details should contain target task name")
		}
	})
}

func TestDetectCycles(t *testing.T) {
	t.Run("No Cycles", func(t *testing.T) {
		flow := NewFlow()

		task1 := &TestTask{Name: "task1", Result: "result1"}
		task2 := &TestTask{Name: "task2", Result: "result2"}
		task3 := &TestTask{Name: "task3", Result: "result3"}

		AddTask(flow, "task1", task1).LinkToID("task2")
		AddTask(flow, "task2", task2).LinkToID("task3")
		AddTask(flow, "task3", task3)

		flow.SetStart("task1")

		err := flow.Validate()
		if err != nil {
			t.Errorf("No cycles should be detected: %v", err)
		}
	})

	t.Run("Simple Cycle", func(t *testing.T) {
		flow := NewFlow()

		task1 := &TestTask{Name: "task1", Result: "result1"}
		task2 := &TestTask{Name: "task2", Result: "result2"}

		AddTask(flow, "task1", task1).LinkToID("task2")
		AddTask(flow, "task2", task2).LinkToID("task1") // Creates cycle

		flow.SetStart("task1")

		err := flow.Validate()
		if err == nil {
			t.Error("Expected validation error for cycle detection")
		}

		// Check for workflow validation error
		if !errors.Is(err, ErrWorkflowValidation) {
			t.Error("Expected ErrWorkflowValidation error")
		}

		if !strings.Contains(err.Error(), "circular dependency detected") {
			t.Errorf("Expected error to contain 'circular dependency detected', got: %s", err.Error())
		}
	})

	t.Run("Self Loop", func(t *testing.T) {
		flow := NewFlow()

		task1 := &TestTask{Name: "task1", Result: "result1"}

		AddTask(flow, "task1", task1).LinkToID("task1") // Self loop

		flow.SetStart("task1")

		err := flow.Validate()
		if err == nil {
			t.Error("Expected validation error for self loop")
		}
	})

	t.Run("Complex Cycle", func(t *testing.T) {
		flow := NewFlow()

		task1 := &TestTask{Name: "task1", Result: "result1"}
		task2 := &TestTask{Name: "task2", Result: "result2"}
		task3 := &TestTask{Name: "task3", Result: "result3"}
		task4 := &TestTask{Name: "task4", Result: "result4"}

		AddTask(flow, "task1", task1).LinkToID("task2")
		AddTask(flow, "task2", task2).LinkToID("task3")
		AddTask(flow, "task3", task3).LinkToID("task4")
		AddTask(flow, "task4", task4).LinkToID("task2") // Creates cycle

		flow.SetStart("task1")

		err := flow.Validate()
		if err == nil {
			t.Error("Expected validation error for complex cycle")
		}
	})
}

func TestUnreachableTasks(t *testing.T) {
	t.Run("All Tasks Reachable", func(t *testing.T) {
		flow := NewFlow()

		task1 := &TestTask{Name: "task1", Result: "result1"}
		task2 := &TestTask{Name: "task2", Result: "result2"}
		task3 := &TestTask{Name: "task3", Result: "result3"}

		AddTask(flow, "task1", task1).LinkToID("task2")
		AddTask(flow, "task2", task2).LinkToID("task3")
		AddTask(flow, "task3", task3)

		flow.SetStart("task1")

		// This should not fail validation but might log warnings
		err := flow.Validate()
		if err != nil {
			t.Errorf("Should not fail validation: %v", err)
		}
	})

	t.Run("Unreachable Tasks Exist", func(t *testing.T) {
		flow := NewFlow()

		task1 := &TestTask{Name: "task1", Result: "result1"}
		task2 := &TestTask{Name: "task2", Result: "result2"}
		unreachableTask := &TestTask{Name: "unreachable", Result: "unreachable"}

		AddTask(flow, "task1", task1).LinkToID("task2")
		AddTask(flow, "task2", task2)
		AddTask(flow, "unreachable", unreachableTask) // This task is unreachable

		flow.SetStart("task1")

		// Validation should pass (unreachable tasks are warnings, not errors)
		err := flow.Validate()
		if err != nil {
			t.Errorf("Should not fail validation for unreachable tasks: %v", err)
		}
	})
}

func TestValidationWithConditionalLinks(t *testing.T) {
	t.Run("Valid Conditional Links", func(t *testing.T) {
		flow := NewFlow()

		conditionalTask := &ConditionalTestTask{Result: true, ReturnData: true}
		task2 := &TestTask{Name: "task2", Result: "result2"}
		task3 := &TestTask{Name: "task3", Result: "result3"}

		AddTask(flow, "conditional", conditionalTask).
			WhenID(func(data bool) bool { return data }, "task2").
			WhenID(func(data bool) bool { return !data }, "task3")

		AddTask(flow, "task2", task2)
		AddTask(flow, "task3", task3)

		flow.SetStart("conditional")

		err := flow.Validate()
		if err != nil {
			t.Errorf("Valid conditional workflow should not fail validation: %v", err)
		}
	})

	t.Run("Conditional Link to Nonexistent Task", func(t *testing.T) {
		flow := NewFlow()

		conditionalTask := &ConditionalTestTask{Result: true, ReturnData: true}

		AddTask(flow, "conditional", conditionalTask).
			WhenID(func(data bool) bool { return data }, "nonexistent")

		flow.SetStart("conditional")

		err := flow.Validate()
		if err == nil {
			t.Error("Expected validation error for conditional link to nonexistent task")
		}
	})
}

func TestValidationPerformance(t *testing.T) {
	t.Run("Large Workflow Validation", func(t *testing.T) {
		flow := NewFlow()

		// Create a large linear workflow
		taskCount := 1000

		for i := 0; i < taskCount; i++ {
			taskID := TaskID("task" + string(rune('0'+i%10)) + string(rune('0'+(i/10)%10)) + string(rune('0'+(i/100)%10)))
			task := &TestTask{Name: string(taskID), Result: "result"}

			AddTask(flow, taskID, task)

			if i > 0 {
				prevTaskID := TaskID("task" + string(rune('0'+(i-1)%10)) + string(rune('0'+((i-1)/10)%10)) + string(rune('0'+((i-1)/100)%10)))
				flow.internal.addLink(prevTaskID, link{to: taskID, on: OutcomeSuccess})
			}

			if i == 0 {
				flow.SetStart(taskID)
			}
		}

		start := time.Now()
		err := flow.Validate()
		duration := time.Since(start)

		if err != nil {
			t.Errorf("Large workflow validation failed: %v", err)
		}

		// Validation should complete reasonably quickly
		if duration > time.Second {
			t.Errorf("Validation took too long: %v", duration)
		}

		t.Logf("Validated %d tasks in %v", taskCount, duration)
	})
}
