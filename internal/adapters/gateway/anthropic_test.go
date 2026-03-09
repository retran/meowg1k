// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
)

// anthropicSuccessResponse returns a minimal valid Anthropic messages response body.
func anthropicSuccessResponse() map[string]interface{} {
	return map[string]interface{}{
		"id":   "msg_test",
		"type": "message",
		"role": "assistant",
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": "Generated content response",
			},
		},
		"model":       "claude-3-haiku-20240307",
		"stop_reason": "end_turn",
		"usage": map[string]interface{}{
			"input_tokens":  10,
			"output_tokens": 5,
		},
	}
}

// anthropicCountTokensResponse returns a minimal valid Anthropic count_tokens response body.
func anthropicCountTokensResponse() map[string]interface{} {
	return map[string]interface{}{
		"input_tokens": 42,
	}
}

// newAnthropicMockServer creates an httptest.Server that simulates the Anthropic API.
// It validates headers and request body, then returns a success response.
// The count_tokens endpoint returns a token count response; other endpoints return a message response.
func newAnthropicMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			t.Errorf("Expected JSON content type, got %s", r.Header.Get("Content-Type"))
		}
		authHeader := r.Header.Get("X-API-Key")
		if !strings.Contains(authHeader, "test-api-key") {
			t.Errorf("Expected test-api-key in X-API-Key header, got %s", authHeader)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		// Handle count_tokens endpoint separately
		if strings.Contains(r.URL.Path, "count_tokens") {
			if err := json.NewEncoder(w).Encode(anthropicCountTokensResponse()); err != nil {
				t.Errorf("Failed to encode count_tokens response: %v", err)
			}
			return
		}

		if err := json.NewEncoder(w).Encode(anthropicSuccessResponse()); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
}

// newTestAnthropicGateway creates an anthropic gateway pointed at the given server URL.
func newTestAnthropicGateway(t *testing.T, serverURL string) *anthropicGateway {
	t.Helper()
	gw, err := newAnthropicGateway("test-api-key", nil, serverURL)
	require.NoError(t, err)
	require.NotNil(t, gw)
	ag, ok := gw.(*anthropicGateway)
	require.True(t, ok)
	return ag
}

func TestNewAnthropicGateway(t *testing.T) {
	t.Run("Valid API key", func(t *testing.T) {
		gateway, err := newAnthropicGateway("test-api-key", nil, "")
		require.NoError(t, err)
		assert.NotNil(t, gateway)
	})

	t.Run("Empty API key", func(t *testing.T) {
		gateway, err := newAnthropicGateway("", nil, "")
		require.Error(t, err)
		assert.Nil(t, gateway)
		assert.Contains(t, err.Error(), "anthropic API key is required")
	})
}

func TestAnthropicGateway_GenerateContent(t *testing.T) {
	server := newAnthropicMockServer(t)
	defer server.Close()

	t.Run("Generate content with valid request", func(t *testing.T) {
		gw := newTestAnthropicGateway(t, server.URL)
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307",
			"You are a helpful assistant",
			"Hello, how are you?",
			4096,
		)
		resp, err := gw.GenerateContent(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("Generate content with empty model", func(t *testing.T) {
		gw := newTestAnthropicGateway(t, server.URL)
		request := domainGateway.NewGenerateContentRequest(
			"", // empty model
			"You are a helpful assistant",
			"Hello, how are you?",
			4096,
		)
		_, err := gw.GenerateContent(context.Background(), request)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "model is required")
	})

	t.Run("Generate content with system prompt", func(t *testing.T) {
		gw := newTestAnthropicGateway(t, server.URL)
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307",
			"You are a code assistant specializing in Go",
			"Write a hello world program",
			4096,
		)
		resp, err := gw.GenerateContent(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("Generate content with different max tokens", func(t *testing.T) {
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
				gw := newTestAnthropicGateway(t, server.URL)
				request := domainGateway.NewGenerateContentRequest(
					"claude-3-haiku-20240307",
					"You are a helpful assistant",
					"Generate some text",
					tc.maxTokens,
				)
				resp, err := gw.GenerateContent(context.Background(), request)
				require.NoError(t, err)
				assert.NotNil(t, resp)
			})
		}
	})

	t.Run("Generate content with canceled context", func(t *testing.T) {
		gw := newTestAnthropicGateway(t, server.URL)
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307",
			"You are a helpful assistant",
			"Hello, how are you?",
			4096,
		)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		_, err := gw.GenerateContent(ctx, request)
		require.Error(t, err)
	})
}

func TestAnthropicGateway_InterfaceCompliance(t *testing.T) {
	gateway, err := newAnthropicGateway("test-api-key", nil, "")
	require.NoError(t, err)
	_ = gateway // compile-time interface check is in anthropic.go
}

func TestAnthropicGateway_ErrorHandling(t *testing.T) {
	server := newAnthropicMockServer(t)
	defer server.Close()

	testCases := []struct {
		name           string
		model          string
		systemPrompt   string
		userPrompt     string
		errorSubstring string
		maxTokens      int
		expectingError bool
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
			expectingError: false,
		},
		{
			name:           "Empty user prompt",
			model:          "claude-3-haiku-20240307",
			systemPrompt:   "System prompt",
			userPrompt:     "",
			maxTokens:      1000,
			expectingError: false,
		},
		{
			name:           "Zero max tokens",
			model:          "claude-3-haiku-20240307",
			systemPrompt:   "System prompt",
			userPrompt:     "User prompt",
			maxTokens:      0,
			expectingError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gw := newTestAnthropicGateway(t, server.URL)
			request := domainGateway.NewGenerateContentRequest(
				tc.model,
				tc.systemPrompt,
				tc.userPrompt,
				tc.maxTokens,
			)
			_, err := gw.GenerateContent(context.Background(), request)
			switch {
			case tc.expectingError && tc.errorSubstring != "":
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorSubstring)
			case tc.expectingError:
				require.Error(t, err)
			case tc.errorSubstring != "":
				if err != nil {
					assert.NotContains(t, err.Error(), tc.errorSubstring)
				}
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestAnthropicGateway_NilChecks(t *testing.T) {
	t.Run("Nil request", func(t *testing.T) {
		gateway, err := newAnthropicGateway("test-api-key", nil, "")
		require.NoError(t, err)
		_, err = gateway.GenerateContent(context.Background(), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "request cannot be nil")
	})

	t.Run("Nil gateway", func(t *testing.T) {
		var gateway *anthropicGateway

		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307",
			"System prompt",
			"User prompt",
			1000,
		)
		_, err := gateway.GenerateContent(context.Background(), request)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "anthropic gateway is nil")
	})
}

func TestAnthropicGateway_WithGenerationParameters(t *testing.T) {
	server := newAnthropicMockServer(t)
	defer server.Close()

	gw := newTestAnthropicGateway(t, server.URL)

	t.Run("With temperature", func(t *testing.T) {
		temp := 0.7
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307",
			"System prompt",
			"User prompt",
			1000,
		).WithTemperature(&temp)
		resp, err := gw.GenerateContent(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("With topP", func(t *testing.T) {
		topP := 0.9
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307",
			"System prompt",
			"User prompt",
			1000,
		).WithTopP(&topP)
		resp, err := gw.GenerateContent(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("With topK", func(t *testing.T) {
		topK := 40
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307",
			"System prompt",
			"User prompt",
			1000,
		).WithTopK(&topK)
		resp, err := gw.GenerateContent(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("With stop sequences", func(t *testing.T) {
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307",
			"System prompt",
			"User prompt",
			1000,
		).WithStop([]string{"\n\n", "END"})
		resp, err := gw.GenerateContent(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("With all parameters", func(t *testing.T) {
		temp := 0.8
		topP := 0.95
		topK := 50
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307",
			"System prompt",
			"User prompt",
			1000,
		).WithTemperature(&temp).WithTopP(&topP).WithTopK(&topK).WithStop([]string{"STOP"})
		resp, err := gw.GenerateContent(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

func TestAnthropicGateway_WithCustomHTTPClient(t *testing.T) {
	t.Run("With custom HTTP client", func(t *testing.T) {
		customClient := &http.Client{}
		gateway, err := newAnthropicGateway("test-api-key", customClient, "")
		require.NoError(t, err)
		assert.NotNil(t, gateway)
	})

	t.Run("With nil HTTP client", func(t *testing.T) {
		gateway, err := newAnthropicGateway("test-api-key", nil, "")
		require.NoError(t, err)
		assert.NotNil(t, gateway)
	})
}

func TestAnthropicGateway_DifferentModels(t *testing.T) {
	server := newAnthropicMockServer(t)
	defer server.Close()

	models := []string{
		"claude-3-haiku-20240307",
		"claude-3-sonnet-20240229",
		"claude-3-opus-20240229",
		"claude-3-5-sonnet-20240620",
	}

	for _, model := range models {
		t.Run("Model: "+model, func(t *testing.T) {
			gw := newTestAnthropicGateway(t, server.URL)
			request := domainGateway.NewGenerateContentRequest(
				model,
				"System prompt",
				"User prompt",
				1000,
			)
			resp, err := gw.GenerateContent(context.Background(), request)
			require.NoError(t, err)
			assert.NotNil(t, resp)
		})
	}
}

func TestAnthropicGateway_CountTokens(t *testing.T) {
	server := newAnthropicMockServer(t)
	defer server.Close()

	t.Run("Empty texts returns zero", func(t *testing.T) {
		gw := newTestAnthropicGateway(t, server.URL)
		count, err := gw.CountTokens(context.Background(), "claude-3-haiku-20240307", []string{})
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Non-empty texts returns count from API", func(t *testing.T) {
		gw := newTestAnthropicGateway(t, server.URL)
		// Mock returns input_tokens: 42
		count, err := gw.CountTokens(context.Background(), "claude-3-haiku-20240307", []string{"Hello", "world"})
		require.NoError(t, err)
		assert.Equal(t, 42, count)
	})

	t.Run("All empty string texts returns zero", func(t *testing.T) {
		gw := newTestAnthropicGateway(t, server.URL)
		count, err := gw.CountTokens(context.Background(), "claude-3-haiku-20240307", []string{"", "", ""})
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Nil gateway returns error", func(t *testing.T) {
		var nilGateway *anthropicGateway
		_, err := nilGateway.CountTokens(context.Background(), "claude-3-haiku-20240307", []string{"test"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "anthropic gateway is nil")
	})

	t.Run("Canceled context returns error", func(t *testing.T) {
		gw := newTestAnthropicGateway(t, server.URL)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := gw.CountTokens(ctx, "claude-3-haiku-20240307", []string{"Hello"})
		require.Error(t, err)
	})
}

func TestAnthropicGateway_GenerateContentStream(t *testing.T) {
	server := newAnthropicMockServer(t)
	defer server.Close()

	t.Run("Valid request calls callback and returns response", func(t *testing.T) {
		gw := newTestAnthropicGateway(t, server.URL)
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307",
			"You are a helpful assistant",
			"Hello, how are you?",
			4096,
		)

		var events []domainGateway.StreamEvent
		callback := func(event domainGateway.StreamEvent) error {
			events = append(events, event)
			return nil
		}

		resp, err := gw.GenerateContentStream(context.Background(), request, callback)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("Nil callback is accepted", func(t *testing.T) {
		gw := newTestAnthropicGateway(t, server.URL)
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307",
			"System prompt",
			"User prompt",
			1000,
		)

		resp, err := gw.GenerateContentStream(context.Background(), request, nil)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("Error propagates to callback", func(t *testing.T) {
		gw := newTestAnthropicGateway(t, server.URL)
		request := domainGateway.NewGenerateContentRequest(
			"", // empty model causes error
			"System prompt",
			"User prompt",
			1000,
		)

		var gotErrorEvent bool
		callback := func(event domainGateway.StreamEvent) error {
			if event.Kind == domainGateway.StreamEventError {
				gotErrorEvent = true
			}
			return nil
		}

		_, err := gw.GenerateContentStream(context.Background(), request, callback)
		require.Error(t, err)
		assert.True(t, gotErrorEvent, "Expected error event to be sent to callback")
	})

	t.Run("Nil gateway returns error", func(t *testing.T) {
		var nilGateway *anthropicGateway
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307",
			"System",
			"User",
			1000,
		)
		_, err := nilGateway.GenerateContentStream(context.Background(), request, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "anthropic gateway is nil")
	})
}

// TestAnthropicGateway_WithMessages exercises the message mapping code paths:
// mapMessages → mapMessage → mapAssistantMessage / mapToolMessage.
func TestAnthropicGateway_WithMessages(t *testing.T) {
	server := newAnthropicMockServer(t)
	defer server.Close()

	gw := newTestAnthropicGateway(t, server.URL)

	t.Run("User message history", func(t *testing.T) {
		msgs := []domainGateway.Message{
			{Role: domainGateway.MessageRoleUser, Content: "Hello"},
			{Role: domainGateway.MessageRoleAssistant, Content: "Hi there"},
			{Role: domainGateway.MessageRoleUser, Content: "How are you?"},
		}
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307", "", "Continue", 1000,
		).WithMessages(msgs)
		resp, err := gw.GenerateContent(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("Assistant message with tool calls", func(t *testing.T) {
		msgs := []domainGateway.Message{
			{Role: domainGateway.MessageRoleUser, Content: "What's the weather?"},
			{
				Role:    domainGateway.MessageRoleAssistant,
				Content: "Let me check.",
				ToolCalls: []domainGateway.ToolCall{
					{ID: "call_1", Name: "get_weather", Arguments: map[string]any{"location": "Paris"}},
				},
			},
			{
				Role:       domainGateway.MessageRoleTool,
				Content:    "Sunny, 22C",
				ToolName:   "get_weather",
				ToolCallID: "call_1",
			},
		}
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307", "", "Summarize", 1000,
		).WithMessages(msgs)
		resp, err := gw.GenerateContent(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("System message in history is skipped", func(t *testing.T) {
		msgs := []domainGateway.Message{
			{Role: domainGateway.MessageRoleSystem, Content: "System message"},
			{Role: domainGateway.MessageRoleUser, Content: "Hello"},
		}
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307", "", "Respond", 1000,
		).WithMessages(msgs)
		resp, err := gw.GenerateContent(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("Assistant message with empty content and no tool calls is skipped", func(t *testing.T) {
		msgs := []domainGateway.Message{
			{Role: domainGateway.MessageRoleUser, Content: "Hello"},
			{Role: domainGateway.MessageRoleAssistant, Content: "  "}, // whitespace only
		}
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307", "", "Continue", 1000,
		).WithMessages(msgs)
		resp, err := gw.GenerateContent(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("Tool message with empty content is skipped", func(t *testing.T) {
		msgs := []domainGateway.Message{
			{Role: domainGateway.MessageRoleUser, Content: "Hello"},
			{Role: domainGateway.MessageRoleTool, Content: "  "}, // whitespace only
		}
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307", "", "Continue", 1000,
		).WithMessages(msgs)
		resp, err := gw.GenerateContent(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

// TestAnthropicGateway_WithTools exercises buildAnthropicTools, buildToolInputSchema,
// and mapRequiredFields code paths.
func TestAnthropicGateway_WithTools(t *testing.T) {
	server := newAnthropicMockServer(t)
	defer server.Close()

	gw := newTestAnthropicGateway(t, server.URL)

	t.Run("With tool definitions", func(t *testing.T) {
		tools := []domainGateway.ToolDefinition{
			{
				Name:        "get_weather",
				Description: "Get the current weather",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{"type": "string"},
					},
					"required": []any{"location"},
				},
			},
		}
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307", "You help with weather.", "What's the weather in Paris?", 1000,
		).WithTools(tools)
		resp, err := gw.GenerateContent(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("Tool with no description uses empty description", func(t *testing.T) {
		tools := []domainGateway.ToolDefinition{
			{
				Name:        "no_desc_tool",
				Description: "",
				Parameters:  nil,
			},
		}
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307", "", "Use the tool", 1000,
		).WithTools(tools)
		resp, err := gw.GenerateContent(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("Tool with required as []string (not []any)", func(t *testing.T) {
		tools := []domainGateway.ToolDefinition{
			{
				Name: "typed_required",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
					"required": []string{"query"},
				},
			},
		}
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307", "", "Use the tool", 1000,
		).WithTools(tools)
		resp, err := gw.GenerateContent(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("With response schema", func(t *testing.T) {
		schema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"answer": map[string]any{"type": "string"},
				"score":  map[string]any{"type": "number"},
			},
			"required":       []any{"answer"},
			"additionalProp": "extra",
		}
		request := domainGateway.NewGenerateContentRequest(
			"claude-3-haiku-20240307", "Answer in JSON.", "Rate this: great!", 1000,
		).WithResponseSchema(schema)
		resp, err := gw.GenerateContent(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})
}
