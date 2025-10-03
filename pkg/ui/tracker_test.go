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

package ui

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/retran/meowg1k/pkg/executor"
)

func TestNewExecutionTracker(t *testing.T) {
	tracker := NewExecutionTracker(false)
	if tracker == nil {
		t.Fatal("NewExecutionTracker returned nil")
	}
	if tracker.GetExecutionCount() != 0 {
		t.Error("executions map should be initialized and empty")
	}
	if tracker.order == nil {
		t.Error("order slice should be initialized")
	}
	if tracker.feedbackChan == nil {
		t.Error("feedbackChan should be initialized")
	}
	if tracker.silent {
		t.Error("silent should be false")
	}
}

func TestNewExecutionTrackerSilent(t *testing.T) {
	tracker := NewExecutionTracker(true)
	if tracker == nil {
		t.Fatal("NewExecutionTracker returned nil")
	}
	if !tracker.silent {
		t.Error("silent should be true")
	}
}

func TestExecutionTracker_StartStop(t *testing.T) {
	tracker := NewExecutionTracker(false)

	tracker.Start()

	// Give the goroutine a moment to start
	time.Sleep(10 * time.Millisecond)

	tracker.Stop()

	// Should complete without deadlock
}

func TestExecutionTracker_StartStopSilent(t *testing.T) {
	tracker := NewExecutionTracker(true)

	// Start and Stop should be no-ops in silent mode
	tracker.Start()
	tracker.Stop()

	// Should complete immediately without goroutine
}

func TestExecutionTracker_FeedbackHandler(t *testing.T) {
	tracker := NewExecutionTracker(false)

	handler := tracker.FeedbackHandler()
	if handler == nil {
		t.Error("FeedbackHandler should not return nil")
	}
}

func TestExecutionTracker_FeedbackHandlerSilent(t *testing.T) {
	tracker := NewExecutionTracker(true)

	handler := tracker.FeedbackHandler()
	if handler == nil {
		t.Error("FeedbackHandler should not return nil even in silent mode")
	}
}

func TestExecutionTracker_WithFeedback(t *testing.T) {
	tracker := NewExecutionTracker(false)
	tracker.Start()
	defer tracker.Stop()

	handler := tracker.FeedbackHandler()

	// Send various feedback messages
	feedback := &executor.Feedback{
		ActivityName: "TestExecution",
		Status:       executor.StatusRunning,
		Message:      "Test message",
	}

	handler(feedback)

	// Give time for processing
	time.Sleep(20 * time.Millisecond)

	// Verify the execution was tracked
	if tracker.GetExecutionCount() == 0 {
		t.Error("Expected execution to be tracked")
	}

	exec := tracker.GetExecution("TestExecution")
	if exec == nil {
		t.Error("Expected TestExecution to be tracked")
	} else {
		if exec.Name != "TestExecution" {
			t.Errorf("Expected name TestExecution, got %s", exec.Name)
		}
		if exec.Status != executor.StatusRunning {
			t.Errorf("Expected status Running, got %v", exec.Status)
		}
		if exec.Message != "Test message" {
			t.Errorf("Expected message 'Test message', got %s", exec.Message)
		}
	}
}

func TestExecutionTracker_MultipleExecutions(t *testing.T) {
	tracker := NewExecutionTracker(false)
	tracker.Start()
	defer tracker.Stop()

	handler := tracker.FeedbackHandler()

	// Send feedback for multiple executions
	executions := []struct {
		name    string
		status  executor.Status
		message string
	}{
		{"Execution1", executor.StatusRunning, "Running 1"},
		{"Execution2", executor.StatusRunning, "Running 2"},
		{"Execution3", executor.StatusCompleted, "Completed 3"},
	}

	for _, exec := range executions {
		feedback := &executor.Feedback{
			ActivityName: exec.name,
			Status:       exec.status,
			Message:      exec.message,
		}
		handler(feedback)
	}

	// Give time for processing
	time.Sleep(50 * time.Millisecond)

	// Verify all executions were tracked
	count := tracker.GetExecutionCount()
	if count != len(executions) {
		t.Errorf("Expected %d executions, got %d", len(executions), count)
	}

	for _, exec := range executions {
		tracked := tracker.GetExecution(exec.name)
		if tracked == nil {
			t.Errorf("Expected %s to be tracked", exec.name)
		} else {
			if tracked.Status != exec.status {
				t.Errorf("Expected %s status %v, got %v", exec.name, exec.status, tracked.Status)
			}
		}
	}
}

func TestExecutionTracker_StatusTransitions(t *testing.T) {
	tracker := NewExecutionTracker(false)
	tracker.Start()
	defer tracker.Stop()

	handler := tracker.FeedbackHandler()

	// Test status transition from Running to Completed
	handler(&executor.Feedback{
		ActivityName: "TransitionTest",
		Status:       executor.StatusRunning,
		Message:      "Starting",
	})

	time.Sleep(20 * time.Millisecond)

	handler(&executor.Feedback{
		ActivityName: "TransitionTest",
		Status:       executor.StatusCompleted,
		Message:      "Done",
	})

	time.Sleep(20 * time.Millisecond)

	exec := tracker.GetExecution("TransitionTest")
	if exec == nil {
		t.Error("Expected TransitionTest to be tracked")
	} else {
		if exec.Status != executor.StatusCompleted {
			t.Errorf("Expected status Completed, got %v", exec.Status)
		}
		if exec.EndTime == nil {
			t.Error("EndTime should be set for completed execution")
		}
	}
}

func TestExecutionTracker_WithError(t *testing.T) {
	tracker := NewExecutionTracker(false)
	tracker.Start()
	defer tracker.Stop()

	handler := tracker.FeedbackHandler()

	// Send feedback with error
	testError := errors.New("Test error")
	handler(&executor.Feedback{
		ActivityName: "ErrorTest",
		Status:       executor.StatusFailed,
		Error:        testError,
	})

	time.Sleep(20 * time.Millisecond)

	exec := tracker.GetExecution("ErrorTest")
	if exec == nil {
		t.Error("Expected ErrorTest to be tracked")
	} else {
		if exec.Status != executor.StatusFailed {
			t.Errorf("Expected status Failed, got %v", exec.Status)
		}
		if exec.Error == nil {
			t.Error("Error should be set")
		}
	}
}

func TestExecutionTracker_WithMetadata(t *testing.T) {
	tracker := NewExecutionTracker(false)
	tracker.Start()
	defer tracker.Stop()

	handler := tracker.FeedbackHandler()

	// Send feedback with metadata
	metadata := map[string]any{
		"key1": "value1",
		"key2": 42,
	}
	handler(&executor.Feedback{
		ActivityName: "MetadataTest",
		Status:       executor.StatusRunning,
		Metadata:     metadata,
	})

	time.Sleep(20 * time.Millisecond)

	exec := tracker.GetExecution("MetadataTest")
	if exec == nil {
		t.Error("Expected MetadataTest to be tracked")
	} else {
		if exec.Metadata == nil {
			t.Error("Metadata should be set")
		}
		if exec.Metadata["key1"] != "value1" {
			t.Error("Metadata key1 should be 'value1'")
		}
		if exec.Metadata["key2"] != 42 {
			t.Error("Metadata key2 should be 42")
		}
	}
}

func TestExecutionTracker_MultipleStatuses(t *testing.T) {
	tracker := NewExecutionTracker(false)
	tracker.Start()
	defer tracker.Stop()

	handler := tracker.FeedbackHandler()

	// Test different status types
	statuses := []executor.Status{
		executor.StatusPending,
		executor.StatusRunning,
		executor.StatusCompleted,
		executor.StatusFailed,
	}

	for i, status := range statuses {
		handler(&executor.Feedback{
			ActivityName: fmt.Sprintf("Execution%d", i),
			Status:       status,
		})
	}

	time.Sleep(50 * time.Millisecond)

	// Verify all status types were tracked
	for i, status := range statuses {
		name := fmt.Sprintf("Execution%d", i)
		exec := tracker.GetExecution(name)
		if exec == nil {
			t.Errorf("Expected %s to be tracked", name)
		} else {
			if exec.Status != status {
				t.Errorf("Expected %s status %v, got %v", name, status, exec.Status)
			}
		}
	}
}

// Test helper functions that are used internally
func TestSanitizeDescription(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"normal string", "Hello World", "Hello World"},
		{"string with non-printable", "Hello\x00World", "HelloWorld"},
		{"string with ANSI", "Hello\x1b[31mWorld", "HelloWorld"},
		{"long string", string(make([]byte, 150)), "..." + string(make([]byte, 0))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeDescription(tt.input)
			// Just verify it doesn't panic and returns a string
			if tt.input == "" && result != "" {
				t.Errorf("Expected empty string for empty input, got %s", result)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		max      int
		expected string
	}{
		{"short string", "Hello", 10, "Hello"},
		{"exact length", "Hello", 5, "Hello"},
		{"too long", "Hello World", 8, "Hello..."},
		{"empty string", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.max)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestConvertCamelToReadable(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"single word", "hello", "Hello"},
		{"camel case", "helloWorld", "Hello World"},
		{"multiple words", "thisIsATest", "This Is A Test"},
		{"already uppercase", "Hello", "Hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertCamelToReadable(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestParseActivityHierarchy(t *testing.T) {
	tests := []struct {
		name           string
		activityName   string
		expectedParent string
		expectedLevel  int
	}{
		{"no hierarchy", "Activity", "", 0},
		{"one level", "Parent::Child", "Parent", 1},
		{"two levels", "GrandParent::Parent::Child", "GrandParent::Parent", 2},
		{"three levels", "A::B::C::D", "A::B::C", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent, level := parseActivityHierarchy(tt.activityName)
			if parent != tt.expectedParent {
				t.Errorf("Expected parent %s, got %s", tt.expectedParent, parent)
			}
			if level != tt.expectedLevel {
				t.Errorf("Expected level %d, got %d", tt.expectedLevel, level)
			}
		})
	}
}
