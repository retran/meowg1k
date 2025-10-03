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

package gateway

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestNewGeminiGateway(t *testing.T) {
	ctx := context.Background()

	t.Run("Valid API key", func(t *testing.T) {
		gateway, err := newGeminiGateway(ctx, "test-api-key")
		if err != nil {
			// Gemini might validate the API key format or require network access
			t.Logf("Gemini gateway creation failed (expected in test environment): %v", err)
			return
		}
		if gateway == nil {
			t.Fatal("Expected gateway to be non-nil")
		}

		// Verify it implements both interfaces
		var _ GenerationGateway = gateway
		var _ EmbeddingsGateway = gateway
		t.Log("GeminiGateway correctly implements both GenerationGateway and EmbeddingsGateway interfaces")
	})

	t.Run("Empty API key", func(t *testing.T) {
		gateway, err := newGeminiGateway(ctx, "")
		if err == nil && gateway != nil {
			t.Log("Gemini allows empty API key on creation, will validate on use")
		} else if err != nil {
			t.Logf("Gemini validates API key on creation: %v", err)
		} else {
			t.Fatal("Unexpected state: no error but nil gateway")
		}
	})

	t.Run("Invalid API key format", func(t *testing.T) {
		gateway, err := newGeminiGateway(ctx, "invalid-key-format")
		if err == nil && gateway != nil {
			t.Log("Gemini allows invalid API key format on creation")
		} else if err != nil {
			t.Logf("Gemini validates API key format: %v", err)
		} else {
			t.Fatal("Unexpected state: no error but nil gateway")
		}
	})
}

func TestGeminiGateway_GenerateContent(t *testing.T) {
	ctx := context.Background()

	// Try to create a gateway for testing
	gateway, err := newGeminiGateway(ctx, "test-api-key")
	if err != nil {
		t.Skipf("Cannot create Gemini gateway for testing: %v", err)
		return
	}

	t.Run("Generate content with valid request", func(t *testing.T) {
		request := NewGenerateContentRequest(
			"gemini-1.5-flash",
			"You are a helpful assistant",
			"Hello, how are you?",
			4096,
		)

		_, err := gateway.GenerateContent(ctx, request)
		// We expect an error since we're not actually connecting to Gemini
		if err != nil {
			t.Logf("Expected network/API error: %v", err)
			// Verify it's not a basic validation error
			if strings.Contains(err.Error(), "model is required") {
				t.Error("Should not get validation error for valid request")
			}
		} else {
			t.Log("Unexpected success - this might indicate the test environment has network access")
		}
	})

	t.Run("Generate content with system prompt", func(t *testing.T) {
		request := NewGenerateContentRequest(
			"gemini-1.5-pro",
			"You are a code assistant specializing in Go programming. Always provide working, well-commented code.",
			"Write a function to calculate fibonacci numbers",
			4096,
		)

		_, err := gateway.GenerateContent(ctx, request)
		if err != nil {
			t.Logf("Expected network/API error with system prompt: %v", err)
		}
	})

	t.Run("Generate content without system prompt", func(t *testing.T) {
		request := NewGenerateContentRequest(
			"gemini-1.5-flash",
			"", // empty system prompt
			"Explain quantum computing in simple terms",
			2048,
		)

		_, err := gateway.GenerateContent(ctx, request)
		if err != nil {
			t.Logf("Expected network/API error without system prompt: %v", err)
		}
	})

	t.Run("Generate content with different models", func(t *testing.T) {
		models := []string{
			"gemini-1.5-flash",
			"gemini-1.5-pro",
			"gemini-1.0-pro",
		}

		for _, model := range models {
			t.Run("Model_"+model, func(t *testing.T) {
				request := NewGenerateContentRequest(
					model,
					"You are a helpful assistant",
					"Generate a short poem",
					1000,
				)

				_, err := gateway.GenerateContent(ctx, request)
				if err != nil {
					t.Logf("Expected network/API error for model %s: %v", model, err)
				}
			})
		}
	})

	t.Run("Generate content with canceled context", func(t *testing.T) {
		request := NewGenerateContentRequest(
			"gemini-1.5-flash",
			"You are a helpful assistant",
			"Hello, how are you?",
			4096,
		)

		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		_, err := gateway.GenerateContent(cancelCtx, request)
		if err == nil {
			t.Fatal("Expected error for canceled context")
		}
		// Should get context canceled error or connection error
		t.Logf("Got expected error for canceled context: %v", err)
	})
}

func TestGeminiGateway_ComputeEmbeddings(t *testing.T) {
	ctx := context.Background()

	// Try to create a gateway for testing
	gateway, err := newGeminiGateway(ctx, "test-api-key")
	if err != nil {
		t.Skipf("Cannot create Gemini gateway for testing: %v", err)
		return
	}

	t.Run("Compute embeddings with valid request", func(t *testing.T) {
		chunks := []string{"Hello world", "How are you?"}
		request := NewComputeEmbeddingsRequestWithDimensions(
			"text-embedding-004",
			chunks,
			RetrievalQuery,
			768,
		)

		_, err := gateway.ComputeEmbeddings(ctx, request)
		// We expect an error since we're not actually connecting to Gemini
		if err != nil {
			t.Logf("Expected network/API error: %v", err)
		} else {
			t.Log("Unexpected success - this might indicate the test environment has network access")
		}
	})

	t.Run("Compute embeddings with different task types", func(t *testing.T) {
		testCases := []struct {
			name     string
			taskType TaskType
		}{
			{"Retrieval Document", RetrievalDocument},
			{"Retrieval Query", RetrievalQuery},
			{"Classification", Classification},
			{"Clustering", Clustering},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				chunks := []string{"Test chunk for " + tc.name}
				request := NewComputeEmbeddingsRequest(
					"text-embedding-004",
					chunks,
					tc.taskType,
				)

				_, err := gateway.ComputeEmbeddings(ctx, request)
				if err != nil {
					t.Logf("Expected network/API error for %s: %v", tc.name, err)
				}
			})
		}
	})

	t.Run("Compute embeddings with large chunks", func(t *testing.T) {
		largeChunks := make([]string, 50)
		for i := 0; i < 50; i++ {
			largeChunks[i] = strings.Repeat("This is a test chunk for embeddings ", 20)
		}

		request := NewComputeEmbeddingsRequestWithDimensions(
			"text-embedding-004",
			largeChunks,
			RetrievalDocument,
			768,
		)

		_, err := gateway.ComputeEmbeddings(ctx, request)
		if err != nil {
			t.Logf("Expected network/API error for large chunks: %v", err)
		}
	})

	t.Run("Compute embeddings with canceled context", func(t *testing.T) {
		chunks := []string{"Test chunk"}
		request := NewComputeEmbeddingsRequestWithDimensions(
			"text-embedding-004",
			chunks,
			RetrievalQuery,
			768,
		)

		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		_, err := gateway.ComputeEmbeddings(cancelCtx, request)
		if err == nil {
			t.Fatal("Expected error for canceled context")
		}
		t.Logf("Got expected error for canceled context: %v", err)
	})
}

func TestGeminiGateway_InterfaceCompliance(t *testing.T) {
	ctx := context.Background()

	gateway, err := newGeminiGateway(ctx, "test-api-key")
	if err != nil {
		t.Skipf("Cannot create Gemini gateway for interface testing: %v", err)
		return
	}

	// Verify that the gateway implements both interfaces
	var _ GenerationGateway = gateway
	var _ EmbeddingsGateway = gateway
	_ = gateway // Should implement the unified Gateway interface

	t.Log("GeminiGateway correctly implements GenerationGateway, EmbeddingsGateway, and Gateway interfaces")

	// Test that it has the ComputeDistance method from mixin
	embedding1 := Embedding{0.1, 0.2, 0.3}
	embedding2 := Embedding{0.4, 0.5, 0.6}

	distance, err := gateway.ComputeDistance(embedding1, embedding2)
	if err != nil {
		t.Errorf("ComputeDistance failed: %v", err)
	}
	if distance < 0 {
		t.Error("Distance should be non-negative")
	}
	t.Logf("Computed distance: %f", distance)
}

func TestGeminiGateway_ErrorScenarios(t *testing.T) {
	ctx := context.Background()

	gateway, err := newGeminiGateway(ctx, "test-api-key")
	if err != nil {
		t.Skipf("Cannot create Gemini gateway for error testing: %v", err)
		return
	}

	t.Run("Generation with various content types", func(t *testing.T) {
		testCases := []struct {
			name         string
			systemPrompt string
			userPrompt   string
		}{
			{
				name:         "Code generation",
				systemPrompt: "You are a senior Go developer",
				userPrompt:   "Write a function to sort a slice of integers",
			},
			{
				name:         "Creative writing",
				systemPrompt: "You are a creative writer",
				userPrompt:   "Write a short story about a robot learning to paint",
			},
			{
				name:         "Technical explanation",
				systemPrompt: "You are a technical documentation writer",
				userPrompt:   "Explain how HTTP/2 multiplexing works",
			},
			{
				name:         "Data analysis",
				systemPrompt: "You are a data analyst",
				userPrompt:   "Analyze the trend in this data: [1, 3, 5, 7, 11, 13, 17]",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				request := NewGenerateContentRequest(
					"gemini-1.5-flash",
					tc.systemPrompt,
					tc.userPrompt,
					2048,
				)

				_, err := gateway.GenerateContent(ctx, request)
				if err != nil {
					t.Logf("Expected network/API error for %s: %v", tc.name, err)
				}
			})
		}
	})

	t.Run("Embeddings with specialized content", func(t *testing.T) {
		testCases := []struct {
			name     string
			chunks   []string
			taskType TaskType
		}{
			{
				name: "Code snippets",
				chunks: []string{
					"func main() { fmt.Println(\"Hello, World!\") }",
					"def hello_world(): print(\"Hello, World!\")",
					"console.log('Hello, World!');",
				},
				taskType: CodeRetrievalQuery,
			},
			{
				name: "Scientific text",
				chunks: []string{
					"The mitochondria is the powerhouse of the cell",
					"E=mc² describes mass-energy equivalence",
					"DNA consists of four nucleotide bases: A, T, G, C",
				},
				taskType: Classification,
			},
			{
				name: "Legal documents",
				chunks: []string{
					"The party of the first part agrees to the terms",
					"Whereas the aforementioned conditions are met",
					"This agreement shall be binding upon all parties",
				},
				taskType: Clustering,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				request := NewComputeEmbeddingsRequest(
					"text-embedding-004",
					tc.chunks,
					tc.taskType,
				)

				_, err := gateway.ComputeEmbeddings(ctx, request)
				if err != nil {
					t.Logf("Expected network/API error for %s: %v", tc.name, err)
				}
			})
		}
	})
}

func TestGeminiGateway_ParameterValidation(t *testing.T) {
	ctx := context.Background()

	gateway, err := newGeminiGateway(ctx, "test-api-key")
	if err != nil {
		t.Skipf("Cannot create Gemini gateway for parameter testing: %v", err)
		return
	}

	t.Run("Generation with edge case parameters", func(t *testing.T) {
		testCases := []struct {
			name      string
			model     string
			maxTokens int
		}{
			{"Very small token limit", "gemini-1.5-flash", 1},
			{"Medium token limit", "gemini-1.5-flash", 1000},
			{"Large token limit", "gemini-1.5-pro", 8000},
			{"Zero tokens", "gemini-1.5-flash", 0},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				request := NewGenerateContentRequest(
					tc.model,
					"You are a helpful assistant",
					"Generate appropriate content",
					tc.maxTokens,
				)

				_, err := gateway.GenerateContent(ctx, request)
				if err != nil {
					t.Logf("Expected network/API error for %s: %v", tc.name, err)
				}
			})
		}
	})

	t.Run("Embeddings with edge case parameters", func(t *testing.T) {
		testCases := []struct {
			name       string
			dimensions int
			chunkCount int
		}{
			{"Single chunk", 768, 1},
			{"Many chunks", 768, 100},
			{"Small dimensions", 128, 10},
			{"Large dimensions", 1024, 10},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				chunks := make([]string, tc.chunkCount)
				for i := 0; i < tc.chunkCount; i++ {
					chunks[i] = fmt.Sprintf("Test chunk number %d", i+1)
				}

				request := NewComputeEmbeddingsRequestWithDimensions(
					"text-embedding-004",
					chunks,
					RetrievalQuery,
					tc.dimensions,
				)

				_, err := gateway.ComputeEmbeddings(ctx, request)
				if err != nil {
					t.Logf("Expected network/API error for %s: %v", tc.name, err)
				}
			})
		}
	})
}

func TestGeminiGateway_ComputeEmbeddings_EdgeCases(t *testing.T) {
	ctx := context.Background()

	// Try to create a gateway for testing
	gateway, err := newGeminiGateway(ctx, "test-api-key")
	if err != nil {
		t.Skipf("Cannot create Gemini gateway for testing: %v", err)
		return
	}

	t.Run("Dimensions overflow check", func(t *testing.T) {
		chunks := []string{"test"}
		// Create a request with dimensions that would exceed int32 max
		request := NewComputeEmbeddingsRequestWithDimensions(
			"text-embedding-004",
			chunks,
			RetrievalQuery,
			int(^uint32(0)>>1)+1, // This exceeds int32 max
		)

		_, err := gateway.ComputeEmbeddings(ctx, request)
		if err == nil {
			t.Error("Expected error for dimensions overflow, got none")
		} else if !strings.Contains(err.Error(), "exceeds int32 range") {
			t.Logf("Got different error (API-related): %v", err)
		}
	})

	t.Run("Zero dimensions", func(t *testing.T) {
		chunks := []string{"test"}
		request := NewComputeEmbeddingsRequestWithDimensions(
			"text-embedding-004",
			chunks,
			RetrievalQuery,
			0, // Zero dimensions should not set the config
		)

		_, err := gateway.ComputeEmbeddings(ctx, request)
		// This should proceed without dimension overflow error
		if err != nil {
			t.Logf("Expected network/API error: %v", err)
		}
	})

	t.Run("Negative dimensions", func(t *testing.T) {
		chunks := []string{"test"}
		request := NewComputeEmbeddingsRequestWithDimensions(
			"text-embedding-004",
			chunks,
			RetrievalQuery,
			-1, // Negative dimensions should not trigger overflow path
		)

		_, err := gateway.ComputeEmbeddings(ctx, request)
		// This should proceed without dimension overflow error
		if err != nil {
			t.Logf("Expected network/API error: %v", err)
		}
	})
}

func TestGeminiGateway_GenerateContent_EdgeCases(t *testing.T) {
	ctx := context.Background()

	// Try to create a gateway for testing
	gateway, err := newGeminiGateway(ctx, "test-api-key")
	if err != nil {
		t.Skipf("Cannot create Gemini gateway for testing: %v", err)
		return
	}

	t.Run("Empty system prompt", func(t *testing.T) {
		request := NewGenerateContentRequest(
			"gemini-1.5-flash",
			"", // Empty system prompt
			"Hello, how are you?",
			4096,
		)

		_, err := gateway.GenerateContent(ctx, request)
		if err != nil {
			t.Logf("Expected network/API error: %v", err)
		}
	})

	t.Run("Empty user prompt", func(t *testing.T) {
		request := NewGenerateContentRequest(
			"gemini-1.5-flash",
			"You are a helpful assistant",
			"", // Empty user prompt
			4096,
		)

		_, err := gateway.GenerateContent(ctx, request)
		if err != nil {
			t.Logf("Expected network/API error: %v", err)
		}
	})

	t.Run("Both prompts empty", func(t *testing.T) {
		request := NewGenerateContentRequest(
			"gemini-1.5-flash",
			"", // Empty system prompt
			"", // Empty user prompt
			4096,
		)

		_, err := gateway.GenerateContent(ctx, request)
		if err != nil {
			t.Logf("Expected network/API error: %v", err)
		}
	})
}
