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

	"github.com/retran/meowg1k/pkg/future"
)

// Status represents the current status of an activity
type Status string

const (
	StatusPending   Status = "pending"
	StatusStarted   Status = "started"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

// Feedback contains information about activity execution progress
type Feedback struct {
	ActivityName string         `json:"activity_name"`
	Status       Status         `json:"status"`
	Progress     float64        `json:"progress"` // 0.0 to 1.0
	Message      string         `json:"message"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	Timestamp    time.Time      `json:"timestamp"`
	Error        error          `json:"error,omitempty"`
}

// FeedbackHandler processes feedback from activities
type FeedbackHandler func(feedback Feedback)

// ExecutorContext provides feedback capabilities to activities and access to the executor
type ExecutorContext struct {
	name         string
	feedbackFunc FeedbackHandler
	executor     Executor // Interface for running sub-activities
}

// Executor defines the interface for executing flows and activities
type Executor interface {
	RunFlow(ctx context.Context, flowName string, flow func(context.Context, *ExecutorContext) error) error
	RunActivity(ctx context.Context, parentCtx *ExecutorContext, activityName string, activity any, input any) *future.Future[any]
}

// NewExecutorContext creates a new activity context with executor access
func NewExecutorContext(activityName string, handler FeedbackHandler, executor Executor) *ExecutorContext {
	return &ExecutorContext{
		name:         activityName,
		feedbackFunc: handler,
		executor:     executor,
	}
}

func (e *ExecutorContext) GetExecutor() Executor {
	return e.executor
}

// sendFeedback sends feedback about activity execution
func (e *ExecutorContext) sendFeedback(status Status, progress float64, message string) {
	if e.feedbackFunc == nil {
		return
	}

	e.feedbackFunc(Feedback{
		ActivityName: e.name,
		Status:       status,
		Progress:     progress,
		Message:      message,
		Timestamp:    time.Now(),
	})
}

func (e *ExecutorContext) SendPending(message string) {
	e.sendFeedback(StatusPending, 0.0, message)
}

func (e *ExecutorContext) SendStarted(message string) {
	e.sendFeedback(StatusStarted, 0.0, message)
}

func (e *ExecutorContext) SendProgress(progress float64, message string) {
	e.sendFeedback(StatusRunning, progress, message)
}

func (e *ExecutorContext) SendCompleted(message string) {
	e.sendFeedback(StatusCompleted, 1.0, message)
}

func (e *ExecutorContext) SendFailed(err error, message string) {
	if e.feedbackFunc == nil {
		return
	}

	e.feedbackFunc(Feedback{
		ActivityName: e.name,
		Status:       StatusFailed,
		Progress:     0.0,
		Message:      message,
		Error:        err,
		Timestamp:    time.Now(),
	})
}

func (e *ExecutorContext) SendRetry(attempt int, err error) {
	if e.feedbackFunc == nil {
		return
	}

	e.feedbackFunc(Feedback{
		ActivityName: e.name,
		Status:       StatusRunning,
		Progress:     0.0,
		Message:      fmt.Sprintf("Retrying attempt %d", attempt),
		Error:        err,
		Metadata: map[string]any{
			"retry_attempt": attempt,
		},
		Timestamp: time.Now(),
	})
}

// Flow represents a sequence of activities that produces a result
// It's just a function that takes standard context and activity context
type Flow[O any] func(ctx context.Context, activityCtx *ExecutorContext) (O, error)

// Activity represents a reusable operation
// It's just a function that takes standard context and activity context
type Activity[I any, O any] func(ctx context.Context, activityCtx *ExecutorContext, input I) (O, error)

// NoOpFeedbackHandler is a feedback handler that does nothing
func NoOpFeedbackHandler(feedback Feedback) {
	// Do nothing
}
