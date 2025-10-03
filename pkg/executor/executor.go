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
	"errors"
	"fmt"
	"time"

	"github.com/retran/meowg1k/pkg/future"
)

var (
	// ErrActivityInvalidType indicates that an activity has an invalid type.
	ErrActivityInvalidType = errors.New("activity has invalid type")
	// ErrFlowFailed indicates that a flow execution has failed.
	ErrFlowFailed = errors.New("flow is failed")
	// ErrUnexpectedEndOfRetry indicates an unexpected condition in the retry logic.
	ErrUnexpectedEndOfRetry = errors.New("unexpected end of retry loop")
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

// Impl is the central component for running activities
// It handles retry logic, feedback, and will handle caching/rate limiting in the future
type Impl struct {
	RetryPolicy     *RetryPolicy
	FeedbackHandler FeedbackHandler
}

// NewExecutor creates a new activity executor with the given configuration
func NewExecutor() *Impl {
	return &Impl{
		RetryPolicy:     DefaultRetryPolicy(),
		FeedbackHandler: NoOpFeedbackHandler,
	}
}

// WithRetryPolicy sets the retry policy for this executor
func (e *Impl) WithRetryPolicy(policy *RetryPolicy) Executor {
	e.RetryPolicy = policy
	return e
}

// WithFeedbackHandler sets the feedback handler for this executor
func (e *Impl) WithFeedbackHandler(handler FeedbackHandler) Executor {
	e.FeedbackHandler = handler
	return e
}

// RunFlow runs a flow asynchronously and returns when it completes or fails
func (e *Impl) RunFlow(
	ctx context.Context,
	flowName string,
	flow Flow,
	retryPolicy *RetryPolicy,
) error {
	fut := future.NewFuture[any]()
	executorCtx := NewContext(flowName, e.FeedbackHandler, e)

	go func() {
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
func (e *Impl) RunActivity(
	ctx context.Context,
	parentCtx *Context,
	activityName string,
	activity Activity[any, any],
	input any,
) *future.Future[any] {
	fut := future.NewFuture[any]()
	fullActivityName := fmt.Sprintf("%s::%s", parentCtx.name, activityName)
	activityCtx := NewContext(fullActivityName, parentCtx.feedbackFunc, e)

	go func() {
		result, err := e.executeActivity(ctx, activityCtx, activity, input, e.RetryPolicy)
		if err != nil {
			fut.CompleteWithError(err)
		} else {
			fut.Complete(result)
		}
	}()

	return fut
}

func (e *Impl) executeFlow(
	ctx context.Context,
	flowCtx *Context,
	flow func(context.Context, *Context) error,
) error {
	select {
	case <-ctx.Done():
		flowCtx.SendFailed(ctx.Err(), fmt.Sprintf("Flow %q is canceled", flowCtx.name))
		return fmt.Errorf("flow %q is canceled: %w", flowCtx.name, ctx.Err())
	default:
	}

	err := flow(ctx, flowCtx)
	if err != nil {
		flowCtx.SendFailed(err, fmt.Sprintf("Flow %q is failed", flowCtx.name))
		return fmt.Errorf("%w: %s: %w", ErrFlowFailed, flowCtx.name, err)
	}

	return nil
}

// executeActivity handles typed sub-activities with retry logic
func (e *Impl) executeActivity(
	ctx context.Context,
	activityCtx *Context,
	activity Activity[any, any],
	input any,
	policy *RetryPolicy,
) (any, error) {
	name := activityCtx.name
	delay := policy.InitialDelay

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			activityCtx.SendFailed(ctx.Err(), fmt.Sprintf("Activity %q is canceled", activityCtx.name))
			return nil, fmt.Errorf("activity %q is canceled: %w", activityCtx.name, ctx.Err())
		default:
		}

		result, err := activity(ctx, activityCtx, input)
		if err == nil {
			return result, nil
		}

		// If this was the last attempt, return the error
		if attempt == policy.MaxAttempts {
			activityCtx.SendFailed(err, fmt.Sprintf("Activity %q failed after %d attempts", name, attempt))
			return nil, fmt.Errorf("activity %q failed after %d attempts: %w", name, attempt, err)
		}

		// Send retry feedback
		activityCtx.SendRetry(attempt+1, err)

		// Wait before retry with exponential backoff
		select {
		case <-ctx.Done():
			activityCtx.SendFailed(ctx.Err(), fmt.Sprintf("Activity %q is canceled", activityCtx.name))
			return nil, fmt.Errorf("activity %q is canceled: %w", activityCtx.name, ctx.Err())
		case <-time.After(delay):
			// Calculate next delay with exponential backoff
			delay = min(time.Duration(float64(delay)*policy.Multiplier), policy.MaxDelay)
		}
	}

	// This should never be reached, but just in case
	return nil, fmt.Errorf("%w: %s", ErrUnexpectedEndOfRetry, name)
}
