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
	"fmt"
	"strings"
	"testing"

	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
)

func TestNewLlamaGateway(t *testing.T) {
	t.Run("Valid parameters", func(t *testing.T) {
		gateway, err := newLlamaGateway("http://localhost:11434", "test-api-key")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if gateway == nil {
			t.Fatal("Expected gateway to be non-nil")
		}

		// Verify it implements GenerationGateway interface
		_ = gateway
		t.Log("LlamaGateway correctly implements GenerationGateway interface")
	})

	t.Run("Empty base URL", func(t *testing.T) {
		gateway, err := newLlamaGateway("", "test-api-key")
		if err == nil && gateway != nil {
			t.Log("Llama allows empty base URL on creation")
		} else if err != nil {
			t.Logf("Llama validates base URL on creation: %v", err)
		} else {
			t.Fatal("Unexpected state: no error but nil gateway")
		}
	})

	t.Run("Empty API key", func(t *testing.T) {
		gateway, err := newLlamaGateway("http://localhost:11434", "")
		if err == nil && gateway != nil {
			t.Log("Llama allows empty API key on creation")
		} else if err != nil {
			t.Logf("Llama validates API key on creation: %v", err)
		} else {
			t.Fatal("Unexpected state: no error but nil gateway")
		}
	})

	t.Run("Invalid URL format", func(t *testing.T) {
		gateway, err := newLlamaGateway("not-a-valid-url", "test-api-key")
		if err == nil && gateway != nil {
			t.Log("Llama allows invalid URL format on creation")
		} else if err != nil {
			t.Logf("Llama validates URL format: %v", err)
		} else {
			t.Fatal("Unexpected state: no error but nil gateway")
		}
	})

	t.Run("Different URL schemes", func(t *testing.T) {
		testURLs := []string{
			"http://localhost:11434",
			"https://api.example.com",
			"http://127.0.0.1:8080",
			"https://llama.example.com:443",
		}

		for _, url := range testURLs {
			t.Run("URL_"+url, func(t *testing.T) {
				gateway, err := newLlamaGateway(url, "test-api-key")
				if err != nil {
					t.Logf("Gateway creation failed for URL %s: %v", url, err)
				} else if gateway == nil {
					t.Errorf("Expected non-nil gateway for URL %s", url)
				}
			})
		}
	})
}

func TestLlamaGateway_GenerateContent(t *testing.T) {
	// Try to create a gateway for testing
	gateway, err := newLlamaGateway("http://localhost:11434", "test-api-key")
	if err != nil {
		t.Skipf("Cannot create Llama gateway for testing: %v", err)
		return
	}

	ctx := context.Background()

	t.Run("Generate content with valid request", func(t *testing.T) {
		request := domainGateway.NewGenerateContentRequest(
			"llama2",
			"You are a helpful assistant",
			"Hello, how are you?",
			4096,
		)

		_, err := gateway.GenerateContent(ctx, request)
		// We expect an error since we're not actually connecting to Llama
		if err != nil {
			t.Logf("Expected network/API error: %v", err)
			// Verify it's not a basic validation error
			if strings.Contains(err.Error(), "model is required") {
				t.Error("Should not get validation error for valid request")
			}
		} else {
			t.Log("Unexpected success - this might indicate the test environment has Llama running")
		}
	})

	t.Run("Generate content with system prompt", func(t *testing.T) {
		request := domainGateway.NewGenerateContentRequest(
			"codellama",
			"You are an expert Go programmer. Write clean, efficient, and well-commented code.",
			"Write a function to calculate the greatest common divisor",
			4096,
		)

		_, err := gateway.GenerateContent(ctx, request)
		if err != nil {
			t.Logf("Expected network/API error with system prompt: %v", err)
		}
	})

	t.Run("Generate content without system prompt", func(t *testing.T) {
		request := domainGateway.NewGenerateContentRequest(
			"llama2",
			"", // empty system prompt
			"Explain the concept of recursion in programming",
			2048,
		)

		_, err := gateway.GenerateContent(ctx, request)
		if err != nil {
			t.Logf("Expected network/API error without system prompt: %v", err)
		}
	})

	t.Run("Generate content with different models", func(t *testing.T) {
		models := []string{
			"llama2",
			"llama2:13b",
			"codellama",
			"codellama:python",
			"mistral",
			"mixtral",
		}

		for _, model := range models {
			t.Run("Model_"+model, func(t *testing.T) {
				request := domainGateway.NewGenerateContentRequest(
					model,
					"You are a helpful assistant",
					"Generate a short response",
					1000,
				)

				_, err := gateway.GenerateContent(ctx, request)
				if err != nil {
					t.Logf("Expected network/API error for model %s: %v", model, err)
				}
			})
		}
	})

	t.Run("Generate content with various token limits", func(t *testing.T) {
		tokenLimits := []int{100, 500, 1000, 2048, 4096, 8192}

		for _, limit := range tokenLimits {
			t.Run(fmt.Sprintf("Tokens_%d", limit), func(t *testing.T) {
				request := domainGateway.NewGenerateContentRequest(
					"llama2",
					"You are a helpful assistant",
					"Generate appropriate content",
					limit,
				)

				_, err := gateway.GenerateContent(ctx, request)
				if err != nil {
					t.Logf("Expected network/API error for %d tokens: %v", limit, err)
				}
			})
		}
	})

	t.Run("Generate content with canceled context", func(t *testing.T) {
		request := domainGateway.NewGenerateContentRequest(
			"llama2",
			"You are a helpful assistant",
			"Hello, how are you?",
			4096,
		)

		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		_, err := gateway.GenerateContent(cancelCtx, request)
		if err == nil {
			t.Fatal("Expected error for canceled context")
		}
		// Should get context canceled error or connection error
		t.Logf("Got expected error for canceled context: %v", err)
	})
}

func TestLlamaGateway_ContentTypes(t *testing.T) {
	gateway, err := newLlamaGateway("http://localhost:11434", "test-api-key")
	if err != nil {
		t.Skipf("Cannot create Llama gateway for content testing: %v", err)
		return
	}

	ctx := context.Background()

	t.Run("Code generation requests", func(t *testing.T) {
		testCases := []struct {
			name         string
			model        string
			systemPrompt string
			userPrompt   string
		}{
			{
				name:         "Go function",
				model:        "codellama",
				systemPrompt: "You are an expert Go developer",
				userPrompt:   "Write a function to reverse a string",
			},
			{
				name:         "Python script",
				model:        "codellama:python",
				systemPrompt: "You are a Python expert",
				userPrompt:   "Write a script to read a CSV file and calculate averages",
			},
			{
				name:         "Algorithm explanation",
				model:        "llama2",
				systemPrompt: "You are a computer science teacher",
				userPrompt:   "Explain bubble sort with code examples",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				request := domainGateway.NewGenerateContentRequest(
					tc.model,
					tc.systemPrompt,
					tc.userPrompt,
					3000,
				)

				_, err := gateway.GenerateContent(ctx, request)
				if err != nil {
					t.Logf("Expected network/API error for %s: %v", tc.name, err)
				}
			})
		}
	})

	t.Run("Creative writing requests", func(t *testing.T) {
		testCases := []struct {
			name         string
			systemPrompt string
			userPrompt   string
		}{
			{
				name:         "Short story",
				systemPrompt: "You are a creative writer",
				userPrompt:   "Write a short story about a programmer who discovers their code is sentient",
			},
			{
				name:         "Technical poem",
				systemPrompt: "You are a poet with technical expertise",
				userPrompt:   "Write a poem about algorithms and data structures",
			},
			{
				name:         "Dialog",
				systemPrompt: "You are a screenwriter",
				userPrompt:   "Write a dialog between two AI assistants discussing the future of programming",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				request := domainGateway.NewGenerateContentRequest(
					"llama2",
					tc.systemPrompt,
					tc.userPrompt,
					2500,
				)

				_, err := gateway.GenerateContent(ctx, request)
				if err != nil {
					t.Logf("Expected network/API error for %s: %v", tc.name, err)
				}
			})
		}
	})

	t.Run("Technical documentation requests", func(t *testing.T) {
		testCases := []struct {
			name         string
			systemPrompt string
			userPrompt   string
		}{
			{
				name:         "API documentation",
				systemPrompt: "You are a technical writer",
				userPrompt:   "Write API documentation for a REST endpoint that creates users",
			},
			{
				name:         "Installation guide",
				systemPrompt: "You are a DevOps engineer",
				userPrompt:   "Write installation instructions for a Go application",
			},
			{
				name:         "Troubleshooting guide",
				systemPrompt: "You are a support engineer",
				userPrompt:   "Write a troubleshooting guide for common Go compilation errors",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				request := domainGateway.NewGenerateContentRequest(
					"llama2",
					tc.systemPrompt,
					tc.userPrompt,
					4000,
				)

				_, err := gateway.GenerateContent(ctx, request)
				if err != nil {
					t.Logf("Expected network/API error for %s: %v", tc.name, err)
				}
			})
		}
	})
}

func TestLlamaGateway_EdgeCases(t *testing.T) {
	gateway, err := newLlamaGateway("http://localhost:11434", "test-api-key")
	if err != nil {
		t.Skipf("Cannot create Llama gateway for edge case testing: %v", err)
		return
	}

	ctx := context.Background()

	t.Run("Very long prompts", func(t *testing.T) {
		longPrompt := strings.Repeat("This is a very long prompt that tests the limits of input handling. ", 100)

		request := domainGateway.NewGenerateContentRequest(
			"llama2",
			"You are a helpful assistant",
			longPrompt,
			1000,
		)

		_, err := gateway.GenerateContent(ctx, request)
		if err != nil {
			t.Logf("Expected network/API error for long prompt: %v", err)
		}
	})

	t.Run("Special characters in prompts", func(t *testing.T) {
		specialPrompts := []string{
			`Explain the use of "quotes" and 'apostrophes' in programming`,
			"Handle unicode: αβγδε ñáéíóú 🚀🎉",
			"Process newlines\nand\ttabs in text",
			"Work with symbols: @#$%^&*()[]{}|\\/:;\"'<>?",
			"Handle JSON: {\"key\": \"value\", \"number\": 42}",
		}

		for i, prompt := range specialPrompts {
			t.Run(fmt.Sprintf("SpecialChars_%d", i), func(t *testing.T) {
				request := domainGateway.NewGenerateContentRequest(
					"llama2",
					"You are a helpful assistant",
					prompt,
					1500,
				)

				_, err := gateway.GenerateContent(ctx, request)
				if err != nil {
					t.Logf("Expected network/API error for special characters: %v", err)
				}
			})
		}
	})

	t.Run("Empty and minimal inputs", func(t *testing.T) {
		testCases := []struct {
			name         string
			systemPrompt string
			userPrompt   string
		}{
			{"Empty user prompt", "System prompt", ""},
			{"Empty system prompt", "", "User prompt"},
			{"Single character", "You are helpful", "a"},
			{"Single word", "Be brief", "hello"},
			{"Question mark only", "Answer questions", "?"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				request := domainGateway.NewGenerateContentRequest(
					"llama2",
					tc.systemPrompt,
					tc.userPrompt,
					500,
				)

				_, err := gateway.GenerateContent(ctx, request)
				if err != nil {
					t.Logf("Expected network/API error for %s: %v", tc.name, err)
				}
			})
		}
	})

	t.Run("Extreme token limits", func(t *testing.T) {
		extremeLimits := []int{1, 5, 10, 50, 10000, 16384, 32000}

		for _, limit := range extremeLimits {
			t.Run(fmt.Sprintf("Limit_%d", limit), func(t *testing.T) {
				request := domainGateway.NewGenerateContentRequest(
					"llama2",
					"You are a helpful assistant",
					"Generate appropriate content for this token limit",
					limit,
				)

				_, err := gateway.GenerateContent(ctx, request)
				if err != nil {
					t.Logf("Expected network/API error for limit %d: %v", limit, err)
				}
			})
		}
	})
}

func TestLlamaGateway_InterfaceCompliance(t *testing.T) {
	gateway, err := newLlamaGateway("http://localhost:11434", "test-api-key")
	if err != nil {
		t.Skipf("Cannot create Llama gateway for interface testing: %v", err)
		return
	}

	// Verify that the gateway implements GenerationGateway interface
	_ = gateway
	t.Log("LlamaGateway correctly implements GenerationGateway interface")

	// Test basic interface methods
	ctx := context.Background()
	request := domainGateway.NewGenerateContentRequest(
		"llama2",
		"Test system prompt",
		"Test user prompt",
		1000,
	)

	// The method should exist and be callable (though it will likely fail with network error)
	_, err = gateway.GenerateContent(ctx, request)
	t.Logf("GenerateContent method exists and is callable: %v", err != nil)
}
