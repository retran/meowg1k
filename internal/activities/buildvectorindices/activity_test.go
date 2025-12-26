// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package buildvectorindices

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockVectorIndexService is a mock implementation of the ports.VectorIndexService.
type mockVectorIndexService struct {
	// Intentionally empty as its methods are not directly called by the parent activity.
}

// Ensure mockVectorIndexService implements ports.VectorIndexService.
var _ ports.VectorIndexService = (*mockVectorIndexService)(nil)

func (m *mockVectorIndexService) BuildAndSave(snapshotName string) error {
	// This method is called within the child activity, which is mocked at the executor level.
	return nil
}

// mockExecutor is a mock implementation of the executor.Executor.
type mockExecutor struct {
	executeActivityFn func(ctx context.Context, parentCtx *executor.Context, activityID string, activity executor.Activity[any, any], params any) (any, error)
}

func (m *mockExecutor) ExecuteActivity(
	ctx context.Context,
	parentCtx *executor.Context,
	activityID string,
	activity executor.Activity[any, any],
	params any,
) (any, error) {
	if m.executeActivityFn != nil {
		return m.executeActivityFn(ctx, parentCtx, activityID, activity, params)
	}
	return struct{}{}, nil
}

func (m *mockExecutor) ExecuteFlow(ctx context.Context, name string, flow executor.Flow) error {
	return nil
}

func (m *mockExecutor) WithRetryPolicy(policy *executor.RetryPolicy) executor.Executor {
	return m
}

func (m *mockExecutor) WithFeedbackHandler(handler executor.FeedbackHandler) executor.Executor {
	return m
}

func TestNewFactory(t *testing.T) {
	t.Run("should return factory when service is not nil", func(t *testing.T) {
		mockSvc := &mockVectorIndexService{}
		factory, err := NewFactory(mockSvc)
		if err != nil {
			t.Fatalf("expected no error, but got: %v", err)
		}
		if factory == nil {
			t.Fatal("expected factory to be not nil")
		}
	})

	t.Run("should return error when service is nil", func(t *testing.T) {
		factory, err := NewFactory(nil)
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		expectedErr := "buildvectorindices.NewFactory: vectorIndexSvc cannot be nil"
		if err.Error() != expectedErr {
			t.Errorf("expected error message '%s', but got '%s'", expectedErr, err.Error())
		}
		if factory != nil {
			t.Fatal("expected factory to be nil")
		}
	})
}

func TestActivity(t *testing.T) {
	mockSvc := &mockVectorIndexService{}
	factory, err := NewFactory(mockSvc)
	if err != nil {
		t.Fatalf("failed to create factory: %v", err)
	}
	activity := factory.NewActivity()

	t.Run("should succeed when all child activities succeed", func(t *testing.T) {
		// Arrange
		exec := &mockExecutor{}
		ctx := context.Background()

		// Capture feedback messages
		var feedbackMessages []*executor.Feedback
		feedbackHandler := func(feedback *executor.Feedback) {
			feedbackMessages = append(feedbackMessages, feedback)
		}

		executorCtx := executor.NewContext("test", feedbackHandler, exec)

		// Configure the mock executor to return successful results.
		exec.executeActivityFn = func(ctx context.Context, parentCtx *executor.Context, activityID string, activity executor.Activity[any, any], params any) (any, error) {
			return struct{}{}, nil
		}

		// Act
		_, err := activity(ctx, executorCtx, struct{}{})
		// Assert
		if err != nil {
			t.Fatalf("activity returned an unexpected error: %v", err)
		}

		if len(feedbackMessages) != 2 {
			t.Fatalf("expected 2 feedback messages, but got %d", len(feedbackMessages))
		}
		if feedbackMessages[0].Status != executor.StatusRunning || feedbackMessages[0].Message != "Building vector indices" {
			t.Errorf("unexpected running message: got %+v", feedbackMessages[0])
		}
		if feedbackMessages[1].Status != executor.StatusCompleted || feedbackMessages[1].Message != "Vector indices ready" {
			t.Errorf("unexpected completed message: got %+v", feedbackMessages[1])
		}
	})

	t.Run("should fail if executor is not in context", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, nil) // No executor set

		// Act
		_, err := activity(ctx, executorCtx, struct{}{})

		// Assert
		if err == nil {
			t.Fatal("expected an error but got nil")
		}
		expectedErr := "executor not available in activity context"
		if err.Error() != expectedErr {
			t.Errorf("expected error '%s', but got '%s'", expectedErr, err.Error())
		}
	})

	// Test failure scenarios for each child activity
	testCases := []struct {
		name          string
		failingChild  string
		expectedError string
	}{
		{"head fails", "build-vector-index-head", "failed to build _head_ index: execute activity \"build-vector-index-head\": child failed"},
		{"stage fails", "build-vector-index-stage", "failed to build _stage_ index: execute activity \"build-vector-index-stage\": child failed"},
		{"workdir fails", "build-vector-index-workdir", "failed to build _workdir_ index: execute activity \"build-vector-index-workdir\": child failed"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("should fail when %s", tc.name), func(t *testing.T) {
			// Arrange
			exec := &mockExecutor{}
			ctx := context.Background()
			executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)
			childErr := errors.New("child failed")

			// Configure the mock executor to return a failing result for the specific child
			exec.executeActivityFn = func(ctx context.Context, parentCtx *executor.Context, activityID string, activity executor.Activity[any, any], params any) (any, error) {
				if activityID == tc.failingChild {
					return nil, childErr
				}
				return struct{}{}, nil
			}

			// Act
			_, err := activity(ctx, executorCtx, struct{}{})

			// Assert
			if err == nil {
				t.Fatalf("expected an error but got nil")
			}
			if !errors.Is(err, childErr) {
				t.Fatalf("expected error to wrap '%v', but it did not", childErr)
			}
			if err.Error() != tc.expectedError {
				t.Errorf("unexpected error message.\nExpected: %s\nGot:      %s", tc.expectedError, err.Error())
			}
		})
	}
}
