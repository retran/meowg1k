// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package llama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		apiKey      string
		errorMsg    string
		expectError bool
	}{
		{
			name:        "Valid service creation",
			baseURL:     "http://localhost:8080",
			apiKey:      "test-key",
			expectError: false,
		},
		{
			name:        "Client creation with empty API key",
			baseURL:     "http://localhost:8080",
			apiKey:      "",
			expectError: false, // API key is optional for llama
		},
		{
			name:        "Client creation with empty base URL",
			baseURL:     "",
			apiKey:      "test-key",
			expectError: true,
			errorMsg:    "base URL cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewClient(tt.baseURL, tt.apiKey, &http.Client{})

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, service)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, service)
				assert.Equal(t, tt.baseURL, service.baseURL)
				assert.Equal(t, tt.apiKey, service.apiKey)
				assert.NotNil(t, service.httpClient)
			}
		})
	}
}

func TestServiceImpl_Complete(t *testing.T) {
	tests := []struct {
		request        *CompletionRequest
		name           string
		mockError      string
		errorMsg       string
		mockResponse   CompletionResponse
		mockStatusCode int
		expectError    bool
	}{
		{
			name: "Successful completion",
			request: &CompletionRequest{
				Prompt:      "Hello world",
				Temperature: 0.7,
				NPredict:    100,
			},
			mockResponse: CompletionResponse{
				Content:         "Hello! How can I help you today?",
				Model:           "llama-3.2-90b",
				TokensEvaluated: 10,
				TokensCached:    2,
			},
			mockStatusCode: 200,
			expectError:    false,
		},
		{
			name: "Server error response",
			request: &CompletionRequest{
				Prompt: "Test prompt",
			},
			mockStatusCode: 500,
			mockError:      "Internal Server Error",
			expectError:    true,
			errorMsg:       "failed with status 500",
		},
		{
			name: "Invalid JSON response",
			request: &CompletionRequest{
				Prompt: "Test prompt",
			},
			mockStatusCode: 200,
			mockError:      "invalid json",
			expectError:    true,
			errorMsg:       "failed to unmarshal response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify the request method and path
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/completion", r.URL.Path)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				// Set status code
				w.WriteHeader(tt.mockStatusCode)

				if tt.mockError != "" {
					w.Write([]byte(tt.mockError))
				} else {
					responseBytes, _ := json.Marshal(tt.mockResponse)
					w.Write(responseBytes)
				}
			}))
			defer server.Close()

			// Create service with test server URL
			service, err := NewClient(server.URL, "test-api-key", &http.Client{})
			require.NoError(t, err)

			// Call Complete method
			ctx := context.Background()
			result, err := service.Complete(ctx, tt.request)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.mockResponse.Content, result.Content)
			assert.Equal(t, tt.mockResponse.Model, result.Model)
			assert.Equal(t, tt.mockResponse.TokensEvaluated, result.TokensEvaluated)
			assert.Equal(t, tt.mockResponse.TokensCached, result.TokensCached)
		})
	}
}

func TestServiceImpl_CompleteWithAuthHeader(t *testing.T) {
	// Test that API key is properly set in Authorization header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Authorization header
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

		w.WriteHeader(200)
		response := CompletionResponse{
			Content: "Test response",
		}
		responseBytes, _ := json.Marshal(response)
		w.Write(responseBytes)
	}))
	defer server.Close()

	service, err := NewClient(server.URL, "test-api-key", &http.Client{})
	require.NoError(t, err)

	ctx := context.Background()
	request := &CompletionRequest{
		Prompt: "Test prompt",
	}

	_, err = service.Complete(ctx, request)
	require.NoError(t, err)
}

func TestServiceImpl_CompleteWithoutAPIKey(t *testing.T) {
	// Test that no Authorization header is set when API key is empty
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify no Authorization header
		assert.Empty(t, r.Header.Get("Authorization"))

		w.WriteHeader(200)
		response := CompletionResponse{
			Content: "Test response",
		}
		responseBytes, _ := json.Marshal(response)
		w.Write(responseBytes)
	}))
	defer server.Close()

	service, err := NewClient(server.URL, "", &http.Client{})
	require.NoError(t, err)

	ctx := context.Background()
	request := &CompletionRequest{
		Prompt: "Test prompt",
	}

	_, err = service.Complete(ctx, request)
	require.NoError(t, err)
}

func TestServiceImpl_CompleteWithContext(t *testing.T) {
	// Test context cancellation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow response
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(200)
		response := CompletionResponse{Content: "Test response"}
		responseBytes, _ := json.Marshal(response)
		w.Write(responseBytes)
	}))
	defer server.Close()

	service, err := NewClient(server.URL, "test-key", &http.Client{})
	require.NoError(t, err)

	// Create a context that will be canceled
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	request := &CompletionRequest{
		Prompt: "Test prompt",
	}

	_, err = service.Complete(ctx, request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestCompletionRequestFieldsValidation(t *testing.T) {
	// Test various completion request fields can be set
	req := &CompletionRequest{
		Prompt:           "Test prompt",
		Temperature:      0.8,
		DynatempRange:    0.5,
		DynatempExponent: 1.0,
		TopK:             40,
		TopP:             0.9,
		MinP:             0.05,
		NPredict:         150,
		NIndent:          0,
		NKeep:            10,
		Stream:           false,
		Stop:             []string{"</s>", "\n"},
		TypicalP:         1.0,
		RepeatPenalty:    1.1,
		RepeatLastN:      64,
		PresencePenalty:  0.0,
		FrequencyPenalty: 0.0,
		DryMultiplier:    0.0,
		DryBase:          1.75,
	}

	// Test marshaling works
	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test unmarshaling works
	var unmarshaledReq CompletionRequest
	err = json.Unmarshal(data, &unmarshaledReq)
	require.NoError(t, err)
	assert.InEpsilon(t, req.Temperature, unmarshaledReq.Temperature, 0.001)
	assert.Equal(t, req.TopK, unmarshaledReq.TopK)
	assert.Equal(t, req.Stop, unmarshaledReq.Stop)
}

func TestServiceImpl_CompleteEdgeCases(t *testing.T) {
	tests := []struct {
		setupServer func() *httptest.Server
		request     *CompletionRequest
		name        string
		errorMsg    string
		expectError bool
	}{
		{
			name: "Request marshaling error - invalid request structure",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					w.Write([]byte(`{"content": "test"}`))
				}))
			},
			request: &CompletionRequest{
				Prompt: "Test", // Valid request, test will use invalid JSON
			},
			expectError: false, // This specific case won't fail marshaling
		},
		{
			name: "HTTP request creation with invalid URL characters",
			setupServer: func() *httptest.Server {
				// This will be overridden with invalid URL
				return nil
			},
			request: &CompletionRequest{
				Prompt: "Test prompt",
			},
			expectError: true,
			errorMsg:    "failed to create HTTP request",
		},
		{
			name: "Response body read error simulation",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					// Write invalid JSON to test unmarshaling error path
					w.Write([]byte(`{"invalid": json}`))
				}))
			},
			request: &CompletionRequest{
				Prompt: "Test prompt",
			},
			expectError: true,
			errorMsg:    "failed to unmarshal response",
		},
		{
			name: "Various HTTP error status codes",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(404)
					w.Write([]byte("Not Found"))
				}))
			},
			request: &CompletionRequest{
				Prompt: "Test prompt",
			},
			expectError: true,
			errorMsg:    "failed with status 404",
		},
		{
			name: "Client error status codes",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(400)
					w.Write([]byte("Bad Request"))
				}))
			},
			request: &CompletionRequest{
				Prompt: "Test prompt",
			},
			expectError: true,
			errorMsg:    "failed with status 400",
		},
		{
			name: "Malformed JSON response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					w.Write([]byte(`{"content": "incomplete json`))
				}))
			},
			request: &CompletionRequest{
				Prompt: "Test prompt",
			},
			expectError: true,
			errorMsg:    "failed to unmarshal response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var service *Client
			var err error

			if tt.name == "HTTP request creation with invalid URL characters" {
				// Test with service that has invalid URL
				service, err = NewClient("ht tp://invalid url with spaces", "key", &http.Client{})
				require.NoError(t, err) // Client creation succeeds

				ctx := context.Background()
				_, err = service.Complete(ctx, tt.request)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				return
			}

			server := tt.setupServer()
			if server != nil {
				defer server.Close()
				service, err = NewClient(server.URL, "test-key", &http.Client{})
			} else {
				service, err = NewClient("http://localhost:8080", "test-key", &http.Client{})
			}

			require.NoError(t, err)

			ctx := context.Background()
			result, err := service.Complete(ctx, tt.request)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
		})
	}
}

func TestCompletionResponseValidation(t *testing.T) {
	// Test comprehensive response structure
	response := CompletionResponse{
		Content:         "Generated content here",
		Model:           "llama-3.2-90b-instruct",
		TokensEvaluated: 150,
		TokensCached:    50,
		GenerationSettings: map[string]any{
			"frequency_penalty": 0.1,
			"presence_penalty":  0.2,
			"repeat_penalty":    1.05,
			"temperature":       0.7,
			"top_k":             40,
			"top_p":             0.9,
			"typical_p":         1.0,
		},
		Prompt:   "Original prompt text",
		Stop:     true,
		StopType: "stop_word",
		Timings: map[string]any{
			"predicted_n":  150,
			"predicted_ms": 2500.5,
		},
		Tokens:    []int{1, 2, 3, 4, 5},
		Truncated: false,
		Probs: []TokenProb{
			{
				ID:      1,
				Token:   "test",
				Logprob: -0.5,
				Bytes:   []int{116, 101, 115, 116},
			},
		},
	}

	// Test marshaling
	data, err := json.Marshal(response)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test unmarshaling
	var unmarshaledResp CompletionResponse
	err = json.Unmarshal(data, &unmarshaledResp)
	require.NoError(t, err)
	assert.Equal(t, response.Content, unmarshaledResp.Content)
	assert.Equal(t, response.Model, unmarshaledResp.Model)
	assert.Equal(t, response.TokensEvaluated, unmarshaledResp.TokensEvaluated)
	assert.Equal(t, response.TokensCached, unmarshaledResp.TokensCached)
	assert.Equal(t, response.GenerationSettings["temperature"], unmarshaledResp.GenerationSettings["temperature"])
	// Use type assertion for map values that could be different numeric types
	assert.InEpsilon(t, float64(150), unmarshaledResp.Timings["predicted_n"], 0.001)
	assert.Equal(t, len(response.Tokens), len(unmarshaledResp.Tokens))
	assert.Equal(t, len(response.Probs), len(unmarshaledResp.Probs))
}

func TestServiceImpl_Embedding(t *testing.T) {
	tests := []struct {
		request        *EmbeddingRequest
		name           string
		mockResponse   string
		errorMsg       string
		expectedEmbed  []float64
		mockStatusCode int
		expectError    bool
	}{
		{
			name: "Successful embedding with nested array format",
			request: &EmbeddingRequest{
				Content: "Hello world",
			},
			mockResponse:   `[[0.1, 0.2, 0.3, 0.4, 0.5]]`,
			mockStatusCode: 200,
			expectedEmbed:  []float64{0.1, 0.2, 0.3, 0.4, 0.5},
			expectError:    false,
		},
		{
			name: "Successful embedding with simple array format",
			request: &EmbeddingRequest{
				Content: "Test text",
			},
			mockResponse:   `[0.9, 0.8, 0.7, 0.6]`,
			mockStatusCode: 200,
			expectedEmbed:  []float64{0.9, 0.8, 0.7, 0.6},
			expectError:    false,
		},
		{
			name: "Successful embedding with object format",
			request: &EmbeddingRequest{
				Content: "Another test",
			},
			mockResponse:   `{"embedding": [0.5, 0.4, 0.3], "model": "test-model", "tokens_evaluated": 5}`,
			mockStatusCode: 200,
			expectedEmbed:  []float64{0.5, 0.4, 0.3},
			expectError:    false,
		},
		{
			name: "Server error response",
			request: &EmbeddingRequest{
				Content: "Test",
			},
			mockStatusCode: 500,
			mockResponse:   "Internal Server Error",
			expectError:    true,
			errorMsg:       "failed with status 500",
		},
		{
			name: "Invalid JSON response",
			request: &EmbeddingRequest{
				Content: "Test",
			},
			mockStatusCode: 200,
			mockResponse:   "not a valid json",
			expectError:    true,
			errorMsg:       "failed to unmarshal response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/embedding", r.URL.Path)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			client, err := NewClient(server.URL, "test-key", &http.Client{})
			require.NoError(t, err)

			ctx := context.Background()
			result, err := client.Embedding(ctx, tt.request)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, len(tt.expectedEmbed), len(result.Embedding))
			for i, val := range tt.expectedEmbed {
				assert.InEpsilon(t, val, result.Embedding[i], 0.0001)
			}
		})
	}
}

func TestServiceImpl_EmbeddingWithAuthHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		w.WriteHeader(200)
		w.Write([]byte(`[0.1, 0.2, 0.3]`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-api-key", &http.Client{})
	require.NoError(t, err)

	ctx := context.Background()
	req := &EmbeddingRequest{Content: "Test"}
	_, err = client.Embedding(ctx, req)
	require.NoError(t, err)
}

func TestServiceImpl_EmbeddingWithoutAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		w.WriteHeader(200)
		w.Write([]byte(`[0.1, 0.2, 0.3]`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "", &http.Client{})
	require.NoError(t, err)

	ctx := context.Background()
	req := &EmbeddingRequest{Content: "Test"}
	_, err = client.Embedding(ctx, req)
	require.NoError(t, err)
}

func TestServiceImpl_EmbeddingWithContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(200)
		w.Write([]byte(`[0.1, 0.2, 0.3]`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-key", &http.Client{})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req := &EmbeddingRequest{Content: "Test"}
	_, err = client.Embedding(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestServiceImpl_EmbeddingNilClient(t *testing.T) {
	var client *Client
	ctx := context.Background()
	req := &EmbeddingRequest{Content: "Test"}

	result, err := client.Embedding(ctx, req)
	require.Error(t, err)
	require.Nil(t, result)
	assert.Contains(t, err.Error(), "client is nil")
}

func TestServiceImpl_EmbeddingNilContext(t *testing.T) {
	client, err := NewClient("http://localhost:8080", "key", &http.Client{})
	require.NoError(t, err)

	req := &EmbeddingRequest{Content: "Test"}
	//nolint:staticcheck // intentionally testing nil context handling
	result, err := client.Embedding(nil, req)
	require.Error(t, err)
	require.Nil(t, result)
	assert.Contains(t, err.Error(), "context cannot be nil")
}

func TestServiceImpl_EmbeddingNilRequest(t *testing.T) {
	client, err := NewClient("http://localhost:8080", "key", &http.Client{})
	require.NoError(t, err)

	ctx := context.Background()
	result, err := client.Embedding(ctx, nil)
	require.Error(t, err)
	require.Nil(t, result)
	assert.Contains(t, err.Error(), "request cannot be nil")
}

func TestServiceImpl_EmbeddingWithNormOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request body contains norm_output flag
		var reqBody EmbeddingRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		assert.True(t, reqBody.NormOutput)

		w.WriteHeader(200)
		w.Write([]byte(`[0.1, 0.2, 0.3]`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "key", &http.Client{})
	require.NoError(t, err)

	ctx := context.Background()
	req := &EmbeddingRequest{
		Content:    "Test",
		NormOutput: true,
	}
	result, err := client.Embedding(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestEmbeddingRequestValidation(t *testing.T) {
	req := &EmbeddingRequest{
		Content:    "Test content",
		NormOutput: true,
		Truncate:   512,
		ImageData: []ImageData{
			{Data: "base64data", ID: 1},
		},
		Lora: []Lora{
			{ID: 1, Scale: 0.5},
		},
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var unmarshaledReq EmbeddingRequest
	err = json.Unmarshal(data, &unmarshaledReq)
	require.NoError(t, err)
	assert.Equal(t, req.Content, unmarshaledReq.Content)
	assert.Equal(t, req.NormOutput, unmarshaledReq.NormOutput)
	assert.Equal(t, req.Truncate, unmarshaledReq.Truncate)
}

func TestEmbeddingResponseValidation(t *testing.T) {
	resp := &EmbeddingResponse{
		Embedding:       []float64{0.1, 0.2, 0.3, 0.4, 0.5},
		Model:           "text-embedding-model",
		TokensEvaluated: 10,
		Timings: map[string]any{
			"prompt_ms": 5.2,
			"eval_ms":   10.8,
		},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var unmarshaledResp EmbeddingResponse
	err = json.Unmarshal(data, &unmarshaledResp)
	require.NoError(t, err)
	assert.Equal(t, len(resp.Embedding), len(unmarshaledResp.Embedding))
	assert.Equal(t, resp.Model, unmarshaledResp.Model)
	assert.Equal(t, resp.TokensEvaluated, unmarshaledResp.TokensEvaluated)
}

func TestServiceImpl_EmbeddingBatch(t *testing.T) {
	tests := []struct {
		name           string
		mockResponse   string
		errorMsg       string
		texts          []string
		mockStatusCode int
		expectedCount  int
		normOutput     bool
		expectError    bool
	}{
		{
			name:       "Successful batch embedding",
			texts:      []string{"Hello", "World", "Test"},
			normOutput: false,
			mockResponse: `[
				{"index": 0, "embedding": [[0.1, 0.2, 0.3]]},
				{"index": 1, "embedding": [[0.4, 0.5, 0.6]]},
				{"index": 2, "embedding": [[0.7, 0.8, 0.9]]}
			]`,
			mockStatusCode: 200,
			expectedCount:  3,
			expectError:    false,
		},
		{
			name:       "Batch with normalized output",
			texts:      []string{"Test1", "Test2"},
			normOutput: true,
			mockResponse: `[
				{"index": 0, "embedding": [[0.5, 0.5]]},
				{"index": 1, "embedding": [[0.6, 0.4]]}
			]`,
			mockStatusCode: 200,
			expectedCount:  2,
			expectError:    false,
		},
		{
			name:           "Empty texts array",
			texts:          []string{},
			normOutput:     false,
			mockResponse:   `[]`,
			mockStatusCode: 200,
			expectedCount:  0,
			expectError:    false,
		},
		{
			name:           "Server error",
			texts:          []string{"Test"},
			normOutput:     false,
			mockResponse:   "Internal Server Error",
			mockStatusCode: 500,
			expectError:    true,
			errorMsg:       "failed with status 500",
		},
		{
			name:           "Invalid JSON response",
			texts:          []string{"Test"},
			normOutput:     false,
			mockResponse:   "invalid json",
			mockStatusCode: 200,
			expectError:    true,
			errorMsg:       "failed to unmarshal batch response",
		},
		{
			name:       "Invalid index in response",
			texts:      []string{"Test1", "Test2"},
			normOutput: false,
			mockResponse: `[
				{"index": 0, "embedding": [[0.1, 0.2]]},
				{"index": 5, "embedding": [[0.3, 0.4]]}
			]`,
			mockStatusCode: 200,
			expectError:    true,
			errorMsg:       "invalid index 5 in batch response",
		},
		{
			name:       "Empty embedding in response",
			texts:      []string{"Test"},
			normOutput: false,
			mockResponse: `[
				{"index": 0, "embedding": []}
			]`,
			mockStatusCode: 200,
			expectError:    true,
			errorMsg:       "empty embedding for index 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/embedding", r.URL.Path)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				// Verify request body if not empty texts
				if len(tt.texts) > 0 {
					var reqBody EmbeddingRequest
					err := json.NewDecoder(r.Body).Decode(&reqBody)
					require.NoError(t, err)
					assert.Equal(t, tt.normOutput, reqBody.NormOutput)
				}

				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			client, err := NewClient(server.URL, "test-key", &http.Client{})
			require.NoError(t, err)

			ctx := context.Background()
			result, err := client.EmbeddingBatch(ctx, tt.texts, tt.normOutput)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, len(result))

			// Verify embeddings are properly ordered
			if tt.expectedCount > 0 {
				for i, embedding := range result {
					assert.NotEmpty(t, embedding, "Embedding at index %d should not be empty", i)
				}
			}
		})
	}
}

func TestServiceImpl_EmbeddingBatchNilClient(t *testing.T) {
	var client *Client
	ctx := context.Background()
	texts := []string{"Test"}

	result, err := client.EmbeddingBatch(ctx, texts, false)
	require.Error(t, err)
	require.Nil(t, result)
	assert.Contains(t, err.Error(), "client is nil")
}

func TestServiceImpl_EmbeddingBatchNilContext(t *testing.T) {
	client, err := NewClient("http://localhost:8080", "key", &http.Client{})
	require.NoError(t, err)

	texts := []string{"Test"}
	//nolint:staticcheck // intentionally testing nil context handling
	result, err := client.EmbeddingBatch(nil, texts, false)
	require.Error(t, err)
	require.Nil(t, result)
	assert.Contains(t, err.Error(), "context cannot be nil")
}

func TestServiceImpl_EmbeddingBatchWithContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(200)
		w.Write([]byte(`[{"index": 0, "embedding": [[0.1, 0.2]]}]`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-key", &http.Client{})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	texts := []string{"Test"}
	_, err = client.EmbeddingBatch(ctx, texts, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestServiceImpl_EmbeddingBatchWithAuthHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		w.WriteHeader(200)
		w.Write([]byte(`[{"index": 0, "embedding": [[0.1, 0.2]]}]`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-api-key", &http.Client{})
	require.NoError(t, err)

	ctx := context.Background()
	texts := []string{"Test"}
	_, err = client.EmbeddingBatch(ctx, texts, false)
	require.NoError(t, err)
}

func TestServiceImpl_EmbeddingBatchWithoutAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		w.WriteHeader(200)
		w.Write([]byte(`[{"index": 0, "embedding": [[0.1, 0.2]]}]`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "", &http.Client{})
	require.NoError(t, err)

	ctx := context.Background()
	texts := []string{"Test"}
	_, err = client.EmbeddingBatch(ctx, texts, false)
	require.NoError(t, err)
}

func TestEmbeddingBatchItemValidation(t *testing.T) {
	item := EmbeddingBatchItem{
		Index:     0,
		Embedding: [][]float64{{0.1, 0.2, 0.3}},
		Object:    "embedding",
		Model:     "test-model",
	}

	data, err := json.Marshal(item)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var unmarshaledItem EmbeddingBatchItem
	err = json.Unmarshal(data, &unmarshaledItem)
	require.NoError(t, err)
	assert.Equal(t, item.Index, unmarshaledItem.Index)
	assert.Equal(t, len(item.Embedding), len(unmarshaledItem.Embedding))
	assert.Equal(t, item.Object, unmarshaledItem.Object)
	assert.Equal(t, item.Model, unmarshaledItem.Model)
}

func TestNewClientNilHTTPClient(t *testing.T) {
	client, err := NewClient("http://localhost:8080", "key", nil)
	require.Error(t, err)
	require.Nil(t, client)
	assert.Contains(t, err.Error(), "HTTP client cannot be nil")
}

func TestServiceImpl_CompleteNilClient(t *testing.T) {
	var client *Client
	ctx := context.Background()
	req := &CompletionRequest{Prompt: "Test"}

	result, err := client.Complete(ctx, req)
	require.Error(t, err)
	require.Nil(t, result)
	assert.Contains(t, err.Error(), "client is nil")
}

func TestServiceImpl_CompleteNilContext(t *testing.T) {
	client, err := NewClient("http://localhost:8080", "key", &http.Client{})
	require.NoError(t, err)

	req := &CompletionRequest{Prompt: "Test"}
	//nolint:staticcheck // intentionally testing nil context handling
	result, err := client.Complete(nil, req)
	require.Error(t, err)
	require.Nil(t, result)
	assert.Contains(t, err.Error(), "context cannot be nil")
}

func TestServiceImpl_CompleteNilRequest(t *testing.T) {
	client, err := NewClient("http://localhost:8080", "key", &http.Client{})
	require.NoError(t, err)

	ctx := context.Background()
	result, err := client.Complete(ctx, nil)
	require.Error(t, err)
	require.Nil(t, result)
	assert.Contains(t, err.Error(), "request cannot be nil")
}
