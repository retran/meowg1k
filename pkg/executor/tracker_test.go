// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"errors"
	"testing"
	"time"
)

func TestNewBubbleTeaTracker(t *testing.T) {
	tracker := NewBubbleTeaTracker(false)
	if tracker == nil {
		t.Fatal("NewBubbleTeaTracker returned nil")
	}
	if tracker.GetExecutionCount() != 0 {
		t.Error("tracker should start with no executions")
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
	tracker := NewBubbleTeaTracker(true)
	tracker.Start()
	time.Sleep(20 * time.Millisecond)
	tracker.Stop()
}

func TestBubbleTeaTracker_FeedbackHandler(t *testing.T) {
	tracker := NewBubbleTeaTracker(true)
	handler := tracker.FeedbackHandler()
	if handler == nil {
		t.Error("FeedbackHandler should not return nil")
	}
}

func TestBubbleTeaTracker_WithFeedback(t *testing.T) {
	tracker := NewBubbleTeaTracker(true)
	tracker.Start()
	defer tracker.Stop()

	handler := tracker.FeedbackHandler()
	handler(&Feedback{
		ActivityName: "TestExecution",
		Status:       StatusRunning,
		Message:      "Reading file",
		Timestamp:    time.Now(),
	})

	time.Sleep(20 * time.Millisecond)

	if tracker.GetExecutionCount() == 0 {
		t.Error("Expected execution to be tracked")
	}
}

func TestBubbleTeaTracker_StatusTransitions(t *testing.T) {
	tracker := NewBubbleTeaTracker(true)
	tracker.Start()
	defer tracker.Stop()

	handler := tracker.FeedbackHandler()
	handler(&Feedback{
		ActivityName: "TransitionTest",
		Status:       StatusRunning,
		Message:      "Running",
		Timestamp:    time.Now(),
	})

	time.Sleep(10 * time.Millisecond)

	handler(&Feedback{
		ActivityName: "TransitionTest",
		Status:       StatusCompleted,
		Message:      "Done",
		Timestamp:    time.Now(),
	})

	time.Sleep(10 * time.Millisecond)

	if tracker.GetExecutionCount() == 0 {
		t.Error("Expected execution to be tracked after completion")
	}
}

func TestBubbleTeaTracker_WithError(t *testing.T) {
	tracker := NewBubbleTeaTracker(true)
	tracker.Start()
	defer tracker.Stop()

	handler := tracker.FeedbackHandler()
	handler(&Feedback{
		ActivityName: "ErrorTest",
		Status:       StatusFailed,
		Error:        errors.New("test error"),
		Timestamp:    time.Now(),
	})

	time.Sleep(10 * time.Millisecond)

	if tracker.GetExecutionCount() == 0 {
		t.Error("Expected execution to be tracked")
	}
}

func TestProgressTracker_Interface(t *testing.T) {
	var _ ProgressTracker = (*BubbleTeaTracker)(nil)
}

func TestNewProgressTracker(t *testing.T) {
	tracker := NewProgressTracker(true)
	if tracker == nil {
		t.Fatal("NewProgressTracker returned nil")
	}
	if tracker.GetExecutionCount() != 0 {
		t.Error("New tracker should have no executions")
	}
}
