// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

// Compile-time interface satisfaction check.
var _ ports.GenerationGateway = (*anthropicGateway)(nil)

// anthropicGateway wraps the Anthropic SDK client for content generation.
type anthropicGateway struct {
	client anthropic.Client
}

// NewAnthropicGateway creates a new Anthropic gateway with an optional HTTP client.
// If httpClient is nil, the SDK will use its default HTTP client.
func newAnthropicGateway(apiKey string, httpClient *http.Client) (ports.GenerationGateway, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic API key is required")
	}

	options := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}

	if httpClient != nil {
		options = append(options, option.WithHTTPClient(httpClient))
	}

	client := anthropic.NewClient(options...)

	return &anthropicGateway{
		client: client,
	}, nil
}

// GenerateContent generates content using Anthropic's API.
func (g *anthropicGateway) GenerateContent(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
) (*gateway.GenerateContentResponse, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if g == nil {
		return nil, fmt.Errorf("anthropic gateway is nil")
	}

	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	model := request.Model()
	if model == "" {
		return nil, fmt.Errorf("model is required for anthropic content generation")
	}

	return RetryWithBackoff(ctx, DefaultRetryConfig(), func(ctx context.Context) (*gateway.GenerateContentResponse, error) {

		messages := g.mapMessages(request)
		if len(messages) == 0 {
			messages = []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock(request.UserPrompt())),
			}
		}

		params := anthropic.MessageNewParams{
			Model:     anthropic.Model(model),
			Messages:  messages,
			MaxTokens: int64(request.MaxOutputTokens()),
		}
		if tools := request.Tools(); len(tools) > 0 {
			params.Tools = buildAnthropicTools(tools)
			params.ToolChoice = anthropic.ToolChoiceUnionParam{
				OfAuto: &anthropic.ToolChoiceAutoParam{Type: "auto"},
			}
		}

		if systemPrompt := request.SystemPrompt(); systemPrompt != "" {
			params.System = []anthropic.TextBlockParam{
				{Text: systemPrompt},
			}
		}

		if temperature := request.Temperature(); temperature != nil {
			params.Temperature = anthropic.Float(*temperature)
		}

		if topP := request.TopP(); topP != nil {
			params.TopP = anthropic.Float(*topP)
		}

		if topK := request.TopK(); topK != nil {
			params.TopK = anthropic.Int(int64(*topK))
		}

		if stop := request.Stop(); len(stop) > 0 {
			params.StopSequences = stop
		}

		// Apply response schema if provided (Anthropic supports structured outputs)
		if responseSchema := request.ResponseSchema(); responseSchema != nil {
			// Convert schema to tool use pattern for Anthropic
			inputSchema := buildToolInputSchema(responseSchema)

			toolDef := &anthropic.ToolParam{
				Name:        "output",
				Description: anthropic.String("Generated structured output"),
				InputSchema: inputSchema,
			}

			params.Tools = []anthropic.ToolUnionParam{
				{OfTool: toolDef},
			}
			params.ToolChoice = anthropic.ToolChoiceUnionParam{
				OfTool: &anthropic.ToolChoiceToolParam{
					Type: "tool",
					Name: "output",
				},
			}
		}

		response, err := g.client.Messages.New(ctx, params)
		if err != nil {
			return nil, err
		}

		if len(response.Content) == 0 {
			return nil, fmt.Errorf("no content in response from Anthropic for model %q", model)
		}

		blocks := parseAnthropicBlocksOrdered(response.Content)

		var usage *gateway.UsageMetadata
		if response.Usage.InputTokens > 0 || response.Usage.OutputTokens > 0 {
			usage = &gateway.UsageMetadata{
				PromptTokens:     int(response.Usage.InputTokens),
				CompletionTokens: int(response.Usage.OutputTokens),
				TotalTokens:      int(response.Usage.InputTokens + response.Usage.OutputTokens),
			}
		}

		return &gateway.GenerateContentResponse{Blocks: blocks, Usage: usage}, nil
	}, fmt.Sprintf("Anthropic GenerateContent for model %q", model))
}

func (g *anthropicGateway) mapMessages(request *gateway.GenerateContentRequest) []anthropic.MessageParam {
	msgs := request.Messages()
	if len(msgs) == 0 {
		return nil
	}

	mapped := make([]anthropic.MessageParam, 0, len(msgs))
	for i := range msgs {
		if msg, ok := g.mapMessage(&msgs[i]); ok {
			mapped = append(mapped, msg)
		}
	}
	return mapped
}

func (g *anthropicGateway) mapMessage(m *gateway.Message) (anthropic.MessageParam, bool) {
	switch m.Role {
	case gateway.MessageRoleAssistant:
		return g.mapAssistantMessage(m)
	case gateway.MessageRoleTool:
		return g.mapToolMessage(m)
	case gateway.MessageRoleSystem:
		return anthropic.MessageParam{}, false
	case gateway.MessageRoleUser:
		if text := strings.TrimSpace(m.Content); text != "" {
			return anthropic.NewUserMessage(anthropic.NewTextBlock(text)), true
		}
	default:
		if text := strings.TrimSpace(m.Content); text != "" {
			return anthropic.NewUserMessage(anthropic.NewTextBlock(text)), true
		}
	}
	return anthropic.MessageParam{}, false
}

func (g *anthropicGateway) mapAssistantMessage(m *gateway.Message) (anthropic.MessageParam, bool) {
	text := strings.TrimSpace(m.Content)
	if len(m.ToolCalls) > 0 {
		for _, c := range m.ToolCalls {
			args, err := json.Marshal(c.Arguments)
			if err != nil {
				args = []byte("{}")
			}
			if text != "" {
				text += "\n\n"
			}
			text += fmt.Sprintf("Tool call: %s (id=%s) args=%s", c.Name, c.ID, string(args))
		}
	}
	if text == "" {
		return anthropic.MessageParam{}, false
	}
	return anthropic.NewAssistantMessage(anthropic.NewTextBlock(text)), true
}

func (g *anthropicGateway) mapToolMessage(m *gateway.Message) (anthropic.MessageParam, bool) {
	text := strings.TrimSpace(m.Content)
	if m.ToolName != "" {
		header := "Tool result: " + m.ToolName
		if m.ToolCallID != "" {
			header += " (tool_call_id=" + m.ToolCallID + ")"
		}
		text = header + "\n" + text
	}
	if text == "" {
		return anthropic.MessageParam{}, false
	}
	return anthropic.NewUserMessage(anthropic.NewTextBlock(text)), true
}

func buildAnthropicTools(tools []gateway.ToolDefinition) []anthropic.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}
	result := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		inputSchema := buildToolInputSchema(tool.Parameters)
		toolParam := &anthropic.ToolParam{
			Name:        tool.Name,
			InputSchema: inputSchema,
		}
		if tool.Description != "" {
			toolParam.Description = anthropic.String(tool.Description)
		}
		result = append(result, anthropic.ToolUnionParam{OfTool: toolParam})
	}
	return result
}

func buildToolInputSchema(params map[string]any) anthropic.ToolInputSchemaParam {
	inputSchema := anthropic.ToolInputSchemaParam{Type: "object"}
	if params == nil {
		return inputSchema
	}

	if props, ok := params["properties"]; ok {
		inputSchema.Properties = props
	}

	if req, ok := params["required"]; ok {
		inputSchema.Required = mapRequiredFields(req)
	}

	// Preserve any remaining schema fields.
	extras := make(map[string]any)
	for k, v := range params {
		if k == "type" || k == "properties" || k == "required" {
			continue
		}
		extras[k] = v
	}
	if len(extras) > 0 {
		inputSchema.ExtraFields = extras
	}

	return inputSchema
}

func mapRequiredFields(req any) []string {
	switch list := req.(type) {
	case []any:
		required := make([]string, 0, len(list))
		for _, item := range list {
			if s, ok := item.(string); ok {
				required = append(required, s)
			}
		}
		return required
	case []string:
		return list
	default:
		return nil
	}
}

func parseAnthropicBlocksOrdered(blocks []anthropic.ContentBlockUnion) []gateway.ContentBlock {
	result := make([]gateway.ContentBlock, 0, len(blocks))
	for i := range blocks {
		block := blocks[i]
		switch block.Type {
		case "text":
			text := block.AsText().Text
			result = append(result, gateway.ContentBlock{Kind: gateway.ContentBlockText, Text: text})
		case "thinking":
			think := block.AsThinking().Thinking
			result = append(result, gateway.ContentBlock{Kind: gateway.ContentBlockReasoning, Text: think})
		case "tool_use":
			tu := block.AsToolUse()
			args := map[string]any{}
			if len(tu.Input) > 0 {
				if err := json.Unmarshal(tu.Input, &args); err != nil {
					args = map[string]any{"_raw": string(tu.Input)}
				}
			}
			call := gateway.ToolCall{ID: tu.ID, Name: tu.Name, Arguments: args}
			result = append(result, gateway.ContentBlock{Kind: gateway.ContentBlockToolCall, ToolCall: &call})
		}
	}
	return result
}

// GenerateContentStream implements streaming for Anthropic by delegating to GenerateContent
// and synthesizing stream events from the aggregated response.
// Full native streaming via the Anthropic SSE API will be added in a future phase.
func (g *anthropicGateway) GenerateContentStream(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
	callback gateway.StreamCallback,
) (*gateway.GenerateContentResponse, error) {
	resp, err := g.GenerateContent(ctx, request)
	if err != nil {
		if callback != nil {
			_ = callback(gateway.StreamEvent{Kind: gateway.StreamEventError, Error: err.Error(), Recoverable: false})
		}
		return nil, err
	}
	return synthesizeStreamEvents(resp, callback)
}

// CountTokens counts tokens using Anthropic's token counting API.
// For embeddings, Anthropic doesn't provide embeddings, so this returns an error.
// For generation, it calls the count_tokens endpoint with the messages.
func (g *anthropicGateway) CountTokens(ctx context.Context, model string, texts []string) (int, error) {
	if g == nil {
		return 0, fmt.Errorf("anthropic gateway is nil")
	}

	if len(texts) == 0 {
		return 0, nil
	}

	messages := make([]anthropic.MessageParam, 0, len(texts))
	for _, text := range texts {
		if text != "" {
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(text)))
		}
	}

	if len(messages) == 0 {
		return 0, nil
	}

	result, err := g.client.Messages.CountTokens(ctx, anthropic.MessageCountTokensParams{
		Model:    anthropic.Model(model),
		Messages: messages,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to count tokens with Anthropic API: %w", err)
	}

	return int(result.InputTokens), nil
}
