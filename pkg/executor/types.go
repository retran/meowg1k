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

// Package executor provides the core components for running activities and flows.
package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/retran/meowg1k/pkg/future"
)

// Status represents the current status of an activity.
type Status string

const (
	// StatusPending indicates that the activity is pending.
	StatusPending Status = "pending"
	// StatusStarted indicates that the activity has started.
	StatusStarted Status = "started"
	// StatusRunning indicates that the activity is running.
	StatusRunning Status = "running"
	// StatusCompleted indicates that the activity has completed.
	StatusCompleted Status = "completed"
	// StatusFailed indicates that the activity has failed.
	StatusFailed Status = "failed"
)

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
	return &Context{
		name:         name,
		feedbackFunc: feedbackFunc,
		Executor:     executor,
	}
}

// Name returns the name of the activity.
func (c *Context) Name() string {
	return c.name
}

// GetExecutor returns the executor associated with the context.
func (c *Context) GetExecutor() Executor {
	return c.Executor
}

func (c *Context) sendFeedback(status Status, progress float64, message string, err error, metadata map[string]any) {
	if c.feedbackFunc == nil {
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

// SendPending sends a pending status update.
func (c *Context) SendPending(message string) {
	c.sendFeedback(StatusPending, 0, message, nil, nil)
}

// SendStarted sends a started status update.
func (c *Context) SendStarted(message string) {
	c.sendFeedback(StatusStarted, 0, message, nil, nil)
}

// SendProgress sends a progress update.
func (c *Context) SendProgress(progress float64, message string) {
	c.sendFeedback(StatusRunning, progress, message, nil, nil)
}

// SendCompleted sends a completed status update.
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

// Flow defines a function that can be executed by the executor.
type Flow func(ctx context.Context, flowCtx *Context) error

// Executor defines the interface for executing flows and activities.
type Executor interface {
	RunActivity(
		ctx context.Context,
		parentCtx *Context,
		name string,
		activity Activity[any, any],
		input any,
	) *future.Future[any]
	RunFlow(ctx context.Context, name string, flow Flow, retryPolicy *RetryPolicy) error
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
