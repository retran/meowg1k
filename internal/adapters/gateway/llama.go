// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/llama"
)

var (
	_ ports.GenerationGateway = (*llamaGateway)(nil)
	_ ports.EmbeddingsGateway = (*llamaGateway)(nil)
)

// llamaGateway is a unified client for a local LLM server compatible with the llama.cpp API.
type llamaGateway struct {
	gateway.ComputeDistanceMixin
	client *llama.Client
}

const (
	systemPromptStart = "<|im_start|>system\n"
	systemPromptEnd   = "<|im_end|>\n"
	userPromptStart   = "<|im_start|>user\n"
	userPromptEnd     = "<|im_end|>\n"
	assistantStart    = "<|im_start|>assistant\n"
)

// newLlamaGateway creates and initializes a new LlamaGateway with a shared HTTP client.
// The HTTP client is provided via dependency injection to allow for better resource management
// and connection pooling across multiple gateway instances.
func newLlamaGateway(baseURL, apiKey string, httpClient *http.Client) (ports.GenerationGateway, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("base URL is required for llama gateway")
	}

	if httpClient == nil {
		return nil, fmt.Errorf("HTTP client is required for llama gateway")
	}

	client, err := llama.NewClient(baseURL, apiKey, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create llama client with base URL %q: %w", baseURL, err)
	}

	return &llamaGateway{
		client: client,
	}, nil
}

// GenerateContent sends a content generation request to the local LLM server.
func (g *llamaGateway) GenerateContent(ctx context.Context, request *gateway.GenerateContentRequest) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("context cannot be nil")
	}

	if g == nil {
		return "", fmt.Errorf("llama gateway is nil")
	}

	if request == nil {
		return "", fmt.Errorf("request cannot be nil")
	}

	prompt := buildLlamaPrompt(request)
	req := newLlamaCompletionRequest(prompt, request)

	response, err := g.client.Complete(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to generate content from local LLM API: %w", err)
	}

	return response.Content, nil
}

func buildLlamaPrompt(request *gateway.GenerateContentRequest) string {
	var promptBuilder strings.Builder

	if request.SystemPrompt() != "" {
		promptBuilder.WriteString(systemPromptStart)
		promptBuilder.WriteString(request.SystemPrompt())
		promptBuilder.WriteString(systemPromptEnd)
	}

	promptBuilder.WriteString(userPromptStart)
	promptBuilder.WriteString(request.UserPrompt())
	promptBuilder.WriteString(userPromptEnd)
	promptBuilder.WriteString(assistantStart)

	return promptBuilder.String()
}

func newLlamaCompletionRequest(prompt string, request *gateway.GenerateContentRequest) *llama.CompletionRequest {
	req := &llama.CompletionRequest{
		Prompt:   prompt,
		NPredict: -1,
	}

	applyLlamaStopSequences(req, request)
	applyLlamaSamplingParams(req, request)
	applyLlamaPenalties(req, request)
	applyLlamaAdvancedParams(req, request)
	applyLlamaResponseParams(req, request)
	applyLlamaLogprobs(req, request)

	return req
}

func applyLlamaStopSequences(req *llama.CompletionRequest, request *gateway.GenerateContentRequest) {
	defaultStops := []string{"<|endoftext|>", "<|im_end|>"}
	if stop := request.Stop(); len(stop) > 0 {
		req.Stop = append(defaultStops, stop...)
		return
	}
	req.Stop = defaultStops
}

func applyLlamaSamplingParams(req *llama.CompletionRequest, request *gateway.GenerateContentRequest) {
	if temperature := request.Temperature(); temperature != nil {
		req.Temperature = *temperature
	} else {
		req.Temperature = 0.8
	}

	if topK := request.TopK(); topK != nil {
		req.TopK = *topK
	} else {
		req.TopK = 40
	}

	if topP := request.TopP(); topP != nil {
		req.TopP = *topP
	} else {
		req.TopP = 0.95
	}

	if seed := request.Seed(); seed != nil {
		req.Seed = *seed
	}
}

func applyLlamaPenalties(req *llama.CompletionRequest, request *gateway.GenerateContentRequest) {
	if frequencyPenalty := request.FrequencyPenalty(); frequencyPenalty != nil {
		req.FrequencyPenalty = *frequencyPenalty
	}

	if presencePenalty := request.PresencePenalty(); presencePenalty != nil {
		req.PresencePenalty = *presencePenalty
	}

	if repetitionPenalty := request.RepetitionPenalty(); repetitionPenalty != nil {
		req.RepeatPenalty = *repetitionPenalty
	}
}

func applyLlamaAdvancedParams(req *llama.CompletionRequest, request *gateway.GenerateContentRequest) {
	if minP := request.MinP(); minP != nil {
		req.MinP = *minP
	}

	if typicalP := request.TypicalP(); typicalP != nil {
		req.TypicalP = *typicalP
	}

	if mirostat := request.Mirostat(); mirostat != nil {
		req.Mirostat = *mirostat
	}

	if mirostatTau := request.MirostatTau(); mirostatTau != nil {
		req.MirostatTau = *mirostatTau
	}

	if mirostatEta := request.MirostatEta(); mirostatEta != nil {
		req.MirostatEta = *mirostatEta
	}

	if grammar := request.Grammar(); grammar != nil {
		req.Grammar = *grammar
	}
}

func applyLlamaResponseParams(req *llama.CompletionRequest, request *gateway.GenerateContentRequest) {
	if responseSchema := request.ResponseSchema(); responseSchema != nil {
		req.JSONSchema = responseSchema
	}

	if logitBias := request.LogitBias(); len(logitBias) > 0 {
		req.LogitBias = logitBias
	}
}

func applyLlamaLogprobs(req *llama.CompletionRequest, request *gateway.GenerateContentRequest) {
	if logProbs := request.LogProbs(); logProbs != nil && *logProbs {
		if topLogProbs := request.TopLogProbs(); topLogProbs != nil {
			req.NProbs = *topLogProbs
		} else {
			req.NProbs = 5
		}
	}
}

// ComputeEmbeddings computes embeddings for the provided chunks using the llama.cpp /embedding endpoint.
// Uses batch processing to send all chunks in a single request.
func (g *llamaGateway) ComputeEmbeddings(ctx context.Context, request *gateway.ComputeEmbeddingsRequest) ([]gateway.Embedding, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if g == nil {
		return nil, fmt.Errorf("llama gateway is nil")
	}

	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	chunks := request.Chunks()
	if len(chunks) == 0 {
		return []gateway.Embedding{}, nil
	}

	// Use batch API to process all chunks at once
	rawEmbeddings, err := g.client.EmbeddingBatch(ctx, chunks, true)
	if err != nil {
		return nil, fmt.Errorf("failed to compute embeddings from llama.cpp API for model %q: %w", request.Model(), err)
	}

	if len(rawEmbeddings) != len(chunks) {
		return nil, fmt.Errorf("expected %d embeddings but got %d from llama.cpp API", len(chunks), len(rawEmbeddings))
	}

	// Convert [][]float64 to []gateway.Embedding
	embeddings := make([]gateway.Embedding, len(rawEmbeddings))
	for i, emb := range rawEmbeddings {
		embeddings[i] = gateway.Embedding(emb)
	}

	return embeddings, nil
}
