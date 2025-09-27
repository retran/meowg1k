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

func TestGetActivityDuration(t *testing.T) {
	tracker := &ExecutionTracker{}

	start := time.Now().Add(-5 * time.Second)
	activity := &ExecutionProgress{
		StartTime: start,
	}

	duration := tracker.getActivityDuration(activity)
	if !strings.Contains(duration, "5.0s") {
		t.Errorf("expected duration to contain '5.0s', got %s", duration)
	}
}
