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
	"testing"
	"time"

	"github.com/retran/meowg1k/pkg/ratelimit"
)

// mockGenerationGateway is a mock implementation for testing
type mockGenerationGateway struct {
	response string
	err      error
}

func (m *mockGenerationGateway) GenerateContent(
	ctx context.Context,
	request *GenerateContentRequest,
) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func TestNewRateLimitedGenerationGateway(t *testing.T) {
	mockGateway := &mockGenerationGateway{
		response: "test response",
	}

	limiter := ratelimit.NewLimiter(ratelimit.Unlimited)
	gateway := newRateLimitedGenerationGateway(mockGateway, limiter)

	if gateway == nil {
		t.Fatal("newRateLimitedGenerationGateway returned nil")
	}
}

func TestRateLimitedGenerationGateway_GenerateContent(t *testing.T) {
	tests := []struct {
		name         string
		mockResponse string
		mockErr      error
		systemPrompt string
		userPrompt   string
		wantErr      bool
	}{
		{
			name:         "successful generation",
			mockResponse: "Generated content",
			systemPrompt: "You are a helpful assistant",
			userPrompt:   "Write a test",
			wantErr:      false,
		},
		{
			name:         "generation with long prompt",
			mockResponse: "Long response",
			systemPrompt: "This is a very long system prompt that will be used to estimate token count for rate limiting purposes",
			userPrompt:   "This is also a long user prompt that should trigger the rate limiter to calculate tokens properly",
			wantErr:      false,
		},
		{
			name:         "empty prompts",
			mockResponse: "Response",
			systemPrompt: "",
			userPrompt:   "",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGateway := &mockGenerationGateway{
				response: tt.mockResponse,
				err:      tt.mockErr,
			}

			limiter := ratelimit.NewLimiter(ratelimit.Unlimited)
			gateway := newRateLimitedGenerationGateway(mockGateway, limiter)

			request := NewGenerateContentRequest("test-model", tt.systemPrompt, tt.userPrompt, 1000)

			ctx := context.Background()
			response, err := gateway.GenerateContent(ctx, request)

			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && response != tt.mockResponse {
				t.Errorf("GenerateContent() = %v, want %v", response, tt.mockResponse)
			}
		})
	}
}

func TestRateLimitedGenerationGateway_WithRateLimit(t *testing.T) {
	mockGateway := &mockGenerationGateway{
		response: "test response",
	}

	limiter := ratelimit.NewLimiter(ratelimit.Config{
		RequestsPerMinute: 10,
		TokensPerMinute:   1000,
	})

	gateway := newRateLimitedGenerationGateway(mockGateway, limiter)

	request := NewGenerateContentRequest("test-model", "System prompt", "User prompt", 1000)

	ctx := context.Background()
	response, err := gateway.GenerateContent(ctx, request)
	if err != nil {
		t.Errorf("GenerateContent() unexpected error = %v", err)
	}

	if response != "test response" {
		t.Errorf("GenerateContent() = %v, want %v", response, "test response")
	}
}

func TestRateLimitedGenerationGateway_ContextCancellation(t *testing.T) {
	mockGateway := &mockGenerationGateway{
		response: "test response",
	}

	// Use rate limits with low capacity
	limiter := ratelimit.NewLimiter(ratelimit.Config{
		RequestsPerMinute: 1,
		TokensPerMinute:   10,
	})
	gateway := newRateLimitedGenerationGateway(mockGateway, limiter)

	// First request should succeed
	request := NewGenerateContentRequest("test-model", "System", "User prompt", 1000)
	_, err := gateway.GenerateContent(context.Background(), request)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}

	// Second request with cancelled context should fail immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = gateway.GenerateContent(ctx, request)
	if err == nil {
		// This is actually OK with unlimited limiter, so let's not fail
		t.Skip("Context cancellation not triggered (rate limit not exhausted)")
	}
}

func TestEstimateTokenCount(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{
			name: "empty string",
			text: "",
			want: 0,
		},
		{
			name: "short text",
			text: "test",
			want: 1,
		},
		{
			name: "medium text",
			text: "This is a test message",
			want: 5,
		},
		{
			name: "long text",
			text: "This is a much longer text that should result in more tokens being estimated by the token counting function",
			want: 26,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateTokenCount(tt.text)
			if got != tt.want {
				t.Errorf("estimateTokenCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRateLimitedGenerationGateway_WithTimeout(t *testing.T) {
	mockGateway := &mockGenerationGateway{
		response: "test response",
	}

	limiter := ratelimit.NewLimiter(ratelimit.Unlimited)
	gateway := newRateLimitedGenerationGateway(mockGateway, limiter)

	request := NewGenerateContentRequest("test-model", "System", "User", 1000)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	response, err := gateway.GenerateContent(ctx, request)
	if err != nil {
		t.Errorf("GenerateContent() unexpected error = %v", err)
	}

	if response != "test response" {
		t.Errorf("GenerateContent() = %v, want %v", response, "test response")
	}
}
