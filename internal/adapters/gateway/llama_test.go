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

	"github.com/retran/meowg1k/internal/adapters/llama"
	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
)

// newLlamaTestServer starts an httptest.Server that handles /completion and /embedding requests.
func newLlamaTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/completion", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := llama.CompletionResponse{
			Content:         "Mock Llama response",
			TokensEvaluated: 10,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/embedding", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		items := []llama.EmbeddingBatchItem{
			{
				Index:     0,
				Embedding: [][]float64{{0.1, 0.2, 0.3}},
			},
		}
		if err := json.NewEncoder(w).Encode(items); err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
		}
	})

	return httptest.NewServer(mux)
}

func TestNewLlamaGateway(t *testing.T) {
	srv := newLlamaTestServer(t)
	defer srv.Close()

	t.Run("Valid parameters", func(t *testing.T) {
		gw, err := newLlamaGateway(srv.URL, "test-api-key", &http.Client{})
		require.NoError(t, err)
		require.NotNil(t, gw)
	})

	t.Run("Empty base URL", func(t *testing.T) {
		gw, err := newLlamaGateway("", "test-api-key", &http.Client{})
		assert.Error(t, err)
		assert.Nil(t, gw)
	})

	t.Run("Empty API key allowed", func(t *testing.T) {
		gw, err := newLlamaGateway(srv.URL, "", &http.Client{})
		require.NoError(t, err)
		require.NotNil(t, gw)
	})
}

func TestNewLlamaGateway_NilHTTPClient(t *testing.T) {
	gw, err := newLlamaGateway("http://localhost:11434", "test-api-key", nil)
	require.Error(t, err)
	assert.Nil(t, gw)
	assert.Contains(t, err.Error(), "HTTP client is required")
}

func TestLlamaGateway_GenerateContent(t *testing.T) {
	srv := newLlamaTestServer(t)
	defer srv.Close()

	gw, err := newLlamaGateway(srv.URL, "test-api-key", srv.Client())
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Valid request", func(t *testing.T) {
		req := domainGateway.NewGenerateContentRequest(
			"llama2", "You are a helpful assistant", "Hello!", 4096,
		)
		resp, err := gw.GenerateContent(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.Blocks)
	})

	t.Run("With system prompt", func(t *testing.T) {
		req := domainGateway.NewGenerateContentRequest(
			"codellama", "You are an expert Go programmer.", "Write a GCD function", 4096,
		)
		resp, err := gw.GenerateContent(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("Without system prompt", func(t *testing.T) {
		req := domainGateway.NewGenerateContentRequest("llama2", "", "Explain recursion", 2048)
		resp, err := gw.GenerateContent(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("Canceled context", func(t *testing.T) {
		req := domainGateway.NewGenerateContentRequest("llama2", "You are helpful", "Hello!", 4096)
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := gw.GenerateContent(cancelCtx, req)
		require.Error(t, err)
	})

	t.Run("Nil request", func(t *testing.T) {
		_, err := gw.GenerateContent(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "request cannot be nil")
	})

	t.Run("Multiple models", func(t *testing.T) {
		for _, model := range []string{"llama2", "codellama", "mistral"} {
			t.Run(model, func(t *testing.T) {
				req := domainGateway.NewGenerateContentRequest(model, "System", "Prompt", 1000)
				resp, err := gw.GenerateContent(ctx, req)
				require.NoError(t, err)
				require.NotNil(t, resp)
			})
		}
	})
}

func TestLlamaGateway_NilChecks(t *testing.T) {
	ctx := context.Background()

	t.Run("Nil request", func(t *testing.T) {
		srv := newLlamaTestServer(t)
		defer srv.Close()
		gw, err := newLlamaGateway(srv.URL, "key", srv.Client())
		require.NoError(t, err)
		_, err = gw.GenerateContent(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "request cannot be nil")
	})

	t.Run("Nil gateway", func(t *testing.T) {
		var nilGw *llamaGateway
		req := domainGateway.NewGenerateContentRequest("llama2", "System", "Prompt", 1000)
		_, err := nilGw.GenerateContent(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "llama gateway is nil")
	})
}

func TestLlamaGateway_ContentTypes(t *testing.T) {
	srv := newLlamaTestServer(t)
	defer srv.Close()

	gw, err := newLlamaGateway(srv.URL, "test-api-key", srv.Client())
	require.NoError(t, err)

	ctx := context.Background()

	cases := []struct {
		name, model, system, user string
	}{
		{"Go function", "codellama", "You are a Go expert", "Reverse a string"},
		{"Python script", "codellama:python", "You are a Python expert", "Read a CSV file"},
		{"Algorithm", "llama2", "You are a CS teacher", "Explain bubble sort"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := domainGateway.NewGenerateContentRequest(tc.model, tc.system, tc.user, 3000)
			resp, err := gw.GenerateContent(ctx, req)
			require.NoError(t, err)
			require.NotNil(t, resp)
		})
	}
}

func TestLlamaGateway_EdgeCases(t *testing.T) {
	srv := newLlamaTestServer(t)
	defer srv.Close()

	gw, err := newLlamaGateway(srv.URL, "test-api-key", srv.Client())
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Very long prompt", func(t *testing.T) {
		longPrompt := strings.Repeat("This is a very long prompt. ", 100)
		req := domainGateway.NewGenerateContentRequest("llama2", "You are helpful", longPrompt, 1000)
		resp, err := gw.GenerateContent(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("Special characters", func(t *testing.T) {
		req := domainGateway.NewGenerateContentRequest(
			"llama2", "You are helpful",
			`Handle unicode: αβγδ and JSON: {"key": "value"}`,
			1500,
		)
		resp, err := gw.GenerateContent(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("Empty user prompt", func(t *testing.T) {
		req := domainGateway.NewGenerateContentRequest("llama2", "System prompt", "", 500)
		resp, err := gw.GenerateContent(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}

func TestLlamaGateway_WithGenerationParameters(t *testing.T) {
	srv := newLlamaTestServer(t)
	defer srv.Close()

	gw, err := newLlamaGateway(srv.URL, "test-api-key", srv.Client())
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("With temperature", func(t *testing.T) {
		temp := 0.7
		req := domainGateway.NewGenerateContentRequest("llama2", "System", "Prompt", 1000).
			WithTemperature(&temp)
		resp, err := gw.GenerateContent(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("With topP and topK", func(t *testing.T) {
		topP := 0.9
		topK := 40
		req := domainGateway.NewGenerateContentRequest("llama2", "System", "Prompt", 1000).
			WithTopP(&topP).WithTopK(&topK)
		resp, err := gw.GenerateContent(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("With stop sequences", func(t *testing.T) {
		req := domainGateway.NewGenerateContentRequest("llama2", "System", "Prompt", 1000).
			WithStop([]string{"END", "STOP"})
		resp, err := gw.GenerateContent(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("With all parameters", func(t *testing.T) {
		temp := 0.8
		topP := 0.95
		topK := 50
		fp := 0.5
		pp := 0.6
		rp := 1.1
		minP := 0.05
		seed := 42
		req := domainGateway.NewGenerateContentRequest("llama2", "System", "Prompt", 1000).
			WithTemperature(&temp).
			WithTopP(&topP).
			WithTopK(&topK).
			WithFrequencyPenalty(&fp).
			WithPresencePenalty(&pp).
			WithRepetitionPenalty(&rp).
			WithMinP(&minP).
			WithSeed(&seed).
			WithStop([]string{"STOP"})
		resp, err := gw.GenerateContent(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}

func TestLlamaGateway_InterfaceCompliance(t *testing.T) {
	srv := newLlamaTestServer(t)
	defer srv.Close()

	gw, err := newLlamaGateway(srv.URL, "test-api-key", srv.Client())
	require.NoError(t, err)
	require.NotNil(t, gw)

	ctx := context.Background()
	req := domainGateway.NewGenerateContentRequest("llama2", "Test system", "Test user", 1000)
	resp, err := gw.GenerateContent(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestLlamaGateway_GenerateContentStream(t *testing.T) {
	srv := newLlamaTestServer(t)
	defer srv.Close()

	gw, err := newLlamaGateway(srv.URL, "test-api-key", srv.Client())
	require.NoError(t, err)

	ctx := context.Background()
	req := domainGateway.NewGenerateContentRequest("llama2", "System", "Prompt", 1000)

	var events []domainGateway.StreamEvent
	resp, err := gw.GenerateContentStream(ctx, req, func(e domainGateway.StreamEvent) error {
		events = append(events, e)
		return nil
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, events)
}

func TestLlamaGateway_CountTokens(t *testing.T) {
	srv := newLlamaTestServer(t)
	defer srv.Close()

	rawGw, err := newLlamaGateway(srv.URL, "test-api-key", srv.Client())
	require.NoError(t, err)

	// CountTokens is on the concrete type via the EmbeddingsGateway interface
	type tokenCounter interface {
		CountTokens(ctx context.Context, model string, texts []string) (int, error)
	}
	llamaGw, ok := rawGw.(tokenCounter)
	require.True(t, ok, "gateway should implement CountTokens")

	ctx := context.Background()

	t.Run("Empty texts", func(t *testing.T) {
		count, err := llamaGw.CountTokens(ctx, "llama2", []string{})
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Non-empty texts", func(t *testing.T) {
		count, err := llamaGw.CountTokens(ctx, "llama2", []string{"Hello world", "How are you?"})
		require.NoError(t, err)
		assert.Greater(t, count, 0)
	})
}
