// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package embedall

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/retran/meowg1k/internal/activities/buildbatches"
	"github.com/retran/meowg1k/internal/activities/embedbatch"
	"github.com/retran/meowg1k/internal/activities/splitfiles"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/pkg/executor"
)

const batchID = "Batch_0-2"

// mockComputeBatchFactory is a mock of the embedbatch factory.
type mockComputeBatchFactory struct{}

func (m *mockComputeBatchFactory) NewActivity() executor.Activity[*embedbatch.Input, *embedbatch.Output] {
	// The activity itself is mocked at the executor level, so this can be a no-op.
	return func(ctx context.Context, executorCtx *executor.Context, input *embedbatch.Input) (*embedbatch.Output, error) {
		return nil, nil
	}
}

// mockExecutor is a mock implementation of the executor.
type mockExecutor struct {
	executeActivityFn func(ctx context.Context, parentCtx *executor.Context, name string, activity executor.Activity[any, any], input any) (any, error)
}

func (m *mockExecutor) ExecuteActivity(ctx context.Context, parentCtx *executor.Context, name string, activity executor.Activity[any, any], input any) (any, error) {
	if m.executeActivityFn != nil {
		return m.executeActivityFn(ctx, parentCtx, name, activity, input)
	}
	return nil, fmt.Errorf("mock not configured")
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
	t.Run("should succeed with a valid factory", func(t *testing.T) {
		mockChildFactory := &mockComputeBatchFactory{}
		factory, err := NewFactory(mockChildFactory)
		if err != nil {
			t.Fatalf("expected no error, but got %v", err)
		}
		if factory == nil {
			t.Fatal("factory should not be nil")
		}
	})

	t.Run("should fail with a nil factory", func(t *testing.T) {
		_, err := NewFactory(nil)
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		expectedErr := "embedall.NewFactory: computeBatchFactory cannot be nil"
		if err.Error() != expectedErr {
			t.Errorf("expected error message '%s', but got '%s'", expectedErr, err.Error())
		}
	})
}

func TestActivity(t *testing.T) {
	mockChildFactory := &mockComputeBatchFactory{}
	factory, _ := NewFactory(mockChildFactory)
	activity := factory.NewActivity()
	exec := &mockExecutor{}

	// Common setup for successful execution
	preparedBatches := &buildbatches.Output{
		ChunkResults: &splitfiles.Output{
			AllChunkTexts: []string{"text1", "text2", "text3"},
		},
		Batches: []buildbatches.Batch{
			{StartIndex: 0, EndIndex: 2, Texts: []string{"text1", "text2"}},
			{StartIndex: 2, EndIndex: 3, Texts: []string{"text3"}},
		},
	}
	input := &Input{
		StateName:       "test_state",
		PreparedBatches: preparedBatches,
	}

	t.Run("should succeed with multiple batches", func(t *testing.T) {
		// Arrange
		ctx := context.Background()

		exec.executeActivityFn = func(ctx context.Context, parentCtx *executor.Context, activityID string, act executor.Activity[any, any], params any) (any, error) {
			output := &embedbatch.Output{}
			switch activityID {
			case batchID:
				output.Embeddings = []gateway.Embedding{{1}, {2}}
			case "Batch_2-3":
				output.Embeddings = []gateway.Embedding{{3}}
			}
			return output, nil
		}

		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

		// Act
		output, err := activity(ctx, executorCtx, input)
		// Assert
		if err != nil {
			t.Fatalf("activity failed: %v", err)
		}
		if len(output.Embeddings) != 3 {
			t.Fatalf("expected 3 embeddings, got %d", len(output.Embeddings))
		}
		if output.Embeddings[0][0] != 1 || output.Embeddings[1][0] != 2 || output.Embeddings[2][0] != 3 {
			t.Errorf("embeddings were not assembled correctly: %v", output.Embeddings)
		}
	})

	t.Run("should succeed with no batches", func(t *testing.T) {
		// Arrange
		ctx := context.Background()

		var feedbackMessages []*executor.Feedback
		feedbackHandler := func(feedback *executor.Feedback) {
			feedbackMessages = append(feedbackMessages, feedback)
		}

		executorCtx := executor.NewContext("test", feedbackHandler, exec)
		emptyInput := &Input{
			StateName: "empty",
			PreparedBatches: &buildbatches.Output{
				ChunkResults: &splitfiles.Output{AllChunkTexts: []string{}},
				Batches:      []buildbatches.Batch{},
			},
		}

		// Act
		output, err := activity(ctx, executorCtx, emptyInput)
		// Assert
		if err != nil {
			t.Fatalf("activity failed: %v", err)
		}
		if len(output.Embeddings) != 0 {
			t.Errorf("expected 0 embeddings, got %d", len(output.Embeddings))
		}
		if len(feedbackMessages) != 2 || feedbackMessages[1].Message != "I've got no embeddings to compute" {
			t.Errorf("expected 'I've got no embeddings to compute' completion message, got %+v", feedbackMessages)
		}
	})

	t.Run("should fail if executor is not in context", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, nil) // No executor

		// Act
		_, err := activity(ctx, executorCtx, input)

		// Assert
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if err.Error() != "executor not available in context" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("should fail if a child activity fails", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		childErr := errors.New("batch failed")

		exec2 := &mockExecutor{}
		exec2.executeActivityFn = func(ctx context.Context, parentCtx *executor.Context, activityID string, act executor.Activity[any, any], params any) (any, error) {
			if activityID == batchID {
				return nil, childErr
			}
			return &embedbatch.Output{}, nil
		}

		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec2)

		// Act
		_, err := activity(ctx, executorCtx, input)

		// Assert
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if !errors.Is(err, childErr) {
			t.Errorf("expected error to wrap '%v'", childErr)
		}
	})

	t.Run("should succeed even with fewer embeddings than expected per batch", func(t *testing.T) {
		// Note: The current implementation doesn't validate embedding counts per batch
		// Arrange
		ctx := context.Background()

		exec3 := &mockExecutor{}
		exec3.executeActivityFn = func(ctx context.Context, parentCtx *executor.Context, activityID string, act executor.Activity[any, any], params any) (any, error) {
			var output *embedbatch.Output
			// Return fewer embeddings than expected for the first batch
			if activityID == batchID {
				output = &embedbatch.Output{Embeddings: []gateway.Embedding{{1}}} // Should be 2, but only 1
			} else {
				output = &embedbatch.Output{Embeddings: []gateway.Embedding{{3}}} // Should be 1, and is 1
			}
			return output, nil
		}

		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec3)

		// Act
		output, err := activity(ctx, executorCtx, input)
		// Assert - currently the implementation doesn't validate per-batch counts
		if err != nil {
			t.Fatalf("activity failed: %v", err)
		}

		// The result will have the expected total length but some zero embeddings
		if len(output.Embeddings) != 3 {
			t.Errorf("expected 3 total embeddings, got %d", len(output.Embeddings))
		}
	})
}
