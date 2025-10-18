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
	"context"
	"fmt"
	"time"
)

// Status represents the current status of an activity.
type Status string

const (
	// StatusPending indicates that the activity is pending.
	StatusPending Status = "pending"
	// StatusRunning indicates that the activity is running.
	StatusRunning Status = "running"
	// StatusCompleted indicates that the activity has completed.
	StatusCompleted Status = "completed"
	// StatusFailed indicates that the activity has failed.
	StatusFailed Status = "failed"
)

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
	}
}

// NoRetryPolicy returns a policy that doesn't retry
func NoRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:  1,
		InitialDelay: 0,
		MaxDelay:     0,
		Multiplier:   1.0,
	}
}

// Feedback contains information about activity execution progress.
type Feedback struct {
	ActivityName string         `json:"activity_name"`
	Status       Status         `json:"status"`
	Progress     float64        `json:"progress"` // 0.0 to 1.0
	Message      string         `json:"message"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	Timestamp    time.Time      `json:"timestamp"`
	Error        error          `json:"-"`
}

// String returns a string representation of the feedback.
func (f *Feedback) String() string {
	if f == nil {
		return ""
	}

	if f.Error != nil {
		if f.Progress > 0 {
			return fmt.Sprintf(
				"[%s] %s: %s (%.1f%%) (%v)",
				f.ActivityName, f.Status, f.Message, f.Progress*100, f.Error,
			)
		}

		return fmt.Sprintf("[%s] %s: %s (%v)", f.ActivityName, f.Status, f.Message, f.Error)
	}

	if f.Progress > 0 {
		return fmt.Sprintf("[%s] %s: %s (%.1f%%)", f.ActivityName, f.Status, f.Message, f.Progress*100)
	}

	return fmt.Sprintf("[%s] %s: %s", f.ActivityName, f.Status, f.Message)
}

// FeedbackHandler processes feedback from activities.
type FeedbackHandler func(feedback *Feedback)

// NoOpFeedbackHandler is a feedback handler that does nothing.
func NoOpFeedbackHandler(feedback *Feedback) {}

// Context provides feedback capabilities to activities and access to the executor.
type Context struct {
	name         string
	feedbackFunc FeedbackHandler
	Executor     Executor // Interface for running sub-activities
}

// NewContext creates a new executor context.
func NewContext(name string, feedbackFunc FeedbackHandler, executor Executor) *Context {
	if feedbackFunc == nil {
		feedbackFunc = NoOpFeedbackHandler
	}
	return &Context{
		name:         name,
		feedbackFunc: feedbackFunc,
		Executor:     executor,
	}
}

// Name returns the name of the activity.
func (c *Context) Name() string {
	if c == nil {
		return ""
	}
	return c.name
}

// GetExecutor returns the executor associated with the context.
func (c *Context) GetExecutor() Executor {
	if c == nil {
		return nil
	}
	return c.Executor
}

func (c *Context) sendFeedback(status Status, progress float64, message string, err error, metadata map[string]any) {
	if c == nil || c.feedbackFunc == nil {
		return
	}

	feedback := &Feedback{
		ActivityName: c.name,
		Status:       status,
		Progress:     progress,
		Message:      message,
		Timestamp:    time.Now(),
		Error:        err,
		Metadata:     metadata,
	}

	c.feedbackFunc(feedback)
}

// SendRunning sends a running status update to indicate that the activity is executing.
// Use this at the beginning of your activity execution.
//
// Docker Compose Style Usage:
// Activities should send concise, action-oriented messages.
// Examples:
//   - "Reading 5 files"
//   - "Generating content"
//   - "Processing data"
func (c *Context) SendRunning(message string) {
	c.sendFeedback(StatusRunning, 0, message, nil, nil)
}

// SendCompleted sends a completed status update.
// Use brief messages that indicate what was accomplished.
// Example: "Read 5 files", "Generated", "Summarized" (not "Successfully completed...").
func (c *Context) SendCompleted(message string) {
	c.sendFeedback(StatusCompleted, 1, message, nil, nil)
}

// SendFailed sends a failed status update.
func (c *Context) SendFailed(err error, message string) {
	c.sendFeedback(StatusFailed, 0, message, err, nil)
}

// SendRetry sends a retry status update.
func (c *Context) SendRetry(attempt int, err error) {
	c.sendFeedback(StatusRunning, 0, fmt.Sprintf("Retrying (%d)", attempt), err, map[string]any{
		"retry_attempt": attempt,
	})
}

// Activity defines a function that can be executed by the executor.
type Activity[T any, K any] func(ctx context.Context, activityCtx *Context, input T) (K, error)

// ActivityFactory creates new instances of activities with specific input and output types.
type ActivityFactory[T any, K any] interface {
	NewActivity() Activity[T, K]
}

// Flow defines a function that can be executed by the executor.
type Flow func(ctx context.Context, flowCtx *Context) error

// Execution represents the progress state of a single execution.
type Execution struct {
	Name       string
	Status     Status
	Message    string
	Result     string
	StartTime  time.Time
	EndTime    *time.Time
	Error      error
	Metadata   map[string]any
	ParentName string
	Children   []string
	Level      int
}

func (e *Execution) getDurationString() string {
	if e.EndTime == nil {
		return ""
	}
	duration := e.EndTime.Sub(e.StartTime)
	switch {
	case duration < time.Second:
		return fmt.Sprintf("%dms", duration.Milliseconds())
	case duration < time.Minute:
		return fmt.Sprintf("%.1fs", duration.Seconds())
	default:
		return fmt.Sprintf("%.1fm", duration.Minutes())
	}
}

// RetryPolicy defines the retry behavior for an operation.
type RetryPolicy struct {
	// MaxAttempts is the maximum number of retry attempts.
	MaxAttempts int

	// InitialDelay is the initial time to wait between attempts.
	InitialDelay time.Duration

	// MaxDelay is the maximum time to wait between attempts.
	MaxDelay time.Duration

	// Multiplier is the factor by which the delay is multiplied after each attempt.
	Multiplier float64
}
