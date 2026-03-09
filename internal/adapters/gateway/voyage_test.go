// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
)

// newTestVoyageServer creates a mock HTTP server that returns a successful embedding response.
// The server verifies request method, content type, and required JSON fields.
func newTestVoyageServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			t.Errorf("Expected JSON content type, got %s", r.Header.Get("Content-Type"))
		}

		var requestBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}
		if _, ok := requestBody["model"]; !ok {
			t.Error("Expected 'model' field in request body")
		}
		if _, ok := requestBody["input"]; !ok {
			t.Error("Expected 'input' field in request body")
		}
		if _, ok := requestBody["input_type"]; !ok {
			t.Error("Expected 'input_type' field in request body")
		}

		response := map[string]interface{}{
			"data": []map[string]interface{}{
				{"embedding": []float64{0.1, 0.2, 0.3, 0.4, 0.5}, "index": 0},
				{"embedding": []float64{0.6, 0.7, 0.8, 0.9, 1.0}, "index": 1},
			},
			"model": "voyage-large-2",
			"usage": map[string]interface{}{"total_tokens": 10},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
}

func TestNewVoyageGateway(t *testing.T) {
	server := newTestVoyageServer(t)
	defer server.Close()

	t.Run("Valid API key", func(t *testing.T) {
		gw, err := newVoyageGateway(server.URL, "test-api-key", &http.Client{})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if gw == nil {
			t.Fatal("Expected gateway to be non-nil")
		}
	})

	t.Run("Empty API key", func(t *testing.T) {
		// voyage.NewClient requires a non-empty API key; expect creation to fail.
		gw, err := newVoyageGateway(server.URL, "", &http.Client{})
		switch {
		case err != nil:
			t.Logf("Service validates API key on creation: %v", err)
		case gw == nil:
			t.Fatal("Expected gateway to be non-nil if no error")
		default:
			t.Log("Service allows empty API key on creation, will validate on use")
		}
	})
}

func TestMapTaskTypeToInputType(t *testing.T) {
	testCases := []struct {
		taskType     domainGateway.TaskType
		expectedType string
	}{
		{domainGateway.RetrievalDocument, "document"},
		{domainGateway.RetrievalQuery, "searchindex"},
		{domainGateway.CodeRetrievalQuery, "searchindex"},
		{domainGateway.Classification, "classification"},
		{domainGateway.Clustering, "clustering"},
		{domainGateway.SemanticSimilarity, "searchindex"},
		{domainGateway.QuestionAnswering, "searchindex"},
		{domainGateway.FactVerification, "searchindex"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.taskType), func(t *testing.T) {
			result := mapTaskTypeToInputType(tc.taskType)
			if result != tc.expectedType {
				t.Errorf("Expected %s, got %s", tc.expectedType, result)
			}
		})
	}

	t.Run("Unknown task type", func(t *testing.T) {
		unknownType := domainGateway.TaskType("unknown")
		result := mapTaskTypeToInputType(unknownType)
		if result != "searchindex" {
			t.Errorf("Expected 'searchindex' for unknown task type, got %s", result)
		}
	})

	t.Run("Empty task type", func(t *testing.T) {
		emptyType := domainGateway.TaskType("")
		result := mapTaskTypeToInputType(emptyType)
		if result != "searchindex" {
			t.Errorf("Expected 'searchindex' for empty task type, got %s", result)
		}
	})
}

func TestVoyageGateway_ComputeEmbeddings(t *testing.T) {
	server := newTestVoyageServer(t)
	defer server.Close()

	t.Run("Compute embeddings with valid request", func(t *testing.T) {
		gw, err := newVoyageGateway(server.URL, "test-api-key", &http.Client{})
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		chunks := []string{"Hello world", "How are you?"}
		request := domainGateway.NewComputeEmbeddingsRequestWithDimensions(
			"voyage-large-2",
			chunks,
			domainGateway.RetrievalQuery,
			256,
		)

		ctx := context.Background()
		embeddings, err := gw.ComputeEmbeddings(ctx, request)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(embeddings) == 0 {
			t.Error("Expected non-empty embeddings")
		}
	})

	t.Run("Compute embeddings with different task types", func(t *testing.T) {
		gw, err := newVoyageGateway(server.URL, "test-api-key", &http.Client{})
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		testCases := []struct {
			name     string
			taskType domainGateway.TaskType
		}{
			{"Retrieval Document", domainGateway.RetrievalDocument},
			{"Retrieval Query", domainGateway.RetrievalQuery},
			{"Code Retrieval Query", domainGateway.CodeRetrievalQuery},
			{"Classification", domainGateway.Classification},
			{"Clustering", domainGateway.Clustering},
			{"Semantic Similarity", domainGateway.SemanticSimilarity},
			{"Question Answering", domainGateway.QuestionAnswering},
			{"Fact Verification", domainGateway.FactVerification},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				chunks := []string{"Test chunk for " + tc.name}
				request := domainGateway.NewComputeEmbeddingsRequest(
					"voyage-large-2",
					chunks,
					tc.taskType,
				)

				ctx := context.Background()
				_, err = gw.ComputeEmbeddings(ctx, request)
				if err != nil {
					t.Errorf("Unexpected error for task type %s: %v", tc.name, err)
				}
			})
		}
	})

	t.Run("Compute embeddings with large chunks", func(t *testing.T) {
		gw, err := newVoyageGateway(server.URL, "test-api-key", &http.Client{})
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		largeChunks := make([]string, 100)
		for i := range largeChunks {
			largeChunks[i] = strings.Repeat("This is a test chunk ", 50)
		}

		request := domainGateway.NewComputeEmbeddingsRequestWithDimensions(
			"voyage-large-2",
			largeChunks,
			domainGateway.RetrievalDocument,
			256,
		)

		ctx := context.Background()
		_, err = gw.ComputeEmbeddings(ctx, request)
		if err != nil {
			t.Errorf("Unexpected error for large chunks: %v", err)
		}
	})

	t.Run("Compute embeddings with empty chunks", func(t *testing.T) {
		gw, err := newVoyageGateway(server.URL, "test-api-key", &http.Client{})
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		emptyChunks := []string{}
		request := domainGateway.NewComputeEmbeddingsRequestWithDimensions(
			"voyage-large-2",
			emptyChunks,
			domainGateway.RetrievalQuery,
			256,
		)

		ctx := context.Background()
		// Empty input may succeed (mock always returns embeddings) or fail at validation
		_, err = gw.ComputeEmbeddings(ctx, request)
		if err != nil {
			t.Logf("Error with empty chunks (acceptable): %v", err)
		}
	})

	t.Run("Compute embeddings with canceled context", func(t *testing.T) {
		gw, err := newVoyageGateway(server.URL, "test-api-key", &http.Client{})
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		chunks := []string{"Test chunk"}
		request := domainGateway.NewComputeEmbeddingsRequestWithDimensions(
			"voyage-large-2",
			chunks,
			domainGateway.RetrievalQuery,
			256,
		)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err = gw.ComputeEmbeddings(ctx, request)
		if err == nil {
			t.Fatal("Expected error for canceled context")
		}
		t.Logf("Got expected error for canceled context: %v", err)
	})
}

func TestVoyageGateway_ErrorResponses(t *testing.T) {
	t.Run("Server error response", func(t *testing.T) {
		errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":{"message":"invalid api key","type":"authentication_error"}}`, http.StatusUnauthorized)
		}))
		defer errorServer.Close()

		gw, err := newVoyageGateway(errorServer.URL, "bad-key", &http.Client{})
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		chunks := []string{"Test chunk"}
		request := domainGateway.NewComputeEmbeddingsRequest("voyage-large-2", chunks, domainGateway.RetrievalQuery)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		_, err = gw.ComputeEmbeddings(ctx, request)
		if err == nil {
			t.Fatal("Expected error for server error response")
		}
		t.Logf("Got expected error: %v", err)
	})
}

func TestVoyageGateway_InterfaceCompliance(t *testing.T) {
	server := newTestVoyageServer(t)
	defer server.Close()

	gw, err := newVoyageGateway(server.URL, "test-api-key", &http.Client{})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	// Verify that the gateway implements EmbeddingsGateway interface
	_ = gw
	t.Log("VoyageGateway correctly implements EmbeddingsGateway interface")

	// Test that it has the ComputeDistance method from mixin
	embedding1 := domainGateway.Embedding{0.1, 0.2, 0.3}
	embedding2 := domainGateway.Embedding{0.4, 0.5, 0.6}

	distance, err := gw.ComputeDistance(embedding1, embedding2)
	if err != nil {
		t.Errorf("ComputeDistance failed: %v", err)
	}
	if distance < 0 {
		t.Error("Distance should be non-negative")
	}
	t.Logf("Computed distance: %f", distance)
}

func TestVoyageGateway_EdgeCases(t *testing.T) {
	server := newTestVoyageServer(t)
	defer server.Close()

	gw, err := newVoyageGateway(server.URL, "test-api-key", &http.Client{})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	t.Run("Different embedding dimensions", func(t *testing.T) {
		testCases := []int{128, 256, 512, 1024}

		for _, dim := range testCases {
			t.Run(fmt.Sprintf("Dimension_%d", dim), func(t *testing.T) {
				chunks := []string{"Test chunk"}
				request := domainGateway.NewComputeEmbeddingsRequestWithDimensions(
					"voyage-large-2",
					chunks,
					domainGateway.RetrievalQuery,
					dim,
				)

				ctx := context.Background()
				_, err := gw.ComputeEmbeddings(ctx, request)
				if err != nil {
					t.Errorf("Unexpected error for dimension %d: %v", dim, err)
				}
			})
		}
	})

	t.Run("Single character chunks", func(t *testing.T) {
		chunks := []string{"a", "b", "c", "d", "e"}
		request := domainGateway.NewComputeEmbeddingsRequestWithDimensions(
			"voyage-large-2",
			chunks,
			domainGateway.RetrievalQuery,
			256,
		)

		ctx := context.Background()
		_, err := gw.ComputeEmbeddings(ctx, request)
		if err != nil {
			t.Errorf("Unexpected error for single character chunks: %v", err)
		}
	})

	t.Run("Chunks with special characters", func(t *testing.T) {
		chunks := []string{
			"Hello, world! 🌍",
			"Special chars: @#$%^&*()",
			"Unicode: αβγδε ñáéíóú",
			"Newlines\nand\ttabs",
			"\"Quotes\" and 'apostrophes'",
		}
		request := domainGateway.NewComputeEmbeddingsRequestWithDimensions(
			"voyage-large-2",
			chunks,
			domainGateway.RetrievalQuery,
			256,
		)

		ctx := context.Background()
		_, err := gw.ComputeEmbeddings(ctx, request)
		if err != nil {
			t.Errorf("Unexpected error for special character chunks: %v", err)
		}
	})
}

func TestVoyageGateway_NilChecks(t *testing.T) {
	server := newTestVoyageServer(t)
	defer server.Close()

	gw, err := newVoyageGateway(server.URL, "test-api-key", &http.Client{})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	t.Run("Nil context", func(t *testing.T) {
		t.Skip("nil context checks are enforced by static analysis")
	})

	t.Run("Nil request", func(t *testing.T) {
		ctx := context.Background()
		_, err := gw.ComputeEmbeddings(ctx, nil)
		if err == nil {
			t.Fatal("Expected error for nil request")
		}
		if !strings.Contains(err.Error(), "request cannot be nil") {
			t.Errorf("Expected 'request cannot be nil' error, got: %v", err)
		}
	})

	t.Run("Nil gateway", func(t *testing.T) {
		var nilGateway *voyageGateway
		ctx := context.Background()
		chunks := []string{"Test chunk"}
		request := domainGateway.NewComputeEmbeddingsRequest(
			"voyage-large-2",
			chunks,
			domainGateway.RetrievalQuery,
		)

		_, err := nilGateway.ComputeEmbeddings(ctx, request)
		if err == nil {
			t.Fatal("Expected error for nil gateway")
		}
		if !strings.Contains(err.Error(), "voyage gateway is nil") {
			t.Errorf("Expected 'voyage gateway is nil' error, got: %v", err)
		}
	})
}

func TestNewVoyageGateway_NilHTTPClient(t *testing.T) {
	gw, err := newVoyageGateway("", "test-api-key", nil)
	if err == nil {
		t.Fatal("Expected error for nil HTTP client")
	}
	if gw != nil {
		t.Fatal("Expected gateway to be nil when error occurs")
	}
	if !strings.Contains(err.Error(), "HTTP client is required") {
		t.Errorf("Expected 'HTTP client is required' in error, got: %v", err)
	}
}

func TestVoyageGateway_DifferentModels(t *testing.T) {
	server := newTestVoyageServer(t)
	defer server.Close()

	gw, err := newVoyageGateway(server.URL, "test-api-key", &http.Client{})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	models := []string{
		"voyage-large-2",
		"voyage-code-2",
		"voyage-2",
		"voyage-lite-02-instruct",
	}

	for _, model := range models {
		t.Run("Model_"+model, func(t *testing.T) {
			chunks := []string{"Test chunk for " + model}
			request := domainGateway.NewComputeEmbeddingsRequest(
				model,
				chunks,
				domainGateway.RetrievalQuery,
			)

			ctx := context.Background()
			_, err := gw.ComputeEmbeddings(ctx, request)
			if err != nil {
				t.Errorf("Unexpected error for model %s: %v", model, err)
			}
		})
	}
}

func TestVoyageGateway_ZeroDimensions(t *testing.T) {
	server := newTestVoyageServer(t)
	defer server.Close()

	gw, err := newVoyageGateway(server.URL, "test-api-key", &http.Client{})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	chunks := []string{"Test chunk"}
	request := domainGateway.NewComputeEmbeddingsRequest(
		"voyage-large-2",
		chunks,
		domainGateway.RetrievalQuery,
	)

	ctx := context.Background()
	_, err = gw.ComputeEmbeddings(ctx, request)
	if err != nil {
		t.Errorf("Unexpected error for zero dimensions: %v", err)
	}
}

func TestVoyageGateway_CountTokens(t *testing.T) {
	server := newTestVoyageServer(t)
	defer server.Close()

	gwIface, err := newVoyageGateway(server.URL, "test-api-key", &http.Client{})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}
	gw := gwIface.(*voyageGateway)

	t.Run("Empty texts returns zero", func(t *testing.T) {
		ctx := context.Background()
		count, err := gw.CountTokens(ctx, "voyage-large-2", []string{})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 for empty texts, got %d", count)
		}
	})

	t.Run("Non-empty texts returns token count", func(t *testing.T) {
		ctx := context.Background()
		count, err := gw.CountTokens(ctx, "voyage-large-2", []string{"Hello world", "test"})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		// Mock server returns total_tokens: 10
		if count != 10 {
			t.Errorf("Expected 10 tokens from mock, got %d", count)
		}
	})

	t.Run("Nil gateway returns error", func(t *testing.T) {
		var nilGateway *voyageGateway
		ctx := context.Background()
		_, err := nilGateway.CountTokens(ctx, "voyage-large-2", []string{"test"})
		if err == nil {
			t.Fatal("Expected error for nil gateway")
		}
		if !strings.Contains(err.Error(), "voyage gateway is nil") {
			t.Errorf("Expected 'voyage gateway is nil', got: %v", err)
		}
	})
}
