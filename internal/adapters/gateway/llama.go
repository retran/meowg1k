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
	"net/http"
	"strings"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/llama"
)

var _ ports.GenerationGateway = (*llamaGateway)(nil)
var _ ports.EmbeddingsGateway = (*llamaGateway)(nil)

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

	prompt := promptBuilder.String()

	req := &llama.CompletionRequest{
		Prompt:   prompt,
		NPredict: -1,
	}

	// Set stop sequences - merge defaults with user-provided ones
	defaultStops := []string{"<|endoftext|>", "<|im_end|>"}
	if stop := request.Stop(); len(stop) > 0 {
		req.Stop = append(defaultStops, stop...)
	} else {
		req.Stop = defaultStops
	}

	// Set generation parameters if provided, otherwise use defaults
	if temperature := request.Temperature(); temperature != nil {
		req.Temperature = *temperature
	} else {
		req.Temperature = 0.8 // default
	}

	if topK := request.TopK(); topK != nil {
		req.TopK = *topK
	} else {
		req.TopK = 40 // default
	}

	if topP := request.TopP(); topP != nil {
		req.TopP = *topP
	} else {
		req.TopP = 0.95 // default
	}

	if frequencyPenalty := request.FrequencyPenalty(); frequencyPenalty != nil {
		req.FrequencyPenalty = *frequencyPenalty
	}

	if presencePenalty := request.PresencePenalty(); presencePenalty != nil {
		req.PresencePenalty = *presencePenalty
	}

	if seed := request.Seed(); seed != nil {
		req.Seed = *seed
	}

	// OpenRouter/Llama.cpp specific parameters
	if repetitionPenalty := request.RepetitionPenalty(); repetitionPenalty != nil {
		req.RepeatPenalty = *repetitionPenalty
	}

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

	// Use JSONSchema if ResponseSchema is provided
	if responseSchema := request.ResponseSchema(); responseSchema != nil {
		req.JSONSchema = responseSchema
	}

	// Use LogitBias if provided
	if logitBias := request.LogitBias(); len(logitBias) > 0 {
		req.LogitBias = logitBias
	}

	// Use NProbs for log probabilities if requested
	if logProbs := request.LogProbs(); logProbs != nil && *logProbs {
		if topLogProbs := request.TopLogProbs(); topLogProbs != nil {
			req.NProbs = *topLogProbs
		} else {
			req.NProbs = 5 // default
		}
	}

	// Note: Some parameters are not supported by llama.cpp completion API:
	// - TopA: Not directly supported by llama.cpp
	// - ResponseFormat: Would need custom implementation (but JSONSchema is supported)
	// - CandidateCount: llama.cpp returns single completion
	// - ServiceTier: Not applicable for local models
	// - User: Not applicable for local models

	response, err := g.client.Complete(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to generate content from local LLM API: %w", err)
	}

	return response.Content, nil
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
