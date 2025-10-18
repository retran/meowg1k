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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

func TestNewOpenRouterGateway(t *testing.T) {
	t.Run("Valid parameters", func(t *testing.T) {
		ctx := context.Background()
		gateway, err := NewOpenRouterGateway(ctx, "https://openrouter.ai/api/v1", "test-api-key", &http.Client{})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if gateway == nil {
			t.Fatal("Expected gateway to be non-nil")
		}
	})

	t.Run("Empty base URL", func(t *testing.T) {
		ctx := context.Background()
		gateway, err := NewOpenRouterGateway(ctx, "", "test-api-key", &http.Client{})
		if err == nil {
			t.Fatal("Expected error for empty base URL")
		}
		if gateway != nil {
			t.Fatal("Expected gateway to be nil when error occurs")
		}
		if !strings.Contains(err.Error(), "base URL is required") {
			t.Errorf("Expected 'base URL is required' in error, got: %v", err)
		}
	})

	t.Run("Empty API key", func(t *testing.T) {
		ctx := context.Background()
		gateway, err := NewOpenRouterGateway(ctx, "https://openrouter.ai/api/v1", "", &http.Client{})
		if err == nil {
			t.Fatal("Expected error for empty API key")
		}
		if gateway != nil {
			t.Fatal("Expected gateway to be nil when error occurs")
		}
		if !strings.Contains(err.Error(), "API key is required") {
			t.Errorf("Expected 'API key is required' in error, got: %v", err)
		}
	})

	t.Run("Nil HTTP client", func(t *testing.T) {
		ctx := context.Background()
		gateway, err := NewOpenRouterGateway(ctx, "https://openrouter.ai/api/v1", "test-api-key", nil)
		if err == nil {
			t.Fatal("Expected error for nil HTTP client")
		}
		if gateway != nil {
			t.Fatal("Expected gateway to be nil when error occurs")
		}
		if !strings.Contains(err.Error(), "HTTP client is required") {
			t.Errorf("Expected 'HTTP client is required' in error, got: %v", err)
		}
	})
}

func TestOpenRouterGateway_InterfaceCompliance(t *testing.T) {
	ctx := context.Background()
	gateway, err := NewOpenRouterGateway(ctx, "https://openrouter.ai/api/v1", "test-api-key", &http.Client{})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	//nolint:staticcheck // explicit interface check for documentation
	var _ ports.GenerationGateway = gateway
	t.Log("OpenRouterGateway correctly implements GenerationGateway interface")
}

func TestOpenRouterGateway_GenerateContent(t *testing.T) {
	t.Run("Successful generation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request method and path
			if r.Method != "POST" {
				t.Errorf("Expected POST request, got %s", r.Method)
			}
			if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
				t.Errorf("Expected path to end with /chat/completions, got %s", r.URL.Path)
			}

			// Verify headers
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
			}
			if !strings.Contains(r.Header.Get("Authorization"), "Bearer test-api-key") {
				t.Errorf("Expected Authorization header with Bearer token")
			}
			if r.Header.Get("HTTP-Referer") == "" {
				t.Error("Expected HTTP-Referer header to be set")
			}
			if r.Header.Get("X-Title") == "" {
				t.Error("Expected X-Title header to be set")
			}

			// Parse request body
			body, _ := io.ReadAll(r.Body)
			var reqBody openrouterRequest
			if err := json.Unmarshal(body, &reqBody); err != nil {
				t.Errorf("Failed to parse request body: %v", err)
			}

			// Verify request structure
			if reqBody.Model == "" {
				t.Error("Expected model to be set")
			}
			if len(reqBody.Messages) == 0 {
				t.Error("Expected messages to be set")
			}

			// Send successful response
			response := openrouterResponse{
				Choices: []openrouterChoice{
					{
						Message: struct {
							Content string `json:"content"`
						}{
							Content: "Generated response",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		ctx := context.Background()
		gateway, err := NewOpenRouterGateway(ctx, server.URL, "test-api-key", &http.Client{})
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		request := domainGateway.NewGenerateContentRequest(
			"openai/gpt-4",
			"You are a helpful assistant",
			"Hello, how are you?",
			1000,
		)

		result, err := gateway.GenerateContent(ctx, request)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if result != "Generated response" {
			t.Errorf("Expected 'Generated response', got '%s'", result)
		}
	})

	t.Run("Nil context", func(t *testing.T) {
		ctx := context.Background()
		gateway, err := NewOpenRouterGateway(ctx, "https://openrouter.ai/api/v1", "test-api-key", &http.Client{})
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		request := domainGateway.NewGenerateContentRequest("openai/gpt-4", "System", "User", 1000)
		//nolint:staticcheck // intentionally testing nil context handling
		_, err = gateway.GenerateContent(nil, request)
		if err == nil {
			t.Fatal("Expected error for nil context")
		}
		if !strings.Contains(err.Error(), "context cannot be nil") {
			t.Errorf("Expected 'context cannot be nil' error, got: %v", err)
		}
	})

	t.Run("Nil request", func(t *testing.T) {
		ctx := context.Background()
		gateway, err := NewOpenRouterGateway(ctx, "https://openrouter.ai/api/v1", "test-api-key", &http.Client{})
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		_, err = gateway.GenerateContent(ctx, nil)
		if err == nil {
			t.Fatal("Expected error for nil request")
		}
		if !strings.Contains(err.Error(), "request cannot be nil") {
			t.Errorf("Expected 'request cannot be nil' error, got: %v", err)
		}
	})

	t.Run("API error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := openrouterResponse{
				Error: &struct {
					Message string `json:"message"`
					Type    string `json:"type"`
					Code    string `json:"code"`
				}{
					Message: "Invalid API key",
					Type:    "authentication_error",
					Code:    "401",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		ctx := context.Background()
		gateway, err := NewOpenRouterGateway(ctx, server.URL, "test-api-key", &http.Client{})
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		request := domainGateway.NewGenerateContentRequest("openai/gpt-4", "System", "User", 1000)
		_, err = gateway.GenerateContent(ctx, request)
		if err == nil {
			t.Fatal("Expected error for API error response")
		}
		if !strings.Contains(err.Error(), "Invalid API key") {
			t.Errorf("Expected API error message in error, got: %v", err)
		}
	})

	t.Run("Non-200 status code", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Bad request"))
		}))
		defer server.Close()

		ctx := context.Background()
		gateway, err := NewOpenRouterGateway(ctx, server.URL, "test-api-key", &http.Client{})
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		request := domainGateway.NewGenerateContentRequest("openai/gpt-4", "System", "User", 1000)
		_, err = gateway.GenerateContent(ctx, request)
		if err == nil {
			t.Fatal("Expected error for non-200 status")
		}
		if !strings.Contains(err.Error(), "status 400") {
			t.Errorf("Expected status 400 in error, got: %v", err)
		}
	})

	t.Run("Empty choices array", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := openrouterResponse{
				Choices: []openrouterChoice{},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		ctx := context.Background()
		gateway, err := NewOpenRouterGateway(ctx, server.URL, "test-api-key", &http.Client{})
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		request := domainGateway.NewGenerateContentRequest("openai/gpt-4", "System", "User", 1000)
		_, err = gateway.GenerateContent(ctx, request)
		if err == nil {
			t.Fatal("Expected error for empty choices")
		}
		if !strings.Contains(err.Error(), "no choices returned") {
			t.Errorf("Expected 'no choices returned' error, got: %v", err)
		}
	})

	t.Run("Invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		ctx := context.Background()
		gateway, err := NewOpenRouterGateway(ctx, server.URL, "test-api-key", &http.Client{})
		if err != nil {
			t.Fatalf("Failed to create gateway: %v", err)
		}

		request := domainGateway.NewGenerateContentRequest("openai/gpt-4", "System", "User", 1000)
		_, err = gateway.GenerateContent(ctx, request)
		if err == nil {
			t.Fatal("Expected error for invalid JSON")
		}
		if !strings.Contains(err.Error(), "failed to parse response") {
			t.Errorf("Expected 'failed to parse response' error, got: %v", err)
		}
	})
}

func TestOpenRouterGateway_WithAllParameters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var reqBody openrouterRequest
		if err := json.Unmarshal(body, &reqBody); err != nil {
			t.Errorf("Failed to parse request body: %v", err)
		}

		// Verify all parameters are set
		if reqBody.Temperature == nil {
			t.Error("Expected temperature to be set")
		}
		if reqBody.TopP == nil {
			t.Error("Expected topP to be set")
		}
		if reqBody.TopK == nil {
			t.Error("Expected topK to be set")
		}
		if reqBody.FrequencyPenalty == nil {
			t.Error("Expected frequencyPenalty to be set")
		}
		if reqBody.PresencePenalty == nil {
			t.Error("Expected presencePenalty to be set")
		}
		if reqBody.RepetitionPenalty == nil {
			t.Error("Expected repetitionPenalty to be set")
		}
		if reqBody.MinP == nil {
			t.Error("Expected minP to be set")
		}
		if reqBody.TopA == nil {
			t.Error("Expected topA to be set")
		}
		if reqBody.Seed == nil {
			t.Error("Expected seed to be set")
		}
		if len(reqBody.Stop) == 0 {
			t.Error("Expected stop sequences to be set")
		}
		if reqBody.N == nil {
			t.Error("Expected n (candidate count) to be set")
		}

		response := openrouterResponse{
			Choices: []openrouterChoice{
				{
					Message: struct {
						Content string `json:"content"`
					}{
						Content: "Response with all params",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctx := context.Background()
	gateway, err := NewOpenRouterGateway(ctx, server.URL, "test-api-key", &http.Client{})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	temp := 0.7
	topP := 0.9
	topK := 40
	fp := 0.5
	pp := 0.6
	rp := 1.1
	minP := 0.05
	topA := 0.2
	seed := 42
	n := 3

	request := domainGateway.NewGenerateContentRequest(
		"openai/gpt-4",
		"System prompt",
		"User prompt",
		1000,
	).WithTemperature(&temp).
		WithTopP(&topP).
		WithTopK(&topK).
		WithFrequencyPenalty(&fp).
		WithPresencePenalty(&pp).
		WithRepetitionPenalty(&rp).
		WithMinP(&minP).
		WithTopA(&topA).
		WithSeed(&seed).
		WithStop([]string{"END", "STOP"}).
		WithCandidateCount(&n)

	result, err := gateway.GenerateContent(ctx, request)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result != "Response with all params" {
		t.Errorf("Expected 'Response with all params', got '%s'", result)
	}
}

func TestOpenRouterGateway_WithSystemPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var reqBody openrouterRequest
		json.Unmarshal(body, &reqBody)

		// Verify system message is included
		if len(reqBody.Messages) < 2 {
			t.Error("Expected at least 2 messages (system + user)")
		}
		if reqBody.Messages[0].Role != "system" {
			t.Errorf("Expected first message to be system, got %s", reqBody.Messages[0].Role)
		}
		if reqBody.Messages[1].Role != "user" {
			t.Errorf("Expected second message to be user, got %s", reqBody.Messages[1].Role)
		}

		response := openrouterResponse{
			Choices: []openrouterChoice{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "Response"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctx := context.Background()
	gateway, _ := NewOpenRouterGateway(ctx, server.URL, "test-api-key", &http.Client{})
	request := domainGateway.NewGenerateContentRequest("openai/gpt-4", "System prompt", "User prompt", 1000)
	gateway.GenerateContent(ctx, request)
}

func TestOpenRouterGateway_WithoutSystemPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var reqBody openrouterRequest
		json.Unmarshal(body, &reqBody)

		// Verify only user message is included
		if len(reqBody.Messages) != 1 {
			t.Errorf("Expected 1 message (user only), got %d", len(reqBody.Messages))
		}
		if reqBody.Messages[0].Role != "user" {
			t.Errorf("Expected message to be user, got %s", reqBody.Messages[0].Role)
		}

		response := openrouterResponse{
			Choices: []openrouterChoice{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "Response"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctx := context.Background()
	gateway, _ := NewOpenRouterGateway(ctx, server.URL, "test-api-key", &http.Client{})
	request := domainGateway.NewGenerateContentRequest("openai/gpt-4", "", "User prompt", 1000)
	gateway.GenerateContent(ctx, request)
}
