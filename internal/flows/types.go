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

package flows

import (
	"context"
	"log/slog"
	"time"
)

// TaskID is a unique identifier for a task within a Flow.
type TaskID string

// OutcomeType defines the result type of task execution that affects control flow.
type OutcomeType int

const (
	// OutcomeSuccess - task completed successfully, proceed to next task via standard link.
	OutcomeSuccess OutcomeType = iota
	// OutcomeExit - task completed successfully and the entire Flow should stop.
	OutcomeExit
	// OutcomeRetry - indicates that the CURRENT task should be retried.
	OutcomeRetry
	// OutcomeConditional - execution result contains data for conditional branching.
	OutcomeConditional
	// OutcomeContinue - indicates that the CURRENT task should restart with NEW input.
	OutcomeContinue
)

// String returns string representation of OutcomeType for logging.
func (ot OutcomeType) String() string {
	switch ot {
	case OutcomeSuccess:
		return "Success"
	case OutcomeExit:
		return "Exit"
	case OutcomeRetry:
		return "Retry"
	case OutcomeConditional:
		return "Conditional"
	case OutcomeContinue:
		return "Continue"
	default:
		return "Unknown"
	}
}

// Outcome represents the control signal returned by a task.
// It is separated from the main result (data) for clarity.
// T is the type of data used for branching decisions.
type Outcome[T any] struct {
	Type OutcomeType
	Data T // May be empty for OutcomeRetry/Continue/Success
}

// Executor is the main interface for any task in a workflow.
// It is a thin wrapper over business logic (services).
type Executor[I any, O any, OT any] interface {
	Execute(ctx context.Context, input I) (O, Outcome[OT], error)
}

// ExecutionContext stores metadata about the current task execution.
type ExecutionContext struct {
	TaskID     TaskID
	RetryCount int
	FlowID     string
}

// RetryPolicy defines exponential backoff behavior for OutcomeRetry.
// Controls how tasks are retried when they return OutcomeRetry outcome.
type RetryPolicy struct {
	// InitialDelay sets the delay before the first retry attempt
	// Minimum recommended: 50ms, Maximum recommended: 5s
	// Example: 100ms, 500ms, 1s
	InitialDelay time.Duration

	// MaxDelay caps the maximum delay between retry attempts
	// Should be larger than InitialDelay to allow exponential growth
	// Example: 5s, 30s, 2m
	MaxDelay time.Duration

	// Multiplier controls the exponential backoff rate (must be > 1.0)
	// Each retry delay = previous_delay * Multiplier (capped by MaxDelay)
	// Common values: 1.5 (gentle), 2.0 (standard), 3.0 (aggressive)
	Multiplier float64

	// MaxRetries sets the maximum number of retry attempts before giving up
	// 0 means no retries, -1 means unlimited retries (not recommended)
	// Common values: 3 (default), 5 (persistent), 10 (very persistent)
	MaxRetries int
}

// DefaultRetryPolicy returns default policy.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		MaxRetries:   3,
	}
}

// TimeoutConfig defines timeout behaviors for workflow operations.
// Controls various timeout values used throughout workflow execution.
type TimeoutConfig struct {
	// ResultTimeout sets the timeout for collecting final results
	// This prevents workflows from hanging when waiting for results
	// Recommended: 100ms-1s for responsive systems, 1s-10s for slower systems
	ResultTimeout time.Duration
}

// DefaultTimeoutConfig returns default timeout configuration.
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		ResultTimeout: 100 * time.Millisecond,
	}
}

// FeedbackStatus represents the status of a workflow or task
type FeedbackStatus string

const (
	// Workflow statuses
	WorkflowStarted   FeedbackStatus = "workflow_started"
	WorkflowCompleted FeedbackStatus = "workflow_completed"
	WorkflowFailed    FeedbackStatus = "workflow_failed"

	// Task statuses
	TaskStarted   FeedbackStatus = "task_started"
	TaskCompleted FeedbackStatus = "task_completed"
	TaskFailed    FeedbackStatus = "task_failed"
	TaskRetrying  FeedbackStatus = "task_retrying"
)

// String returns the string representation of the FeedbackStatus
func (s FeedbackStatus) String() string {
	return string(s)
}

// IsWorkflowStatus returns true if the status is related to workflow events
func (s FeedbackStatus) IsWorkflowStatus() bool {
	return s == WorkflowStarted || s == WorkflowCompleted || s == WorkflowFailed
}

// IsTaskStatus returns true if the status is related to task events
func (s FeedbackStatus) IsTaskStatus() bool {
	return s == TaskStarted || s == TaskCompleted || s == TaskFailed || s == TaskRetrying
}

// Feedback represents data for sending status and metrics to external systems.
type Feedback struct {
	TaskID      TaskID                 `json:"task_id"`
	Status      FeedbackStatus         `json:"status"`
	Description string                 `json:"description"` // Human-readable task description
	Progress    float64                `json:"progress"`    // From 0.0 to 1.0
	Metrics     map[string]interface{} `json:"metrics"`     // For collecting statistics
	Timestamp   time.Time              `json:"timestamp"`
}

// FeedbackHandler is a function for processing feedback from tasks.
type FeedbackHandler func(Feedback)

// ReduceFunc is a function used for aggregating elements.
type ReduceFunc[A any, I any] func(accumulator A, item I) A

// Context keys
type contextKey string

const (
	executionContextKey = contextKey("executionContext")
	feedbackKey         = contextKey("feedbackSender")
	loggerKey           = contextKey("logger")
)

// NewContextWithExecutionState creates a new context containing ExecutionContext.
func NewContextWithExecutionState(ctx context.Context, state ExecutionContext) context.Context {
	return context.WithValue(ctx, executionContextKey, state)
}

// GetExecutionState extracts ExecutionContext from context.
// If context is not found, returns empty struct.
func GetExecutionState(ctx context.Context) ExecutionContext {
	state, _ := ctx.Value(executionContextKey).(ExecutionContext)
	return state
}

// NewContextWithFeedback creates a new context containing FeedbackHandler.
func NewContextWithFeedback(ctx context.Context, handler FeedbackHandler, taskID TaskID) context.Context {
	// Wrap handler to automatically fill TaskID and timestamp
	wrappedHandler := func(f Feedback) {
		f.TaskID = taskID
		f.Timestamp = time.Now()
		handler(f)
	}
	return context.WithValue(ctx, feedbackKey, wrappedHandler)
}

// GetFeedbackSender extracts feedback sending function from context.
func GetFeedbackSender(ctx context.Context) func(Feedback) {
	if sender, ok := ctx.Value(feedbackKey).(func(Feedback)); ok {
		return sender
	}
	// Return no-op function if feedback is not configured
	return func(Feedback) {}
}

// NewContextWithLogger creates a new context containing logger.
func NewContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// GetLogger extracts logger from context.
func GetLogger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}
	// Return default logger if not configured
	return slog.Default()
}
