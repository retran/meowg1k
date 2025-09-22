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
)

// Standard error values for workflow operations.
var (
	// Validation errors
	ErrStartTaskNotSet    = errors.New("start task is not set")
	ErrCircularDependency = errors.New("circular dependency detected in workflow")
	ErrWorkflowValidation = errors.New("workflow validation failed")

	// Execution errors
	ErrTaskNotFound       = errors.New("task not found")
	ErrMaxRetriesExceeded = errors.New("maximum retries exceeded")
	ErrInvalidInput       = errors.New("invalid input type")
	ErrWorkflowExecution  = errors.New("workflow execution failed")
	ErrCancelled          = errors.New("workflow cancelled")

	// Special errors
	ErrBranchFinished = errors.New("branch finished at join point")
)

// Helper functions for creating wrapped errors with context.

func NewWorkflowValidationError(message string, details ...map[string]interface{}) error {
	err := fmt.Errorf("%w: %s", ErrWorkflowValidation, message)
	if len(details) > 0 && len(details[0]) > 0 {
		return fmt.Errorf("%w (details: %v)", err, details[0])
	}
	return err
}

func NewTaskNotFoundError(taskID TaskID) error {
	return fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
}

func NewMaxRetriesExceededError(taskID TaskID, maxRetries int, lastError error) error {
	if lastError != nil {
		return fmt.Errorf("%w: task %s failed after %d retries: %w", ErrMaxRetriesExceeded, taskID, maxRetries, lastError)
	}
	return fmt.Errorf("%w: task %s failed after %d retries", ErrMaxRetriesExceeded, taskID, maxRetries)
}

func NewInvalidInputError(taskID TaskID, expectedType, actualType string) error {
	return fmt.Errorf("%w: task %s expected %s, got %s", ErrInvalidInput, taskID, expectedType, actualType)
}

func NewWorkflowExecutionError(taskID TaskID, message string, cause error) error {
	if cause != nil {
		return fmt.Errorf("%w: task %s: %s: %w", ErrWorkflowExecution, taskID, message, cause)
	}
	return fmt.Errorf("%w: task %s: %s", ErrWorkflowExecution, taskID, message)
}

func NewCancelledError(taskID TaskID) error {
	return fmt.Errorf("%w: at task %s", ErrCancelled, taskID)
}

func NewContextCancelledError(taskID TaskID, cause error) error {
	// Provide more detailed context based on the type of cancellation
	var contextType string
	var details string

	if cause != nil {
		switch cause {
		case context.Canceled:
			contextType = "user cancellation"
			details = "operation was cancelled by user request"
		case context.DeadlineExceeded:
			contextType = "timeout"
			details = "operation exceeded configured timeout"
		default:
			contextType = "context cancellation"
			details = cause.Error()
		}
	} else {
		contextType = "unknown cancellation"
		details = "context was cancelled for unknown reason"
	}

	return fmt.Errorf("%w: task %s cancelled by %s (%s): %w",
		ErrCancelled, taskID, contextType, details, cause)
}

func NewContextTimeoutError(taskID TaskID, cause error) error {
	return fmt.Errorf("%w: task %s timed out: %w", ErrCancelled, taskID, cause)
}
