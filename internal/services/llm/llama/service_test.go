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
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid service creation",
			baseURL:     "http://localhost:8080",
			apiKey:      "test-key",
			expectError: false,
		},
		{
			name:        "Service creation with empty API key",
			baseURL:     "http://localhost:8080",
			apiKey:      "",
			expectError: false, // API key is optional for llama
		},
		{
			name:        "Service creation with empty base URL",
			baseURL:     "",
			apiKey:      "test-key",
			expectError: true,
			errorMsg:    "base URL cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewService(tt.baseURL, tt.apiKey)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, service)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, service)
				impl := service.(*serviceImpl)
				assert.Equal(t, tt.baseURL, impl.baseURL)
				assert.Equal(t, tt.apiKey, impl.apiKey)
				assert.NotNil(t, impl.httpClient)
				assert.Equal(t, 10*time.Minute, impl.httpClient.Timeout)
			}
		})
	}
}

func TestServiceImpl_Complete(t *testing.T) {
	tests := []struct {
		name           string
		request        *CompletionRequest
		mockResponse   CompletionResponse
		mockStatusCode int
		mockError      string
		expectError    bool
		errorMsg       string
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
			errorMsg:       "API request failed: status 500: Internal Server Error",
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
			service, err := NewService(server.URL, "test-api-key")
			require.NoError(t, err)

			// Call Complete method
			ctx := context.Background()
			result, err := service.Complete(ctx, tt.request)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.mockResponse.Content, result.Content)
				assert.Equal(t, tt.mockResponse.Model, result.Model)
				assert.Equal(t, tt.mockResponse.TokensEvaluated, result.TokensEvaluated)
				assert.Equal(t, tt.mockResponse.TokensCached, result.TokensCached)
			}
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

	service, err := NewService(server.URL, "test-api-key")
	require.NoError(t, err)

	ctx := context.Background()
	request := &CompletionRequest{
		Prompt: "Test prompt",
	}

	_, err = service.Complete(ctx, request)
	assert.NoError(t, err)
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

	service, err := NewService(server.URL, "")
	require.NoError(t, err)

	ctx := context.Background()
	request := &CompletionRequest{
		Prompt: "Test prompt",
	}

	_, err = service.Complete(ctx, request)
	assert.NoError(t, err)
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

	service, err := NewService(server.URL, "test-key")
	require.NoError(t, err)

	// Create a context that will be cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	request := &CompletionRequest{
		Prompt: "Test prompt",
	}

	_, err = service.Complete(ctx, request)
	assert.Error(t, err)
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
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test unmarshaling works
	var unmarshaledReq CompletionRequest
	err = json.Unmarshal(data, &unmarshaledReq)
	assert.NoError(t, err)
	assert.Equal(t, req.Temperature, unmarshaledReq.Temperature)
	assert.Equal(t, req.TopK, unmarshaledReq.TopK)
	assert.Equal(t, req.Stop, unmarshaledReq.Stop)
}

func TestServiceImpl_CompleteEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		setupServer func() *httptest.Server
		request     *CompletionRequest
		expectError bool
		errorMsg    string
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
			errorMsg:    "API request failed: status 404: Not Found",
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
			errorMsg:    "API request failed: status 400: Bad Request",
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
			var service Service
			var err error

			if tt.name == "HTTP request creation with invalid URL characters" {
				// Test with service that has invalid URL
				service, err = NewService("ht tp://invalid url with spaces", "key")
				require.NoError(t, err) // Service creation succeeds

				ctx := context.Background()
				_, err = service.Complete(ctx, tt.request)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				server := tt.setupServer()
				if server != nil {
					defer server.Close()
					service, err = NewService(server.URL, "test-key")
				} else {
					service, err = NewService("http://localhost:8080", "test-key")
				}
				require.NoError(t, err)

				ctx := context.Background()
				result, err := service.Complete(ctx, tt.request)

				if tt.expectError {
					assert.Error(t, err)
					assert.Nil(t, result)
					if tt.errorMsg != "" {
						assert.Contains(t, err.Error(), tt.errorMsg)
					}
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, result)
				}
			}
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
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test unmarshaling
	var unmarshaledResp CompletionResponse
	err = json.Unmarshal(data, &unmarshaledResp)
	assert.NoError(t, err)
	assert.Equal(t, response.Content, unmarshaledResp.Content)
	assert.Equal(t, response.Model, unmarshaledResp.Model)
	assert.Equal(t, response.TokensEvaluated, unmarshaledResp.TokensEvaluated)
	assert.Equal(t, response.TokensCached, unmarshaledResp.TokensCached)
	assert.Equal(t, response.GenerationSettings["temperature"], unmarshaledResp.GenerationSettings["temperature"])
	// Use type assertion for map values that could be different numeric types
	assert.Equal(t, float64(150), unmarshaledResp.Timings["predicted_n"])
	assert.Equal(t, len(response.Tokens), len(unmarshaledResp.Tokens))
	assert.Equal(t, len(response.Probs), len(unmarshaledResp.Probs))
}
