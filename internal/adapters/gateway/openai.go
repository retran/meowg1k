// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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

// GenerateContentStream sends a streaming content generation request to the OpenAI-compatible API.
// It calls callback for each streaming event and returns the aggregated response when done.
func (g *openaiGateway) GenerateContentStream(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
	callback gateway.StreamCallback,
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
		// Request usage in the final chunk.
		params.StreamOptions = openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: openai.Bool(true),
		}

		stream := g.client.Chat.Completions.NewStreaming(ctx, params)
		defer stream.Close()

		// Accumulators for assembling the full response.
		var textBuilder strings.Builder
		// tool call accumulators keyed by index
		type pendingToolCall struct {
			id   string
			name string
			args strings.Builder
		}
		pendingTools := make(map[int]*pendingToolCall)
		var toolCallOrder []int // preserve ordering by index

		var finalUsage *gateway.UsageMetadata

		for stream.Next() {
			chunk := stream.Current()

			// Usage comes in the last chunk (empty choices).
			if chunk.Usage.TotalTokens > 0 {
				finalUsage = &gateway.UsageMetadata{
					PromptTokens:     int(chunk.Usage.PromptTokens),
					CompletionTokens: int(chunk.Usage.CompletionTokens),
					TotalTokens:      int(chunk.Usage.TotalTokens),
				}
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			delta := chunk.Choices[0].Delta

			// Text delta.
			if delta.Content != "" {
				textBuilder.WriteString(delta.Content)
				if err := callback(gateway.StreamEvent{
					Kind:  gateway.StreamEventText,
					Delta: delta.Content,
				}); err != nil {
					return nil, err
				}
			}

			// Tool call deltas.
			for i := range delta.ToolCalls {
				tc := &delta.ToolCalls[i]
				idx := int(tc.Index)

				if _, exists := pendingTools[idx]; !exists {
					pendingTools[idx] = &pendingToolCall{}
					toolCallOrder = append(toolCallOrder, idx)
				}

				pt := pendingTools[idx]
				if tc.ID != "" {
					pt.id = tc.ID
				}
				if tc.Function.Name != "" {
					pt.name = tc.Function.Name
				}
				pt.args.WriteString(tc.Function.Arguments)
			}
		}

		if err := stream.Err(); err != nil {
			return nil, err
		}

		// Build blocks from accumulated data.
		var toolCalls []gateway.ToolCall
		for _, idx := range toolCallOrder {
			pt := pendingTools[idx]
			args := map[string]any{}
			if raw := pt.args.String(); raw != "" {
				if err := json.Unmarshal([]byte(raw), &args); err != nil {
					args = map[string]any{"_raw": raw}
				}
			}
			toolCalls = append(toolCalls, gateway.ToolCall{
				ID:        pt.id,
				Name:      pt.name,
				Arguments: args,
			})
		}

		blocks := make([]gateway.ContentBlock, 0, 1+len(toolCalls))
		if text := textBuilder.String(); text != "" {
			blocks = append(blocks, gateway.ContentBlock{Kind: gateway.ContentBlockText, Text: text})
		}
		for i := range toolCalls {
			call := toolCalls[i]
			blocks = append(blocks, gateway.ContentBlock{Kind: gateway.ContentBlockToolCall, ToolCall: &call})
		}

		resp := &gateway.GenerateContentResponse{Blocks: blocks, Usage: finalUsage}

		// Fire done event.
		doneEvent := gateway.StreamEvent{Kind: gateway.StreamEventDone}
		if finalUsage != nil {
			doneEvent.Usage = finalUsage
		}
		if err := callback(doneEvent); err != nil {
			return nil, err
		}

		return resp, nil
	}, fmt.Sprintf("OpenAI GenerateContentStream for model %q", request.Model()))
}

func buildOpenAIChatParams(request *gateway.GenerateContentRequest) openai.ChatCompletionNewParams {
	var messages []openai.ChatCompletionMessageParamUnion

	if msgs := request.Messages(); len(msgs) > 0 {
		messages = mapOpenAIMessages(msgs, request.SystemPrompt())
	} else {
		messages = []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(request.SystemPrompt()),
			openai.UserMessage(request.UserPrompt()),
		}
	}

	params := openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    request.Model(),
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

// mapOpenAIMessages converts gateway messages to OpenAI message params, prepending system prompt if set.
func mapOpenAIMessages(msgs []gateway.Message, systemPrompt string) []openai.ChatCompletionMessageParamUnion {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(msgs)+1)

	if systemPrompt != "" {
		result = append(result, openai.SystemMessage(systemPrompt))
	}

	for i := range msgs {
		m := &msgs[i]
		switch m.Role {
		case gateway.MessageRoleSystem:
			result = append(result, openai.SystemMessage(m.Content))
		case gateway.MessageRoleUser:
			result = append(result, openai.UserMessage(m.Content))
		case gateway.MessageRoleAssistant:
			if len(m.ToolCalls) > 0 {
				toolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, 0, len(m.ToolCalls))
				for _, tc := range m.ToolCalls {
					args, err := json.Marshal(tc.Arguments)
					if err != nil {
						args = []byte("{}")
					}
					toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID: tc.ID,
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      tc.Name,
								Arguments: string(args),
							},
						},
					})
				}
				result = append(result, openai.ChatCompletionMessageParamUnion{
					OfAssistant: &openai.ChatCompletionAssistantMessageParam{
						Content:   openai.ChatCompletionAssistantMessageParamContentUnion{OfString: openai.String(m.Content)},
						ToolCalls: toolCalls,
					},
				})
			} else {
				result = append(result, openai.AssistantMessage(m.Content))
			}
		case gateway.MessageRoleTool:
			result = append(result, openai.ToolMessage(m.Content, m.ToolCallID))
		default:
			result = append(result, openai.UserMessage(m.Content))
		}
	}

	return result
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

func applyOpenAIResponseParams(params *openai.ChatCompletionNewParams, request *gateway.GenerateContentRequest) {
	// Handle response schema (structured outputs)
	if responseSchema := request.ResponseSchema(); responseSchema != nil {
		// Extract schema name if provided, otherwise use default
		schemaName := "response"
		if name, ok := responseSchema["name"].(string); ok && name != "" {
			schemaName = name
		}

		// Extract description if provided
		var description string
		if desc, ok := responseSchema["description"].(string); ok {
			description = desc
		}

		params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				Type: constant.JSONSchema("json_schema"),
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:        schemaName,
					Description: openai.String(description),
					Schema:      responseSchema,
					Strict:      openai.Bool(true),
				},
			},
		}
		return
	}

	// Handle simple response format (json_object or text)
	if responseFormat := request.ResponseFormat(); responseFormat != nil {
		switch *responseFormat {
		case "json_object", "json":
			params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONObject: &shared.ResponseFormatJSONObjectParam{
					Type: constant.JSONObject("json_object"),
				},
			}
		case "text":
			params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
				OfText: &shared.ResponseFormatTextParam{
					Type: constant.Text("text"),
				},
			}
		}
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
