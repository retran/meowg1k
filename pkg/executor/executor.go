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

// Package executor provides a framework for defining and executing activities
// with support for retries, feedback, and sub-activities.
package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/retran/meowg1k/pkg/future"
)

// RetryPolicy defines how activities should be retried on failure
type RetryPolicy struct {
	MaxAttempts  int           `json:"max_attempts"`
	InitialDelay time.Duration `json:"initial_delay"`
	MaxDelay     time.Duration `json:"max_delay"`
	Multiplier   float64       `json:"multiplier"`
}

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

// ExecutorImpl is the central component for running activities
// It handles retry logic, feedback, and will handle caching/rate limiting in the future
type ExecutorImpl struct {
	RetryPolicy     *RetryPolicy
	FeedbackHandler FeedbackHandler
}

// NewExecutor creates a new activity executor with the given configuration
func NewExecutor() *ExecutorImpl {
	return &ExecutorImpl{
		RetryPolicy:     DefaultRetryPolicy(),
		FeedbackHandler: NoOpFeedbackHandler,
	}
}

// WithRetryPolicy sets the retry policy for this executor
func (e *ExecutorImpl) WithRetryPolicy(policy *RetryPolicy) Executor {
	e.RetryPolicy = policy
	return e
}

// WithFeedbackHandler sets the feedback handler for this executor
func (e *ExecutorImpl) WithFeedbackHandler(handler FeedbackHandler) Executor {
	e.FeedbackHandler = handler
	return e
}

// RunFlow runs a flow asynchronously and returns when it completes or fails
func (e *ExecutorImpl) RunFlow(ctx context.Context, flowName string, flow func(context.Context, *ExecutorContext) error) error {
	fut := future.NewFuture[any]()
	executorCtx := NewExecutorContext(flowName, e.FeedbackHandler, e)
	executorCtx.SendFeedback(StatusPending, 0, fmt.Sprintf("%s is pending", flowName))

	go func() {
		// Try to cast to a function with the right signature
		err := e.executeFlow(ctx, executorCtx, flow)
		if err != nil {
			fut.CompleteWithError(err)
		} else {
			fut.Complete(nil)
		}
	}()

	_, err := fut.Get(ctx)
	return err
}

// RunActivity runs a sub-activity asynchronously and returns a future for its result
func (e *ExecutorImpl) RunActivity(ctx context.Context, parentCtx *ExecutorContext, activityName string, activity any, input any) *future.Future[any] {
	future := future.NewFuture[any]()
	fullActivityName := fmt.Sprintf("%s.%s", parentCtx.name, activityName)
	activityCtx := NewExecutorContext(fullActivityName, parentCtx.feedbackFunc, e)
	activityCtx.SendFeedback(StatusPending, 0, fmt.Sprintf("%s is pending", activityName))

	go func() {
		if activityFunc, ok := activity.(func(context.Context, *ExecutorContext, any) (any, error)); ok {
			result, err := e.executeActivity(ctx, activityCtx, activityFunc, input, e.RetryPolicy)
			if err != nil {
				future.CompleteWithError(err)
			} else {
				future.Complete(result)
			}
		} else {
			future.CompleteWithError(fmt.Errorf("activity %s has invalid type", activityName))
		}
	}()

	return future
}

func (e *ExecutorImpl) executeFlow(
	ctx context.Context,
	flowCtx *ExecutorContext,
	flow func(context.Context, *ExecutorContext) error,
) error {
	flowCtx.SendFeedback(StatusStarted, 0.0, fmt.Sprintf("Starting %s", flowCtx.name))

	select {
	case <-ctx.Done():
		flowCtx.SendError(ctx.Err(), fmt.Sprintf("%s cancelled", flowCtx.name))
		return ctx.Err()
	default:
	}

	err := flow(ctx, flowCtx)
	if err != nil {
		flowCtx.SendError(err, fmt.Sprintf("%s failed", flowCtx.name))
		return fmt.Errorf("%s failed: %v", flowCtx.name, err)
	}

	flowCtx.SendFeedback(StatusCompleted, 1.0, fmt.Sprintf("%s completed successfully", flowCtx.name))
	return nil
}

// executeActivity handles typed sub-activities with retry logic
func (e *ExecutorImpl) executeActivity(
	ctx context.Context,
	activityCtx *ExecutorContext,
	activity func(context.Context, *ExecutorContext, any) (any, error),
	input any,
	policy *RetryPolicy,
) (any, error) {
	name := activityCtx.name
	activityCtx.SendFeedback(StatusStarted, 0.0, fmt.Sprintf("Starting %s", name))

	delay := policy.InitialDelay

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		// Check if context is cancelled before each attempt
		select {
		case <-ctx.Done():
			activityCtx.SendError(ctx.Err(), "Sub-activity cancelled")
			return nil, ctx.Err()
		default:
		}

		result, err := activity(ctx, activityCtx, input)

		if err == nil {
			activityCtx.SendFeedback(StatusCompleted, 1.0, fmt.Sprintf("%s completed successfully", name))
			return result, nil
		}

		// If this was the last attempt, return the error
		if attempt == policy.MaxAttempts {
			activityCtx.SendError(err, fmt.Sprintf("%s failed after %d attempts", name, attempt))
			return nil, fmt.Errorf("sub-activity %s failed after %d attempts: %w", name, attempt, err)
		}

		// Send retry feedback
		activityCtx.SendRetry(attempt+1, err)

		// Wait before retry with exponential backoff
		select {
		case <-ctx.Done():
			activityCtx.SendError(ctx.Err(), "Sub-activity cancelled during retry delay")
			return nil, ctx.Err()
		case <-time.After(delay):
			// Calculate next delay with exponential backoff
			delay = min(time.Duration(float64(delay)*policy.Multiplier), policy.MaxDelay)
		}
	}

	// This should never be reached, but just in case
	return nil, fmt.Errorf("sub-activity %s: unexpected end of retry loop", name)
}
