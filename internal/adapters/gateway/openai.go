// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"fmt"
	"net/http"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

var (
	_ ports.GenerationGateway = (*openaiGateway)(nil)
	_ ports.EmbeddingsGateway = (*openaiGateway)(nil)
)

type openaiGateway struct {
	gateway.ComputeDistanceMixin
	client *openai.Client
}

// newOpenAIGateway creates and initializes a new OpenAI-compatible gateway.
// If httpClient is nil, the SDK will use its default HTTP client.
func newOpenAIGateway(baseURL, apiKey string, httpClient *http.Client) ports.Gateway {
	options := []option.RequestOption{
		option.WithBaseURL(baseURL),
	}

	if apiKey != "" {
		options = append(options, option.WithAPIKey(apiKey))
	}

	if httpClient != nil {
		options = append(options, option.WithHTTPClient(httpClient))
	}

	client := openai.NewClient(options...)

	return &openaiGateway{client: &client}
}

// GenerateContent sends a content generation request to the OpenAI-compatible API.
func (g *openaiGateway) GenerateContent(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
) (string, error) {
	if g == nil {
		return "", fmt.Errorf("openai gateway is nil")
	}

	if ctx == nil {
		return "", fmt.Errorf("context cannot be nil")
	}

	if request == nil {
		return "", fmt.Errorf("request cannot be nil")
	}

	params := buildOpenAIChatParams(request)

	response, err := g.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to generate content from OpenAI-compatible API for model %q: %w", request.Model(), err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("failed to generate content: no choices returned from OpenAI-compatible API for model %q", request.Model())
	}

	return response.Choices[0].Message.Content, nil
}

func buildOpenAIChatParams(request *gateway.GenerateContentRequest) openai.ChatCompletionNewParams {
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(request.SystemPrompt()),
			openai.UserMessage(request.UserPrompt()),
		},
		Model:     request.Model(),
		MaxTokens: openai.Int(int64(request.MaxOutputTokens())),
	}

	applyOpenAISamplingParams(&params, request)
	applyOpenAIResponseParams(&params, request)
	applyOpenAICandidateParams(&params, request)
	applyOpenAILogprobParams(&params, request)
	applyOpenAILogitBias(&params, request)
	applyOpenAISystemParams(&params, request)

	return params
}

func applyOpenAISamplingParams(params *openai.ChatCompletionNewParams, request *gateway.GenerateContentRequest) {
	if temperature := request.Temperature(); temperature != nil {
		params.Temperature = openai.Float(*temperature)
	}

	if topP := request.TopP(); topP != nil {
		params.TopP = openai.Float(*topP)
	}

	if frequencyPenalty := request.FrequencyPenalty(); frequencyPenalty != nil {
		params.FrequencyPenalty = openai.Float(*frequencyPenalty)
	}

	if presencePenalty := request.PresencePenalty(); presencePenalty != nil {
		params.PresencePenalty = openai.Float(*presencePenalty)
	}

	if seed := request.Seed(); seed != nil {
		params.Seed = openai.Int(int64(*seed))
	}

	if stop := request.Stop(); len(stop) > 0 {
		params.Stop = openai.ChatCompletionNewParamsStopUnion{
			OfStringArray: stop,
		}
	}
}

func applyOpenAIResponseParams(_ *openai.ChatCompletionNewParams, request *gateway.GenerateContentRequest) {
	if responseFormat := request.ResponseFormat(); responseFormat != nil {
		// TODO: Implement ResponseFormat based on OpenAI SDK version.
		_ = responseFormat
	}

	if responseSchema := request.ResponseSchema(); responseSchema != nil {
		// TODO: Implement ResponseSchema integration with ResponseFormat.
		_ = responseSchema
	}
}

func applyOpenAICandidateParams(params *openai.ChatCompletionNewParams, request *gateway.GenerateContentRequest) {
	if candidateCount := request.CandidateCount(); candidateCount != nil {
		params.N = openai.Int(int64(*candidateCount))
	}
}

func applyOpenAILogprobParams(params *openai.ChatCompletionNewParams, request *gateway.GenerateContentRequest) {
	if logProbs := request.LogProbs(); logProbs != nil && *logProbs {
		params.Logprobs = openai.Bool(true)
		if topLogProbs := request.TopLogProbs(); topLogProbs != nil {
			params.TopLogprobs = openai.Int(int64(*topLogProbs))
		}
	}
}

func applyOpenAILogitBias(params *openai.ChatCompletionNewParams, request *gateway.GenerateContentRequest) {
	if logitBias := request.LogitBias(); len(logitBias) > 0 {
		biasMap := make(map[string]int64)
		for k, v := range logitBias {
			biasMap[k] = int64(v)
		}
		params.LogitBias = biasMap
	}
}

func applyOpenAISystemParams(params *openai.ChatCompletionNewParams, request *gateway.GenerateContentRequest) {
	if serviceTier := request.ServiceTier(); serviceTier != nil {
		params.ServiceTier = openai.ChatCompletionNewParamsServiceTier(*serviceTier)
	}

	if user := request.User(); user != nil {
		params.User = openai.String(*user)
	}

	if repetitionPenalty := request.RepetitionPenalty(); repetitionPenalty != nil {
		// TODO: Add as extra param if SDK supports it.
		_ = repetitionPenalty
	}

	if minP := request.MinP(); minP != nil {
		_ = minP // TODO: Add as extra param if SDK supports it.
	}

	if topA := request.TopA(); topA != nil {
		_ = topA // TODO: Add as extra param if SDK supports it.
	}
}

// ComputeEmbeddings sends a request to the OpenAI-compatible API to compute embeddings for the given text chunks.
func (g *openaiGateway) ComputeEmbeddings(
	ctx context.Context,
	request *gateway.ComputeEmbeddingsRequest,
) ([]gateway.Embedding, error) {
	if g == nil {
		return nil, fmt.Errorf("openai gateway is nil")
	}

	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	params := openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: request.Chunks(),
		},
		Model: request.Model(),
	}

	if request.Dimensions() > 0 {
		params.Dimensions = openai.Int(int64(request.Dimensions()))
	}

	response, err := g.client.Embeddings.New(ctx, params)
	if err != nil {
		return []gateway.Embedding{}, fmt.Errorf("failed to compute embeddings from OpenAI-compatible API for model %q: %w", request.Model(), err)
	}

	embeddings := make([]gateway.Embedding, 0, len(response.Data))
	for i := range response.Data {
		embeddings = append(embeddings, response.Data[i].Embedding)
	}

	return embeddings, nil
}
