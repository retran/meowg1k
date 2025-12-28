// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package splitfiles

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/retran/meowg1k/internal/activities/splitfile"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockChunkFileFactory is a mock implementation of the splitfile factory.
type mockChunkFileFactory struct {
	activity executor.Activity[*splitfile.Input, *splitfile.Output]
}

func (m *mockChunkFileFactory) NewActivity() executor.Activity[*splitfile.Input, *splitfile.Output] {
	return m.activity
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
	t.Run("should succeed with valid factory", func(t *testing.T) {
		mockChildFactory := &mockChunkFileFactory{}
		factory, err := NewFactory(mockChildFactory)
		if err != nil {
			t.Fatalf("expected no error, but got %v", err)
		}
		if factory == nil {
			t.Fatal("factory should not be nil")
		}
	})

	t.Run("should fail with nil factory", func(t *testing.T) {
		_, err := NewFactory(nil)
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		expectedErr := "splitfiles.NewFactory: chunkFileFactory cannot be nil"
		if err.Error() != expectedErr {
			t.Errorf("expected error message '%s', but got '%s'", expectedErr, err.Error())
		}
	})
}

func TestActivity(t *testing.T) {
	t.Run("should succeed with multiple files", func(t *testing.T) {
		// Arrange
		mockChildFactory := &mockChunkFileFactory{}
		// Set up a mock activity
		mockChildFactory.activity = func(ctx context.Context, executorCtx *executor.Context, input *splitfile.Input) (*splitfile.Output, error) {
			if input.FilePath == "file1.go" {
				return &splitfile.Output{
					FilePath:    "file1.go",
					ContentHash: "hash1",
					Content:     []byte("content1"),
					Chunks:      []domainindex.ChunkData{{TextContent: "chunk1a"}, {TextContent: "chunk1b"}},
				}, nil
			} else {
				return &splitfile.Output{
					FilePath:    "file2.go",
					ContentHash: "hash2",
					Content:     []byte("content2"),
					Chunks:      []domainindex.ChunkData{{TextContent: "chunk2a"}},
				}, nil
			}
		}

		factory, _ := NewFactory(mockChildFactory)
		activityFunc := factory.NewActivity()

		exec := &mockExecutor{}
		exec.executeActivityFn = func(ctx context.Context, parentCtx *executor.Context, name string, activity executor.Activity[any, any], input any) (any, error) {
			params := input.(*splitfile.Input)

			// Execute the actual activity
			return mockChildFactory.activity(ctx, parentCtx, params)
		}

		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

		input := &Input{
			StateName: "test_state",
			Files: []domainindex.FileToProcess{
				{
					FilePath: "file1.go",
					State: domainindex.FileState{
						ContentHash: "hash1",
						Content:     []byte("content1"),
					},
				},
				{
					FilePath: "file2.go",
					State: domainindex.FileState{
						ContentHash: "hash2",
						Content:     []byte("content2"),
					},
				},
			},
		}

		// Act
		output, err := activityFunc(ctx, executorCtx, input)
		// Assert
		if err != nil {
			t.Fatalf("activity returned an unexpected error: %v", err)
		}

		if output.StateName != "test_state" {
			t.Errorf("expected StateName 'test_state', got '%s'", output.StateName)
		}
		if len(output.FileChunks) != 2 {
			t.Fatalf("expected 2 FileChunks, got %d", len(output.FileChunks))
		}
		if len(output.AllChunkTexts) != 3 {
			t.Fatalf("expected 3 AllChunkTexts, got %d", len(output.AllChunkTexts))
		}
		if len(output.ChunkToFileIndex) != 3 {
			t.Fatalf("expected 3 ChunkToFileIndex entries, got %d", len(output.ChunkToFileIndex))
		}

		expectedOrder := []string{"file1.go", "file2.go"}
		for i, chunk := range output.FileChunks {
			if chunk.FilePath != expectedOrder[i] {
				t.Errorf("expected file %s at index %d, got %s", expectedOrder[i], i, chunk.FilePath)
			}
		}

		expectedChunkCounts := []int{2, 1}
		for fileIdx, expectedCount := range expectedChunkCounts {
			count := 0
			for _, idx := range output.ChunkToFileIndex {
				if idx == fileIdx {
					count++
				}
			}
			if count != expectedCount {
				t.Errorf("expected %d chunks for file index %d, got %d", expectedCount, fileIdx, count)
			}
		}
	})

	t.Run("should handle no files", func(t *testing.T) {
		// Arrange
		mockChildFactory := &mockChunkFileFactory{}
		factory, _ := NewFactory(mockChildFactory)
		activityFunc := factory.NewActivity()

		exec := &mockExecutor{}
		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)
		input := &Input{StateName: "empty_state", Files: []domainindex.FileToProcess{}}

		// Act
		output, err := activityFunc(ctx, executorCtx, input)
		// Assert
		if err != nil {
			t.Fatalf("activity returned an unexpected error: %v", err)
		}
		if len(output.FileChunks) != 0 {
			t.Error("expected zero FileChunks")
		}
		if len(output.AllChunkTexts) != 0 {
			t.Error("expected zero AllChunkTexts")
		}
	})

	t.Run("should fail if executor is not in context", func(t *testing.T) {
		// Arrange
		mockChildFactory := &mockChunkFileFactory{}
		factory, _ := NewFactory(mockChildFactory)
		activityFunc := factory.NewActivity()

		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, nil) // No executor
		input := &Input{StateName: "test", Files: []domainindex.FileToProcess{}}

		// Act
		_, err := activityFunc(ctx, executorCtx, input)

		// Assert
		if err == nil {
			t.Fatal("expected an error but got nil")
		}
		if err.Error() != "executor not available in context" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("should fail if a child activity fails", func(t *testing.T) {
		// Arrange
		childErr := errors.New("chunking failed")
		mockChildFactory := &mockChunkFileFactory{}
		mockChildFactory.activity = func(ctx context.Context, executorCtx *executor.Context, input *splitfile.Input) (*splitfile.Output, error) {
			return nil, childErr
		}

		factory, _ := NewFactory(mockChildFactory)
		activityFunc := factory.NewActivity()

		exec := &mockExecutor{}
		exec.executeActivityFn = func(ctx context.Context, parentCtx *executor.Context, name string, activity executor.Activity[any, any], input any) (any, error) {
			return nil, childErr
		}

		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)
		input := &Input{
			StateName: "fail_state",
			Files: []domainindex.FileToProcess{
				{
					FilePath: "fail.go",
					State:    domainindex.FileState{},
				},
			},
		}

		// Act
		_, err := activityFunc(ctx, executorCtx, input)

		// Assert
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if !errors.Is(err, childErr) {
			t.Errorf("expected error to wrap '%v'", childErr)
		}
	})
}

func TestFormatChunkWithMetadata(t *testing.T) {
	chunk := domainindex.ChunkData{
		TextContent: "some text content",
		StartLine:   10,
		EndLine:     15,
	}

	testCases := []struct {
		stateName    string
		expectedDesc string
	}{
		{"head", "committed (HEAD)"},
		{"staging", "staged for commit"},
		{"workspace", "modified in workspace"},
		{"custom_state", "custom_state"},
	}

	for _, tc := range testCases {
		t.Run(tc.stateName, func(t *testing.T) {
			result := formatChunkWithMetadata(chunk, "file.go", tc.stateName)
			expected := fmt.Sprintf(
				"[file: %s, lines: %d-%d, source: %s]\n%s",
				"file.go", 10, 15, tc.expectedDesc, "some text content",
			)
			if result != expected {
				t.Errorf("unexpected format.\nExpected: %s\nGot:      %s", expected, result)
			}
		})
	}
}
