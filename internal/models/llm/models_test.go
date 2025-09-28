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

package llm

import "testing"

func TestTokenizerTypeConstants(t *testing.T) {
	// Test that all tokenizer constants are defined correctly
	tokenizers := map[TokenizerType]string{
		TokenizerCL100K:        "cl100k_base",
		TokenizerGPT2:          "gpt2",
		TokenizerSentencePiece: "sentencepiece",
		TokenizerTikToken:      "tiktoken",
		TokenizerGemini:        "gemini",
		TokenizerLlama:         "llama",
		TokenizerUnknown:       "unknown",
	}

	for tokenizer, expected := range tokenizers {
		if string(tokenizer) != expected {
			t.Errorf("Expected tokenizer %s to equal '%s', got '%s'", tokenizer, expected, string(tokenizer))
		}
	}
}

func TestTokenizerTypeString(t *testing.T) {
	// Test that TokenizerType can be converted to string
	tokenizer := TokenizerCL100K
	if string(tokenizer) != "cl100k_base" {
		t.Errorf("Expected string conversion to work: got %s", string(tokenizer))
	}
}

func TestModelInfo(t *testing.T) {
	// Test ModelInfo struct initialization and field access
	model := ModelInfo{
		Provider:              "OpenAI",
		MaxContextTokens:      128000,
		MaxOutputTokens:       4096,
		TokenizerType:         TokenizerCL100K,
		Description:           "GPT-4 Turbo model",
		DefaultEmbedDimension: 1536,
	}

	if model.Provider != "OpenAI" {
		t.Errorf("Expected Provider 'OpenAI', got '%s'", model.Provider)
	}

	if model.MaxContextTokens != 128000 {
		t.Errorf("Expected MaxContextTokens 128000, got %d", model.MaxContextTokens)
	}

	if model.MaxOutputTokens != 4096 {
		t.Errorf("Expected MaxOutputTokens 4096, got %d", model.MaxOutputTokens)
	}

	if model.TokenizerType != TokenizerCL100K {
		t.Errorf("Expected TokenizerType %s, got %s", TokenizerCL100K, model.TokenizerType)
	}

	if model.Description != "GPT-4 Turbo model" {
		t.Errorf("Expected Description 'GPT-4 Turbo model', got '%s'", model.Description)
	}

	if model.DefaultEmbedDimension != 1536 {
		t.Errorf("Expected DefaultEmbedDimension 1536, got %d", model.DefaultEmbedDimension)
	}
}

func TestModelInfoZeroValues(t *testing.T) {
	// Test ModelInfo with zero values
	model := ModelInfo{}

	if model.Provider != "" {
		t.Errorf("Expected empty Provider, got '%s'", model.Provider)
	}

	if model.MaxContextTokens != 0 {
		t.Errorf("Expected MaxContextTokens 0, got %d", model.MaxContextTokens)
	}

	if model.MaxOutputTokens != 0 {
		t.Errorf("Expected MaxOutputTokens 0, got %d", model.MaxOutputTokens)
	}

	if model.TokenizerType != "" {
		t.Errorf("Expected empty TokenizerType, got %s", model.TokenizerType)
	}

	if model.Description != "" {
		t.Errorf("Expected empty Description, got '%s'", model.Description)
	}

	if model.DefaultEmbedDimension != 0 {
		t.Errorf("Expected DefaultEmbedDimension 0, got %d", model.DefaultEmbedDimension)
	}
}

func TestTokenizerTypeComparison(t *testing.T) {
	// Test that TokenizerType values can be compared
	if TokenizerCL100K == TokenizerGPT2 {
		t.Error("Different tokenizer types should not be equal")
	}

	// Test self-equality
	if TokenizerCL100K == TokenizerCL100K {
		// This is expected behavior - same constant should equal itself
	} else {
		t.Error("Same tokenizer type should be equal to itself")
	}
}

func TestEmbeddingModelInfo(t *testing.T) {
	// Test ModelInfo for embedding models (MaxOutputTokens = 0)
	embeddingModel := ModelInfo{
		Provider:              "Voyage",
		MaxContextTokens:      32000,
		MaxOutputTokens:       0, // Embeddings don't produce text output
		TokenizerType:         TokenizerCL100K,
		Description:           "Voyage embedding model",
		DefaultEmbedDimension: 1024,
	}

	// Test embedding-specific fields
	if embeddingModel.MaxOutputTokens != 0 {
		t.Errorf("Expected MaxOutputTokens 0 for embedding model, got %d", embeddingModel.MaxOutputTokens)
	}

	if embeddingModel.DefaultEmbedDimension != 1024 {
		t.Errorf("Expected DefaultEmbedDimension 1024, got %d", embeddingModel.DefaultEmbedDimension)
	}

	if embeddingModel.Provider != "Voyage" {
		t.Errorf("Expected Provider 'Voyage', got %q", embeddingModel.Provider)
	}

	if embeddingModel.TokenizerType != TokenizerCL100K {
		t.Errorf("Expected TokenizerType %v, got %v", TokenizerCL100K, embeddingModel.TokenizerType)
	}

	if embeddingModel.DefaultEmbedDimension != 1024 {
		t.Errorf("Expected DefaultEmbedDimension 1024, got %d", embeddingModel.DefaultEmbedDimension)
	}
}
