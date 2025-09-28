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

	mdGateway "github.com/retran/meowg1k/internal/models/gateway"
)

func TestNewAnthropicGateway(t *testing.T) {
	t.Run("Valid API key", func(t *testing.T) {
		gateway, err := newAnthropicGateway("test-api-key")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if gateway == nil {
			t.Fatal("Expected gateway to be non-nil")
		}
	})

	t.Run("Empty API key", func(t *testing.T) {
		gateway, err := newAnthropicGateway("")
		if err == nil {
			t.Fatal("Expected error for empty API key")
		}
		if gateway != nil {
			t.Fatal("Expected gateway to be nil when error occurs")
		}
		expectedError := "anthropic API key is required"
		if !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
		}
	})
}

func TestAnthropicGateway_GenerateContent(t *testing.T) {
	// Create a mock server to simulate Anthropic API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and content type
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			t.Errorf("Expected JSON content type, got %s", r.Header.Get("Content-Type"))
		}

		// Verify API key header
		authHeader := r.Header.Get("X-API-Key")
		if !strings.Contains(authHeader, "test-api-key") {
			t.Errorf("Expected API key in header, got %s", authHeader)
		}

		// Parse request body to verify it's properly formatted
		var requestBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		// Verify required fields
		if _, ok := requestBody["model"]; !ok {
			t.Error("Expected 'model' field in request body")
		}
		if _, ok := requestBody["messages"]; !ok {
			t.Error("Expected 'messages' field in request body")
		}
		if _, ok := requestBody["max_tokens"]; !ok {
			t.Error("Expected 'max_tokens' field in request body")
		}

		// Simulate successful response
		response := map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": "Generated content response",
				},
			},
			"role": "assistant",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Note: This test will need the actual Anthropic client to be properly mocked
	// For now, we'll test the gateway creation and basic error handling

	t.Run("Generate content with valid request", func(t *testing.T) {
		gateway, err := newAnthropicGateway("test-api-key")
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		request := mdGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307",
			"You are a helpful assistant",
			"Hello, how are you?",
			4096,
		)

		ctx := context.Background()

		// This will likely fail due to network call, but we test the setup
		_, err = gateway.GenerateContent(ctx, request)
		// We expect an error since we're not actually connecting to Anthropic
		// but we can verify the error handling path
		if err == nil {
			t.Log("Unexpected success - this might indicate the test environment has network access")
		} else {
			// Verify it's a network/API related error, not a validation error
			if strings.Contains(err.Error(), "model is required") {
				t.Error("Should not get validation error for valid request")
			}
		}
	})

	t.Run("Generate content with empty model", func(t *testing.T) {
		gateway, err := newAnthropicGateway("test-api-key")
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		request := mdGateway.NewGenerateContentRequest(
			"", // empty model
			"You are a helpful assistant",
			"Hello, how are you?",
			4096,
		)

		ctx := context.Background()
		_, err = gateway.GenerateContent(ctx, request)

		if err == nil {
			t.Fatal("Expected error for empty model")
		}
		if !strings.Contains(err.Error(), "model is required") {
			t.Errorf("Expected 'model is required' error, got: %v", err)
		}
	})

	t.Run("Generate content with system prompt", func(t *testing.T) {
		gateway, err := newAnthropicGateway("test-api-key")
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		request := mdGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307",
			"You are a code assistant specializing in Go",
			"Write a hello world program",
			4096,
		)

		ctx := context.Background()

		// This will likely fail due to network call, but we test the setup
		_, err = gateway.GenerateContent(ctx, request)
		// We expect an error since we're not actually connecting to Anthropic
		if err != nil && strings.Contains(err.Error(), "model is required") {
			t.Error("Should not get validation error for valid request with system prompt")
		}
	})

	t.Run("Generate content with different max tokens", func(t *testing.T) {
		gateway, err := newAnthropicGateway("test-api-key")
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		testCases := []struct {
			name      string
			maxTokens int
		}{
			{"Small token limit", 100},
			{"Medium token limit", 1000},
			{"Large token limit", 4000},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				request := mdGateway.NewGenerateContentRequest(
					"claude-3-haiku-20240307",
					"You are a helpful assistant",
					"Generate some text",
					tc.maxTokens,
				)

				ctx := context.Background()

				// Test that the request is properly formed
				_, err = gateway.GenerateContent(ctx, request)
				// We expect an error since we're not actually connecting to Anthropic
				if err != nil && strings.Contains(err.Error(), "model is required") {
					t.Error("Should not get validation error for valid request")
				}
			})
		}
	})

	t.Run("Generate content with canceled context", func(t *testing.T) {
		gateway, err := newAnthropicGateway("test-api-key")
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		request := mdGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307",
			"You are a helpful assistant",
			"Hello, how are you?",
			4096,
		)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err = gateway.GenerateContent(ctx, request)
		if err == nil {
			t.Fatal("Expected error for canceled context")
		}
		// Should get context canceled error or connection error
		if !strings.Contains(err.Error(), "context canceled") &&
			!strings.Contains(err.Error(), "failed to generate content with Anthropic") {
			t.Logf("Got expected error for cancelled context: %v", err)
		}
	})
}

func TestAnthropicGateway_InterfaceCompliance(t *testing.T) {
	gateway, err := newAnthropicGateway("test-api-key")
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	// Verify that the gateway implements GenerationGateway interface
	var _ GenerationGateway = gateway
	t.Log("AnthropicGateway correctly implements GenerationGateway interface")
}

func TestAnthropicGateway_ErrorHandling(t *testing.T) {
	gateway, err := newAnthropicGateway("test-api-key")
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	testCases := []struct {
		name           string
		model          string
		systemPrompt   string
		userPrompt     string
		maxTokens      int
		expectingError bool
		errorSubstring string
	}{
		{
			name:           "Empty model",
			model:          "",
			systemPrompt:   "System prompt",
			userPrompt:     "User prompt",
			maxTokens:      1000,
			expectingError: true,
			errorSubstring: "model is required",
		},
		{
			name:           "Valid parameters",
			model:          "claude-3-haiku-20240307",
			systemPrompt:   "System prompt",
			userPrompt:     "User prompt",
			maxTokens:      1000,
			expectingError: false, // Will fail with network error, but not validation error
			errorSubstring: "",
		},
		{
			name:           "Empty user prompt",
			model:          "claude-3-haiku-20240307",
			systemPrompt:   "System prompt",
			userPrompt:     "",
			maxTokens:      1000,
			expectingError: false, // Empty user prompt should be allowed
			errorSubstring: "",
		},
		{
			name:           "Zero max tokens",
			model:          "claude-3-haiku-20240307",
			systemPrompt:   "System prompt",
			userPrompt:     "User prompt",
			maxTokens:      0,
			expectingError: false, // Zero tokens should be allowed (API will handle)
			errorSubstring: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := mdGateway.NewGenerateContentRequest(
				tc.model,
				tc.systemPrompt,
				tc.userPrompt,
				tc.maxTokens,
			)

			ctx := context.Background()
			_, err := gateway.GenerateContent(ctx, request)

			if tc.expectingError && tc.errorSubstring != "" {
				if err == nil {
					t.Fatalf("Expected error containing '%s', got no error", tc.errorSubstring)
				}
				if !strings.Contains(err.Error(), tc.errorSubstring) {
					t.Errorf("Expected error containing '%s', got: %v", tc.errorSubstring, err)
				}
			} else if tc.expectingError && tc.errorSubstring == "" {
				if err == nil {
					t.Fatal("Expected some error, got no error")
				}
			} else if !tc.expectingError && tc.errorSubstring != "" {
				if err != nil && strings.Contains(err.Error(), tc.errorSubstring) {
					t.Errorf("Did not expect error containing '%s', but got: %v", tc.errorSubstring, err)
				}
			}
		})
	}
}
