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

package executor

import (
	"errors"
	"testing"
)

func TestContext_Name_NilContext(t *testing.T) {
	var ctx *Context
	name := ctx.Name()
	if name != "" {
		t.Errorf("expected empty string for nil context, got %q", name)
	}
}

func TestContext_GetExecutor_NilContext(t *testing.T) {
	var ctx *Context
	exec := ctx.GetExecutor()
	if exec != nil {
		t.Error("expected nil executor for nil context")
	}
}

func TestNewContext_WithNilFeedbackHandler(t *testing.T) {
	executor := NewExecutor(0)
	ctx := NewContext("test", nil, executor)

	if ctx == nil {
		t.Fatal("NewContext returned nil")
	}

	if ctx.Name() != "test" {
		t.Errorf("expected name 'test', got %q", ctx.Name())
	}

	if ctx.GetExecutor() != executor {
		t.Error("executor mismatch")
	}

	// Should not panic when sending feedback with nil handler replaced by NoOp
	ctx.SendRunning("test")
	ctx.SendCompleted("done")
}

func TestContext_SendFeedback(t *testing.T) {
	var received *Feedback
	handler := func(f *Feedback) {
		received = f
	}

	ctx := NewContext("test-activity", handler, nil)

	ctx.SendRunning("running message")
	if received == nil {
		t.Fatal("feedback not received")
	}
	if received.ActivityName != "test-activity" {
		t.Errorf("expected activity name 'test-activity', got %q", received.ActivityName)
	}
	if received.Status != StatusRunning {
		t.Errorf("expected status Running, got %v", received.Status)
	}
	if received.Message != "running message" {
		t.Errorf("expected message 'running message', got %q", received.Message)
	}

	ctx.SendCompleted("completed message")
	if received.Status != StatusCompleted {
		t.Errorf("expected status Completed, got %v", received.Status)
	}
	if received.Message != "completed message" {
		t.Errorf("expected message 'completed message', got %q", received.Message)
	}

	testErr := errors.New("test error")
	ctx.SendFailed(testErr, "error message")
	if received.Status != StatusFailed {
		t.Errorf("expected status Failed, got %v", received.Status)
	}
	if received.Message != "error message" {
		t.Errorf("expected message 'error message', got %q", received.Message)
	}
	if received.Error != testErr {
		t.Errorf("expected error to be set")
	}
}

func TestFeedback_String(t *testing.T) {
	tests := []struct {
		name     string
		feedback *Feedback
		expected string
	}{
		{
			name: "running status",
			feedback: &Feedback{
				ActivityName: "TestActivity",
				Status:       StatusRunning,
				Message:      "processing",
			},
			expected: "[TestActivity] running: processing",
		},
		{
			name: "completed status",
			feedback: &Feedback{
				ActivityName: "TestActivity",
				Status:       StatusCompleted,
				Message:      "done",
			},
			expected: "[TestActivity] completed: done",
		},
		{
			name: "failed status",
			feedback: &Feedback{
				ActivityName: "TestActivity",
				Status:       StatusFailed,
				Message:      "error occurred",
			},
			expected: "[TestActivity] failed: error occurred",
		},
		{
			name: "empty message",
			feedback: &Feedback{
				ActivityName: "TestActivity",
				Status:       StatusRunning,
				Message:      "",
			},
			expected: "[TestActivity] running: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.feedback.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
