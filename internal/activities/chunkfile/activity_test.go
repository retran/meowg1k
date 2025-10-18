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

package chunkfile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockChunkerService is a mock implementation of the ports.ChunkerService.
type mockChunkerService struct {
	mu      sync.Mutex
	ChunkFn func(content []byte, filePath string) ([]domainindex.ChunkData, error)
}

func (m *mockChunkerService) Chunk(content []byte, filePath string) ([]domainindex.ChunkData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ChunkFn != nil {
		return m.ChunkFn(content, filePath)
	}
	return nil, nil
}

func TestNewFactory(t *testing.T) {
	t.Run("should succeed with a valid service", func(t *testing.T) {
		mockSvc := &mockChunkerService{}
		factory, err := NewFactory(mockSvc)
		if err != nil {
			t.Fatalf("expected no error, but got %v", err)
		}
		if factory == nil {
			t.Fatal("factory should not be nil")
		}
	})

	t.Run("should fail with a nil service", func(t *testing.T) {
		_, err := NewFactory(nil)
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		expectedErr := "chunkfile.NewFactory: chunkerService cannot be nil"
		if err.Error() != expectedErr {
			t.Errorf("expected error message '%s', but got '%s'", expectedErr, err.Error())
		}
	})
}

func TestActivity(t *testing.T) {
	mockSvc := &mockChunkerService{}
	factory, _ := NewFactory(mockSvc)
	activity := factory.NewActivity()

	t.Run("should succeed and return correct output", func(t *testing.T) {
		// Arrange
		ctx := context.Background()

		// Capture feedback messages
		var feedbackMessages []*executor.Feedback
		feedbackHandler := func(feedback *executor.Feedback) {
			feedbackMessages = append(feedbackMessages, feedback)
		}

		executorCtx := executor.NewContext("test", feedbackHandler, executor.NewExecutor(0))
		input := &Input{
			FilePath: "test.go",
			Content:  []byte("package main"),
		}
		expectedChunks := []domainindex.ChunkData{{TextContent: "package main"}}
		mockSvc.ChunkFn = func(content []byte, filePath string) ([]domainindex.ChunkData, error) {
			return expectedChunks, nil
		}

		// Act
		output, err := activity(ctx, executorCtx, input)
		// Assert
		if err != nil {
			t.Fatalf("activity returned an unexpected error: %v", err)
		}
		if output.FilePath != input.FilePath {
			t.Errorf("expected FilePath '%s', got '%s'", input.FilePath, output.FilePath)
		}
		if !reflect.DeepEqual(output.Content, input.Content) {
			t.Error("output Content does not match input Content")
		}
		if !reflect.DeepEqual(output.Chunks, expectedChunks) {
			t.Errorf("unexpected chunks returned")
		}

		expectedHash := computeContentHash(input.Content)
		if output.ContentHash != expectedHash {
			t.Errorf("expected ContentHash '%s', got '%s'", expectedHash, output.ContentHash)
		}

		if len(feedbackMessages) != 2 {
			t.Fatalf("expected 2 feedback messages, but got %d", len(feedbackMessages))
		}
	})

	t.Run("should fail if chunker service returns an error", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, executor.NewExecutor(0))
		input := &Input{FilePath: "error.go"}
		serviceErr := errors.New("chunking failed")
		mockSvc.ChunkFn = func(content []byte, filePath string) ([]domainindex.ChunkData, error) {
			return nil, serviceErr
		}

		// Act
		_, err := activity(ctx, executorCtx, input)

		// Assert
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if !errors.Is(err, serviceErr) {
			t.Errorf("expected error to wrap '%v'", serviceErr)
		}
		expectedErrMsg := fmt.Sprintf("failed to chunk file %s: %s", input.FilePath, serviceErr)
		if err.Error() != expectedErrMsg {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func TestComputeContentHash(t *testing.T) {
	content := []byte("hello world")
	hash := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(hash[:])

	result := computeContentHash(content)

	if result != expectedHash {
		t.Errorf("expected hash '%s', but got '%s'", expectedHash, result)
	}
}
