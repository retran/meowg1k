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

import "testing"

func TestProviderConstants(t *testing.T) {
	// Test that all provider constants are defined correctly
	providers := map[Provider]string{
		Llama:            "llama",
		Gemini:           "gemini",
		OpenAI:           "openai",
		OpenRouter:       "openrouter",
		OpenAICompatible: "openai-compatible",
		Anthropic:        "anthropic",
		Voyage:           "voyage",
	}

	for provider, expected := range providers {
		if string(provider) != expected {
			t.Errorf("Expected provider %s to equal '%s', got '%s'", provider, expected, string(provider))
		}
	}
}

func TestNewGenerateContentRequest(t *testing.T) {
	model := "gpt-3.5-turbo"
	systemPrompt := "You are a helpful assistant"
	userPrompt := "Hello, world!"
	maxTokens := 1000

	request := NewGenerateContentRequest(model, systemPrompt, userPrompt, maxTokens)

	if request == nil {
		t.Fatal("NewGenerateContentRequest returned nil")
	}

	if request.Model() != model {
		t.Errorf("Expected model '%s', got '%s'", model, request.Model())
	}

	if request.SystemPrompt() != systemPrompt {
		t.Errorf("Expected system prompt '%s', got '%s'", systemPrompt, request.SystemPrompt())
	}

	if request.UserPrompt() != userPrompt {
		t.Errorf("Expected user prompt '%s', got '%s'", userPrompt, request.UserPrompt())
	}

	if request.MaxOutputTokens() != maxTokens {
		t.Errorf("Expected max output tokens %d, got %d", maxTokens, request.MaxOutputTokens())
	}
}

func TestGenerateContentRequestGetters(t *testing.T) {
	request := &GenerateContentRequest{
		model:           "test-model",
		systemPrompt:    "test-system",
		userPrompt:      "test-user",
		maxOutputTokens: 500,
	}

	if request.Model() != "test-model" {
		t.Errorf("Expected model 'test-model', got '%s'", request.Model())
	}

	if request.SystemPrompt() != "test-system" {
		t.Errorf("Expected system prompt 'test-system', got '%s'", request.SystemPrompt())
	}

	if request.UserPrompt() != "test-user" {
		t.Errorf("Expected user prompt 'test-user', got '%s'", request.UserPrompt())
	}

	if request.MaxOutputTokens() != 500 {
		t.Errorf("Expected max output tokens 500, got %d", request.MaxOutputTokens())
	}
}

func TestEmbeddingType(t *testing.T) {
	// Test that Embedding type works as expected
	embedding := Embedding{0.1, 0.2, 0.3, 0.4, 0.5}
	
	if len(embedding) != 5 {
		t.Errorf("Expected embedding length 5, got %d", len(embedding))
	}

	if embedding[0] != 0.1 {
		t.Errorf("Expected first element 0.1, got %f", embedding[0])
	}

	if embedding[4] != 0.5 {
		t.Errorf("Expected last element 0.5, got %f", embedding[4])
	}
}

func TestEmbeddingNil(t *testing.T) {
	var embedding Embedding
	
	if len(embedding) != 0 {
		t.Errorf("Expected nil embedding length 0, got %d", len(embedding))
	}
}

func TestEmbeddingEmpty(t *testing.T) {
	embedding := Embedding{}
	
	if len(embedding) != 0 {
		t.Errorf("Expected empty embedding length 0, got %d", len(embedding))
	}
}