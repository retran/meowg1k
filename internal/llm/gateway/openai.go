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
	"errors"
	"fmt"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

// Compile-time checks to ensure OpenAIGateway implements both interfaces.
var _ GenerationGateway = (*OpenAIGateway)(nil)
var _ EmbeddingGateway = (*OpenAIGateway)(nil)

type OpenAIGateway struct {
	ComputeDistanceMixin
	client *openai.Client
}

// NewOpenAIGateway creates and initializes a new unified OpenAIGateway.
func NewOpenAIGateway(baseURL string, apiKey string) (*OpenAIGateway, error) {
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)

	return &OpenAIGateway{
		client: &client,
	}, nil
}

// GenerateContent sends a content generation request to the OpenAI-compatible API.
func (g *OpenAIGateway) GenerateContent(ctx context.Context, request *GenerateContentRequest) (string, error) {
	response, err := g.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(request.SystemPrompt()),
			openai.UserMessage(request.UserPrompt()),
		},
		Model: request.Model(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", errors.New("failed to generate content: no choices returned from OpenAI-compatible API")
	}

	return response.Choices[0].Message.Content, nil
}

// ComputeEmbeddings sends a request to the OpenAI-compatible API to compute embeddings for the given text chunks.
func (g *OpenAIGateway) ComputeEmbeddings(ctx context.Context, request *ComputeEmbeddingsRequest) ([]Embedding, error) {
	params := openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: request.chunks,
		},
		Model: request.model,
	}

	// Set dimensions if specified
	if request.Dimensions() > 0 {
		params.Dimensions = openai.Int(int64(request.Dimensions()))
	}

	response, err := g.client.Embeddings.New(ctx, params)

	if err != nil {
		return []Embedding{}, fmt.Errorf("failed to compute embedding: %w", err)
	}

	embeddings := make([]Embedding, 0, len(response.Data))
	for _, value := range response.Data {
		embeddings = append(embeddings, Embedding(value.Embedding))
	}

	return embeddings, nil
}
