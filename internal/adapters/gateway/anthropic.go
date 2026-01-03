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

	// Set generation parameters if provided
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

	// Anthropic doesn't directly support all parameters like logprobs, logit_bias, etc.
	// They are primarily OpenAI-specific features
	// However, we can document which parameters are not supported

	// User identifier (not directly supported by Anthropic Messages API as of current version)
	// ServiceTier (OpenAI-specific)
	// LogitBias (OpenAI-specific)
	// CandidateCount/N (OpenAI-specific, Anthropic returns single response)
	// LogProbs (OpenAI/Gemini-specific)

	response, err := g.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to write content from Anthropic for model %q: %w", model, err)
	}

	if len(response.Content) == 0 {
		return nil, fmt.Errorf("no content in response from Anthropic for model %q", model)
	}

	blocks := parseAnthropicBlocksOrdered(response.Content)
	return &gateway.GenerateContentResponse{Blocks: blocks}, nil
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
