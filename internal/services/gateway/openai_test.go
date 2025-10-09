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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	coreGateway "github.com/retran/meowg1k/internal/core/gateway"
	"github.com/retran/meowg1k/internal/core/profile"
)

// Mock HTTP server responses for OpenAI API
func createOpenAIMockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/chat/completions"):
			handleOpenAIChatCompletion(w, r)
		case strings.Contains(r.URL.Path, "/embeddings"):
			handleOpenAIEmbeddings(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
}

func handleOpenAIChatCompletion(w http.ResponseWriter, _ *http.Request) {
	// Simulate OpenAI chat completion response
	response := map[string]interface{}{
		"id":      "chatcmpl-123",
		"object":  "chat.completion",
		"created": 1677652288,
		"model":   "gpt-4",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "This is a test response from the mocked OpenAI API.",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     9,
			"completion_tokens": 12,
			"total_tokens":      21,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleOpenAIEmbeddings(w http.ResponseWriter, _ *http.Request) {
	// Simulate OpenAI embeddings response
	response := map[string]interface{}{
		"object": "list",
		"data": []map[string]interface{}{
			{
				"object":    "embedding",
				"index":     0,
				"embedding": []float64{0.1, 0.2, 0.3, 0.4, 0.5},
			},
		},
		"model": "text-embedding-ada-002",
		"usage": map[string]interface{}{
			"prompt_tokens": 8,
			"total_tokens":  8,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func TestNewOpenAIGateway(t *testing.T) {
	t.Run("with API key", func(t *testing.T) {
		gateway := newOpenAIGateway("https://api.openai.com/v1", "test-api-key")
		if gateway == nil {
			t.Fatal("Expected gateway to be non-nil")
		}
	})

	t.Run("without API key", func(t *testing.T) {
		gateway := newOpenAIGateway("https://api.openai.com/v1", "")
		if gateway == nil {
			t.Fatal("Expected gateway to be non-nil even without API key")
		}
	})

	t.Run("with custom base URL", func(t *testing.T) {
		gateway := newOpenAIGateway("https://custom.example.com/v1", "test-key")
		if gateway == nil {
			t.Fatal("Expected gateway to be non-nil with custom URL")
		}
	})
}

func TestOpenAIGatewayGenerateContent(t *testing.T) {
	// Create mock server
	mockServer := createOpenAIMockServer()
	defer mockServer.Close()

	// Create profile with mock server URL
	profile := &profile.ResolvedProfile{
		BaseURL: mockServer.URL,
		APIKey:  "test-api-key",
	}

	// Create gateway
	gateway := newOpenAIGateway(profile.BaseURL, profile.APIKey)

	// Create test request
	request := coreGateway.NewGenerateContentRequest(
		"gpt-4",
		"You are a helpful assistant.",
		"Hello, how are you?",
		100,
	)

	// Test successful generation
	ctx := context.Background()
	content, err := gateway.GenerateContent(ctx, request)
	if err != nil {
		t.Fatalf("GenerateContent failed: %v", err)
	}

	if content == "" {
		t.Error("Expected non-empty content")
	}

	expectedContent := "This is a test response from the mocked OpenAI API."
	if content != expectedContent {
		t.Errorf("Expected content '%s', got '%s'", expectedContent, content)
	}
}

func TestOpenAIGatewayGenerateContentError(t *testing.T) {
	// Create error server
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer errorServer.Close()

	profile := &profile.ResolvedProfile{
		BaseURL: errorServer.URL,
		APIKey:  "test-api-key",
	}

	gateway := newOpenAIGateway(profile.BaseURL, profile.APIKey)

	request := coreGateway.NewGenerateContentRequest(
		"gpt-4",
		"You are a helpful assistant.",
		"Hello, how are you?",
		100,
	)

	ctx := context.Background()
	_, err := gateway.GenerateContent(ctx, request)

	if err == nil {
		t.Error("Expected error from failing server")
	}

	if !strings.Contains(err.Error(), "failed to generate content") {
		t.Errorf("Expected 'failed to generate content' in error, got: %v", err)
	}
}

func TestOpenAIGatewayComputeEmbeddings(t *testing.T) {
	// Create mock server
	mockServer := createOpenAIMockServer()
	defer mockServer.Close()

	profile := &profile.ResolvedProfile{
		BaseURL: mockServer.URL,
		APIKey:  "test-api-key",
	}

	gateway := newOpenAIGateway(profile.BaseURL, profile.APIKey)

	// Create embeddings request
	request := coreGateway.NewComputeEmbeddingsRequest(
		"text-embedding-ada-002",
		[]string{"Hello world", "This is a test"},
		coreGateway.RetrievalQuery,
	)

	// Test successful embeddings computation
	ctx := context.Background()
	embeddings, err := gateway.ComputeEmbeddings(ctx, request)
	if err != nil {
		t.Fatalf("ComputeEmbeddings failed: %v", err)
	}

	if len(embeddings) == 0 {
		t.Error("Expected non-empty embeddings")
	}

	// Check that we got the expected embedding values
	if len(embeddings) != 1 {
		t.Errorf("Expected 1 embedding, got %d", len(embeddings))
	}

	if len(embeddings[0]) != 5 {
		t.Errorf("Expected embedding of length 5, got %d", len(embeddings[0]))
	}

	// Verify the embedding values match our mock
	expectedValues := []float64{0.1, 0.2, 0.3, 0.4, 0.5}
	for i, expected := range expectedValues {
		if embeddings[0][i] != expected {
			t.Errorf("Expected embedding[%d] = %f, got %f", i, expected, embeddings[0][i])
		}
	}
}

func TestOpenAIGatewayComputeEmbeddingsError(t *testing.T) {
	// Create error server
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Bad Request", http.StatusBadRequest)
	}))
	defer errorServer.Close()

	gateway := newOpenAIGateway(errorServer.URL, "test-api-key")

	request := coreGateway.NewComputeEmbeddingsRequest(
		"text-embedding-ada-002",
		[]string{"Hello world"},
		coreGateway.RetrievalQuery,
	)

	ctx := context.Background()
	_, err := gateway.ComputeEmbeddings(ctx, request)

	if err == nil {
		t.Error("Expected error from failing server")
	}

	if !strings.Contains(err.Error(), "failed to compute embedding") {
		t.Errorf("Expected 'failed to compute embedding' in error, got: %v", err)
	}
}

func TestOpenAIGatewayEmptyResponse(t *testing.T) {
	// Create server that returns empty choices
	emptyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id":      "chatcmpl-123",
			"object":  "chat.completion",
			"created": 1677652288,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{}, // Empty choices
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer emptyServer.Close()

	gateway := newOpenAIGateway(emptyServer.URL, "test-api-key")

	request := coreGateway.NewGenerateContentRequest(
		"gpt-4",
		"System prompt",
		"User prompt",
		100,
	)

	ctx := context.Background()
	_, err := gateway.GenerateContent(ctx, request)

	if err == nil {
		t.Error("Expected error for empty choices")
	}

	expectedError := "no choices returned from OpenAI-compatible API"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
	}
}

func TestOpenAIGatewayTimeout(t *testing.T) {
	// Create slow server
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // Longer than client timeout
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer slowServer.Close()

	gateway := newOpenAIGateway(slowServer.URL, "test-api-key")

	request := coreGateway.NewGenerateContentRequest(
		"gpt-4",
		"System prompt",
		"User prompt",
		100,
	)

	// Use context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := gateway.GenerateContent(ctx, request)

	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestOpenAIGatewayComputeEmbeddingsWithDimensions(t *testing.T) {
	// Create mock server
	mockServer := createOpenAIMockServer()
	defer mockServer.Close()

	gateway := newOpenAIGateway(mockServer.URL, "test-api-key")

	// Create embeddings request with dimensions
	request := coreGateway.NewComputeEmbeddingsRequestWithDimensions(
		"text-embedding-ada-002",
		[]string{"Hello world"},
		coreGateway.RetrievalQuery,
		512, // This should trigger the dimensions parameter setting
	)

	// Test embeddings computation with dimensions
	ctx := context.Background()
	embeddings, err := gateway.ComputeEmbeddings(ctx, request)
	if err != nil {
		t.Fatalf("ComputeEmbeddings with dimensions failed: %v", err)
	}

	if len(embeddings) == 0 {
		t.Error("Expected non-empty embeddings")
	}
}
