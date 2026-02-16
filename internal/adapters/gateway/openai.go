// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/openai/openai-go/v2/shared"
	"github.com/openai/openai-go/v2/shared/constant"
	tiktoken "github.com/pkoukk/tiktoken-go"

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
) (*gateway.GenerateContentResponse, error) {
	if g == nil {
		return nil, fmt.Errorf("openai gateway is nil")
	}

	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	return RetryWithBackoff(ctx, DefaultRetryConfig(), func(ctx context.Context) (*gateway.GenerateContentResponse, error) {
		params := buildOpenAIChatParams(request)
		if tools := request.Tools(); len(tools) > 0 {
			params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{OfAuto: openai.String("auto")}
			params.Tools = buildOpenAITools(tools)
		}

		response, err := g.client.Chat.Completions.New(ctx, params)
		if err != nil {
			return nil, err
		}

		if len(response.Choices) == 0 {
			return nil, fmt.Errorf("no choices returned from OpenAI-compatible API for model %q", request.Model())
		}

		msg := response.Choices[0].Message
		toolCalls := mapOpenAIToolCalls(msg.ToolCalls)

		blocks := make([]gateway.ContentBlock, 0, 1+len(toolCalls))
		if msg.Content != "" {
			blocks = append(blocks, gateway.ContentBlock{Kind: gateway.ContentBlockText, Text: msg.Content})
		}
		for i := range toolCalls {
			call := toolCalls[i]
			blocks = append(blocks, gateway.ContentBlock{Kind: gateway.ContentBlockToolCall, ToolCall: &call})
		}

		// Extract usage information
		var usage *gateway.UsageMetadata
		if response.Usage.TotalTokens > 0 {
			usage = &gateway.UsageMetadata{
				PromptTokens:     int(response.Usage.PromptTokens),
				CompletionTokens: int(response.Usage.CompletionTokens),
				TotalTokens:      int(response.Usage.TotalTokens),
			}
		}

		return &gateway.GenerateContentResponse{Blocks: blocks, Usage: usage}, nil
	}, fmt.Sprintf("OpenAI GenerateContent for model %q", request.Model()))
}

func buildOpenAIChatParams(request *gateway.GenerateContentRequest) openai.ChatCompletionNewParams {
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(request.SystemPrompt()),
			openai.UserMessage(request.UserPrompt()),
		},
		Model: request.Model(),
		// Use MaxCompletionTokens (includes reasoning tokens) for compatibility with newer models.
		MaxCompletionTokens: openai.Int(int64(request.MaxOutputTokens())),
	}

	applyOpenAISamplingParams(&params, request)
	applyOpenAIResponseParams(&params, request)
	applyOpenAICandidateParams(&params, request)
	applyOpenAILogprobParams(&params, request)
	applyOpenAILogitBias(&params, request)
	applyOpenAISystemParams(&params, request)

	return params
}

func buildOpenAITools(tools []gateway.ToolDefinition) []openai.ChatCompletionToolUnionParam {
	if len(tools) == 0 {
		return nil
	}
	result := make([]openai.ChatCompletionToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		result = append(result, openai.ChatCompletionToolUnionParam{
			OfFunction: &openai.ChatCompletionFunctionToolParam{
				Type: constant.Function("function"),
				Function: shared.FunctionDefinitionParam{
					Name:        tool.Name,
					Description: openai.String(tool.Description),
					Parameters:  shared.FunctionParameters(tool.Parameters),
				},
			},
		})
	}
	return result
}

func mapOpenAIToolCalls(calls []openai.ChatCompletionMessageToolCallUnion) []gateway.ToolCall {
	if len(calls) == 0 {
		return nil
	}
	result := make([]gateway.ToolCall, 0, len(calls))
	for i := range calls {
		call := calls[i]
		// Only map function tool calls.
		if call.Type != "function" {
			continue
		}
		args := map[string]any{}
		if call.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				// Be resilient to invalid JSON; preserve the raw payload.
				args = map[string]any{"_raw": call.Function.Arguments}
			}
		}
		result = append(result, gateway.ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: args,
		})
	}
	return result
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

	return RetryWithBackoff(ctx, DefaultRetryConfig(), func(ctx context.Context) ([]gateway.Embedding, error) {
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
			return nil, err
		}

		embeddings := make([]gateway.Embedding, 0, len(response.Data))
		for i := range response.Data {
			embeddings = append(embeddings, response.Data[i].Embedding)
		}

		return embeddings, nil
	}, fmt.Sprintf("OpenAI ComputeEmbeddings for model %q", request.Model()))
}

// CountTokens estimates the number of tokens in the given text using tiktoken.
// For embeddings, it concatenates all chunks and counts the total tokens.
// Returns an error if the model encoding cannot be determined.
func (g *openaiGateway) CountTokens(ctx context.Context, model string, texts []string) (int, error) {
	if g == nil {
		return 0, fmt.Errorf("openai gateway is nil")
	}

	if len(texts) == 0 {
		return 0, nil
	}

	// Get the appropriate encoding for the model
	encoding, err := tiktoken.EncodingForModel(model)
	if err != nil {
		// If we can't find the model-specific encoding, try cl100k_base (used by most modern models)
		encoding, err = tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			return 0, fmt.Errorf("failed to get tiktoken encoding: %w", err)
		}
	}

	// Count tokens across all texts
	totalTokens := 0
	for _, text := range texts {
		tokens := encoding.Encode(text, nil, nil)
		totalTokens += len(tokens)
	}

	return totalTokens, nil
}
