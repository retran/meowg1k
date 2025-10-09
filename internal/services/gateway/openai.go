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

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"

	"github.com/retran/meowg1k/internal/core/gateway"
)

var (
	_ GenerationGateway = (*openaiGateway)(nil)
	_ EmbeddingsGateway = (*openaiGateway)(nil)
)

type openaiGateway struct {
	gateway.ComputeDistanceMixin
	client *openai.Client
}

// newOpenAIGateway creates and initializes a new OpenAI-compatible gateway.
func newOpenAIGateway(baseURL, apiKey string) Gateway {
	options := []option.RequestOption{
		option.WithBaseURL(baseURL),
	}

	if apiKey != "" {
		options = append(options, option.WithAPIKey(apiKey))
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

	response, err := g.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(request.SystemPrompt()),
			openai.UserMessage(request.UserPrompt()),
		},
		Model:     request.Model(),
		MaxTokens: openai.Int(int64(request.MaxOutputTokens())),
	})
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
