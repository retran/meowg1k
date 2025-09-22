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
	"errors"
	"strings"
	"testing"
)

func TestErrWorkflowValidation(t *testing.T) {
	t.Run("Basic Error", func(t *testing.T) {
		err := NewWorkflowValidationError("test validation error")

		if !errors.Is(err, ErrWorkflowValidation) {
			t.Error("Error should wrap ErrWorkflowValidation")
		}

		expectedSubstring := "test validation error"
		if !strings.Contains(err.Error(), expectedSubstring) {
			t.Errorf("Expected error to contain '%s', got '%s'", expectedSubstring, err.Error())
		}
	})

	t.Run("Error with Details", func(t *testing.T) {
		details := map[string]interface{}{
			"task_id": "test_task",
			"count":   5,
		}

		err := NewWorkflowValidationError("validation failed", details)

		if !errors.Is(err, ErrWorkflowValidation) {
			t.Error("Error should wrap ErrWorkflowValidation")
		}

		errorStr := err.Error()
		if !strings.Contains(errorStr, "validation failed") {
			t.Errorf("Expected error to contain validation message, got '%s'", errorStr)
		}

		if !strings.Contains(errorStr, "test_task") {
			t.Errorf("Expected error to contain details, got '%s'", errorStr)
		}
	})
}

func TestErrTaskNotFound(t *testing.T) {
	err := NewTaskNotFoundError("missing_task")

	if !errors.Is(err, ErrTaskNotFound) {
		t.Error("Error should wrap ErrTaskNotFound")
	}

	if !strings.Contains(err.Error(), "missing_task") {
		t.Errorf("Expected error to contain task ID, got '%s'", err.Error())
	}
}

func TestErrMaxRetriesExceeded(t *testing.T) {
	t.Run("Without Last Error", func(t *testing.T) {
		err := NewMaxRetriesExceededError("retry_task", 3, nil)

		if !errors.Is(err, ErrMaxRetriesExceeded) {
			t.Error("Error should wrap ErrMaxRetriesExceeded")
		}

		errorStr := err.Error()
		if !strings.Contains(errorStr, "retry_task") {
			t.Errorf("Expected error to contain task ID, got '%s'", errorStr)
		}

		if !strings.Contains(errorStr, "3") {
			t.Errorf("Expected error to contain retry count, got '%s'", errorStr)
		}
	})

	t.Run("With Last Error", func(t *testing.T) {
		lastErr := errors.New("original error")
		err := NewMaxRetriesExceededError("retry_task", 2, lastErr)

		if !errors.Is(err, ErrMaxRetriesExceeded) {
			t.Error("Error should wrap ErrMaxRetriesExceeded")
		}

		if !errors.Is(err, lastErr) {
			t.Error("Error should also wrap the last error")
		}

		errorStr := err.Error()
		if !strings.Contains(errorStr, "original error") {
			t.Errorf("Expected error to contain last error, got '%s'", errorStr)
		}
	})
}

func TestErrInvalidInput(t *testing.T) {
	err := NewInvalidInputError("test_task", "string", "int")

	if !errors.Is(err, ErrInvalidInput) {
		t.Error("Error should wrap ErrInvalidInput")
	}

	errorStr := err.Error()
	expectedParts := []string{"test_task", "string", "int"}

	for _, part := range expectedParts {
		if !strings.Contains(errorStr, part) {
			t.Errorf("Expected error to contain '%s', got '%s'", part, errorStr)
		}
	}
}

func TestErrWorkflowExecution(t *testing.T) {
	t.Run("Without Cause", func(t *testing.T) {
		err := NewWorkflowExecutionError("exec_task", "execution failed", nil)

		if !errors.Is(err, ErrWorkflowExecution) {
			t.Error("Error should wrap ErrWorkflowExecution")
		}

		errorStr := err.Error()
		if !strings.Contains(errorStr, "exec_task") {
			t.Errorf("Expected error to contain task ID, got '%s'", errorStr)
		}

		if !strings.Contains(errorStr, "execution failed") {
			t.Errorf("Expected error to contain message, got '%s'", errorStr)
		}
	})

	t.Run("With Cause", func(t *testing.T) {
		cause := errors.New("root cause")
		err := NewWorkflowExecutionError("exec_task", "execution failed", cause)

		if !errors.Is(err, ErrWorkflowExecution) {
			t.Error("Error should wrap ErrWorkflowExecution")
		}

		if !errors.Is(err, cause) {
			t.Error("Error should also wrap the cause")
		}

		errorStr := err.Error()
		if !strings.Contains(errorStr, "root cause") {
			t.Errorf("Expected error to contain cause, got '%s'", errorStr)
		}
	})
}

func TestErrCancelled(t *testing.T) {
	err := NewCancelledError("cancelled_task")

	if !errors.Is(err, ErrCancelled) {
		t.Error("Error should wrap ErrCancelled")
	}

	if !strings.Contains(err.Error(), "cancelled_task") {
		t.Errorf("Expected error to contain task ID, got '%s'", err.Error())
	}
}

func TestStandardErrors(t *testing.T) {
	standardErrors := []error{
		ErrStartTaskNotSet,
		ErrCircularDependency,
		ErrWorkflowValidation,
		ErrTaskNotFound,
		ErrMaxRetriesExceeded,
		ErrInvalidInput,
		ErrWorkflowExecution,
		ErrCancelled,
		ErrBranchFinished,
	}

	for _, err := range standardErrors {
		if err.Error() == "" {
			t.Errorf("Standard error should have non-empty message: %v", err)
		}
	}
}

func TestErrorWrapping(t *testing.T) {
	// Test that our error constructors properly wrap base errors
	originalErr := errors.New("original error")

	wrappedErrors := []error{
		NewMaxRetriesExceededError("task1", 3, originalErr),
		NewWorkflowExecutionError("task2", "failed", originalErr),
	}

	for _, wrappedErr := range wrappedErrors {
		if !errors.Is(wrappedErr, originalErr) {
			t.Errorf("Error should wrap original error: %v", wrappedErr)
		}
	}
}
