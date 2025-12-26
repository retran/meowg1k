// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestNewBubbleTeaTracker(t *testing.T) {
	tracker := NewBubbleTeaTracker(false)
	if tracker == nil {
		t.Fatal("NewBubbleTeaTracker returned nil")
	}
	if tracker.GetExecutionCount() != 0 {
		t.Error("executions map should be initialized and empty")
	}
	if tracker.silent {
		t.Error("silent should be false")
	}
}

func TestNewBubbleTeaTrackerSilent(t *testing.T) {
	tracker := NewBubbleTeaTracker(true)
	if tracker == nil {
		t.Fatal("NewBubbleTeaTracker returned nil")
	}
	if !tracker.silent {
		t.Error("silent should be true")
	}
}

func TestBubbleTeaTracker_StartStop(t *testing.T) {
	tracker := NewBubbleTeaTracker(true) // Use silent mode to avoid TUI

	tracker.Start()
	time.Sleep(50 * time.Millisecond)
	tracker.Stop()

	// Should complete without deadlock
}

func TestBubbleTeaTracker_FeedbackHandler(t *testing.T) {
	tracker := NewBubbleTeaTracker(true) // Use silent mode

	handler := tracker.FeedbackHandler()
	if handler == nil {
		t.Error("FeedbackHandler should not return nil")
	}
}

func TestBubbleTeaTracker_WithFeedback(t *testing.T) {
	tracker := NewBubbleTeaTracker(true) // Use silent mode to avoid TUI
	tracker.Start()
	defer tracker.Stop()

	handler := tracker.FeedbackHandler()

	// Send various feedback messages
	feedback := &Feedback{
		ActivityName: "TestExecution",
		Status:       StatusRunning,
		Message:      "Test message",
		Timestamp:    time.Now(),
	}

	handler(feedback)

	// Give time for processing
	time.Sleep(50 * time.Millisecond)

	// Verify the execution was tracked
	if tracker.GetExecutionCount() == 0 {
		t.Error("Expected execution to be tracked")
	}

	exec := tracker.GetExecution("TestExecution")
	if exec == nil {
		t.Error("Expected TestExecution to be tracked")
		return
	}
	if exec.Name != "TestExecution" {
		t.Errorf("Expected name TestExecution, got %s", exec.Name)
	}
	if exec.Status != StatusRunning {
		t.Errorf("Expected status Running, got %v", exec.Status)
	}
	if exec.Message != "Test message" {
		t.Errorf("Expected message 'Test message', got %s", exec.Message)
	}
}

func TestBubbleTeaTracker_MultipleExecutions(t *testing.T) {
	tracker := NewBubbleTeaTracker(true) // Use silent mode
	tracker.Start()
	defer tracker.Stop()

	handler := tracker.FeedbackHandler()

	// Send feedback for multiple executions
	executions := []struct {
		name    string
		status  Status
		message string
	}{
		{"Execution1", StatusRunning, "Running 1"},
		{"Execution2", StatusRunning, "Running 2"},
		{"Execution3", StatusCompleted, "Completed 3"},
	}

	now := time.Now()
	for _, exec := range executions {
		feedback := &Feedback{
			ActivityName: exec.name,
			Status:       exec.status,
			Message:      exec.message,
			Timestamp:    now,
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
		} else if tracked.Status != exec.status {
			t.Errorf("Expected %s status %v, got %v", exec.name, exec.status, tracked.Status)
		}
	}
}

func TestBubbleTeaTracker_StatusTransitions(t *testing.T) {
	tracker := NewBubbleTeaTracker(true) // Use silent mode
	tracker.Start()
	defer tracker.Stop()

	handler := tracker.FeedbackHandler()

	// Test status transition from Running to Completed
	handler(&Feedback{
		ActivityName: "TransitionTest",
		Status:       StatusRunning,
		Message:      "Starting",
		Timestamp:    time.Now(),
	})

	time.Sleep(20 * time.Millisecond)

	handler(&Feedback{
		ActivityName: "TransitionTest",
		Status:       StatusCompleted,
		Message:      "Done",
		Timestamp:    time.Now(),
	})

	time.Sleep(20 * time.Millisecond)

	exec := tracker.GetExecution("TransitionTest")
	if exec == nil {
		t.Error("Expected TransitionTest to be tracked")
		return
	}

	if exec.Status != StatusCompleted {
		t.Errorf("Expected status Completed, got %v", exec.Status)
	}
	if exec.Result != "Done" {
		t.Errorf("Expected result 'Done', got %s", exec.Result)
	}
	if exec.EndTime == nil {
		t.Error("EndTime should be set for completed execution")
	}
}

func TestBubbleTeaTracker_WithError(t *testing.T) {
	tracker := NewBubbleTeaTracker(true) // Use silent mode
	tracker.Start()
	defer tracker.Stop()

	handler := tracker.FeedbackHandler()

	// Send feedback with error
	testError := errors.New("test error")
	handler(&Feedback{
		ActivityName: "ErrorTest",
		Status:       StatusFailed,
		Error:        testError,
		Timestamp:    time.Now(),
	})

	time.Sleep(20 * time.Millisecond)

	exec := tracker.GetExecution("ErrorTest")
	if exec == nil {
		t.Error("Expected ErrorTest to be tracked")
	} else {
		if exec.Status != StatusFailed {
			t.Errorf("Expected status Failed, got %v", exec.Status)
		}
		if exec.Error == nil {
			t.Error("Error should be set")
		}
	}
}

func TestBubbleTeaTracker_WithMetadata(t *testing.T) {
	tracker := NewBubbleTeaTracker(true) // Use silent mode
	tracker.Start()
	defer tracker.Stop()

	handler := tracker.FeedbackHandler()

	// Send feedback with metadata
	metadata := map[string]any{
		"key1": "value1",
		"key2": 42,
	}
	handler(&Feedback{
		ActivityName: "MetadataTest",
		Status:       StatusRunning,
		Metadata:     metadata,
		Timestamp:    time.Now(),
	})

	time.Sleep(20 * time.Millisecond)

	exec := tracker.GetExecution("MetadataTest")
	if exec == nil {
		t.Error("Expected MetadataTest to be tracked")
		return
	}
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

func TestBubbleTeaTracker_MultipleStatuses(t *testing.T) {
	tracker := NewBubbleTeaTracker(true) // Use silent mode
	tracker.Start()
	defer tracker.Stop()

	handler := tracker.FeedbackHandler()

	// Test different status types
	statuses := []Status{
		StatusPending,
		StatusRunning,
		StatusCompleted,
		StatusFailed,
	}

	now := time.Now()
	for i, status := range statuses {
		handler(&Feedback{
			ActivityName: fmt.Sprintf("Execution%d", i),
			Status:       status,
			Timestamp:    now,
		})
	}

	time.Sleep(50 * time.Millisecond)

	// Verify all status types were tracked
	for i, status := range statuses {
		name := fmt.Sprintf("Execution%d", i)
		exec := tracker.GetExecution(name)
		if exec == nil {
			t.Errorf("Expected %s to be tracked", name)
		} else if exec.Status != status {
			t.Errorf("Expected %s status %v, got %v", name, status, exec.Status)
		}
	}
}

func TestBubbleTeaTracker_HierarchicalExecutions(t *testing.T) {
	tracker := NewBubbleTeaTracker(true) // Use silent mode
	tracker.Start()
	defer tracker.Stop()

	handler := tracker.FeedbackHandler()

	// Send hierarchical executions
	handler(&Feedback{
		ActivityName: "Parent",
		Status:       StatusRunning,
		Timestamp:    time.Now(),
	})

	handler(&Feedback{
		ActivityName: "Parent::Child1",
		Status:       StatusRunning,
		Timestamp:    time.Now(),
	})

	handler(&Feedback{
		ActivityName: "Parent::Child2",
		Status:       StatusCompleted,
		Timestamp:    time.Now(),
	})

	time.Sleep(50 * time.Millisecond)

	// Verify hierarchy
	parent := tracker.GetExecution("Parent")
	if parent == nil {
		t.Error("Expected Parent to be tracked")
		return
	}
	if parent.Level != 0 {
		t.Errorf("Expected Parent level 0, got %d", parent.Level)
	}

	child1 := tracker.GetExecution("Parent::Child1")
	if child1 == nil {
		t.Error("Expected Parent::Child1 to be tracked")
	} else {
		if child1.Level != 1 {
			t.Errorf("Expected Child1 level 1, got %d", child1.Level)
		}
		if child1.ParentName != "Parent" {
			t.Errorf("Expected Child1 parent 'Parent', got %s", child1.ParentName)
		}
	}
}

func TestProgressTracker_Interface(t *testing.T) {
	// Test that BubbleTeaTracker implements ProgressTracker
	var _ ProgressTracker = NewBubbleTeaTracker(true)
}

func TestNewProgressTracker(t *testing.T) {
	// Test factory function
	tracker := NewProgressTracker(true)
	if tracker == nil {
		t.Fatal("NewProgressTracker returned nil")
	}

	// Verify basic functionality
	if tracker.GetExecutionCount() != 0 {
		t.Error("New tracker should have no executions")
	}
}
