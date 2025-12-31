// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"fmt"
	"time"
)

// Status represents the current status of an activity.
type Status string

const (
	// StatusRunning indicates that the activity is running.
	StatusRunning Status = "running"
	// StatusProgress indicates that the activity has a progress update.
	StatusProgress Status = "progress"
	// StatusCompleted indicates that the activity has completed.
	StatusCompleted Status = "completed"
	// StatusFailed indicates that the activity has failed.
	StatusFailed Status = "failed"
)

// DefaultRetryPolicy returns a sensible default retry policy.
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
	}
}

// NoRetryPolicy returns a policy that doesn't retry.
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
	Timestamp    time.Time `json:"timestamp"`
	Error        error     `json:"-"`
	ActivityName string    `json:"activity_name"`
	Status       Status    `json:"status"`
	Message      string    `json:"message"`
	Details      string    `json:"details,omitempty"`
	Progress     float64   `json:"progress"`
}

// String returns a string representation of the feedback.
func (f *Feedback) String() string {
	if f == nil {
		return ""
	}

	if f.Error != nil {
		return fmt.Sprintf("[%s] %s: %s (%v)", f.ActivityName, f.Status, f.Message, f.Error)
	}

	return fmt.Sprintf("[%s] %s: %s", f.ActivityName, f.Status, f.Message)
}

// FeedbackHandler processes feedback from activities.
type FeedbackHandler func(feedback *Feedback)

// NoOpFeedbackHandler is a feedback handler that does nothing.
func NoOpFeedbackHandler(_ *Feedback) {}

// Context provides feedback capabilities to activities and access to the executor.
type Context struct {
	Executor     Executor
	feedbackFunc FeedbackHandler
	name         string
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

// Child returns a new context that reports feedback under the current activity.
func (c *Context) Child(name string) *Context {
	if c == nil {
		return nil
	}
	if name == "" {
		name = "child"
	}
	fullName := name
	if c.name != "" {
		fullName = fmt.Sprintf("%s::%s", c.name, name)
	}
	return &Context{
		name:         fullName,
		feedbackFunc: c.feedbackFunc,
		Executor:     c.Executor,
	}
}

// GetExecutor returns the executor associated with the context.
func (c *Context) GetExecutor() Executor {
	if c == nil {
		return nil
	}
	return c.Executor
}

func (c *Context) sendFeedback(status Status, progress float64, message string, details string, err error) {
	if c == nil || c.feedbackFunc == nil {
		return
	}

	feedback := &Feedback{
		ActivityName: c.name,
		Status:       status,
		Progress:     progress,
		Message:      message,
		Details:      details,
		Timestamp:    time.Now(),
		Error:        err,
	}

	c.feedbackFunc(feedback)
}

// SendRunning sends a running status feedback with the given message.
func (c *Context) SendRunning(message string) {
	c.SendRunningWithDetails(message, "")
}

// SendCompleted sends a completed status feedback with the given message.
func (c *Context) SendCompleted(message string) {
	c.SendCompletedWithDetails(message, "")
}

// SendFailed sends a failed status feedback with the given error and message.
func (c *Context) SendFailed(err error, message string) {
	c.SendFailedWithDetails(err, message, "")
}

// SendRetry sends a retry status feedback with the given attempt number and error.
func (c *Context) SendRetry(attempt int, err error) {
	c.SendRetryWithDetails("I'm retrying the operation", "", attempt, err)
}

// SendRunningWithDetails sends a running status feedback with the given message and details.
func (c *Context) SendRunningWithDetails(message string, details string) {
	c.sendFeedback(StatusRunning, 0, message, details, nil)
}

// SendCompletedWithDetails sends a completed status feedback with the given message and details.
func (c *Context) SendCompletedWithDetails(message string, details string) {
	c.sendFeedback(StatusCompleted, 1, message, details, nil)
}

// SendFailedWithDetails sends a failed status feedback with the given error, message, and details.
func (c *Context) SendFailedWithDetails(err error, message string, details string) {
	c.sendFeedback(StatusFailed, 0, message, details, err)
}

// SendRetryWithDetails sends a retry status feedback with the given message, details, attempt number, and error.
func (c *Context) SendRetryWithDetails(message string, details string, attempt int, err error) {
	_ = attempt
	c.sendFeedback(StatusRunning, 0, message, details, err)
}

// SendProgress sends a running status feedback intended to be logged as progress.
func (c *Context) SendProgress(message string) {
	c.SendProgressWithDetails(message, "")
}

// SendProgressWithDetails sends a running status feedback intended to be logged as progress.
func (c *Context) SendProgressWithDetails(message string, details string) {
	c.sendFeedback(StatusProgress, 0, message, details, nil)
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
