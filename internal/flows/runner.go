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
	"errors"
	"fmt"
	"math"
	"time"

	"golang.org/x/sync/errgroup"
)

// run executes workflow asynchronously and in parallel.
func (f *flowInternal) run(ctx context.Context, initialInput interface{}) (interface{}, error) {
	f.RLock()
	startTaskID := f.startTask
	logger := f.logger
	feedbackHandler := f.feedbackHandler
	f.RUnlock()

	// Log workflow start
	startTime := time.Now()
	logger.Info("Workflow execution started",
		"flow_id", f.id,
		"start_task", startTaskID,
		"start_time", startTime,
	)

	// Send feedback about workflow start
	feedbackHandler(Feedback{
		TaskID:    startTaskID,
		Status:    WorkflowStarted,
		Progress:  0.0,
		Timestamp: startTime,
		Metrics: map[string]interface{}{
			"flow_id": f.id,
		},
	})

	g, gCtx := errgroup.WithContext(ctx)
	finalResultChan := make(chan interface{}, 1) // Buffered channel to prevent blocking
	var finalResult interface{}
	var workflowErr error

	// Channel for graceful cleanup coordination
	cleanupDone := make(chan struct{})
	defer close(cleanupDone)

	var schedule func(taskID TaskID, input interface{})

	schedule = func(taskID TaskID, input interface{}) {
		g.Go(func() error {
			// Check for early termination before starting task
			select {
			case <-gCtx.Done():
				if errors.Is(gCtx.Err(), context.Canceled) {
					return NewContextCancelledError(taskID, gCtx.Err())
				}
				if errors.Is(gCtx.Err(), context.DeadlineExceeded) {
					return NewContextTimeoutError(taskID, gCtx.Err())
				}
				return gCtx.Err()
			default:
			}

			output, outcome, err := f.executeTaskWithRetries(gCtx, taskID, input)
			if err != nil {
				// Ignore special error from joinTask
				if errors.Is(err, ErrBranchFinished) {
					return nil
				}
				// Pass through special errors as-is
				if errors.Is(err, ErrMaxRetriesExceeded) || errors.Is(err, ErrCancelled) {
					return err
				}
				// Wrap context errors with domain-specific information
				if errors.Is(err, context.Canceled) {
					return NewContextCancelledError(taskID, err)
				}
				if errors.Is(err, context.DeadlineExceeded) {
					return NewContextTimeoutError(taskID, err)
				}
				return NewWorkflowExecutionError(taskID, "task execution failed", err)
			}

			// Now we use the actual outcome type from the task execution
			outcomeType := outcome.Type
			data := outcome.Data
			// outcomeData holds the data associated with the outcome, used for conditional branching
			// It's defined separately from data to provide clarity about its usage in condition evaluation
			outcomeData := data

			if outcomeType == OutcomeExit {
				select {
				case finalResultChan <- output:
					// Successfully sent result
				case <-gCtx.Done():
					// Context cancelled, don't block
				default:
					// No reader available, continue without blocking
					logger.Debug("Final result channel not ready, continuing",
						"task_id", taskID,
						"flow_id", f.id,
					)
				}
				return nil // End goroutine, but not the entire errgroup
			}

			if outcomeType == OutcomeContinue {
				logger.Debug("Task outcome: Continue",
					"task_id", taskID,
					"flow_id", f.id,
				)
				schedule(taskID, output) // Recursion with same task but new input
				return nil               // This execution branch is finished
			}

			// Find ALL suitable successors for parallel launch (fan-out)
			f.RLock()
			links := f.links[taskID]
			f.RUnlock()

			var nextTasks []TaskID
			for _, l := range links {
				// Check for context cancellation during link processing
				select {
				case <-gCtx.Done():
					if errors.Is(gCtx.Err(), context.Canceled) {
						return NewContextCancelledError(taskID, gCtx.Err())
					}
					if errors.Is(gCtx.Err(), context.DeadlineExceeded) {
						return NewContextTimeoutError(taskID, gCtx.Err())
					}
					return gCtx.Err()
				default:
					// Continue processing link
				}

				if l.on == outcomeType {
					if l.on == OutcomeConditional {
						if l.condition != nil && l.condition(outcomeData) {
							nextTasks = append(nextTasks, l.to)
						}
					} else { // Success
						nextTasks = append(nextTasks, l.to)
					}
				}
			}

			// Schedule execution of all next tasks
			if len(nextTasks) == 0 {
				// If no next tasks, this is a terminal task
				select {
				case finalResultChan <- output:
				case <-gCtx.Done():
					// Context cancelled, wrap with domain-specific error for consistency
					if errors.Is(gCtx.Err(), context.Canceled) {
						return NewContextCancelledError(taskID, gCtx.Err())
					}
					if errors.Is(gCtx.Err(), context.DeadlineExceeded) {
						return NewContextTimeoutError(taskID, gCtx.Err())
					}
					return gCtx.Err()
				default:
					// Channel already has a result, this is fine for terminal tasks
				}
			} else {
				for _, nextID := range nextTasks {
					if nextID != "" {
						select {
						case <-gCtx.Done():
							// Stop scheduling new tasks if context is cancelled, wrap error consistently
							if errors.Is(gCtx.Err(), context.Canceled) {
								return NewContextCancelledError(taskID, gCtx.Err())
							}
							if errors.Is(gCtx.Err(), context.DeadlineExceeded) {
								return NewContextTimeoutError(taskID, gCtx.Err())
							}
							return gCtx.Err()
						default:
							schedule(nextID, output)
						}
					}
				}
			}
			return nil
		})
	}

	// Start workflow
	schedule(startTaskID, initialInput)

	// Wait for all goroutines with proper cleanup
	workflowErr = g.Wait()

	// Close finalResultChan after all goroutines complete to prevent deadlocks
	close(finalResultChan)

	// Drain any remaining feedback events before completing
	// This helps ensure proper cleanup of feedback channels
	select {
	case <-cleanupDone:
		// Cleanup signal already sent
	default:
	}

	// Get final result with timeout to prevent hanging, but respect the original context
	f.RLock()
	timeoutConfig := f.timeoutConfig
	f.RUnlock()

	resultCtx, resultCancel := context.WithTimeout(ctx, timeoutConfig.ResultTimeout)
	defer resultCancel()

	select {
	case result := <-finalResultChan:
		finalResult = result
	case <-resultCtx.Done():
		// If no result in channel and timeout, workflow finished without explicit result
		if workflowErr == nil {
			finalResult = nil
		}
	}

	// Log workflow completion
	duration := time.Since(startTime)
	if workflowErr != nil {
		logger.Error("Workflow execution failed",
			"flow_id", f.id,
			"duration", duration,
			"error", workflowErr,
		)
		feedbackHandler(Feedback{
			Status:    WorkflowFailed,
			Progress:  1.0,
			Timestamp: time.Now(),
			Metrics: map[string]interface{}{
				"flow_id":  f.id,
				"duration": duration,
				"error":    workflowErr.Error(),
			},
		})
		return nil, workflowErr
	}

	logger.Info("Workflow execution completed",
		"flow_id", f.id,
		"duration", duration,
	)
	feedbackHandler(Feedback{
		Status:    WorkflowCompleted,
		Progress:  1.0,
		Timestamp: time.Now(),
		Metrics: map[string]interface{}{
			"flow_id":  f.id,
			"duration": duration,
		},
	})

	return finalResult, nil
}

// executeTaskWithRetries executes a single task with retry policy.
func (f *flowInternal) executeTaskWithRetries(ctx context.Context, taskID TaskID, input interface{}) (interface{}, Outcome[any], error) {
	f.RLock()
	task, exists := f.tasks[taskID]
	retryPolicy := f.retryPolicy
	logger := f.logger
	feedbackHandler := f.feedbackHandler
	f.RUnlock()

	if !exists {
		return nil, Outcome[any]{}, NewTaskNotFoundError(taskID)
	}

	taskCtx := NewContextWithExecutionState(ctx, ExecutionContext{
		TaskID:     taskID,
		RetryCount: 0,
		FlowID:     f.id,
	})

	// Add logger and feedback sender to context
	taskCtx = NewContextWithLogger(taskCtx, logger)
	taskCtx = NewContextWithFeedback(taskCtx, feedbackHandler, taskID)

	for attempt := 0; ; attempt++ {
		// Check for context cancellation at the start of each retry attempt
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil, Outcome[any]{}, NewContextCancelledError(taskID, ctx.Err())
			}
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, Outcome[any]{}, NewContextTimeoutError(taskID, ctx.Err())
			}
			return nil, Outcome[any]{}, ctx.Err()
		default:
			// Continue with retry attempt
		}

		// Log task execution start
		logger.Debug("Task execution started",
			"task_id", taskID,
			"attempt", attempt+1,
			"flow_id", f.id,
		)

		// Send feedback about task start
		feedbackHandler(Feedback{
			TaskID:      taskID,
			Status:      TaskStarted,
			Description: f.getTaskDescription(taskID, "started"),
			Progress:    0.0,
			Metrics: map[string]interface{}{
				"attempt": attempt + 1,
			},
		})

		output, outcome, err := task.execute(taskCtx, input)

		// If there's an error - end workflow with error
		if err != nil {
			logger.Error("Task execution failed with error",
				"task_id", taskID,
				"attempt", attempt+1,
				"error", err,
				"flow_id", f.id,
			)

			feedbackHandler(Feedback{
				TaskID:      taskID,
				Status:      TaskFailed,
				Description: f.getTaskDescription(taskID, "failed"),
				Progress:    1.0,
				Metrics: map[string]interface{}{
					"attempt": attempt + 1,
					"error":   err.Error(),
				},
			})

			return nil, Outcome[any]{}, err
		}

		// Check outcome for retry
		if outcome.Type == OutcomeRetry {
			logger.Warn("Task returned OutcomeRetry",
				"task_id", taskID,
				"attempt", attempt+1,
				"flow_id", f.id,
			)

			if attempt >= retryPolicy.MaxRetries {
				feedbackHandler(Feedback{
					TaskID:      taskID,
					Status:      TaskFailed,
					Description: f.getTaskDescription(taskID, "failed"),
					Progress:    0.0,
					Metrics: map[string]interface{}{
						"attempt": attempt + 1,
						"error":   "max retries exceeded",
					},
					Timestamp: time.Now(),
				})
				return nil, Outcome[any]{}, NewMaxRetriesExceededError(taskID, retryPolicy.MaxRetries, fmt.Errorf("max retries exceeded for task %s with OutcomeRetry", taskID))
			}

			// Wait before retry
			backoff := calculateBackoff(retryPolicy, attempt)
			logger.Warn("Retrying task",
				"task_id", taskID,
				"attempt", attempt+1,
				"delay", backoff,
				"flow_id", f.id,
			)

			feedbackHandler(Feedback{
				TaskID:      taskID,
				Status:      TaskRetrying,
				Description: f.getTaskDescription(taskID, "retrying"),
				Progress:    0.0,
				Metrics: map[string]interface{}{
					"attempt":    attempt + 1,
					"delay":      backoff,
					"next_retry": time.Now().Add(backoff),
				},
			})

			select {
			case <-time.After(backoff):
				// Update context for next attempt, preserving the parent context chain
				currentState := GetExecutionState(taskCtx)
				currentState.RetryCount = attempt + 1
				// Use taskCtx as parent to preserve context chain, not original ctx
				taskCtx = NewContextWithExecutionState(taskCtx, currentState)
				taskCtx = NewContextWithLogger(taskCtx, logger)
				taskCtx = NewContextWithFeedback(taskCtx, feedbackHandler, taskID)
				continue
			case <-ctx.Done():
				if errors.Is(ctx.Err(), context.Canceled) {
					return nil, Outcome[any]{}, NewContextCancelledError(taskID, ctx.Err())
				}
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					return nil, Outcome[any]{}, NewContextTimeoutError(taskID, ctx.Err())
				}
				return nil, Outcome[any]{}, NewCancelledError(taskID)
			}
		}

		// Task executed successfully
		logger.Info("Task executed successfully",
			"task_id", taskID,
			"attempt", attempt+1,
			"flow_id", f.id,
		)

		feedbackHandler(Feedback{
			TaskID:      taskID,
			Status:      TaskCompleted,
			Description: f.getTaskDescription(taskID, "completed"),
			Progress:    1.0,
			Metrics: map[string]interface{}{
				"attempt": attempt + 1,
			},
		})

		return output, outcome, nil
	}

	// This should never be reached due to infinite loop above
}

// calculateBackoff calculates delay for exponential backoff.
func calculateBackoff(policy RetryPolicy, attempt int) time.Duration {
	backoff := float64(policy.InitialDelay) * math.Pow(policy.Multiplier, float64(attempt))
	delay := time.Duration(backoff)
	if delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}
	return delay
}

// getTaskDescription returns a human-readable description for a task ID based on status.
// This is a fallback function used when no custom description is set for a task.
func getTaskDescription(taskID TaskID, status string) string {
	// Default descriptions for common task patterns
	taskKey := string(taskID)

	// For generate flow tasks, provide meaningful defaults
	switch taskKey {
	case "resolve-params":
		switch status {
		case "started":
			return "Resolving parameters"
		case "completed":
			return "Resolved parameters"
		case "failed":
			return "Failed to resolve parameters"
		case "retrying":
			return "Retrying parameter resolution"
		}
	case "create-gateway":
		switch status {
		case "started":
			return "Setting up AI gateway"
		case "completed":
			return "Set up AI gateway"
		case "failed":
			return "Failed to set up AI gateway"
		case "retrying":
			return "Retrying gateway setup"
		}
	case "generate-content":
		switch status {
		case "started":
			return "Generating content"
		case "completed":
			return "Generated content"
		case "failed":
			return "Failed to generate content"
		case "retrying":
			return "Retrying content generation"
		}
	}

	// Generic fallback based on status
	switch status {
	case "started":
		return "Running " + taskKey
	case "completed":
		return "Completed " + taskKey
	case "failed":
		return "Failed " + taskKey
	case "retrying":
		return "Retrying " + taskKey
	default:
		return taskKey
	}
}
