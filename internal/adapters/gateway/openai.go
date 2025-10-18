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

	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(request.SystemPrompt()),
			openai.UserMessage(request.UserPrompt()),
		},
		Model:     request.Model(),
		MaxTokens: openai.Int(int64(request.MaxOutputTokens())),
	}

	// Set generation parameters if provided
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

	// ResponseFormat for structured outputs
	// Note: The OpenAI SDK has different structures for ResponseFormat
	// This would need specific SDK methods or types that may vary by SDK version
	// For now, we document this as available but may need SDK-specific implementation
	if responseFormat := request.ResponseFormat(); responseFormat != nil {
		// TODO: Implement ResponseFormat based on OpenAI SDK version
		// The exact structure depends on the SDK being used
		_ = responseFormat
	}

	// ResponseSchema may be used with response_format in OpenAI
	if responseSchema := request.ResponseSchema(); responseSchema != nil {
		// TODO: Implement ResponseSchema integration with ResponseFormat
		_ = responseSchema
	}

	// Number of completions to generate
	if candidateCount := request.CandidateCount(); candidateCount != nil {
		params.N = openai.Int(int64(*candidateCount))
	}

	// LogProbs parameters
	if logProbs := request.LogProbs(); logProbs != nil && *logProbs {
		params.Logprobs = openai.Bool(true)
		if topLogProbs := request.TopLogProbs(); topLogProbs != nil {
			params.TopLogprobs = openai.Int(int64(*topLogProbs))
		}
	}

	// LogitBias for token likelihood modification
	if logitBias := request.LogitBias(); len(logitBias) > 0 {
		biasMap := make(map[string]int64)
		for k, v := range logitBias {
			biasMap[k] = int64(v)
		}
		params.LogitBias = biasMap
	}

	// ServiceTier (e.g., "auto", "default")
	if serviceTier := request.ServiceTier(); serviceTier != nil {
		params.ServiceTier = openai.ChatCompletionNewParamsServiceTier(*serviceTier)
	}

	// User identifier for tracking
	if user := request.User(); user != nil {
		params.User = openai.String(*user)
	}

	// OpenRouter-specific parameters (also supported by some other providers)
	// These will be passed through to the provider if supported
	if repetitionPenalty := request.RepetitionPenalty(); repetitionPenalty != nil {
		// Note: This is OpenRouter/Llama.cpp specific
		// The OpenAI SDK may not have direct support, but it can be passed as extra params
		_ = repetitionPenalty // TODO: Add as extra param if SDK supports it
	}

	if minP := request.MinP(); minP != nil {
		// Note: This is OpenRouter/Llama.cpp specific
		_ = minP // TODO: Add as extra param if SDK supports it
	}

	if topA := request.TopA(); topA != nil {
		// Note: This is OpenRouter specific
		_ = topA // TODO: Add as extra param if SDK supports it
	}

	response, err := g.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to generate content from OpenAI-compatible API for model %q: %w", request.Model(), err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("failed to generate content: no choices returned from OpenAI-compatible API for model %q", request.Model())
	}

	return response.Choices[0].Message.Content, nil
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
