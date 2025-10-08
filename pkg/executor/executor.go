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
	// ErrExecutorIsNil indicates that the executor is nil.
	ErrExecutorIsNil = errors.New("executor is nil")
	// ErrContextIsNil indicates that the context is nil.
	ErrContextIsNil = errors.New("context is nil")
	// ErrFlowIsNil indicates that the flow is nil.
	ErrFlowIsNil = errors.New("flow is nil")
	// ErrActivityIsNil indicates that the activity is nil.
	ErrActivityIsNil = errors.New("activity is nil")
	// ErrRetryPolicyIsNil indicates that the retry policy is nil.
	ErrRetryPolicyIsNil = errors.New("retry policy is nil")
	// ErrFlowNameIsEmpty indicates that the flow name is empty.
	ErrFlowNameIsEmpty = errors.New("flow name is empty")
	// ErrActivityNameIsEmpty indicates that the activity name is empty.
	ErrActivityNameIsEmpty = errors.New("activity name is empty")
	// ErrInvalidRetryPolicy indicates that the retry policy has invalid values.
	ErrInvalidRetryPolicy = errors.New("invalid retry policy")
	// ErrContextCannotBeNil indicates that the context parameter is nil.
	ErrContextCannotBeNil = errors.New("context cannot be nil")
	// ErrInvalidMaxAttempts indicates that MaxAttempts is less than 1.
	ErrInvalidMaxAttempts = errors.New("max attempts must be at least 1")
	// ErrInvalidMultiplier indicates that Multiplier is less than 1.0.
	ErrInvalidMultiplier = errors.New("multiplier must be at least 1.0")
	// ErrFlowCanceled indicates that a flow was canceled.
	ErrFlowCanceled = errors.New("flow is canceled")
	// ErrActivityCanceled indicates that an activity was canceled.
	ErrActivityCanceled = errors.New("activity is canceled")
	// ErrActivityFailed indicates that an activity failed after retries.
	ErrActivityFailed = errors.New("activity failed after retries")
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

// Compile-time check to ensure Impl implements Executor interface
var _ Executor = (*Impl)(nil)

// NewExecutor creates a new activity executor with the given configuration
func NewExecutor() *Impl {
	return &Impl{
		RetryPolicy:     DefaultRetryPolicy(),
		FeedbackHandler: NoOpFeedbackHandler,
	}
}

// WithRetryPolicy sets the retry policy for this executor
func (e *Impl) WithRetryPolicy(policy *RetryPolicy) Executor {
	if policy == nil {
		policy = DefaultRetryPolicy()
	}
	e.RetryPolicy = policy
	return e
}

// WithFeedbackHandler sets the feedback handler for this executor
func (e *Impl) WithFeedbackHandler(handler FeedbackHandler) Executor {
	if handler == nil {
		handler = NoOpFeedbackHandler
	}
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
	if e == nil {
		return ErrExecutorIsNil
	}
	if ctx == nil {
		return ErrContextCannotBeNil
	}
	if flowName == "" {
		return ErrFlowNameIsEmpty
	}
	if flow == nil {
		return ErrFlowIsNil
	}

	fut := future.NewFuture[any]()
	executorCtx := NewContext(flowName, e.FeedbackHandler, e)

	go func() {
		err := e.executeFlow(ctx, executorCtx, flow)
		if err != nil {
			_ = fut.CompleteWithError(err)
		} else {
			_ = fut.Complete(nil)
		}
	}()

	_, err := fut.Get(ctx)

	return err
}

// runActivity runs a sub-activity asynchronously and returns a future for its result
// This is the internal implementation used by the generic RunActivity function.
func (e *Impl) runActivity(
	ctx context.Context,
	parentCtx *Context,
	activityName string,
	activity Activity[any, any],
	input any,
) *future.Future[any] {
	fut := future.NewFuture[any]()

	// Validate inputs before starting goroutine
	if e == nil {
		_ = fut.CompleteWithError(ErrExecutorIsNil)
		return fut
	}
	if ctx == nil {
		_ = fut.CompleteWithError(ErrContextCannotBeNil)
		return fut
	}
	if parentCtx == nil {
		_ = fut.CompleteWithError(ErrContextIsNil)
		return fut
	}
	if activityName == "" {
		_ = fut.CompleteWithError(ErrActivityNameIsEmpty)
		return fut
	}
	if activity == nil {
		_ = fut.CompleteWithError(ErrActivityIsNil)
		return fut
	}
	if e.RetryPolicy == nil {
		_ = fut.CompleteWithError(ErrRetryPolicyIsNil)
		return fut
	}

	fullActivityName := fmt.Sprintf("%s::%s", parentCtx.name, activityName)
	activityCtx := NewContext(fullActivityName, parentCtx.feedbackFunc, e)

	go func() {
		result, err := e.executeActivity(ctx, activityCtx, activity, input, e.RetryPolicy)
		if err != nil {
			_ = fut.CompleteWithError(err)
		} else {
			_ = fut.Complete(result)
		}
	}()

	return fut
}

func (e *Impl) executeFlow(
	ctx context.Context,
	flowCtx *Context,
	flow func(context.Context, *Context) error,
) error {
	if e == nil {
		return ErrExecutorIsNil
	}
	if ctx == nil {
		return ErrContextCannotBeNil
	}
	if flowCtx == nil {
		return ErrContextIsNil
	}
	if flow == nil {
		return ErrFlowIsNil
	}

	select {
	case <-ctx.Done():
		flowCtx.SendFailed(ctx.Err(), fmt.Sprintf("Flow %q is canceled", flowCtx.name))
		return errors.Join(ErrFlowCanceled, ctx.Err())
	default:
	}

	err := flow(ctx, flowCtx)
	if err != nil {
		flowCtx.SendFailed(err, fmt.Sprintf("Flow %q is failed", flowCtx.name))
		return errors.Join(ErrFlowFailed, err)
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
	if e == nil {
		return nil, ErrExecutorIsNil
	}
	if ctx == nil {
		return nil, ErrContextCannotBeNil
	}
	if activityCtx == nil {
		return nil, ErrContextIsNil
	}
	if activity == nil {
		return nil, ErrActivityIsNil
	}
	if policy == nil {
		return nil, ErrRetryPolicyIsNil
	}
	if policy.MaxAttempts < 1 {
		return nil, ErrInvalidMaxAttempts
	}
	if policy.Multiplier < 1.0 {
		return nil, ErrInvalidMultiplier
	}

	name := activityCtx.name
	delay := policy.InitialDelay

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			activityCtx.SendFailed(ctx.Err(), fmt.Sprintf("Activity %q is canceled", activityCtx.name))
			return nil, errors.Join(ErrActivityCanceled, ctx.Err())
		default:
		}

		result, err := activity(ctx, activityCtx, input)
		if err == nil {
			return result, nil
		}

		// If this was the last attempt, return the error
		if attempt == policy.MaxAttempts {
			activityCtx.SendFailed(err, fmt.Sprintf("Activity %q failed after %d attempts", name, attempt))
			return nil, errors.Join(ErrActivityFailed, err)
		}

		// Send retry feedback
		activityCtx.SendRetry(attempt+1, err)

		// Wait before retry with exponential backoff
		select {
		case <-ctx.Done():
			activityCtx.SendFailed(ctx.Err(), fmt.Sprintf("Activity %q is canceled", activityCtx.name))
			return nil, errors.Join(ErrActivityCanceled, ctx.Err())
		case <-time.After(delay):
			// Calculate next delay with exponential backoff
			delay = min(time.Duration(float64(delay)*policy.Multiplier), policy.MaxDelay)
		}
	}

	// This should never be reached, but just in case
	return nil, ErrUnexpectedEndOfRetry
}
