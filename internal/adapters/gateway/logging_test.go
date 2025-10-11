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
	"sync"
	"testing"
	"time"

	"github.com/retran/meowg1k/internal/adapters/tracelog"
	"github.com/retran/meowg1k/internal/domain/gateway"
)

// mockTraceLogger implements TraceLogger for testing
type mockTraceLogger struct {
	mu              sync.Mutex
	apiInteractions []*tracelog.APIInteractionEntry
}

func (m *mockTraceLogger) LogAPIInteraction(entry *tracelog.APIInteractionEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.apiInteractions = append(m.apiInteractions, entry)
	return nil
}

func (m *mockTraceLogger) getInteractions() []*tracelog.APIInteractionEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*tracelog.APIInteractionEntry{}, m.apiInteractions...)
}

// mockLoggingGenerationGateway implements ports.GenerationGateway for testing
type mockLoggingGenerationGateway struct {
	response string
	err      error
}

func (m *mockLoggingGenerationGateway) GenerateContent(ctx context.Context, request *gateway.GenerateContentRequest) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

// mockLoggingEmbeddingsGateway implements ports.EmbeddingsGateway for testing
type mockLoggingEmbeddingsGateway struct {
	embeddings []gateway.Embedding
	distance   float64
	err        error
}

func (m *mockLoggingEmbeddingsGateway) ComputeEmbeddings(ctx context.Context, request *gateway.ComputeEmbeddingsRequest) ([]gateway.Embedding, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.embeddings, nil
}

func (m *mockLoggingEmbeddingsGateway) ComputeDistance(first, second gateway.Embedding) (float64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.distance, nil
}

func TestLoggingGenerationGateway_Success(t *testing.T) {
	logger := &mockTraceLogger{}
	inner := &mockLoggingGenerationGateway{
		response: "Generated content",
	}

	wrapped := newLoggingGenerationGateway(inner, logger, "commit", "default", "openai")

	req := gateway.NewGenerateContentRequest("gpt-4", "System prompt", "User prompt", 1000)

	ctx := context.Background()
	content, err := wrapped.GenerateContent(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if content != "Generated content" {
		t.Errorf("Expected 'Generated content', got %s", content)
	}

	// Wait for async logging
	time.Sleep(50 * time.Millisecond)

	interactions := logger.getInteractions()
	if len(interactions) != 1 {
		t.Fatalf("Expected 1 interaction logged, got %d", len(interactions))
	}

	entry := interactions[0]
	if entry.Command != "commit" {
		t.Errorf("Expected command 'commit', got %s", entry.Command)
	}
	if entry.Profile != "default" {
		t.Errorf("Expected profile 'default', got %s", entry.Profile)
	}
	if entry.Provider != "openai" {
		t.Errorf("Expected provider 'openai', got %s", entry.Provider)
	}
	if entry.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got %s", entry.Model)
	}
	if entry.Response.Content != "Generated content" {
		t.Errorf("Expected response 'Generated content', got %s", entry.Response.Content)
	}
}

func TestLoggingGenerationGateway_Error(t *testing.T) {
	logger := &mockTraceLogger{}
	inner := &mockLoggingGenerationGateway{
		err: fmt.Errorf("API error"),
	}

	wrapped := newLoggingGenerationGateway(inner, logger, "commit", "default", "openai")

	req := gateway.NewGenerateContentRequest("gpt-4", "", "", 0)

	ctx := context.Background()
	_, err := wrapped.GenerateContent(ctx, req)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Wait for async logging
	time.Sleep(50 * time.Millisecond)

	interactions := logger.getInteractions()
	if len(interactions) != 1 {
		t.Fatalf("Expected 1 interaction logged, got %d", len(interactions))
	}

	entry := interactions[0]
	if entry.Response.Error != "API error" {
		t.Errorf("Expected error 'API error', got %s", entry.Response.Error)
	}
}

func TestLoggingGenerationGateway_NilLogger(t *testing.T) {
	inner := &mockLoggingGenerationGateway{
		response: "Generated content",
	}

	wrapped := newLoggingGenerationGateway(inner, nil, "commit", "default", "openai")

	// Should return the inner gateway when logger is nil
	if wrapped != inner {
		t.Error("Expected wrapped gateway to be the inner gateway when logger is nil")
	}
}

func TestLoggingEmbeddingsGateway_Success(t *testing.T) {
	logger := &mockTraceLogger{}
	inner := &mockLoggingEmbeddingsGateway{
		embeddings: []gateway.Embedding{
			{0.1, 0.2, 0.3},
			{0.4, 0.5, 0.6},
		},
	}

	wrapped := newLoggingEmbeddingsGateway(inner, logger, "commit", "default", "voyage")

	req := gateway.NewComputeEmbeddingsRequest("voyage-2", []string{"chunk1", "chunk2"}, gateway.RetrievalDocument)

	ctx := context.Background()
	embeddings, err := wrapped.ComputeEmbeddings(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(embeddings) != 2 {
		t.Errorf("Expected 2 embeddings, got %d", len(embeddings))
	}

	// Wait for async logging
	time.Sleep(50 * time.Millisecond)

	interactions := logger.getInteractions()
	if len(interactions) != 1 {
		t.Fatalf("Expected 1 interaction logged, got %d", len(interactions))
	}

	entry := interactions[0]
	if entry.Command != "commit" {
		t.Errorf("Expected command 'commit', got %s", entry.Command)
	}
	if entry.Model != "voyage-2" {
		t.Errorf("Expected model 'voyage-2', got %s", entry.Model)
	}
}

func TestLoggingEmbeddingsGateway_Error(t *testing.T) {
	logger := &mockTraceLogger{}
	inner := &mockLoggingEmbeddingsGateway{
		err: fmt.Errorf("embeddings API error"),
	}

	wrapped := newLoggingEmbeddingsGateway(inner, logger, "commit", "default", "voyage")

	req := gateway.NewComputeEmbeddingsRequest("voyage-2", []string{"chunk"}, gateway.RetrievalDocument)

	ctx := context.Background()
	_, err := wrapped.ComputeEmbeddings(ctx, req)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Wait for async logging
	time.Sleep(50 * time.Millisecond)

	interactions := logger.getInteractions()
	if len(interactions) != 1 {
		t.Fatalf("Expected 1 interaction logged, got %d", len(interactions))
	}

	entry := interactions[0]
	if entry.Response.Error != "embeddings API error" {
		t.Errorf("Expected error 'embeddings API error', got %s", entry.Response.Error)
	}
}

func TestLoggingEmbeddingsGateway_ComputeDistance(t *testing.T) {
	logger := &mockTraceLogger{}
	inner := &mockLoggingEmbeddingsGateway{
		distance: 0.85,
	}

	wrapped := newLoggingEmbeddingsGateway(inner, logger, "commit", "default", "voyage")

	emb1 := gateway.Embedding{0.1, 0.2, 0.3}
	emb2 := gateway.Embedding{0.4, 0.5, 0.6}

	distance, err := wrapped.ComputeDistance(emb1, emb2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if distance != 0.85 {
		t.Errorf("Expected distance 0.85, got %f", distance)
	}

	// ComputeDistance should not log anything
	time.Sleep(50 * time.Millisecond)
	interactions := logger.getInteractions()
	if len(interactions) != 0 {
		t.Errorf("Expected 0 interactions logged for ComputeDistance, got %d", len(interactions))
	}
}

func TestLoggingEmbeddingsGateway_NilLogger(t *testing.T) {
	inner := &mockLoggingEmbeddingsGateway{
		embeddings: []gateway.Embedding{{0.1, 0.2}},
	}

	wrapped := newLoggingEmbeddingsGateway(inner, nil, "commit", "default", "voyage")

	// Should return the inner gateway when logger is nil
	if wrapped != inner {
		t.Error("Expected wrapped gateway to be the inner gateway when logger is nil")
	}
}

func TestFormatChunks(t *testing.T) {
	// Test with few chunks
	chunks := []string{"chunk1", "chunk2"}
	result := formatChunks(chunks)
	if result == "" {
		t.Error("Expected non-empty result")
	}

	// Test with many chunks (should truncate)
	manyChunks := make([]string, 15)
	for i := range manyChunks {
		manyChunks[i] = fmt.Sprintf("chunk%d", i)
	}
	result = formatChunks(manyChunks)
	if result == "" {
		t.Error("Expected non-empty result")
	}

	// Test with long chunk (should truncate)
	longChunk := string(make([]byte, 200))
	result = formatChunks([]string{longChunk})
	if len(result) > 500 {
		t.Error("Expected truncated result")
	}

	// Test with empty chunks
	result = formatChunks([]string{})
	if result != "" {
		t.Errorf("Expected empty string for empty chunks, got %s", result)
	}
}

func TestFormatEmbeddingsResult(t *testing.T) {
	// Test with embeddings
	embeddings := []gateway.Embedding{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
	}
	result := formatEmbeddingsResult(embeddings)
	if result != "2 embeddings with 3 dimensions each" {
		t.Errorf("Expected '2 embeddings with 3 dimensions each', got %s", result)
	}

	// Test with empty embeddings
	result = formatEmbeddingsResult([]gateway.Embedding{})
	if result != "0 embeddings" {
		t.Errorf("Expected '0 embeddings', got %s", result)
	}
}
