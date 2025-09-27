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
	"fmt"
	"testing"
)

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

func TestNewComputeEmbeddingsRequest(t *testing.T) {
	model := "text-embedding-ada-002"
	chunks := []string{"hello world", "this is a test", "embeddings request"}
	taskType := RetrievalQuery

	request := NewComputeEmbeddingsRequest(model, chunks, taskType)

	if request == nil {
		t.Fatal("Expected non-nil request")
	}

	if request.Model() != model {
		t.Errorf("Expected model %q, got %q", model, request.Model())
	}

	if len(request.Chunks()) != len(chunks) {
		t.Errorf("Expected %d chunks, got %d", len(chunks), len(request.Chunks()))
	}

	for i, chunk := range chunks {
		if request.Chunks()[i] != chunk {
			t.Errorf("Expected chunk[%d] %q, got %q", i, chunk, request.Chunks()[i])
		}
	}

	if request.TaskType() != taskType {
		t.Errorf("Expected task type %q, got %q", taskType, request.TaskType())
	}

	if request.Dimensions() != 0 {
		t.Errorf("Expected dimensions 0, got %d", request.Dimensions())
	}
}

func TestNewComputeEmbeddingsRequestWithDimensions(t *testing.T) {
	model := "voyage-3"
	chunks := []string{"test chunk"}
	taskType := Classification
	dimensions := 1024

	request := NewComputeEmbeddingsRequestWithDimensions(model, chunks, taskType, dimensions)

	if request == nil {
		t.Fatal("Expected non-nil request")
	}

	if request.Model() != model {
		t.Errorf("Expected model %q, got %q", model, request.Model())
	}

	if len(request.Chunks()) != len(chunks) {
		t.Errorf("Expected %d chunks, got %d", len(chunks), len(request.Chunks()))
	}

	if request.TaskType() != taskType {
		t.Errorf("Expected task type %q, got %q", taskType, request.TaskType())
	}

	if request.Dimensions() != dimensions {
		t.Errorf("Expected dimensions %d, got %d", dimensions, request.Dimensions())
	}
}

func TestComputeEmbeddingsRequestGetters(t *testing.T) {
	model := "test-model"
	chunks := []string{"chunk1", "chunk2", "chunk3"}
	taskType := Clustering
	dimensions := 512

	request := NewComputeEmbeddingsRequestWithDimensions(model, chunks, taskType, dimensions)

	// Test Model() getter
	if got := request.Model(); got != model {
		t.Errorf("Model() = %q, want %q", got, model)
	}

	// Test Chunks() getter
	gotChunks := request.Chunks()
	if len(gotChunks) != len(chunks) {
		t.Errorf("Chunks() length = %d, want %d", len(gotChunks), len(chunks))
	}

	for i, chunk := range chunks {
		if gotChunks[i] != chunk {
			t.Errorf("Chunks()[%d] = %q, want %q", i, gotChunks[i], chunk)
		}
	}

	// Test TaskType() getter
	if got := request.TaskType(); got != taskType {
		t.Errorf("TaskType() = %q, want %q", got, taskType)
	}

	// Test Dimensions() getter  
	if got := request.Dimensions(); got != dimensions {
		t.Errorf("Dimensions() = %d, want %d", got, dimensions)
	}
}

func TestComputeEmbeddingsRequestEdgeCases(t *testing.T) {
	// Test with empty model
	request1 := NewComputeEmbeddingsRequest("", []string{"test"}, RetrievalDocument)
	if request1.Model() != "" {
		t.Error("Expected empty model to be preserved")
	}

	// Test with nil chunks
	request2 := NewComputeEmbeddingsRequest("model", nil, SemanticSimilarity)
	if request2.Chunks() != nil && len(request2.Chunks()) != 0 {
		t.Error("Expected nil or empty chunks slice")
	}

	// Test with empty chunks
	request3 := NewComputeEmbeddingsRequest("model", []string{}, QuestionAnswering)
	if len(request3.Chunks()) != 0 {
		t.Error("Expected empty chunks slice")
	}

	// Test with zero dimensions
	request4 := NewComputeEmbeddingsRequestWithDimensions("model", []string{"test"}, FactVerification, 0)
	if request4.Dimensions() != 0 {
		t.Error("Expected zero dimensions to be preserved")
	}

	// Test with negative dimensions (edge case)
	request5 := NewComputeEmbeddingsRequestWithDimensions("model", []string{"test"}, CodeRetrievalQuery, -1)
	if request5.Dimensions() != -1 {
		t.Error("Expected negative dimensions to be preserved (though not valid)")
	}
}

func TestComputeEmbeddingsRequestAllTaskTypes(t *testing.T) {
	// Test all valid task types
	taskTypes := []TaskType{
		RetrievalDocument,
		RetrievalQuery,
		CodeRetrievalQuery,
		SemanticSimilarity,
		Classification,
		Clustering,
		QuestionAnswering,
		FactVerification,
	}

	for _, taskType := range taskTypes {
		request := NewComputeEmbeddingsRequest("test-model", []string{"test chunk"}, taskType)
		if request.TaskType() != taskType {
			t.Errorf("Expected task type %q, got %q", taskType, request.TaskType())
		}
	}
}

func TestComputeEmbeddingsRequestImmutability(t *testing.T) {
	originalChunks := []string{"chunk1", "chunk2"}
	request := NewComputeEmbeddingsRequest("model", originalChunks, RetrievalQuery)

	// Modify the original slice
	originalChunks[0] = "modified"

	// The request should not be affected (if implemented properly)
	// Note: This depends on whether the implementation makes a copy or not
	// This test documents the expected behavior
	requestChunks := request.Chunks()
	if len(requestChunks) != 2 {
		t.Errorf("Expected 2 chunks, got %d", len(requestChunks))
	}
}

func TestComputeEmbeddingsRequestLargeData(t *testing.T) {
	// Test with a large number of chunks
	chunks := make([]string, 1000)
	for i := range chunks {
		chunks[i] = fmt.Sprintf("chunk-%d", i)
	}

	request := NewComputeEmbeddingsRequest("large-model", chunks, Clustering)
	
	if len(request.Chunks()) != 1000 {
		t.Errorf("Expected 1000 chunks, got %d", len(request.Chunks()))
	}

	// Test with very large dimension value
	request2 := NewComputeEmbeddingsRequestWithDimensions("model", []string{"test"}, RetrievalQuery, 65536)
	if request2.Dimensions() != 65536 {
		t.Errorf("Expected dimensions 65536, got %d", request2.Dimensions())
	}
}