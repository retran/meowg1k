// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package retrievecontext

import (
	"context"
	"errors"
	"testing"

	"github.com/retran/meowg1k/internal/core/retrieval"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockRetrievalService is a mock implementation of retrieval.Retriever.
type mockRetrievalService struct {
	RetrieveContextFn func(ctx context.Context, queryText string, snapshotPriority []string, topK int, minScore float32) (string, error)
}

func (m *mockRetrievalService) Search(ctx context.Context, queryText string, snapshotPriority []string, topK int, minScore float32) ([]retrieval.SearchResult, error) {
	// Not used in retrievecontext activity
	return nil, nil
}

func (m *mockRetrievalService) RetrieveContext(ctx context.Context, queryText string, snapshotPriority []string, topK int, minScore float32) (string, error) {
	if m.RetrieveContextFn != nil {
		return m.RetrieveContextFn(ctx, queryText, snapshotPriority, topK, minScore)
	}
	return "", nil
}

func TestNewFactory(t *testing.T) {
	t.Run("should succeed with valid retrieval service", func(t *testing.T) {
		mockService := &mockRetrievalService{}
		factory, err := NewFactory(mockService)
		if err != nil {
			t.Fatalf("expected no error, but got %v", err)
		}
		if factory == nil {
			t.Fatal("factory should not be nil")
		}
	})

	t.Run("should fail with nil retrieval service", func(t *testing.T) {
		_, err := NewFactory(nil)
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		expectedErr := "retrievecontext.NewFactory: retrievalService cannot be nil"
		if err.Error() != expectedErr {
			t.Errorf("expected error message '%s', but got '%s'", expectedErr, err.Error())
		}
	})
}

func TestActivity(t *testing.T) {
	mockService := &mockRetrievalService{}
	factory, _ := NewFactory(mockService)
	activity := factory.NewActivity()

	// Valid input for most tests
	validInput := &Input{
		QueryText:        "test query",
		SnapshotPriority: []string{"_workdir_", "_stage_", "_head_"},
		TopK:             10,
		MinScore:         0.5,
	}

	t.Run("should succeed with valid input", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		var feedbackMessages []*executor.Feedback
		feedbackHandler := func(feedback *executor.Feedback) {
			feedbackMessages = append(feedbackMessages, feedback)
		}
		executorCtx := executor.NewContext("test", feedbackHandler, executor.NewExecutor(0))

		expectedContext := "file1.go:10-20\nfunc test() {\n    // test code\n}\n\nfile2.go:5-15\ntype TestStruct struct {\n    field string\n}"

		mockService.RetrieveContextFn = func(ctx context.Context, queryText string, snapshotPriority []string, topK int, minScore float32) (string, error) {
			// Verify parameters
			if queryText != "test query" {
				t.Errorf("expected queryText 'test query', got '%s'", queryText)
			}
			if len(snapshotPriority) != 3 {
				t.Errorf("expected 3 snapshots, got %d", len(snapshotPriority))
			}
			if topK != 10 {
				t.Errorf("expected topK 10, got %d", topK)
			}
			if minScore != 0.5 {
				t.Errorf("expected minScore 0.5, got %f", minScore)
			}
			return expectedContext, nil
		}

		// Act
		output, err := activity(ctx, executorCtx, validInput)
		// Assert
		if err != nil {
			t.Fatalf("activity failed: %v", err)
		}
		if output == nil {
			t.Fatal("output should not be nil")
		}
		if output.Context != expectedContext {
			t.Errorf("expected context '%s', got '%s'", expectedContext, output.Context)
		}

		// Check feedback messages
		if len(feedbackMessages) != 2 {
			t.Fatalf("expected 2 feedback messages, got %d", len(feedbackMessages))
		}
		if feedbackMessages[0].Message != `Retrieving context for: "test query"` {
			t.Errorf("unexpected running message: %s", feedbackMessages[0].Message)
		}
		if feedbackMessages[1].Message != "Context retrieved" {
			t.Errorf("unexpected completion message: %s", feedbackMessages[1].Message)
		}
	})

	t.Run("should fail with nil input", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, executor.NewExecutor(0))

		// Act
		_, err := activity(ctx, executorCtx, nil)

		// Assert
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if err.Error() != "input cannot be nil" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("should fail with empty query text", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, executor.NewExecutor(0))
		input := &Input{
			QueryText:        "",
			SnapshotPriority: []string{"_workdir_"},
			TopK:             10,
			MinScore:         0.5,
		}

		// Act
		_, err := activity(ctx, executorCtx, input)

		// Assert
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if err.Error() != "query text cannot be empty" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("should fail with empty snapshot priority", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, executor.NewExecutor(0))
		input := &Input{
			QueryText:        "test",
			SnapshotPriority: []string{},
			TopK:             10,
			MinScore:         0.5,
		}

		// Act
		_, err := activity(ctx, executorCtx, input)

		// Assert
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if err.Error() != "snapshot priority list cannot be empty" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("should fail with invalid topK", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, executor.NewExecutor(0))
		input := &Input{
			QueryText:        "test",
			SnapshotPriority: []string{"_workdir_"},
			TopK:             0,
			MinScore:         0.5,
		}

		// Act
		_, err := activity(ctx, executorCtx, input)

		// Assert
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if err.Error() != "topK must be positive, got 0" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("should fail when retrieval service returns error", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, executor.NewExecutor(0))
		retrievalErr := errors.New("retrieval service error")

		mockService.RetrieveContextFn = func(ctx context.Context, queryText string, snapshotPriority []string, topK int, minScore float32) (string, error) {
			return "", retrievalErr
		}

		// Act
		_, err := activity(ctx, executorCtx, validInput)

		// Assert
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if !errors.Is(err, retrievalErr) {
			t.Errorf("expected error to wrap retrieval error, got: %v", err)
		}
	})

	t.Run("should handle empty context", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		var feedbackMessages []*executor.Feedback
		feedbackHandler := func(feedback *executor.Feedback) {
			feedbackMessages = append(feedbackMessages, feedback)
		}
		executorCtx := executor.NewContext("test", feedbackHandler, executor.NewExecutor(0))

		mockService.RetrieveContextFn = func(ctx context.Context, queryText string, snapshotPriority []string, topK int, minScore float32) (string, error) {
			return "", nil
		}

		// Act
		output, err := activity(ctx, executorCtx, validInput)
		// Assert
		if err != nil {
			t.Fatalf("activity failed: %v", err)
		}
		if output.Context != "" {
			t.Errorf("expected empty context, got '%s'", output.Context)
		}
		if len(feedbackMessages) != 2 {
			t.Fatalf("expected 2 feedback messages, got %d", len(feedbackMessages))
		}
		if feedbackMessages[1].Message != "No context found" {
			t.Errorf("unexpected completion message: %s", feedbackMessages[1].Message)
		}
	})
}
