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

	messages := []anthropic.MessageParam{}
	if msgs := request.Messages(); len(msgs) > 0 {
		for _, m := range msgs {
			switch m.Role {
			case gateway.MessageRoleAssistant:
				text := strings.TrimSpace(m.Content)
				if len(m.ToolCalls) > 0 {
					for _, c := range m.ToolCalls {
						args, _ := json.Marshal(c.Arguments)
						if text != "" {
							text += "\n\n"
						}
						text += fmt.Sprintf("Tool call: %s (id=%s) args=%s", c.Name, c.ID, string(args))
					}
				}
				if text != "" {
					messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(text)))
				}
			case gateway.MessageRoleTool:
				text := strings.TrimSpace(m.Content)
				if m.ToolName != "" {
					header := "Tool result: " + m.ToolName
					if m.ToolCallID != "" {
						header += " (tool_call_id=" + m.ToolCallID + ")"
					}
					text = header + "\n" + text
				}
				if text != "" {
					messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(text)))
				}
			case gateway.MessageRoleSystem:
				// handled via params.System below
			case gateway.MessageRoleUser:
				text := strings.TrimSpace(m.Content)
				if text != "" {
					messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(text)))
				}
			default:
				text := strings.TrimSpace(m.Content)
				if text != "" {
					messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(text)))
				}
			}
		}
	}
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

	blocks, err := parseAnthropicBlocksOrdered(response.Content)
	if err != nil {
		return nil, err
	}
	return &gateway.GenerateContentResponse{Blocks: blocks}, nil
}

func buildAnthropicTools(tools []gateway.ToolDefinition) []anthropic.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}
	result := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		inputSchema := anthropic.ToolInputSchemaParam{Type: "object"}
		if tool.Parameters != nil {
			if props, ok := tool.Parameters["properties"]; ok {
				inputSchema.Properties = props
			}
			if req, ok := tool.Parameters["required"]; ok {
				if list, ok := req.([]any); ok {
					required := make([]string, 0, len(list))
					for _, item := range list {
						if s, ok := item.(string); ok {
							required = append(required, s)
						}
					}
					inputSchema.Required = required
				} else if list, ok := req.([]string); ok {
					inputSchema.Required = list
				}
			}
			// Preserve any remaining schema fields.
			extras := make(map[string]any)
			for k, v := range tool.Parameters {
				if k == "type" || k == "properties" || k == "required" {
					continue
				}
				extras[k] = v
			}
			if len(extras) > 0 {
				inputSchema.ExtraFields = extras
			}
		}

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

func parseAnthropicBlocksOrdered(blocks []anthropic.ContentBlockUnion) ([]gateway.ContentBlock, error) {
	result := make([]gateway.ContentBlock, 0, len(blocks))
	for _, block := range blocks {
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
	return result, nil
}
