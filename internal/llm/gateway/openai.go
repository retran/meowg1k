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
	client *openai.Client
}

// NewOpenAIGateway creates and initializes a new unified OpenAIGateway.
func NewOpenAIGateway(ctx context.Context, baseURL string, apiKey string) (*OpenAIGateway, error) {
	fmt.Print(baseURL)
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
	completion, err := g.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(request.SystemPrompt()),
			openai.UserMessage(request.UserPrompt()),
		},
		Model: request.Model(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(completion.Choices) == 0 {
		return "", errors.New("failed to generate content: no choices returned from OpenAI API")
	}

	return completion.Choices[0].Message.Content, nil
}

// ComputeEmbeddings sends a request to the OpenAI-compatible API to compute embeddings for the given text chunks.
func (g *OpenAIGateway) ComputeEmbeddings(ctx context.Context, request *ComputeEmbeddingsRequest) ([]Embedding, error) {
	return []Embedding{}, errors.New("not implemented")
}

// ComputeDistance calculates the cosine similarity between two embeddings.
// It returns a value between -1 (opposite) and 1 (identical), where 0 indicates orthogonality.
func (g *OpenAIGateway) ComputeDistance(a, b Embedding) (float64, error) {
	return 0, errors.New("not implemented")
}
