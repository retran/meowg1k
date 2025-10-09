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

package model

import (
	"reflect"
	"slices"
	"testing"

	"github.com/retran/meowg1k/internal/core/model"
)

func TestNewService(t *testing.T) {
	service := NewRegistry()
	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestGetModelInfo(t *testing.T) {
	service := NewRegistry()

	// Test known model
	info := service.GetModelInfo("gpt-4o")
	if info.Provider != "openai" {
		t.Errorf("Expected provider 'openai', got '%s'", info.Provider)
	}
	if info.MaxContextTokens != 128000 {
		t.Errorf("Expected MaxContextTokens 128000, got %d", info.MaxContextTokens)
	}
	if info.TokenizerType != model.TokenizerCL100K {
		t.Errorf("Expected TokenizerCL100K, got %s", info.TokenizerType)
	}

	// Test unknown model
	unknownInfo := service.GetModelInfo("unknown-model")
	expected := model.ModelInfo{
		Provider:         "unknown",
		MaxContextTokens: 8192,
		TokenizerType:    model.TokenizerUnknown,
		Description:      "Unknown model",
	}
	if !reflect.DeepEqual(unknownInfo, expected) {
		t.Errorf("Expected %+v for unknown model, got %+v", expected, unknownInfo)
	}
}

func TestGetMaxContextTokens(t *testing.T) {
	service := NewRegistry()

	// Test known model
	tokens := service.GetMaxContextTokens("claude-sonnet-4")
	if tokens != 1000000 {
		t.Errorf("Expected 1000000 tokens for claude-sonnet-4, got %d", tokens)
	}

	// Test unknown model
	tokens = service.GetMaxContextTokens("unknown-model")
	if tokens != 8192 {
		t.Errorf("Expected 8192 tokens for unknown model, got %d", tokens)
	}
}

func TestGetTokenizerType(t *testing.T) {
	service := NewRegistry()

	// Test CL100K tokenizer
	tokenizerType := service.GetTokenizerType("gpt-4o")
	if tokenizerType != model.TokenizerCL100K {
		t.Errorf("Expected TokenizerCL100K for gpt-4o, got %s", tokenizerType)
	}

	// Test Gemini tokenizer
	tokenizerType = service.GetTokenizerType("gemini-2.5-pro")
	if tokenizerType != model.TokenizerGemini {
		t.Errorf("Expected TokenizerGemini for gemini-2.5-pro, got %s", tokenizerType)
	}

	// Test Llama tokenizer
	tokenizerType = service.GetTokenizerType("meta-llama/llama-3.3-70b-instruct")
	if tokenizerType != model.TokenizerLlama {
		t.Errorf("Expected TokenizerLlama for llama model, got %s", tokenizerType)
	}

	// Test unknown model
	tokenizerType = service.GetTokenizerType("unknown-model")
	if tokenizerType != model.TokenizerUnknown {
		t.Errorf("Expected TokenizerUnknown for unknown model, got %s", tokenizerType)
	}
}

func TestGetDefaultEmbedDimension(t *testing.T) {
	service := NewRegistry()

	// Test embedding model
	dimension := service.GetDefaultEmbedDimension("text-embedding-3-large")
	if dimension != 3072 {
		t.Errorf("Expected 3072 for text-embedding-3-large, got %d", dimension)
	}

	// Test non-embedding model
	dimension = service.GetDefaultEmbedDimension("gpt-4o")
	if dimension != 0 {
		t.Errorf("Expected 0 for non-embedding model gpt-4o, got %d", dimension)
	}

	// Test unknown model
	dimension = service.GetDefaultEmbedDimension("unknown-model")
	if dimension != 0 {
		t.Errorf("Expected 0 for unknown model, got %d", dimension)
	}
}

func TestGetProvider(t *testing.T) {
	service := NewRegistry()

	// Test OpenAI model
	provider := service.GetProvider("gpt-4o")
	if provider != "openai" {
		t.Errorf("Expected 'openai' for gpt-4o, got '%s'", provider)
	}

	// Test Anthropic model
	provider = service.GetProvider("claude-sonnet-4")
	if provider != "anthropic" {
		t.Errorf("Expected 'anthropic' for claude-sonnet-4, got '%s'", provider)
	}

	// Test Google model
	provider = service.GetProvider("gemini-2.5-pro")
	if provider != "gemini" {
		t.Errorf("Expected 'gemini' for gemini-2.5-pro, got '%s'", provider)
	}

	// Test unknown model
	provider = service.GetProvider("unknown-model")
	if provider != "unknown" {
		t.Errorf("Expected 'unknown' for unknown model, got '%s'", provider)
	}
}

func TestGetMaxOutputTokens(t *testing.T) {
	service := NewRegistry()

	// Test model with specific max output tokens
	tokens := service.GetMaxOutputTokens("gpt-4o")
	if tokens != 32768 {
		t.Errorf("Expected 32768 for gpt-4o, got %d", tokens)
	}

	// Test model with zero max output tokens (should return default)
	tokens = service.GetMaxOutputTokens("x-ai/grok-code-fast-1")
	if tokens != 4096 {
		t.Errorf("Expected 4096 default for model with zero max output tokens, got %d", tokens)
	}

	// Test unknown model
	tokens = service.GetMaxOutputTokens("unknown-model")
	if tokens != 4096 {
		t.Errorf("Expected 4096 default for unknown model, got %d", tokens)
	}
}

func TestListKnownModels(t *testing.T) {
	service := NewRegistry()

	models := service.ListKnownModels()

	// Verify we have models
	if len(models) == 0 {
		t.Fatal("ListKnownModels() returned empty slice")
	}

	// Verify all models are unique
	modelSet := make(map[string]bool)
	for _, model := range models {
		if modelSet[model] {
			t.Errorf("Duplicate model found: %s", model)
		}
		modelSet[model] = true
	}

	// Verify the list contains expected models (order may vary since it's from a map)
	expectedModels := []string{
		"gpt-4o",
		"claude-sonnet-4",
		"gemini-2.5-pro",
		"text-embedding-3-large",
		"voyage-3-large",
	}

	for _, expected := range expectedModels {
		found := slices.Contains(models, expected)
		if !found {
			t.Errorf("Expected model %s not found in list", expected)
		}
	}
}
