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

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

var (
	_ ports.GenerationGateway = (*geminiGateway)(nil)
	_ ports.EmbeddingsGateway = (*geminiGateway)(nil)
)

// geminiGateway is a unified client for the Google Gemini API,
// implementing both GenerationGateway and EmbeddingGateway.
type geminiGateway struct {
	gateway.ComputeDistanceMixin
	client *genai.Client
}

// NewGeminiGateway creates and initializes a new unified GeminiGateway.
func newGeminiGateway(ctx context.Context, apiKey string) (ports.Gateway, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &geminiGateway{
		client: client,
	}, nil
}

// GenerateContent sends a content generation request to the Google Gemini API.
func (g *geminiGateway) GenerateContent(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("context cannot be nil")
	}

	if g == nil {
		return "", fmt.Errorf("gemini gateway is nil")
	}

	if request == nil {
		return "", fmt.Errorf("request cannot be nil")
	}

	generationConfig := &genai.GenerateContentConfig{}

	if request.SystemPrompt() != "" {
		parts := genai.Text(request.SystemPrompt())
		if len(parts) > 0 {
			generationConfig.SystemInstruction = parts[0]
		}
	}

	// Set generation parameters if provided
	if temperature := request.Temperature(); temperature != nil {
		temp := float32(*temperature)
		generationConfig.Temperature = &temp
	}

	if topP := request.TopP(); topP != nil {
		p := float32(*topP)
		generationConfig.TopP = &p
	}

	if topK := request.TopK(); topK != nil {
		k := float32(*topK)
		generationConfig.TopK = &k
	}

	if maxTokens := request.MaxOutputTokens(); maxTokens > 0 {
		if maxTokens > 2147483647 { // int32 max value
			maxTokens = 2147483647
		}
		generationConfig.MaxOutputTokens = int32(maxTokens) // #nosec G115 -- range checked above
	}

	if frequencyPenalty := request.FrequencyPenalty(); frequencyPenalty != nil {
		fp := float32(*frequencyPenalty)
		generationConfig.FrequencyPenalty = &fp
	}

	if presencePenalty := request.PresencePenalty(); presencePenalty != nil {
		pp := float32(*presencePenalty)
		generationConfig.PresencePenalty = &pp
	}

	if seed := request.Seed(); seed != nil {
		seedVal := *seed
		if seedVal > 2147483647 { // int32 max value
			seedVal = 2147483647
		}
		if seedVal < -2147483648 { // int32 min value
			seedVal = -2147483648
		}
		s := int32(seedVal) // #nosec G115 -- range checked above
		generationConfig.Seed = &s
	}

	if stop := request.Stop(); len(stop) > 0 {
		generationConfig.StopSequences = stop
	}

	// ResponseFormat maps to ResponseMIMEType in Gemini API
	if responseFormat := request.ResponseFormat(); responseFormat != nil {
		switch *responseFormat {
		case "json_object", "json_schema":
			generationConfig.ResponseMIMEType = "application/json"
		case "text":
			generationConfig.ResponseMIMEType = "text/plain"
		}
	}

	// ResponseSchema is directly supported by Gemini
	// Note: Would need type conversion from map[string]interface{} to *genai.Schema
	// This is complex and may require additional helper functions
	if responseSchema := request.ResponseSchema(); responseSchema != nil {
		// TODO: Convert map[string]interface{} to *genai.Schema
		// For now, this parameter will need to be set manually or through a helper
		_ = responseSchema
	}

	// CandidateCount controls number of responses
	if candidateCount := request.CandidateCount(); candidateCount != nil {
		count := *candidateCount
		if count > 2147483647 { // int32 max value
			count = 2147483647
		}
		if count < 0 {
			count = 0
		}
		generationConfig.CandidateCount = int32(count) // #nosec G115 -- range checked above
	}

	// LogProbs control
	if logProbs := request.LogProbs(); logProbs != nil {
		generationConfig.ResponseLogprobs = *logProbs
	}

	// TopLogProbs maps to Logprobs in Gemini
	if topLogProbs := request.TopLogProbs(); topLogProbs != nil {
		logProbs := *topLogProbs
		if logProbs > 2147483647 { // int32 max value
			logProbs = 2147483647
		}
		if logProbs < 0 {
			logProbs = 0
		}
		lp := int32(logProbs) // #nosec G115 -- range checked above
		generationConfig.Logprobs = &lp
	}

	userPrompt := genai.Text(request.UserPrompt())

	result, err := g.client.Models.GenerateContent(ctx, request.Model(), userPrompt, generationConfig)
	if err != nil {
		return "", fmt.Errorf("failed to fetch response from Gemini API for model %q: %w", request.Model(), err)
	}

	if len(result.Candidates) > 0 && result.Candidates[0].FinishReason != genai.FinishReasonStop &&
		result.Candidates[0].FinishReason != genai.FinishReasonMaxTokens {
		return "", fmt.Errorf("generation stopped for model %q with reason: %s", request.Model(), result.Candidates[0].FinishReason)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		if result.PromptFeedback != nil && result.PromptFeedback.BlockReason != genai.BlockedReasonUnspecified {
			return "", fmt.Errorf("request was blocked by Gemini API for model %q with reason: %s", request.Model(), result.PromptFeedback.BlockReason)
		}

		return "", fmt.Errorf("gemini API returned an empty response for model %q", request.Model())
	}

	return result.Text(), nil
}

// ComputeEmbeddings sends a request to the Google Gemini API to compute embeddings for the given text chunks.
func (g *geminiGateway) ComputeEmbeddings(
	ctx context.Context,
	request *gateway.ComputeEmbeddingsRequest,
) ([]gateway.Embedding, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if g == nil {
		return nil, fmt.Errorf("gemini gateway is nil")
	}

	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	contents := make([]*genai.Content, 0, len(request.Chunks()))
	for _, value := range request.Chunks() {
		contents = append(contents, genai.NewContentFromText(value, genai.RoleUser))
	}

	config := &genai.EmbedContentConfig{
		TaskType: string(request.TaskType()),
	}

	if request.Dimensions() > 0 {
		dimensions := request.Dimensions()
		if dimensions > math.MaxInt32 {
			return nil, fmt.Errorf("dimensions value %d exceeds int32 range for model %q", dimensions, request.Model())
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
		return []gateway.Embedding{}, fmt.Errorf("failed to compute embeddings from Gemini API for model %q: %w", request.Model(), err)
	}

	embeddings := make([]gateway.Embedding, 0, len(response.Embeddings))

	for _, value := range response.Embeddings {
		values := make([]float64, len(value.Values))
		for i, v := range value.Values {
			values[i] = float64(v)
		}

		embeddings = append(embeddings, values)
	}

	return embeddings, nil
}
