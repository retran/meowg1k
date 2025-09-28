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
	"math"

	"google.golang.org/genai"

	mdGateway "github.com/retran/meowg1k/internal/models/gateway"
)

var (
	_ GenerationGateway = (*geminiGateway)(nil)
	_ EmbeddingsGateway = (*geminiGateway)(nil)
)

var (
	ErrGenerationStopped        = errors.New("generation stopped for reason")
	ErrRequestBlocked           = errors.New("request was blocked by the API")
	ErrEmptyResponse            = errors.New("gemini API returned an empty response")
	ErrDimensionsOutOfRange     = errors.New("dimensions value exceeds int32 range")
	ErrFailedToCreateClient     = errors.New("failed to create Gemini client")
	ErrFailedToFetchResponse    = errors.New("failed to fetch response from Gemini API")
	ErrFailedToComputeEmbedding = errors.New("failed to compute embedding")
)

// geminiGateway is a unified client for the Google Gemini API,
// implementing both GenerationGateway and EmbeddingGateway.
type geminiGateway struct {
	ComputeDistanceMixin
	client *genai.Client
}

// NewGeminiGateway creates and initializes a new unified GeminiGateway.
func newGeminiGateway(ctx context.Context, apiKey string) (Gateway, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToCreateClient, err)
	}

	return &geminiGateway{
		client: client,
	}, nil
}

// GenerateContent sends a content generation request to the Google Gemini API.
func (g *geminiGateway) GenerateContent(
	ctx context.Context,
	request *mdGateway.GenerateContentRequest,
) (string, error) {
	generationConfig := &genai.GenerateContentConfig{}

	if request.SystemPrompt() != "" {
		parts := genai.Text(request.SystemPrompt())
		if len(parts) > 0 {
			generationConfig.SystemInstruction = parts[0]
		}
	}

	userPrompt := genai.Text(request.UserPrompt())

	result, err := g.client.Models.GenerateContent(ctx, request.Model(), userPrompt, generationConfig)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrFailedToFetchResponse, err)
	}

	if len(result.Candidates) > 0 && result.Candidates[0].FinishReason != genai.FinishReasonStop &&
		result.Candidates[0].FinishReason != genai.FinishReasonMaxTokens {
		return "", fmt.Errorf("%w: %s", ErrGenerationStopped, result.Candidates[0].FinishReason)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		if result.PromptFeedback != nil && result.PromptFeedback.BlockReason != genai.BlockedReasonUnspecified {
			return "", fmt.Errorf("%w: %s", ErrRequestBlocked, result.PromptFeedback.BlockReason)
		}

		return "", ErrEmptyResponse
	}

	return result.Text(), nil
}

// ComputeEmbeddings sends a request to the Google Gemini API to compute embeddings for the given text chunks.
func (g *geminiGateway) ComputeEmbeddings(
	ctx context.Context,
	request *mdGateway.ComputeEmbeddingsRequest,
) ([]mdGateway.Embedding, error) {
	contents := make([]*genai.Content, 0, len(request.Chunks()))
	for _, value := range request.Chunks() {
		contents = append(contents, genai.NewContentFromText(value, genai.RoleUser))
	}

	config := &genai.EmbedContentConfig{
		TaskType: string(request.TaskType()),
	}

	// Set output dimensionality if specified
	if request.Dimensions() > 0 {
		dimensions := request.Dimensions()
		// Check for integer overflow when converting to int32
		if dimensions > math.MaxInt32 {
			return nil, fmt.Errorf("%w: %d", ErrDimensionsOutOfRange, dimensions)
		}

		dims := int32(dimensions) // #nosec G115 // overflow checked above
		config.OutputDimensionality = &dims
	}

	response, err := g.client.Models.EmbedContent(ctx,
		request.Model(),
		contents,
		config,
	)
	if err != nil {
		return []mdGateway.Embedding{}, fmt.Errorf("%w: %w", ErrFailedToComputeEmbedding, err)
	}

	embeddings := make([]mdGateway.Embedding, 0, len(response.Embeddings))

	for _, value := range response.Embeddings {
		values := make([]float64, len(value.Values))
		for i, v := range value.Values {
			values[i] = float64(v)
		}

		embeddings = append(embeddings, mdGateway.Embedding(values))
	}

	return embeddings, nil
}
