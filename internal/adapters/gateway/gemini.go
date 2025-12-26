// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

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

	generationConfig := buildGeminiGenerationConfig(request)

	userPrompt := genai.Text(request.UserPrompt())

	result, err := g.client.Models.GenerateContent(ctx, request.Model(), userPrompt, generationConfig)
	if err != nil {
		return "", fmt.Errorf("failed to fetch response from Gemini API for model %q: %w", request.Model(), err)
	}

	if err := validateGeminiResponse(result, request.Model()); err != nil {
		return "", err
	}

	return result.Text(), nil
}

// GenerateContentWithTools sends a content generation request with tool definitions.
func (g *geminiGateway) GenerateContentWithTools(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
	tools []gateway.ToolDefinition,
) (*gateway.GenerateContentResponse, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if g == nil {
		return nil, fmt.Errorf("gemini gateway is nil")
	}

	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if len(tools) == 0 {
		return nil, gateway.ErrToolCallingNotSupported
	}

	config := buildGeminiGenerationConfig(request)
	config.Tools = buildGeminiTools(tools)
	config.ToolConfig = &genai.ToolConfig{
		FunctionCallingConfig: &genai.FunctionCallingConfig{
			Mode: genai.FunctionCallingConfigModeAuto,
		},
	}

	userPrompt := genai.Text(request.UserPrompt())

	result, err := g.client.Models.GenerateContent(ctx, request.Model(), userPrompt, config)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch response from Gemini API for model %q: %w", request.Model(), err)
	}

	if err := validateGeminiResponse(result, request.Model()); err != nil {
		return nil, err
	}

	return &gateway.GenerateContentResponse{
		Content:   result.Text(),
		ToolCalls: mapGeminiToolCalls(result.FunctionCalls()),
	}, nil
}

func buildGeminiGenerationConfig(request *gateway.GenerateContentRequest) *genai.GenerateContentConfig {
	config := &genai.GenerateContentConfig{}

	applyGeminiSystemPrompt(config, request)
	applyGeminiSamplingConfig(config, request)
	applyGeminiPenaltyConfig(config, request)
	applyGeminiResponseConfig(config, request)
	applyGeminiCandidateConfig(config, request)
	applyGeminiLogprobConfig(config, request)

	return config
}

func buildGeminiTools(tools []gateway.ToolDefinition) []*genai.Tool {
	if len(tools) == 0 {
		return nil
	}

	functions := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		functions = append(functions, &genai.FunctionDeclaration{
			Name:                 tool.Name,
			Description:          tool.Description,
			ParametersJsonSchema: tool.Parameters,
		})
	}

	return []*genai.Tool{
		{
			FunctionDeclarations: functions,
		},
	}
}

func mapGeminiToolCalls(calls []*genai.FunctionCall) []gateway.ToolCall {
	if len(calls) == 0 {
		return nil
	}

	result := make([]gateway.ToolCall, 0, len(calls))
	for _, call := range calls {
		if call == nil {
			continue
		}
		result = append(result, gateway.ToolCall{
			ID:        call.ID,
			Name:      call.Name,
			Arguments: call.Args,
		})
	}
	return result
}

func applyGeminiSystemPrompt(config *genai.GenerateContentConfig, request *gateway.GenerateContentRequest) {
	if request.SystemPrompt() == "" {
		return
	}

	parts := genai.Text(request.SystemPrompt())
	if len(parts) > 0 {
		config.SystemInstruction = parts[0]
	}
}

func applyGeminiSamplingConfig(config *genai.GenerateContentConfig, request *gateway.GenerateContentRequest) {
	if temperature := request.Temperature(); temperature != nil {
		temp := float32(*temperature)
		config.Temperature = &temp
	}

	if topP := request.TopP(); topP != nil {
		p := float32(*topP)
		config.TopP = &p
	}

	if topK := request.TopK(); topK != nil {
		k := float32(*topK)
		config.TopK = &k
	}

	if maxTokens := request.MaxOutputTokens(); maxTokens > 0 {
		config.MaxOutputTokens = clampToInt32(maxTokens)
	}

	if seed := request.Seed(); seed != nil {
		config.Seed = toInt32Pointer(*seed)
	}

	if stop := request.Stop(); len(stop) > 0 {
		config.StopSequences = stop
	}
}

func applyGeminiPenaltyConfig(config *genai.GenerateContentConfig, request *gateway.GenerateContentRequest) {
	if frequencyPenalty := request.FrequencyPenalty(); frequencyPenalty != nil {
		fp := float32(*frequencyPenalty)
		config.FrequencyPenalty = &fp
	}

	if presencePenalty := request.PresencePenalty(); presencePenalty != nil {
		pp := float32(*presencePenalty)
		config.PresencePenalty = &pp
	}
}

func applyGeminiResponseConfig(config *genai.GenerateContentConfig, request *gateway.GenerateContentRequest) {
	if responseFormat := request.ResponseFormat(); responseFormat != nil {
		switch *responseFormat {
		case "json_object", "json_schema":
			config.ResponseMIMEType = "application/json"
		case "text":
			config.ResponseMIMEType = "text/plain"
		}
	}

	if responseSchema := request.ResponseSchema(); responseSchema != nil {
		// TODO: Convert map[string]interface{} to *genai.Schema
		_ = responseSchema
	}
}

func applyGeminiCandidateConfig(config *genai.GenerateContentConfig, request *gateway.GenerateContentRequest) {
	if candidateCount := request.CandidateCount(); candidateCount != nil {
		count := *candidateCount
		if count < 0 {
			count = 0
		}
		config.CandidateCount = clampToInt32(count)
	}
}

func applyGeminiLogprobConfig(config *genai.GenerateContentConfig, request *gateway.GenerateContentRequest) {
	if logProbs := request.LogProbs(); logProbs != nil {
		config.ResponseLogprobs = *logProbs
	}

	if topLogProbs := request.TopLogProbs(); topLogProbs != nil {
		count := *topLogProbs
		if count < 0 {
			count = 0
		}
		config.Logprobs = toInt32Pointer(count)
	}
}

func clampToInt32(value int) int32 {
	if value > math.MaxInt32 {
		return math.MaxInt32
	}
	if value < math.MinInt32 {
		return math.MinInt32
	}
	return int32(value)
}

func toInt32Pointer(value int) *int32 {
	clamped := clampToInt32(value)
	return &clamped
}

func validateGeminiResponse(result *genai.GenerateContentResponse, model string) error {
	if len(result.Candidates) > 0 && result.Candidates[0].FinishReason != genai.FinishReasonStop &&
		result.Candidates[0].FinishReason != genai.FinishReasonMaxTokens {
		return fmt.Errorf("generation stopped for model %q with reason: %s", model, result.Candidates[0].FinishReason)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		if result.PromptFeedback != nil && result.PromptFeedback.BlockReason != genai.BlockedReasonUnspecified {
			return fmt.Errorf("request was blocked by Gemini API for model %q with reason: %s", model, result.PromptFeedback.BlockReason)
		}

		return fmt.Errorf("gemini API returned an empty response for model %q", model)
	}

	return nil
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
