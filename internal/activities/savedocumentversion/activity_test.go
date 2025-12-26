// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package savedocumentversion

import (
	"context"
	"errors"
	"testing"

	"github.com/retran/meowg1k/internal/core/index"
	"github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockIndexService is a mock implementation of ports.IndexService.
type mockIndexService struct {
	SaveNewVersionFn func(ctx context.Context, input interface{}) (interface{}, error)
}

func (m *mockIndexService) PrepareForProcessing(ctx context.Context, workspaceState interface{}) (interface{}, error) {
	return nil, errors.New("PrepareForProcessing not implemented in mock")
}

func (m *mockIndexService) SaveNewVersion(ctx context.Context, input interface{}) (interface{}, error) {
	if m.SaveNewVersionFn != nil {
		return m.SaveNewVersionFn(ctx, input)
	}
	return nil, nil
}

func (m *mockIndexService) FinalizeLiveSnapshots(ctx context.Context, input interface{}) error {
	return errors.New("FinalizeLiveSnapshots not implemented in mock")
}

func TestNewFactory(t *testing.T) {
	t.Run("should succeed with valid index service", func(t *testing.T) {
		mockService := &mockIndexService{}
		factory, err := NewFactory(mockService)
		if err != nil {
			t.Fatalf("expected no error, but got %v", err)
		}
		if factory == nil {
			t.Fatal("factory should not be nil")
		}
	})

	t.Run("should fail with nil index service", func(t *testing.T) {
		_, err := NewFactory(nil)
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		expectedErr := "savedocumentversion.NewFactory: indexService cannot be nil"
		if err.Error() != expectedErr {
			t.Errorf("expected error message '%s', but got '%s'", expectedErr, err.Error())
		}
	})
}

func TestActivity(t *testing.T) {
	mockService := &mockIndexService{}
	factory, _ := NewFactory(mockService)
	activity := factory.NewActivity()

	// Valid input for most tests
	validInput := &Input{
		FilePath:    "test/file.go",
		Content:     []byte("package main\n\nfunc main() {}"),
		ContentHash: "abc123",
		Chunks: []domainindex.ChunkData{
			{
				StartLine:   1,
				EndLine:     3,
				TextContent: "package main\n\nfunc main() {}",
			},
		},
		Embeddings: []gateway.Embedding{
			{0.1, 0.2, 0.3, 0.4, 0.5},
		},
	}

	t.Run("should succeed with valid input", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		var feedbackMessages []*executor.Feedback
		feedbackHandler := func(feedback *executor.Feedback) {
			feedbackMessages = append(feedbackMessages, feedback)
		}
		executorCtx := executor.NewContext("test", feedbackHandler, executor.NewExecutor(0))

		expectedOutput := &index.SaveVersionOutput{
			FilePath:  "test/file.go",
			VersionID: 123,
		}

		saveCallCount := 0
		mockService.SaveNewVersionFn = func(ctx context.Context, input interface{}) (interface{}, error) {
			saveCallCount++
			// Verify the input type and content
			saveInput, ok := input.(*index.SaveVersionInput)
			if !ok {
				t.Errorf("expected *index.SaveVersionInput, got %T", input)
				return nil, errors.New("invalid input type")
			}
			if saveInput.FilePath != "test/file.go" {
				t.Errorf("expected FilePath 'test/file.go', got '%s'", saveInput.FilePath)
			}
			if saveInput.ContentHash != "abc123" {
				t.Errorf("expected ContentHash 'abc123', got '%s'", saveInput.ContentHash)
			}
			if len(saveInput.Chunks) != 1 {
				t.Errorf("expected 1 chunk, got %d", len(saveInput.Chunks))
			}
			if len(saveInput.Embeddings) != 1 {
				t.Errorf("expected 1 embedding, got %d", len(saveInput.Embeddings))
			}
			return expectedOutput, nil
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
		if output.FilePath != "test/file.go" {
			t.Errorf("expected FilePath 'test/file.go', got '%s'", output.FilePath)
		}
		if output.VersionID != 123 {
			t.Errorf("expected VersionID 123, got %d", output.VersionID)
		}
		if saveCallCount != 1 {
			t.Errorf("expected SaveNewVersion to be called once, got %d", saveCallCount)
		}

		// Check feedback messages
		if len(feedbackMessages) != 2 {
			t.Fatalf("expected 2 feedback messages, got %d", len(feedbackMessages))
		}
		if feedbackMessages[0].Message != "Saving: test/file.go" {
			t.Errorf("unexpected running message: %s", feedbackMessages[0].Message)
		}
		if feedbackMessages[1].Message != "test/file.go: 1 chunks" {
			t.Errorf("unexpected completion message: %s", feedbackMessages[1].Message)
		}
	})

	t.Run("should fail when save service returns error", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, executor.NewExecutor(0))
		saveErr := errors.New("save service error")

		mockService.SaveNewVersionFn = func(ctx context.Context, input interface{}) (interface{}, error) {
			return nil, saveErr
		}

		// Act
		_, err := activity(ctx, executorCtx, validInput)

		// Assert
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if !errors.Is(err, saveErr) {
			t.Errorf("expected error to wrap save error, got: %v", err)
		}
	})

	t.Run("should fail when save service returns wrong type", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, executor.NewExecutor(0))

		mockService.SaveNewVersionFn = func(ctx context.Context, input interface{}) (interface{}, error) {
			return "wrong type", nil
		}

		// Act
		_, err := activity(ctx, executorCtx, validInput)

		// Assert
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if err.Error() != "unexpected result type from SaveNewVersion" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("should handle empty chunks and embeddings", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		var feedbackMessages []*executor.Feedback
		feedbackHandler := func(feedback *executor.Feedback) {
			feedbackMessages = append(feedbackMessages, feedback)
		}
		executorCtx := executor.NewContext("test", feedbackHandler, executor.NewExecutor(0))

		emptyInput := &Input{
			FilePath:    "test/empty.txt",
			Content:     []byte(""),
			ContentHash: "empty123",
			Chunks:      []domainindex.ChunkData{},
			Embeddings:  []gateway.Embedding{},
		}

		expectedOutput := &index.SaveVersionOutput{
			FilePath:  "test/empty.txt",
			VersionID: 456,
		}

		mockService.SaveNewVersionFn = func(ctx context.Context, input interface{}) (interface{}, error) {
			saveInput, ok := input.(*index.SaveVersionInput)
			if !ok {
				return nil, errors.New("invalid input type")
			}
			if len(saveInput.Chunks) != 0 {
				t.Errorf("expected 0 chunks, got %d", len(saveInput.Chunks))
			}
			if len(saveInput.Embeddings) != 0 {
				t.Errorf("expected 0 embeddings, got %d", len(saveInput.Embeddings))
			}
			return expectedOutput, nil
		}

		// Act
		output, err := activity(ctx, executorCtx, emptyInput)
		// Assert
		if err != nil {
			t.Fatalf("activity failed: %v", err)
		}
		if output.FilePath != "test/empty.txt" {
			t.Errorf("expected FilePath 'test/empty.txt', got '%s'", output.FilePath)
		}
		if feedbackMessages[1].Message != "test/empty.txt: 0 chunks" {
			t.Errorf("unexpected completion message: expected 'test/empty.txt: 0 chunks', got '%s'", feedbackMessages[1].Message)
		}
	})

	t.Run("should handle multiple chunks and embeddings", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		var feedbackMessages []*executor.Feedback
		feedbackHandler := func(feedback *executor.Feedback) {
			feedbackMessages = append(feedbackMessages, feedback)
		}
		executorCtx := executor.NewContext("test", feedbackHandler, executor.NewExecutor(0))

		multiInput := &Input{
			FilePath:    "test/multi.go",
			Content:     []byte("package main\n\nfunc main() {}\n\nfunc helper() {}"),
			ContentHash: "multi123",
			Chunks: []domainindex.ChunkData{
				{StartLine: 1, EndLine: 3, TextContent: "package main\n\nfunc main() {}"},
				{StartLine: 3, EndLine: 5, TextContent: "func main() {}\n\nfunc helper() {}"},
			},
			Embeddings: []gateway.Embedding{
				{0.1, 0.2, 0.3, 0.4, 0.5},
				{0.6, 0.7, 0.8, 0.9, 1.0},
			},
		}

		expectedOutput := &index.SaveVersionOutput{
			FilePath:  "test/multi.go",
			VersionID: 789,
		}

		mockService.SaveNewVersionFn = func(ctx context.Context, input interface{}) (interface{}, error) {
			saveInput, ok := input.(*index.SaveVersionInput)
			if !ok {
				return nil, errors.New("invalid input type")
			}
			if len(saveInput.Chunks) != 2 {
				t.Errorf("expected 2 chunks, got %d", len(saveInput.Chunks))
			}
			if len(saveInput.Embeddings) != 2 {
				t.Errorf("expected 2 embeddings, got %d", len(saveInput.Embeddings))
			}
			return expectedOutput, nil
		}

		// Act
		output, err := activity(ctx, executorCtx, multiInput)
		// Assert
		if err != nil {
			t.Fatalf("activity failed: %v", err)
		}
		if output.VersionID != 789 {
			t.Errorf("expected VersionID 789, got %d", output.VersionID)
		}
		if feedbackMessages[1].Message != "test/multi.go: 2 chunks" {
			t.Errorf("unexpected completion message: expected 'test/multi.go: 2 chunks', got '%s'", feedbackMessages[1].Message)
		}
	})
}
