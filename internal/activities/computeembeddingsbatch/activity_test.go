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

package computeembeddingsbatch

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockEmbeddingsGateway is a mock implementation of ports.EmbeddingsGateway.
type mockEmbeddingsGateway struct {
	mu                  sync.Mutex
	ComputeEmbeddingsFn func(ctx context.Context, req *gateway.ComputeEmbeddingsRequest) ([]gateway.Embedding, error)
}

func (m *mockEmbeddingsGateway) ComputeEmbeddings(ctx context.Context, req *gateway.ComputeEmbeddingsRequest) ([]gateway.Embedding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ComputeEmbeddingsFn != nil {
		return m.ComputeEmbeddingsFn(ctx, req)
	}
	return nil, nil
}

func (m *mockEmbeddingsGateway) ComputeDistance(first, second gateway.Embedding) (float64, error) {
	// Simple mock implementation
	return 0.5, nil
}

func TestNewFactory(t *testing.T) {
	mockGW := &mockEmbeddingsGateway{}
	modelName := "test-model"

	t.Run("should succeed with valid arguments", func(t *testing.T) {
		factory, err := NewFactory(mockGW, modelName)
		if err != nil {
			t.Fatalf("expected no error, but got: %v", err)
		}
		if factory == nil {
			t.Fatal("factory should not be nil")
		}
	})

	t.Run("should fail with nil gateway", func(t *testing.T) {
		_, err := NewFactory(nil, modelName)
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if err.Error() != "computeembeddingsbatch.NewFactory: embeddingGW cannot be nil" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("should fail with empty model name", func(t *testing.T) {
		_, err := NewFactory(mockGW, "")
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if err.Error() != "computeembeddingsbatch.NewFactory: modelName cannot be empty" {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func TestActivity(t *testing.T) {
	mockGW := &mockEmbeddingsGateway{}
	factory, _ := NewFactory(mockGW, "test-model")
	activity := factory.NewActivity()
	ctx := context.Background()

	t.Run("should succeed with valid chunks", func(t *testing.T) {
		// Arrange
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, nil)
		input := &Input{ChunkTexts: []string{"text1", "text2"}}
		expectedEmbeddings := []gateway.Embedding{{1.0}, {2.0}}

		mockGW.ComputeEmbeddingsFn = func(ctx context.Context, req *gateway.ComputeEmbeddingsRequest) ([]gateway.Embedding, error) {
			if len(req.Chunks()) != len(input.ChunkTexts) {
				return nil, fmt.Errorf("expected %d texts, got %d", len(input.ChunkTexts), len(req.Chunks()))
			}
			return expectedEmbeddings, nil
		}

		// Act
		output, err := activity(ctx, executorCtx, input)

		// Assert
		if err != nil {
			t.Fatalf("activity failed: %v", err)
		}
		if len(output.Embeddings) != len(expectedEmbeddings) {
			t.Errorf("expected %d embeddings, got %d", len(expectedEmbeddings), len(output.Embeddings))
		}
	})

	t.Run("should succeed with no chunks", func(t *testing.T) {
		// Arrange
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, nil)
		input := &Input{ChunkTexts: []string{}}

		// Act
		output, err := activity(ctx, executorCtx, input)

		// Assert
		if err != nil {
			t.Fatalf("activity failed: %v", err)
		}
		if len(output.Embeddings) != 0 {
			t.Errorf("expected 0 embeddings, got %d", len(output.Embeddings))
		}
		// Note: Cannot test feedback messages since GetMessages() doesn't exist on Context
	})

	t.Run("should fail if gateway returns an error", func(t *testing.T) {
		// Arrange
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, nil)
		input := &Input{ChunkTexts: []string{"text1"}}
		gwErr := errors.New("gateway failure")
		mockGW.ComputeEmbeddingsFn = func(ctx context.Context, req *gateway.ComputeEmbeddingsRequest) ([]gateway.Embedding, error) {
			return nil, gwErr
		}

		// Act
		_, err := activity(ctx, executorCtx, input)

		// Assert
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if !errors.Is(err, gwErr) {
			t.Errorf("expected error to wrap '%v'", gwErr)
		}
	})

	t.Run("should fail on embedding count mismatch", func(t *testing.T) {
		// Arrange
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, nil)
		input := &Input{ChunkTexts: []string{"text1", "text2"}}
		mockGW.ComputeEmbeddingsFn = func(ctx context.Context, req *gateway.ComputeEmbeddingsRequest) ([]gateway.Embedding, error) {
			return []gateway.Embedding{{1.0}}, nil // Return only one embedding
		}

		// Act
		_, err := activity(ctx, executorCtx, input)

		// Assert
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		expectedMsg := "embedding count mismatch: got 1, expected 2"
		if err.Error() != expectedMsg {
			t.Errorf("unexpected error message.\nExpected: %s\nGot:      %s", expectedMsg, err.Error())
		}
	})
}
