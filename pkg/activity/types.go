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

package activity

import (
	"context"
	"fmt"
	"time"

	"github.com/retran/meowg1k/pkg/future"
)

// Status represents the current status of an activity
type Status string

const (
	StatusStarted   Status = "started"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

// Feedback contains information about activity execution progress
type Feedback struct {
	ActivityName string                 `json:"activity_name"`
	Status       Status                 `json:"status"`
	Progress     float64                `json:"progress"` // 0.0 to 1.0
	Message      string                 `json:"message"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
	Error        error                  `json:"error,omitempty"`
}

// FeedbackHandler processes feedback from activities
type FeedbackHandler func(feedback Feedback)

// ActivityContext provides feedback capabilities to activities and access to the executor
type ActivityContext struct {
	name         string
	feedbackFunc FeedbackHandler
	executor     ActivityExecutor // Interface for running sub-activities
}

// ActivityExecutor defines the interface for executing activities
// This allows ActivityContext to run sub-activities
type ActivityExecutor interface {
	RunActivity(ctx context.Context, activityName string, activity interface{}, input interface{}) *future.Future[interface{}]
	RunSubActivity(parentCtx *ActivityContext, ctx context.Context, activityName string, activity interface{}, input interface{}) *future.Future[interface{}]
}

// NewActivityContext creates a new activity context
func NewActivityContext(activityName string, handler FeedbackHandler) *ActivityContext {
	return &ActivityContext{
		name:         activityName,
		feedbackFunc: handler,
		executor:     nil, // Will be set by executor when needed
	}
}

// NewActivityContextWithExecutor creates a new activity context with executor access
func NewActivityContextWithExecutor(activityName string, handler FeedbackHandler, executor ActivityExecutor) *ActivityContext {
	return &ActivityContext{
		name:         activityName,
		feedbackFunc: handler,
		executor:     executor,
	}
}

// SendFeedback sends feedback about activity execution
func (ac *ActivityContext) SendFeedback(status Status, progress float64, message string) {
	if ac.feedbackFunc == nil {
		return
	}

	ac.feedbackFunc(Feedback{
		ActivityName: ac.name,
		Status:       status,
		Progress:     progress,
		Message:      message,
		Timestamp:    time.Now(),
	})
}

// SendProgress is a convenience method for progress updates
func (ac *ActivityContext) SendProgress(progress float64, message string) {
	ac.SendFeedback(StatusRunning, progress, message)
}

// SendPending is a convenience method for pending status
func (ac *ActivityContext) SendPending(message string) {
	ac.SendFeedback(StatusRunning, 0.0, message)
}

// SendError sends error feedback
func (ac *ActivityContext) SendError(err error, message string) {
	if ac.feedbackFunc == nil {
		return
	}

	ac.feedbackFunc(Feedback{
		ActivityName: ac.name,
		Status:       StatusFailed,
		Progress:     0.0,
		Message:      message,
		Error:        err,
		Timestamp:    time.Now(),
	})
}

// SendRetry sends retry feedback
func (ac *ActivityContext) SendRetry(attempt int, err error) {
	if ac.feedbackFunc == nil {
		return
	}

	ac.feedbackFunc(Feedback{
		ActivityName: ac.name,
		Status:       StatusRunning,
		Progress:     0.0,
		Message:      fmt.Sprintf("Retrying attempt %d", attempt),
		Error:        err,
		Metadata: map[string]interface{}{
			"retry_attempt": attempt,
		},
		Timestamp: time.Now(),
	})
}

// Activity represents a reusable operation
// It's just a function that takes standard context and activity context
type Activity[I any, O any] func(ctx context.Context, activityCtx *ActivityContext, input I) (O, error)

// NoOpFeedbackHandler is a feedback handler that does nothing
func NoOpFeedbackHandler(feedback Feedback) {
	// Do nothing
}
