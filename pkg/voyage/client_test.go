// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package voyage

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
		apiKey      string
		baseURL     string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid service creation",
			apiKey:      "test-api-key",
			baseURL:     "https://api.voyageai.com/v1",
			expectError: false,
		},
		{
			name:        "Client creation with default base URL",
			apiKey:      "test-api-key",
			expectError: false,
		},
		{
			name:        "Client creation with default base URL 2",
			apiKey:      "test-api-key",
			baseURL:     "https://custom.api.com",
			expectError: false,
		},
		{
			name:        "Client creation without API key",
			baseURL:     "https://api.voyageai.com/v1",
			expectError: true,
			errorMsg:    "voyage API key is required",
		},
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewClient(tt.baseURL, tt.apiKey, httpClient)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, service)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, service)

				assert.Equal(t, tt.apiKey, service.apiKey)

				expectedbaseURL := tt.baseURL
				if expectedbaseURL == "" {
					expectedbaseURL = DefaultBaseURL
				}
				assert.Equal(t, expectedbaseURL, service.baseURL)

				// Verify that httpClient was set
				assert.NotNil(t, service.httpClient)
				assert.Equal(t, httpClient, service.httpClient)
			}
		})
	}
}

func TestServiceImpl_CreateEmbeddings(t *testing.T) {
	tests := []struct {
		name           string
		request        EmbeddingRequest
		mockResponse   EmbeddingResponse
		mockStatusCode int
		mockError      string
		expectError    bool
		errorMsg       string
	}{
		{
			name: "Successful embedding creation",
			request: EmbeddingRequest{
				Input:     []string{"Hello world", "Test text"},
				Model:     "voyage-3.5",
				InputType: "document",
			},
			mockResponse: EmbeddingResponse{
				Object: "list",
				Data: []struct {
					Object    string    `json:"object"`
					Index     int       `json:"index"`
					Embedding []float64 `json:"embedding"`
				}{
					{
						Object:    "embedding",
						Index:     0,
						Embedding: []float64{0.1, 0.2, 0.3},
					},
					{
						Object:    "embedding",
						Index:     1,
						Embedding: []float64{0.4, 0.5, 0.6},
					},
				},
				Model: "voyage-3.5",
				Usage: struct {
					TotalTokens int `json:"total_tokens"`
				}{
					TotalTokens: 5,
				},
			},
			mockStatusCode: 200,
			expectError:    false,
		},
		{
			name: "Request with default model",
			request: EmbeddingRequest{
				Input: []string{"Test text"},
				// Model is empty, should use default
			},
			mockResponse: EmbeddingResponse{
				Object: "list",
				Data: []struct {
					Object    string    `json:"object"`
					Index     int       `json:"index"`
					Embedding []float64 `json:"embedding"`
				}{
					{
						Object:    "embedding",
						Index:     0,
						Embedding: []float64{0.1, 0.2, 0.3},
					},
				},
				Model: DefaultModel,
			},
			mockStatusCode: 200,
			expectError:    false,
		},
		{
			name: "API error response",
			request: EmbeddingRequest{
				Input: []string{"Test text"},
			},
			mockStatusCode: 400,
			mockError: `{
				"error": {
					"message": "Invalid input",
					"type": "invalid_request_error",
					"code": "invalid_input"
				}
			}`,
			expectError: true,
			errorMsg:    "Invalid input",
		},
		{
			name: "Server error without proper error format",
			request: EmbeddingRequest{
				Input: []string{"Test text"},
			},
			mockStatusCode: 500,
			mockError:      "Internal Server Error",
			expectError:    true,
			errorMsg:       "failed with status 500",
		},
		{
			name: "Invalid JSON response",
			request: EmbeddingRequest{
				Input: []string{"Test text"},
			},
			mockStatusCode: 200,
			mockError:      "invalid json response",
			expectError:    true,
			errorMsg:       "failed to unmarshal embedding response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify the request method and path
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/embeddings", r.URL.Path)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

				// Verify request body contains expected model
				var requestBody EmbeddingRequest
				err := json.NewDecoder(r.Body).Decode(&requestBody)
				assert.NoError(t, err)

				expectedModel := tt.request.Model
				if expectedModel == "" {
					expectedModel = DefaultModel
				}
				assert.Equal(t, expectedModel, requestBody.Model)
				assert.Equal(t, tt.request.Input, requestBody.Input)

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
			testHTTPClient := &http.Client{Timeout: 5 * time.Second}
			service, err := NewClient(
				server.URL,
				"test-api-key",
				testHTTPClient,
			)
			require.NoError(t, err)

			// Call CreateEmbeddings method
			ctx := context.Background()
			result, err := service.CreateEmbeddings(ctx, tt.request)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.mockResponse.Object, result.Object)
				assert.Equal(t, tt.mockResponse.Model, result.Model)
				assert.Equal(t, len(tt.mockResponse.Data), len(result.Data))
				if len(result.Data) > 0 {
					assert.Equal(t, tt.mockResponse.Data[0].Object, result.Data[0].Object)
					assert.Equal(t, tt.mockResponse.Data[0].Index, result.Data[0].Index)
					assert.Equal(t, tt.mockResponse.Data[0].Embedding, result.Data[0].Embedding)
				}
			}
		})
	}
}

func TestServiceImpl_CreateEmbeddingsWithContext(t *testing.T) {
	// Test context cancellation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow response
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(200)
		response := EmbeddingResponse{Object: "list"}
		responseBytes, _ := json.Marshal(response)
		w.Write(responseBytes)
	}))
	defer server.Close()

	testHTTPClient := &http.Client{Timeout: 5 * time.Second}
	service, err := NewClient(
		server.URL,
		"test-api-key",
		testHTTPClient,
	)
	require.NoError(t, err)

	// Create a context that will be canceled
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	request := EmbeddingRequest{
		Input: []string{"Test text"},
	}

	_, err = service.CreateEmbeddings(ctx, request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestEmbeddingRequestSerialization(t *testing.T) {
	// Test various embedding request fields can be set and serialized
	outputDim := 512
	req := EmbeddingRequest{
		Input:           []string{"Hello world", "Test document"},
		Model:           "voyage-3.5",
		InputType:       "document",
		OutputDimension: &outputDim,
	}

	// Test marshaling works
	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test unmarshaling works
	var unmarshaledReq EmbeddingRequest
	err = json.Unmarshal(data, &unmarshaledReq)
	require.NoError(t, err)
	assert.Equal(t, req.Input, unmarshaledReq.Input)
	assert.Equal(t, req.Model, unmarshaledReq.Model)
	assert.Equal(t, req.InputType, unmarshaledReq.InputType)
	assert.Equal(t, *req.OutputDimension, *unmarshaledReq.OutputDimension)
}

func TestConstants(t *testing.T) {
	// Test that constants are properly defined
	assert.Equal(t, "https://api.voyageai.com/v1", DefaultBaseURL)
	assert.Equal(t, "voyage-3.5", DefaultModel)
}
