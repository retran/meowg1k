// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package chunkallfiles

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/retran/meowg1k/internal/activities/chunkfile"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/future"
)

// mockChunkFileFactory is a mock implementation of the chunkfile factory.
type mockChunkFileFactory struct {
	activity executor.Activity[*chunkfile.Input, *chunkfile.Output]
}

func (m *mockChunkFileFactory) NewActivity() executor.Activity[*chunkfile.Input, *chunkfile.Output] {
	return m.activity
}

// mockExecutor is a mock implementation of the executor.
type mockExecutor struct {
	executeActivityFn func(ctx context.Context, parentCtx *executor.Context, name string, activity executor.Activity[any, any], input any) *future.Future[any]
}

func (m *mockExecutor) ExecuteActivity(ctx context.Context, parentCtx *executor.Context, name string, activity executor.Activity[any, any], input any) *future.Future[any] {
	if m.executeActivityFn != nil {
		return m.executeActivityFn(ctx, parentCtx, name, activity, input)
	}
	// Default implementation
	f := future.NewFuture[any]()
	f.CompleteWithError(fmt.Errorf("mock not configured"))
	return f
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
		expectedErr := "chunkallfiles.NewFactory: chunkFileFactory cannot be nil"
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
		mockChildFactory.activity = func(ctx context.Context, executorCtx *executor.Context, input *chunkfile.Input) (*chunkfile.Output, error) {
			if input.FilePath == "file1.go" {
				return &chunkfile.Output{
					FilePath:    "file1.go",
					ContentHash: "hash1",
					Content:     []byte("content1"),
					Chunks:      []domainindex.ChunkData{{TextContent: "chunk1a"}, {TextContent: "chunk1b"}},
				}, nil
			} else {
				return &chunkfile.Output{
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
		exec.executeActivityFn = func(ctx context.Context, parentCtx *executor.Context, name string, activity executor.Activity[any, any], input any) *future.Future[any] {
			params := input.(*chunkfile.Input)
			f := future.NewFuture[any]()

			// Execute the actual activity
			result, err := mockChildFactory.activity(ctx, parentCtx, params)
			if err != nil {
				f.CompleteWithError(err)
			} else {
				f.Complete(result)
			}
			return f
		}

		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

		input := &Input{
			StateName: "test_state",
			Files: map[string]domainindex.FileState{
				"file1.go": {ContentHash: "hash1", Content: []byte("content1")},
				"file2.go": {ContentHash: "hash2", Content: []byte("content2")},
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

		// Since map iteration order is not guaranteed, we need to be flexible about the index mapping
		// Just verify that the indices are valid and the counts are correct
		for _, idx := range output.ChunkToFileIndex {
			if idx < 0 || idx >= len(output.FileChunks) {
				t.Errorf("invalid file index %d", idx)
			}
		}

		// Count chunks per file index
		chunkCounts := make(map[int]int)
		for _, idx := range output.ChunkToFileIndex {
			chunkCounts[idx]++
		}

		// We should have exactly 2 files, one with 2 chunks and one with 1 chunk
		if len(chunkCounts) != 2 {
			t.Errorf("expected chunks for 2 files, got chunks for %d files", len(chunkCounts))
		}

		totalChunks := 0
		for _, count := range chunkCounts {
			totalChunks += count
		}
		if totalChunks != 3 {
			t.Errorf("expected 3 total chunks, got %d", totalChunks)
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
		input := &Input{StateName: "empty_state", Files: map[string]domainindex.FileState{}}

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
		input := &Input{StateName: "test", Files: map[string]domainindex.FileState{}}

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
		mockChildFactory.activity = func(ctx context.Context, executorCtx *executor.Context, input *chunkfile.Input) (*chunkfile.Output, error) {
			return nil, childErr
		}

		factory, _ := NewFactory(mockChildFactory)
		activityFunc := factory.NewActivity()

		exec := &mockExecutor{}
		exec.executeActivityFn = func(ctx context.Context, parentCtx *executor.Context, name string, activity executor.Activity[any, any], input any) *future.Future[any] {
			f := future.NewFuture[any]()
			f.CompleteWithError(childErr)
			return f
		}

		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)
		input := &Input{
			StateName: "fail_state",
			Files:     map[string]domainindex.FileState{"fail.go": {}},
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
