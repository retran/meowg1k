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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/retran/meowg1k/pkg/executor"
)

func TestSanitizeDescription(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal text", "normal text"},
		{"text with \x1b[31mcolor\x1b[0m", "text with [31mcolor[0m"},
		{"", ""},
		{strings.Repeat("a", 200), strings.Repeat("a", 97) + "..."},
	}

	for _, test := range tests {
		result := sanitizeDescription(test.input)
		if result != test.expected {
			t.Errorf("sanitizeDescription(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestParseActivityHierarchy(t *testing.T) {
	tracker := &ExecutionTracker{}

	tests := []struct {
		input          string
		expectedParent string
		expectedLevel  int
	}{
		{"root", "", 0},
		{"root.child", "root", 1},
		{"root.child.grandchild", "root.child", 2},
		{"a.b.c.d", "a.b.c", 3},
	}

	for _, test := range tests {
		parent, level := tracker.parseActivityHierarchy(test.input)
		if parent != test.expectedParent || level != test.expectedLevel {
			t.Errorf("parseActivityHierarchy(%q) = (%q, %d), expected (%q, %d)", test.input, parent, level, test.expectedParent, test.expectedLevel)
		}
	}
}

func TestNewExecutionTracker(t *testing.T) {
	tracker := NewExecutionTracker(false)
	if tracker == nil {
		t.Error("expected tracker to be created")
	}
	if tracker.silent {
		t.Error("expected silent to be false")
	}
	if len(tracker.spinnerChars) == 0 {
		t.Error("expected spinner chars to be set")
	}
	if tracker.ticker == nil {
		t.Error("expected ticker to be set when not silent")
	}
}

func TestNewExecutionTrackerSilent(t *testing.T) {
	tracker := NewExecutionTracker(true)
	if !tracker.silent {
		t.Error("expected silent to be true")
	}
	if tracker.ticker != nil {
		t.Error("expected no ticker when silent")
	}
}

func TestFeedbackHandler(t *testing.T) {
	tracker := NewExecutionTracker(false) // not silent
	defer tracker.Stop()                  // stop to clean up
	handler := tracker.FeedbackHandler()

	feedback := executor.Feedback{
		ActivityName: "test",
		Status:       executor.StatusStarted,
		Message:      "starting",
		Timestamp:    time.Now(),
	}

	handler(feedback)

	tracker.mu.RLock()
	defer tracker.mu.RUnlock()

	activity, exists := tracker.executions["test"]
	if !exists {
		t.Error("expected activity to be created")
	}
	if activity.Status != executor.StatusStarted {
		t.Error("expected status to be started")
	}
}

func TestUpdateActivity(t *testing.T) {
	tracker := NewExecutionTracker(false)
	defer tracker.Stop()

	feedback := executor.Feedback{
		ActivityName: "test",
		Status:       executor.StatusCompleted,
		Message:      "done",
		Timestamp:    time.Now(),
		Progress:     1.0,
	}

	tracker.UpdateActivity(feedback)

	tracker.mu.RLock()
	defer tracker.mu.RUnlock()

	activity, exists := tracker.executions["test"]
	if !exists {
		t.Error("expected activity to be created")
	}
	if activity.Status != executor.StatusCompleted {
		t.Error("expected status to be completed")
	}
	if activity.Progress != 1.0 {
		t.Error("expected progress to be 1.0")
	}
	if activity.EndTime == nil {
		t.Error("expected end time to be set")
	}
}

func TestStartAndStop(t *testing.T) {
	tracker := NewExecutionTracker(false)
	
	// Test Start
	if tracker.isRunning {
		t.Error("expected tracker to not be running initially")
	}
	
	tracker.Start()
	
	if !tracker.isRunning {
		t.Error("expected tracker to be running after Start()")
	}
	
	// Test Stop
	tracker.Stop()
	
	if tracker.isRunning {
		t.Error("expected tracker to not be running after Stop()")
	}
}

func TestUpdateActivitySilentMode(t *testing.T) {
	tracker := NewExecutionTracker(true) // silent mode
	
	feedback := executor.Feedback{
		ActivityName: "test",
		Status:       executor.StatusStarted,
		Message:      "starting",
		Timestamp:    time.Now(),
	}

	// Should not panic in silent mode
	tracker.UpdateActivity(feedback)

	// Activity should not be tracked in silent mode  
	tracker.mu.RLock()
	defer tracker.mu.RUnlock()
	
	_, exists := tracker.executions["test"]
	if exists {
		t.Error("expected activity to not be created in silent mode")
	}
}

func TestGetActivityDuration(t *testing.T) {
	tracker := NewExecutionTracker(false)
	defer tracker.Stop()

	now := time.Now()
	later := now.Add(5 * time.Second)
	
	// Test with completed activity
	activityCompleted := &ExecutionProgress{
		StartTime: now,
		EndTime:   &later,
		Status:    executor.StatusCompleted,
	}
	
	duration := tracker.getActivityDuration(activityCompleted)
	if duration == "" {
		t.Error("expected non-empty duration for completed activity")
	}

	// Test with running activity
	activityRunning := &ExecutionProgress{
		StartTime: now,
		EndTime:   nil,
		Status:    executor.StatusStarted,
	}
	
	duration = tracker.getActivityDuration(activityRunning)
	if duration == "" {
		t.Error("expected non-empty duration for running activity")
	}

	// Test with nil activity
	duration = tracker.getActivityDuration(nil)
	if duration != "0s" {
		t.Errorf("expected '0s' duration for nil activity, got '%s'", duration)
	}
}

func TestHierarchicalActivities(t *testing.T) {
	tracker := NewExecutionTracker(false)
	defer tracker.Stop()

	// Add parent activity
	parentFeedback := executor.Feedback{
		ActivityName: "parent",
		Status:       executor.StatusStarted,
		Message:      "parent started",
		Timestamp:    time.Now(),
	}
	tracker.UpdateActivity(parentFeedback)

	// Add child activity
	childFeedback := executor.Feedback{
		ActivityName: "parent.child",
		Status:       executor.StatusStarted,
		Message:      "child started",
		Timestamp:    time.Now(),
	}
	tracker.UpdateActivity(childFeedback)

	tracker.mu.RLock()
	defer tracker.mu.RUnlock()

	// Check parent activity
	parent, exists := tracker.executions["parent"]
	if !exists {
		t.Error("expected parent activity to exist")
	}
	if parent.Level != 0 {
		t.Errorf("expected parent level to be 0, got %d", parent.Level)
	}

	// Check child activity
	child, exists := tracker.executions["parent.child"]
	if !exists {
		t.Error("expected child activity to exist")
	}
	if child.Level != 1 {
		t.Errorf("expected child level to be 1, got %d", child.Level)
	}
	if child.ParentName != "parent" {
		t.Errorf("expected child parent to be 'parent', got '%s'", child.ParentName)
	}
}

func TestMultipleActivityStatuses(t *testing.T) {
	tracker := NewExecutionTracker(false)
	defer tracker.Stop()

	statuses := []executor.Status{
		executor.StatusStarted,
		executor.StatusCompleted,
		executor.StatusFailed,
	}

	for i, status := range statuses {
		activityName := fmt.Sprintf("activity_%d", i)
		feedback := executor.Feedback{
			ActivityName: activityName,
			Status:       status,
			Message:      fmt.Sprintf("status %s", status),
			Timestamp:    time.Now(),
		}
		tracker.UpdateActivity(feedback)

		tracker.mu.RLock()
		activity, exists := tracker.executions[activityName]
		tracker.mu.RUnlock()

		if !exists {
			t.Errorf("expected activity %s to exist", activityName)
			continue
		}
		if activity.Status != status {
			t.Errorf("expected status %s, got %s", status, activity.Status)
		}
	}
}
