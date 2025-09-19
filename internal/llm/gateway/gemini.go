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
	"math"

	"google.golang.org/genai"
)

// Compile-time checks to ensure GeminiGateway implements both interfaces.
var _ GenerationGateway = (*GeminiGateway)(nil)
var _ EmbeddingGateway = (*GeminiGateway)(nil)

// GeminiGateway is a unified client for the Google Gemini API,
// implementing both GenerationGateway and EmbeddingGateway.
type GeminiGateway struct {
	client *genai.Client
}

// NewGeminiGateway creates and initializes a new unified GeminiGateway.
func NewGeminiGateway(ctx context.Context, apiKey string) (*GeminiGateway, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &GeminiGateway{
		client: client,
	}, nil
}

// GenerateContent sends a content generation request to the Google Gemini API.
func (g *GeminiGateway) GenerateContent(ctx context.Context, request *GenerateContentRequest) (string, error) {
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
		return "", fmt.Errorf("failed to fetch response from Gemini API: %w", err)
	}

	if len(result.Candidates) > 0 && result.Candidates[0].FinishReason != genai.FinishReasonStop &&
		result.Candidates[0].FinishReason != genai.FinishReasonMaxTokens {
		return "", fmt.Errorf("generation stopped for reason: %s", result.Candidates[0].FinishReason)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		if result.PromptFeedback != nil && result.PromptFeedback.BlockReason != genai.BlockedReasonUnspecified {
			return "", fmt.Errorf(
				"request was blocked by the API for reason: %s",
				result.PromptFeedback.BlockReason,
			)
		}
		return "", fmt.Errorf("gemini API returned an empty response")
	}

	return result.Text(), nil
}

// ComputeEmbeddings sends a request to the Google Gemini API to compute embeddings for the given text chunks.
func (g *GeminiGateway) ComputeEmbeddings(ctx context.Context, request *ComputeEmbeddingsRequest) ([]Embedding, error) {
	var contents []*genai.Content
	for _, value := range request.Chunks() {
		contents = append(contents, genai.NewContentFromText(value, genai.RoleUser))
	}

	response, err := g.client.Models.EmbedContent(ctx,
		request.Model(),
		contents,
		&genai.EmbedContentConfig{
			TaskType: string(request.TaskType()),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to compute embedding: %w", err)
	}

	embeddings := make([]Embedding, 0, len(response.Embeddings))
	for _, value := range response.Embeddings {
		embeddings = append(embeddings, Embedding(value.Values))
	}

	return embeddings, nil
}

// ComputeDistance calculates the cosine similarity between two embeddings.
// It returns a value between -1 (opposite) and 1 (identical), where 0 indicates orthogonality.
func (g *GeminiGateway) ComputeDistance(a, b Embedding) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("vectors must have the same length")
	}

	var dotProduct, aMagnitude, bMagnitude float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		aMagnitude += float64(a[i]) * float64(a[i])
		bMagnitude += float64(b[i]) * float64(b[i])
	}

	if aMagnitude == 0 || bMagnitude == 0 {
		return 0, nil
	}

	return dotProduct / (math.Sqrt(aMagnitude) * math.Sqrt(bMagnitude)), nil
}
