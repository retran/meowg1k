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

// ExecuteActivity is a generic wrapper around the executor's ExecuteActivity method.
// It provides compile-time type safety for activity inputs and outputs.
func ExecuteActivity[T, K any](
	e Executor,
	ctx context.Context,
	parentCtx *Context,
	name string,
	activity Activity[T, K],
	input T,
) *future.Future[K] {
	typedFuture := future.NewFuture[K]()

	// Validate inputs
	if e == nil {
		_ = typedFuture.CompleteWithError(fmt.Errorf("executor cannot be nil"))
		return typedFuture
	}

	if ctx == nil {
		_ = typedFuture.CompleteWithError(fmt.Errorf("context cannot be nil"))
		return typedFuture
	}

	if parentCtx == nil {
		_ = typedFuture.CompleteWithError(fmt.Errorf("parent context cannot be nil"))
		return typedFuture
	}

	if name == "" {
		_ = typedFuture.CompleteWithError(fmt.Errorf("activity name cannot be empty"))
		return typedFuture
	}

	if activity == nil {
		_ = typedFuture.CompleteWithError(fmt.Errorf("activity %q cannot be nil", name))
		return typedFuture
	}

	// Create a type-erased activity that calls the typed activity
	untypedActivity := func(ctx context.Context, activityCtx *Context, input any) (any, error) {
		typedInput, ok := input.(T)
		if !ok {
			return nil, fmt.Errorf("invalid input type for activity %q: expected %T, got %T", name, *new(T), input)
		}
		return activity(ctx, activityCtx, typedInput)
	}

	// Call the untyped executor method
	untypedFuture := e.ExecuteActivity(ctx, parentCtx, name, untypedActivity, input)

	go func() {
		result, err := untypedFuture.Get(ctx)
		if err != nil {
			_ = typedFuture.CompleteWithError(err)
			return
		}

		typedResult, ok := result.(K)
		if !ok {
			_ = typedFuture.CompleteWithError(fmt.Errorf("invalid output type for activity %q: expected %T, got %T", name, *new(K), result))
			return
		}

		_ = typedFuture.Complete(typedResult)
	}()

	return typedFuture
}

// Executor defines the interface for executing flows and activities.
type Executor interface {
	ExecuteActivity(
		ctx context.Context,
		parentCtx *Context,
		name string,
		activity Activity[any, any],
		input any,
	) *future.Future[any]
	ExecuteFlow(ctx context.Context, name string, flow Flow) error
	WithRetryPolicy(policy *RetryPolicy) Executor
	WithFeedbackHandler(handler FeedbackHandler) Executor
}

// executorImpl is the central component for running activities
// It handles retry logic, feedback, and will handle caching/rate limiting in the future
type executorImpl struct {
	RetryPolicy     *RetryPolicy
	FeedbackHandler FeedbackHandler
	workerSemaphore chan struct{} // Semaphore to limit concurrent workers
}

// Compile-time check to ensure Impl implements Executor interface
var _ Executor = (*executorImpl)(nil)

// NewExecutor creates a new activity executor with the given configuration
// concurrency limits the number of activities that can run in parallel
// If concurrency <= 0, no limit is applied
func NewExecutor(concurrency int) *executorImpl {
	var semaphore chan struct{}
	if concurrency > 0 {
		semaphore = make(chan struct{}, concurrency)
	}

	return &executorImpl{
		RetryPolicy:     DefaultRetryPolicy(),
		FeedbackHandler: NoOpFeedbackHandler,
		workerSemaphore: semaphore,
	}
}

// WithRetryPolicy sets the retry policy for this executor
func (e *executorImpl) WithRetryPolicy(policy *RetryPolicy) Executor {
	if policy == nil {
		policy = DefaultRetryPolicy()
	}

	e.RetryPolicy = policy
	return e
}

// WithFeedbackHandler sets the feedback handler for this executor
func (e *executorImpl) WithFeedbackHandler(handler FeedbackHandler) Executor {
	if handler == nil {
		handler = NoOpFeedbackHandler
	}

	e.FeedbackHandler = handler
	return e
}

// ExecuteFlow runs a flow asynchronously and returns when it completes or fails
func (e *executorImpl) ExecuteFlow(
	ctx context.Context,
	flowName string,
	flow Flow,
) error {
	if e == nil {
		return fmt.Errorf("executor is nil")
	}

	if ctx == nil {
		return fmt.Errorf("context cannot be nil")
	}

	if flowName == "" {
		return fmt.Errorf("flow name is empty")
	}

	if flow == nil {
		return fmt.Errorf("flow is nil")
	}

	fut := future.NewFuture[any]()
	executorCtx := NewContext(flowName, e.FeedbackHandler, e)

	go func() {
		err := e.executeFlowImpl(ctx, executorCtx, flow)
		if err != nil {
			_ = fut.CompleteWithError(err)
		} else {
			_ = fut.Complete(nil)
		}
	}()

	_, err := fut.Get(ctx)
	if err != nil {
		return fmt.Errorf("flow execution failed: %w", err)
	}

	return nil
}

// ExecuteActivity runs a sub-activity asynchronously and returns a future for its result
// This is the internal implementation used by the generic RunActivity function.
func (e *executorImpl) ExecuteActivity(
	ctx context.Context,
	parentCtx *Context,
	activityName string,
	activity Activity[any, any],
	input any,
) *future.Future[any] {
	fut := future.NewFuture[any]()

	// Validate inputs before starting goroutine
	if e == nil {
		_ = fut.CompleteWithError(fmt.Errorf("executor is nil"))
		return fut
	}

	if ctx == nil {
		_ = fut.CompleteWithError(fmt.Errorf("context cannot be nil"))
		return fut
	}

	if parentCtx == nil {
		_ = fut.CompleteWithError(fmt.Errorf("parent context is nil"))
		return fut
	}

	if activityName == "" {
		_ = fut.CompleteWithError(fmt.Errorf("activity name %q is empty", activityName))
		return fut
	}

	if activity == nil {
		_ = fut.CompleteWithError(fmt.Errorf("activity is nil for activity name %q", activityName))
		return fut
	}

	if e.RetryPolicy == nil {
		_ = fut.CompleteWithError(fmt.Errorf("retry policy is nil for activity %q", activityName))
		return fut
	}

	fullActivityName := fmt.Sprintf("%s::%s", parentCtx.name, activityName)
	activityCtx := NewContext(fullActivityName, parentCtx.feedbackFunc, e)

	go func() {
		// Acquire worker slot if semaphore is configured
		if e.workerSemaphore != nil {
			e.workerSemaphore <- struct{}{}
			defer func() { <-e.workerSemaphore }()
		}

		result, err := e.executeActivityImpl(ctx, activityCtx, activity, input, e.RetryPolicy)
		if err != nil {
			_ = fut.CompleteWithError(err)
		} else {
			_ = fut.Complete(result)
		}
	}()

	return fut
}

func (e *executorImpl) executeFlowImpl(
	ctx context.Context,
	flowCtx *Context,
	flow func(context.Context, *Context) error,
) error {
	if e == nil {
		return fmt.Errorf("executor is nil")
	}

	if ctx == nil {
		return fmt.Errorf("context cannot be nil")
	}

	if flowCtx == nil {
		return fmt.Errorf("flow context is nil")
	}

	if flow == nil {
		return fmt.Errorf("flow is nil")
	}

	select {
	case <-ctx.Done():
		flowCtx.SendFailed(ctx.Err(), fmt.Sprintf("Flow %q is canceled", flowCtx.name))
		return fmt.Errorf("flow %q is canceled: %w", flowCtx.name, ctx.Err())
	default:
	}

	err := flow(ctx, flowCtx)
	if err != nil {
		flowCtx.SendFailed(err, fmt.Sprintf("Flow %q is failed", flowCtx.name))
		return fmt.Errorf("flow %q failed: %w", flowCtx.name, err)
	}

	return nil
}

func (e *executorImpl) executeActivityImpl(
	ctx context.Context,
	activityCtx *Context,
	activity Activity[any, any],
	input any,
	policy *RetryPolicy,
) (any, error) {
	if e == nil {
		return nil, fmt.Errorf("executor is nil")
	}

	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if activityCtx == nil {
		return nil, fmt.Errorf("activity context is nil")
	}

	if activity == nil {
		return nil, fmt.Errorf("activity is nil for %q", activityCtx.name)
	}

	if policy == nil {
		return nil, fmt.Errorf("retry policy is nil for activity %q", activityCtx.name)
	}

	if policy.MaxAttempts < 1 {
		return nil, fmt.Errorf("invalid retry policy for activity %q: max attempts must be at least 1, got %d", activityCtx.name, policy.MaxAttempts)
	}

	if policy.Multiplier < 1.0 {
		return nil, fmt.Errorf("invalid retry policy for activity %q: multiplier must be at least 1.0, got %f", activityCtx.name, policy.Multiplier)
	}

	name := activityCtx.name
	delay := policy.InitialDelay

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			activityCtx.SendFailed(ctx.Err(), fmt.Sprintf("Activity %q is canceled", activityCtx.name))
			return nil, fmt.Errorf("activity %q canceled at attempt %d: %w", activityCtx.name, attempt, ctx.Err())
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
			return nil, fmt.Errorf("activity %q canceled during retry backoff at attempt %d: %w", activityCtx.name, attempt, ctx.Err())
		case <-time.After(delay):
			// Calculate next delay with exponential backoff
			delay = min(time.Duration(float64(delay)*policy.Multiplier), policy.MaxDelay)
		}
	}

	// This should never be reached, but just in case
	return nil, fmt.Errorf("unexpected end of retry loop for activity %q after %d attempts", name, policy.MaxAttempts)
}
