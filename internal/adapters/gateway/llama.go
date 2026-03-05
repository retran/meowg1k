// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/retran/meowg1k/internal/adapters/llama"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
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
func (g *llamaGateway) GenerateContent(ctx context.Context, request *gateway.GenerateContentRequest) (*gateway.GenerateContentResponse, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if g == nil {
		return nil, fmt.Errorf("llama gateway is nil")
	}

	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if len(request.Tools()) > 0 {
		return nil, gateway.ErrToolCallingNotSupported
	}

	return RetryWithBackoff(ctx, DefaultRetryConfig(), func(ctx context.Context) (*gateway.GenerateContentResponse, error) {
		prompt := buildLlamaPrompt(request)
		req := newLlamaCompletionRequest(prompt, request)

		response, err := g.client.Complete(ctx, req)
		if err != nil {
			return nil, err
		}

		// Extract usage information from Llama response
		var usage *gateway.UsageMetadata
		if response.TokensEvaluated > 0 || response.TokensCached > 0 {
			// Llama doesn't separate prompt/completion tokens clearly
			// TokensEvaluated is typically the total processed
			usage = &gateway.UsageMetadata{
				TotalTokens: response.TokensEvaluated,
			}
		}

		return &gateway.GenerateContentResponse{
			Blocks: []gateway.ContentBlock{{Kind: gateway.ContentBlockText, Text: response.Content}},
			Usage:  usage,
		}, nil
	}, fmt.Sprintf("Llama GenerateContent for model %q", request.Model()))
}

func buildLlamaPrompt(request *gateway.GenerateContentRequest) string {
	var promptBuilder strings.Builder

	msgs := request.Messages()
	if len(msgs) > 0 {
		for i := range msgs {
			writeLlamaMessage(&promptBuilder, &msgs[i])
		}
		promptBuilder.WriteString(assistantStart)
		return promptBuilder.String()
	}

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

func writeLlamaMessage(sb *strings.Builder, m *gateway.Message) {
	switch m.Role {
	case gateway.MessageRoleSystem:
		if strings.TrimSpace(m.Content) != "" {
			sb.WriteString(systemPromptStart)
			sb.WriteString(strings.TrimSpace(m.Content))
			sb.WriteString(systemPromptEnd)
		}
	case gateway.MessageRoleUser:
		if strings.TrimSpace(m.Content) != "" {
			sb.WriteString(userPromptStart)
			sb.WriteString(strings.TrimSpace(m.Content))
			sb.WriteString(userPromptEnd)
		}
	case gateway.MessageRoleAssistant:
		writeLlamaAssistantMessage(sb, m)
	case gateway.MessageRoleTool:
		writeLlamaToolMessage(sb, m)
	}
}

func writeLlamaAssistantMessage(sb *strings.Builder, m *gateway.Message) {
	if strings.TrimSpace(m.Content) == "" && len(m.ToolCalls) == 0 {
		return
	}
	sb.WriteString(assistantStart)
	if strings.TrimSpace(m.Content) != "" {
		sb.WriteString(strings.TrimSpace(m.Content))
		sb.WriteString("\n")
	}
	if len(m.ToolCalls) > 0 {
		sb.WriteString("ToolCalls:\n")
		for _, c := range m.ToolCalls {
			sb.WriteString("- ")
			sb.WriteString(c.Name)
			if c.ID != "" {
				sb.WriteString(" (id=")
				sb.WriteString(c.ID)
				sb.WriteString(")")
			}
			sb.WriteString("\n")
		}
	}
	sb.WriteString(systemPromptEnd)
}

func writeLlamaToolMessage(sb *strings.Builder, m *gateway.Message) {
	if strings.TrimSpace(m.Content) == "" {
		return
	}
	sb.WriteString(userPromptStart)
	sb.WriteString("ToolResult")
	if m.ToolName != "" {
		sb.WriteString(" ")
		sb.WriteString(m.ToolName)
	}
	if m.ToolCallID != "" {
		sb.WriteString(" (tool_call_id=")
		sb.WriteString(m.ToolCallID)
		sb.WriteString(")")
	}
	sb.WriteString(":\n")
	sb.WriteString(strings.TrimSpace(m.Content))
	sb.WriteString(userPromptEnd)
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
		defaultStops = append(defaultStops, stop...)
		req.Stop = defaultStops
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

// GenerateContentStream implements streaming for Llama by delegating to GenerateContent
// and synthesizing stream events from the aggregated response.
// Full native streaming via the llama.cpp streaming API will be added in a future phase.
func (g *llamaGateway) GenerateContentStream(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
	callback gateway.StreamCallback,
) (*gateway.GenerateContentResponse, error) {
	resp, err := g.GenerateContent(ctx, request)
	if err != nil {
		if callback != nil {
			_ = callback(gateway.StreamEvent{Kind: gateway.StreamEventError, Error: err.Error(), Recoverable: false})
		}
		return nil, err
	}
	return synthesizeStreamEvents(resp, callback)
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

	return RetryWithBackoff(ctx, DefaultRetryConfig(), func(ctx context.Context) ([]gateway.Embedding, error) {
		// Use batch API to process all chunks at once
		rawEmbeddings, err := g.client.EmbeddingBatch(ctx, chunks, true)
		if err != nil {
			return nil, err
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
	}, fmt.Sprintf("Llama ComputeEmbeddings for model %q", request.Model()))
}

// CountTokens estimates token count for Llama models using character-based approximation.
// Since llama.cpp doesn't provide a built-in tokenization API, we use an estimation:
// approximately 1 token per 4 characters (similar to GPT models).
func (g *llamaGateway) CountTokens(ctx context.Context, model string, texts []string) (int, error) {
	if g == nil {
		return 0, fmt.Errorf("llama gateway is nil")
	}

	if len(texts) == 0 {
		return 0, nil
	}

	// Count total characters across all texts
	totalChars := 0
	for _, text := range texts {
		totalChars += len(text)
	}

	// Estimate tokens: approximately 1 token per 4 characters
	// Using the improved formula: (chars + 2) / 3 for better accuracy
	estimatedTokens := (totalChars + 2) / 3

	return estimatedTokens, nil
}
