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

// RetryPolicy defines how activities should be retried on failure
type RetryPolicy struct {
	MaxAttempts  int           `json:"max_attempts"`
	InitialDelay time.Duration `json:"initial_delay"`
	MaxDelay     time.Duration `json:"max_delay"`
	Multiplier   float64       `json:"multiplier"`
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
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

// ExecutorConfig defines configuration for the activity executor
type ExecutorConfig struct {
	RetryPolicy     RetryPolicy
	FeedbackHandler FeedbackHandler
	// TODO: Add caching, rate limiting, etc.
	// CacheConfig     CacheConfig
	// RateLimitConfig RateLimitConfig
}

// DefaultExecutorConfig returns default executor configuration
func DefaultExecutorConfig() ExecutorConfig {
	return ExecutorConfig{
		RetryPolicy:     DefaultRetryPolicy(),
		FeedbackHandler: NoOpFeedbackHandler,
	}
}

// Executor is the central component for running activities
// It handles retry logic, feedback, and will handle caching/rate limiting in the future
type Executor struct {
	config ExecutorConfig
}

// NewExecutor creates a new activity executor with the given configuration
func NewExecutor(config ExecutorConfig) *Executor {
	return &Executor{
		config: config,
	}
}

// NewDefaultExecutor creates a new executor with default configuration
func NewDefaultExecutor() *Executor {
	return NewExecutor(DefaultExecutorConfig())
}

// WithRetryPolicy sets the retry policy for this executor
func (e *Executor) WithRetryPolicy(policy RetryPolicy) *Executor {
	e.config.RetryPolicy = policy
	return e
}

// WithFeedbackHandler sets the feedback handler for this executor
func (e *Executor) WithFeedbackHandler(handler FeedbackHandler) *Executor {
	e.config.FeedbackHandler = handler
	return e
}

// RunActivity method for ActivityExecutor interface - handles untyped activities asynchronously
func (e *Executor) RunActivity(ctx context.Context, activityName string, activity interface{}, input interface{}) *future.Future[interface{}] {
	fut := future.NewFuture[interface{}]()

	go func() {
		// Try to cast to a function with the right signature
		if activityFunc, ok := activity.(func(context.Context, *ActivityContext, interface{}) (interface{}, error)); ok {
			result, err := e.executeUntypedWithRetry(ctx, activityName, activityFunc, input, e.config.RetryPolicy)
			if err != nil {
				fut.CompleteWithError(err)
			} else {
				fut.Complete(result)
			}
		} else {
			fut.CompleteWithError(fmt.Errorf("unsupported activity type for %s", activityName))
		}
	}()

	return fut
}

// RunSubActivity runs a sub-activity with inherited context from parent asynchronously
func (e *Executor) RunSubActivity(parentCtx *ActivityContext, ctx context.Context, activityName string, activity interface{}, input interface{}) *future.Future[interface{}] {
	fut := future.NewFuture[interface{}]()

	go func() {
		// Create full activity name with parent prefix
		fullActivityName := fmt.Sprintf("%s.%s", parentCtx.name, activityName)

		// Try to cast to a function with the right signature
		if activityFunc, ok := activity.(func(context.Context, *ActivityContext, interface{}) (interface{}, error)); ok {
			result, err := e.executeUntypedWithRetryForSubActivity(ctx, fullActivityName, activityFunc, input, e.config.RetryPolicy, parentCtx.feedbackFunc)
			if err != nil {
				fut.CompleteWithError(err)
			} else {
				fut.Complete(result)
			}
		} else {
			fut.CompleteWithError(fmt.Errorf("unsupported activity type for %s", fullActivityName))
		}
	}()

	return fut
}

// executeUntypedWithRetry handles untyped activities with retry logic
func (e *Executor) executeUntypedWithRetry(
	ctx context.Context,
	activityName string,
	activity func(context.Context, *ActivityContext, interface{}) (interface{}, error),
	input interface{},
	policy RetryPolicy,
) (interface{}, error) {
	activityCtx := NewActivityContextWithExecutor(activityName, e.config.FeedbackHandler, e)
	activityCtx.SendFeedback(StatusStarted, 0.0, fmt.Sprintf("Starting %s", activityName))

	delay := policy.InitialDelay

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		// Check if context is cancelled before each attempt
		select {
		case <-ctx.Done():
			activityCtx.SendError(ctx.Err(), "Activity cancelled")
			return nil, ctx.Err()
		default:
		}

		result, err := activity(ctx, activityCtx, input)

		if err == nil {
			activityCtx.SendFeedback(StatusCompleted, 1.0, fmt.Sprintf("%s completed successfully", activityName))
			return result, nil
		}

		// If this was the last attempt, return the error
		if attempt == policy.MaxAttempts {
			activityCtx.SendError(err, fmt.Sprintf("%s failed after %d attempts", activityName, attempt))
			return nil, fmt.Errorf("activity %s failed after %d attempts: %w", activityName, attempt, err)
		}

		// Send retry feedback
		activityCtx.SendRetry(attempt+1, err)

		// Wait before retry with exponential backoff
		select {
		case <-ctx.Done():
			activityCtx.SendError(ctx.Err(), "Activity cancelled during retry delay")
			return nil, ctx.Err()
		case <-time.After(delay):
			// Calculate next delay with exponential backoff
			delay = time.Duration(float64(delay) * policy.Multiplier)
			if delay > policy.MaxDelay {
				delay = policy.MaxDelay
			}
		}
	}

	// This should never be reached, but just in case
	return nil, fmt.Errorf("activity %s: unexpected end of retry loop", activityName)
}

// executeUntypedWithRetryForSubActivity handles sub-activities with inherited feedback handler
func (e *Executor) executeUntypedWithRetryForSubActivity(
	ctx context.Context,
	activityName string,
	activity func(context.Context, *ActivityContext, interface{}) (interface{}, error),
	input interface{},
	policy RetryPolicy,
	parentFeedbackHandler FeedbackHandler,
) (interface{}, error) {
	// Use parent's feedback handler for sub-activities
	activityCtx := NewActivityContextWithExecutor(activityName, parentFeedbackHandler, e)
	activityCtx.SendFeedback(StatusStarted, 0.0, fmt.Sprintf("Starting %s", activityName))

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
			activityCtx.SendFeedback(StatusCompleted, 1.0, fmt.Sprintf("%s completed successfully", activityName))
			return result, nil
		}

		// If this was the last attempt, return the error
		if attempt == policy.MaxAttempts {
			activityCtx.SendError(err, fmt.Sprintf("%s failed after %d attempts", activityName, attempt))
			return nil, fmt.Errorf("sub-activity %s failed after %d attempts: %w", activityName, attempt, err)
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
			delay = time.Duration(float64(delay) * policy.Multiplier)
			if delay > policy.MaxDelay {
				delay = policy.MaxDelay
			}
		}
	}

	// This should never be reached, but just in case
	return nil, fmt.Errorf("sub-activity %s: unexpected end of retry loop", activityName)
}

// Execute runs an activity with the configured retry policy and feedback handling
func Execute[I any, O any](
	executor *Executor,
	ctx context.Context,
	activityName string,
	activity Activity[I, O],
	input I,
) (O, error) {
	return executeWithRetry(executor, ctx, activityName, activity, input, executor.config.RetryPolicy)
}

// ExecuteWithCustomRetry runs an activity with a custom retry policy
func ExecuteWithCustomRetry[I any, O any](
	executor *Executor,
	ctx context.Context,
	activityName string,
	activity Activity[I, O],
	input I,
	retryPolicy RetryPolicy,
) (O, error) {
	return executeWithRetry(executor, ctx, activityName, activity, input, retryPolicy)
}

// executeWithRetry is the internal function that handles retry logic
func executeWithRetry[I any, O any](
	executor *Executor,
	ctx context.Context,
	activityName string,
	activity Activity[I, O],
	input I,
	policy RetryPolicy,
) (O, error) {
	var zero O

	activityCtx := NewActivityContext(activityName, executor.config.FeedbackHandler)
	activityCtx.executor = executor // Set the executor reference for sub-activities
	activityCtx.SendFeedback(StatusStarted, 0.0, fmt.Sprintf("Starting %s", activityName))

	delay := policy.InitialDelay

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		// Check if context is cancelled before each attempt
		select {
		case <-ctx.Done():
			activityCtx.SendError(ctx.Err(), "Activity cancelled")
			return zero, ctx.Err()
		default:
		}

		// TODO: Add caching check here
		// if cached, ok := e.getCachedResult(activityName, input); ok {
		//     return cached, nil
		// }

		// TODO: Add rate limiting check here
		// if err := e.checkRateLimit(activityName); err != nil {
		//     return zero, err
		// }

		result, err := activity(ctx, activityCtx, input)

		if err == nil {
			// TODO: Cache successful result
			// e.cacheResult(activityName, input, result)

			activityCtx.SendFeedback(StatusCompleted, 1.0, fmt.Sprintf("%s completed successfully", activityName))
			return result, nil
		}

		// If this was the last attempt, return the error
		if attempt == policy.MaxAttempts {
			activityCtx.SendError(err, fmt.Sprintf("%s failed after %d attempts", activityName, attempt))
			return zero, fmt.Errorf("activity %s failed after %d attempts: %w", activityName, attempt, err)
		}

		// Send retry feedback
		activityCtx.SendRetry(attempt+1, err)

		// Wait before retry with exponential backoff
		select {
		case <-ctx.Done():
			activityCtx.SendError(ctx.Err(), "Activity cancelled during retry delay")
			return zero, ctx.Err()
		case <-time.After(delay):
			// Calculate next delay with exponential backoff
			delay = time.Duration(float64(delay) * policy.Multiplier)
			if delay > policy.MaxDelay {
				delay = policy.MaxDelay
			}
		}
	}

	// This should never be reached, but just in case
	return zero, fmt.Errorf("activity %s: unexpected end of retry loop", activityName)
}

// RetryBuilder provides a fluent interface for building retry policies
type RetryBuilder struct {
	policy RetryPolicy
}

// NewRetryBuilder creates a new retry policy builder with defaults
func NewRetryBuilder() *RetryBuilder {
	return &RetryBuilder{
		policy: DefaultRetryPolicy(),
	}
}

// MaxAttempts sets the maximum number of retry attempts
func (rb *RetryBuilder) MaxAttempts(attempts int) *RetryBuilder {
	if attempts < 1 {
		attempts = 1
	}
	rb.policy.MaxAttempts = attempts
	return rb
}

// InitialDelay sets the initial delay before the first retry
func (rb *RetryBuilder) InitialDelay(delay time.Duration) *RetryBuilder {
	rb.policy.InitialDelay = delay
	return rb
}

// MaxDelay sets the maximum delay between retries
func (rb *RetryBuilder) MaxDelay(delay time.Duration) *RetryBuilder {
	rb.policy.MaxDelay = delay
	return rb
}

// Multiplier sets the multiplier for exponential backoff
func (rb *RetryBuilder) Multiplier(multiplier float64) *RetryBuilder {
	if multiplier < 1.0 {
		multiplier = 1.0
	}
	rb.policy.Multiplier = multiplier
	return rb
}

// Build returns the configured retry policy
func (rb *RetryBuilder) Build() RetryPolicy {
	return rb.policy
}

// Quick presets for common retry scenarios
func QuickRetry() RetryPolicy {
	return NewRetryBuilder().
		MaxAttempts(2).
		InitialDelay(50 * time.Millisecond).
		MaxDelay(500 * time.Millisecond).
		Build()
}

func AggressiveRetry() RetryPolicy {
	return NewRetryBuilder().
		MaxAttempts(5).
		InitialDelay(200 * time.Millisecond).
		MaxDelay(10 * time.Second).
		Multiplier(2.5).
		Build()
}

func ConservativeRetry() RetryPolicy {
	return NewRetryBuilder().
		MaxAttempts(3).
		InitialDelay(500 * time.Millisecond).
		MaxDelay(3 * time.Second).
		Multiplier(1.5).
		Build()
}

// Convenience functions for backward compatibility and simple usage

// Run executes an activity without retry using the default executor
func Run[I any, O any](
	ctx context.Context,
	activityName string,
	activity Activity[I, O],
	input I,
	feedbackHandler FeedbackHandler,
) (O, error) {
	executor := NewExecutor(ExecutorConfig{
		RetryPolicy:     NoRetryPolicy(),
		FeedbackHandler: feedbackHandler,
	})
	return Execute(executor, ctx, activityName, activity, input)
}

// RunWithRetry executes an activity with retry using the default executor
func RunWithRetry[I any, O any](
	ctx context.Context,
	activityName string,
	activity Activity[I, O],
	input I,
	policy RetryPolicy,
	feedbackHandler FeedbackHandler,
) (O, error) {
	executor := NewExecutor(ExecutorConfig{
		RetryPolicy:     policy,
		FeedbackHandler: feedbackHandler,
	})
	return Execute(executor, ctx, activityName, activity, input)
}
