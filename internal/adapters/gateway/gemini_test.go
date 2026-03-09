// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

// geminiGenerateContentResponse returns a minimal valid Gemini generateContent response.
func geminiGenerateContentResponse() map[string]interface{} {
	return map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"role": "model",
					"parts": []map[string]interface{}{
						{"text": "Generated content response"},
					},
				},
				"finishReason": "STOP",
			},
		},
		"usageMetadata": map[string]interface{}{
			"promptTokenCount":     10,
			"candidatesTokenCount": 5,
			"totalTokenCount":      15,
		},
	}
}

// geminiBatchEmbedContentsResponse returns a minimal valid Gemini batchEmbedContents response.
func geminiBatchEmbedContentsResponse() map[string]interface{} {
	return map[string]interface{}{
		"embeddings": []map[string]interface{}{
			{"values": []float32{0.1, 0.2, 0.3}},
		},
	}
}

// geminiCountTokensResponse returns a minimal valid Gemini countTokens response.
func geminiCountTokensResponse() map[string]interface{} {
	return map[string]interface{}{
		"totalTokens": 42,
	}
}

// newGeminiMockServer creates an httptest.Server that handles Gemini API requests.
func newGeminiMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var resp interface{}
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, ":generateContent"):
			resp = geminiGenerateContentResponse()
		case strings.HasSuffix(path, ":batchEmbedContents"):
			resp = geminiBatchEmbedContentsResponse()
		case strings.HasSuffix(path, ":countTokens"):
			resp = geminiCountTokensResponse()
		default:
			t.Errorf("Unexpected path: %s", path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
}

// newTestGeminiGateway creates a gemini gateway pointed at the given mock server URL.
func newTestGeminiGateway(t *testing.T, serverURL string) *geminiGateway {
	t.Helper()
	ctx := context.Background()
	gw, err := newGeminiGateway(ctx, "test-api-key", serverURL)
	require.NoError(t, err)
	require.NotNil(t, gw)
	gg, ok := gw.(*geminiGateway)
	require.True(t, ok)
	return gg
}

func TestNewGeminiGateway(t *testing.T) {
	server := newGeminiMockServer(t)
	defer server.Close()

	ctx := context.Background()

	t.Run("Valid API key with mock server", func(t *testing.T) {
		gw, err := newGeminiGateway(ctx, "test-api-key", server.URL)
		require.NoError(t, err)
		assert.NotNil(t, gw)

		var _ ports.GenerationGateway = gw
		var _ ports.EmbeddingsGateway = gw
	})

	t.Run("Empty API key returns error", func(t *testing.T) {
		_, err := newGeminiGateway(ctx, "", server.URL)
		assert.Error(t, err)
	})
}

func TestGeminiGateway_GenerateContent(t *testing.T) {
	server := newGeminiMockServer(t)
	defer server.Close()

	gw := newTestGeminiGateway(t, server.URL)
	ctx := context.Background()

	t.Run("Generate content with valid request", func(t *testing.T) {
		request := domainGateway.NewGenerateContentRequest(
			"gemini-1.5-flash",
			"You are a helpful assistant",
			"Hello, how are you?",
			4096,
		)

		resp, err := gw.GenerateContent(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.Blocks)
	})

	t.Run("Generate content with system prompt", func(t *testing.T) {
		request := domainGateway.NewGenerateContentRequest(
			"gemini-1.5-pro",
			"You are a code assistant specializing in Go programming.",
			"Write a function to calculate fibonacci numbers",
			4096,
		)

		resp, err := gw.GenerateContent(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("Generate content without system prompt", func(t *testing.T) {
		request := domainGateway.NewGenerateContentRequest(
			"gemini-1.5-flash",
			"",
			"Explain quantum computing in simple terms",
			2048,
		)

		resp, err := gw.GenerateContent(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("Generate content with different models", func(t *testing.T) {
		models := []string{
			"gemini-1.5-flash",
			"gemini-1.5-pro",
			"gemini-1.0-pro",
		}

		for _, model := range models {
			t.Run("Model_"+model, func(t *testing.T) {
				request := domainGateway.NewGenerateContentRequest(
					model,
					"You are a helpful assistant",
					"Generate a short poem",
					1000,
				)

				resp, err := gw.GenerateContent(ctx, request)
				require.NoError(t, err)
				require.NotNil(t, resp)
			})
		}
	})

	t.Run("Generate content with canceled context", func(t *testing.T) {
		request := domainGateway.NewGenerateContentRequest(
			"gemini-1.5-flash",
			"You are a helpful assistant",
			"Hello, how are you?",
			4096,
		)

		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		_, err := gw.GenerateContent(cancelCtx, request)
		assert.Error(t, err)
	})

	t.Run("Nil context returns error", func(t *testing.T) {
		request := domainGateway.NewGenerateContentRequest(
			"gemini-1.5-flash",
			"",
			"Hello",
			4096,
		)
		_, err := gw.GenerateContent(nil, request) //nolint:staticcheck // intentionally passing nil context to test nil-check guard
		assert.Error(t, err)
	})

	t.Run("Nil request returns error", func(t *testing.T) {
		_, err := gw.GenerateContent(ctx, nil)
		assert.Error(t, err)
	})
}

func TestGeminiGateway_ComputeEmbeddings(t *testing.T) {
	server := newGeminiMockServer(t)
	defer server.Close()

	gw := newTestGeminiGateway(t, server.URL)
	ctx := context.Background()

	t.Run("Compute embeddings with valid request", func(t *testing.T) {
		chunks := []string{"Hello world", "How are you?"}
		request := domainGateway.NewComputeEmbeddingsRequestWithDimensions(
			"text-embedding-004",
			chunks,
			domainGateway.RetrievalQuery,
			768,
		)

		embeddings, err := gw.ComputeEmbeddings(ctx, request)
		require.NoError(t, err)
		assert.NotEmpty(t, embeddings)
	})

	t.Run("Compute embeddings with different task types", func(t *testing.T) {
		testCases := []struct {
			name     string
			taskType domainGateway.TaskType
		}{
			{"Retrieval Document", domainGateway.RetrievalDocument},
			{"Retrieval Query", domainGateway.RetrievalQuery},
			{"Classification", domainGateway.Classification},
			{"Clustering", domainGateway.Clustering},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				chunks := []string{"Test chunk for " + tc.name}
				request := domainGateway.NewComputeEmbeddingsRequest(
					"text-embedding-004",
					chunks,
					tc.taskType,
				)

				embeddings, err := gw.ComputeEmbeddings(ctx, request)
				require.NoError(t, err)
				assert.NotEmpty(t, embeddings)
			})
		}
	})

	t.Run("Compute embeddings with canceled context", func(t *testing.T) {
		chunks := []string{"Test chunk"}
		request := domainGateway.NewComputeEmbeddingsRequestWithDimensions(
			"text-embedding-004",
			chunks,
			domainGateway.RetrievalQuery,
			768,
		)

		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		_, err := gw.ComputeEmbeddings(cancelCtx, request)
		assert.Error(t, err)
	})

	t.Run("Nil context returns error", func(t *testing.T) {
		chunks := []string{"test"}
		request := domainGateway.NewComputeEmbeddingsRequest("text-embedding-004", chunks, domainGateway.RetrievalQuery)
		_, err := gw.ComputeEmbeddings(nil, request) //nolint:staticcheck // intentionally passing nil context to test nil-check guard
		assert.Error(t, err)
	})

	t.Run("Nil request returns error", func(t *testing.T) {
		_, err := gw.ComputeEmbeddings(ctx, nil)
		assert.Error(t, err)
	})
}

func TestGeminiGateway_InterfaceCompliance(t *testing.T) {
	server := newGeminiMockServer(t)
	defer server.Close()

	gw := newTestGeminiGateway(t, server.URL)

	var _ ports.GenerationGateway = gw
	var _ ports.EmbeddingsGateway = gw

	// Test ComputeDistance from mixin
	embedding1 := domainGateway.Embedding{0.1, 0.2, 0.3}
	embedding2 := domainGateway.Embedding{0.4, 0.5, 0.6}

	distance, err := gw.ComputeDistance(embedding1, embedding2)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, distance, 0.0)
}

func TestGeminiGateway_ErrorScenarios(t *testing.T) {
	server := newGeminiMockServer(t)
	defer server.Close()

	gw := newTestGeminiGateway(t, server.URL)
	ctx := context.Background()

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
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				request := domainGateway.NewGenerateContentRequest(
					"gemini-1.5-flash",
					tc.systemPrompt,
					tc.userPrompt,
					2048,
				)

				resp, err := gw.GenerateContent(ctx, request)
				require.NoError(t, err)
				require.NotNil(t, resp)
			})
		}
	})

	t.Run("Embeddings with specialized content", func(t *testing.T) {
		testCases := []struct {
			name     string
			taskType domainGateway.TaskType
			chunks   []string
		}{
			{
				name: "Code snippets",
				chunks: []string{
					"func main() { fmt.Println(\"Hello, World!\") }",
					"def hello_world(): print(\"Hello, World!\")",
				},
				taskType: domainGateway.CodeRetrievalQuery,
			},
			{
				name: "Scientific text",
				chunks: []string{
					"The mitochondria is the powerhouse of the cell",
					"E=mc² describes mass-energy equivalence",
				},
				taskType: domainGateway.Classification,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				request := domainGateway.NewComputeEmbeddingsRequest(
					"text-embedding-004",
					tc.chunks,
					tc.taskType,
				)

				embeddings, err := gw.ComputeEmbeddings(ctx, request)
				require.NoError(t, err)
				assert.NotEmpty(t, embeddings)
			})
		}
	})
}

func TestGeminiGateway_ParameterValidation(t *testing.T) {
	server := newGeminiMockServer(t)
	defer server.Close()

	gw := newTestGeminiGateway(t, server.URL)
	ctx := context.Background()

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
				request := domainGateway.NewGenerateContentRequest(
					tc.model,
					"You are a helpful assistant",
					"Generate appropriate content",
					tc.maxTokens,
				)

				resp, err := gw.GenerateContent(ctx, request)
				require.NoError(t, err)
				require.NotNil(t, resp)
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
			{"Many chunks", 768, 5},
			{"Small dimensions", 128, 3},
			{"Large dimensions", 1024, 3},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				chunks := make([]string, tc.chunkCount)
				for i := 0; i < tc.chunkCount; i++ {
					chunks[i] = fmt.Sprintf("Test chunk number %d", i+1)
				}

				request := domainGateway.NewComputeEmbeddingsRequestWithDimensions(
					"text-embedding-004",
					chunks,
					domainGateway.RetrievalQuery,
					tc.dimensions,
				)

				embeddings, err := gw.ComputeEmbeddings(ctx, request)
				require.NoError(t, err)
				assert.NotEmpty(t, embeddings)
			})
		}
	})
}

func TestGeminiGateway_ComputeEmbeddings_EdgeCases(t *testing.T) {
	server := newGeminiMockServer(t)
	defer server.Close()

	gw := newTestGeminiGateway(t, server.URL)
	ctx := context.Background()

	t.Run("Dimensions overflow check", func(t *testing.T) {
		chunks := []string{"test"}
		// Dimensions that exceed int32 max
		request := domainGateway.NewComputeEmbeddingsRequestWithDimensions(
			"text-embedding-004",
			chunks,
			domainGateway.RetrievalQuery,
			math.MaxInt32+1,
		)

		_, err := gw.ComputeEmbeddings(ctx, request)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds int32 range")
	})

	t.Run("Zero dimensions", func(t *testing.T) {
		chunks := []string{"test"}
		request := domainGateway.NewComputeEmbeddingsRequestWithDimensions(
			"text-embedding-004",
			chunks,
			domainGateway.RetrievalQuery,
			0,
		)

		embeddings, err := gw.ComputeEmbeddings(ctx, request)
		require.NoError(t, err)
		assert.NotEmpty(t, embeddings)
	})

	t.Run("Negative dimensions", func(t *testing.T) {
		chunks := []string{"test"}
		request := domainGateway.NewComputeEmbeddingsRequestWithDimensions(
			"text-embedding-004",
			chunks,
			domainGateway.RetrievalQuery,
			-1,
		)

		embeddings, err := gw.ComputeEmbeddings(ctx, request)
		require.NoError(t, err)
		assert.NotEmpty(t, embeddings)
	})

	t.Run("Empty chunks returns no error", func(t *testing.T) {
		request := domainGateway.NewComputeEmbeddingsRequest(
			"text-embedding-004",
			[]string{},
			domainGateway.RetrievalQuery,
		)

		// Empty chunks: the API call is still made; result depends on server behavior.
		_, err := gw.ComputeEmbeddings(ctx, request)
		require.NoError(t, err)
	})
}

func TestGeminiGateway_GenerateContent_EdgeCases(t *testing.T) {
	server := newGeminiMockServer(t)
	defer server.Close()

	gw := newTestGeminiGateway(t, server.URL)
	ctx := context.Background()

	t.Run("Empty system prompt", func(t *testing.T) {
		request := domainGateway.NewGenerateContentRequest(
			"gemini-1.5-flash",
			"",
			"Hello, how are you?",
			4096,
		)

		resp, err := gw.GenerateContent(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("Empty user prompt", func(t *testing.T) {
		request := domainGateway.NewGenerateContentRequest(
			"gemini-1.5-flash",
			"You are a helpful assistant",
			"",
			4096,
		)

		resp, err := gw.GenerateContent(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("Both prompts empty", func(t *testing.T) {
		request := domainGateway.NewGenerateContentRequest(
			"gemini-1.5-flash",
			"",
			"",
			4096,
		)

		resp, err := gw.GenerateContent(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}
