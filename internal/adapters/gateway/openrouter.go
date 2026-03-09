// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

// openrouterGateway implements gateway interfaces for OpenRouter API.
type openrouterGateway struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

// NewOpenRouterGateway creates a new OpenRouter gateway with a shared HTTP client.
// The HTTP client is provided via dependency injection to allow for better resource management
// and connection pooling across multiple gateway instances.
func NewOpenRouterGateway(
	_ context.Context,
	baseURL string,
	apiKey string,
	httpClient *http.Client,
) (ports.GenerationGateway, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("base URL is required for OpenRouter gateway")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("API key is required for OpenRouter gateway")
	}

	if httpClient == nil {
		return nil, fmt.Errorf("HTTP client is required for OpenRouter gateway")
	}

	return &openrouterGateway{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  httpClient,
	}, nil
}

// OpenRouter API request/response structures.
type openrouterMessage struct {
	Role       string               `json:"role"`
	Content    string               `json:"content,omitempty"`
	Reasoning  string               `json:"reasoning,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
	ToolCalls  []openrouterToolCall `json:"tool_calls,omitempty"`
}

type openrouterRequest struct {
	Temperature       *float64             `json:"temperature,omitempty"`
	TopP              *float64             `json:"top_p,omitempty"`
	Tools             *[]openrouterTool    `json:"tools,omitempty"`
	Model             *string              `json:"model"`
	ToolChoice        *string              `json:"tool_choice,omitempty"`
	FrequencyPenalty  *float64             `json:"frequency_penalty,omitempty"`
	PresencePenalty   *float64             `json:"presence_penalty,omitempty"`
	RepetitionPenalty *float64             `json:"repetition_penalty,omitempty"`
	Stop              *[]string            `json:"stop,omitempty"`
	TopA              *float64             `json:"top_a,omitempty"`
	Messages          *[]openrouterMessage `json:"messages"`
	MinP              *float64             `json:"min_p,omitempty"`
	N                 *int                 `json:"n,omitempty"`
	Seed              *int                 `json:"seed,omitempty"`
	TopK              *int                 `json:"top_k,omitempty"`
	ResponseFormat    map[string]any       `json:"response_format,omitempty"`
	MaxTokens         int                  `json:"max_tokens,omitempty"`
}

type openrouterTool struct {
	Function openrouterToolFunction `json:"function"`
	Type     string                 `json:"type"`
}

type openrouterToolFunction struct {
	Parameters  map[string]any `json:"parameters,omitempty"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
}

type openrouterToolCall struct {
	ID       string                  `json:"id,omitempty"`
	Type     string                  `json:"type,omitempty"`
	Function openrouterToolCallEntry `json:"function"`
}

type openrouterToolCallEntry struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
}

type openrouterChoice struct {
	Message struct {
		Content   string               `json:"content"`
		Reasoning string               `json:"reasoning,omitempty"`
		ToolCalls []openrouterToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
}

type openrouterResponse struct {
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Choices []openrouterChoice `json:"choices"`
}

// GenerateContent sends a content generation request to OpenRouter API.
func (g *openrouterGateway) GenerateContent(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
) (*gateway.GenerateContentResponse, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	return RetryWithBackoff(ctx, DefaultRetryConfig(), func(ctx context.Context) (*gateway.GenerateContentResponse, error) {
		messages := buildOpenRouterMessages(request)
		reqBody := buildOpenRouterRequest(request, messages)
		if tools := request.Tools(); len(tools) > 0 {
			toolList := buildOpenRouterTools(tools)
			if len(toolList) > 0 {
				reqBody.Tools = &toolList
			}
			reqBody.ToolChoice = stringPtr("auto")
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(
			ctx,
			"POST",
			g.baseURL+"/chat/completions",
			bytes.NewBuffer(jsonData),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)
		httpReq.Header.Set("HTTP-Referer", "https://github.com/retran/meowg1k")
		httpReq.Header.Set("X-Title", "meowg1k")

		resp, err := g.client.Do(httpReq) // URL is constructed from g.baseURL validated in NewOpenRouterGateway constructor.
		if err != nil {
			return nil, fmt.Errorf("failed to execute openrouter HTTP request: %w", err)
		}
		defer func() { _ = resp.Body.Close() }() //nolint:errcheck // Defer close errors are not critical

		return parseOpenRouterResponse(resp)
	}, fmt.Sprintf("OpenRouter GenerateContent for model %q", request.Model()))
}

func buildOpenRouterMessages(request *gateway.GenerateContentRequest) []openrouterMessage {
	msgs := request.Messages()
	if len(msgs) > 0 {
		mapped := make([]openrouterMessage, 0, len(msgs))
		for i := range msgs {
			mapped = append(mapped, mapToOpenRouterMessage(&msgs[i]))
		}
		return mapped
	}

	messages := []openrouterMessage{}
	if request.SystemPrompt() != "" {
		messages = append(messages, openrouterMessage{
			Role:    "system",
			Content: request.SystemPrompt(),
		})
	}
	messages = append(messages, openrouterMessage{
		Role:    "user",
		Content: request.UserPrompt(),
	})
	return messages
}

func mapToOpenRouterMessage(m *gateway.Message) openrouterMessage {
	om := openrouterMessage{Role: string(m.Role), Content: m.Content}
	if len(m.ToolCalls) > 0 {
		om.ToolCalls = mapOpenRouterToolCalls(m.ToolCalls)
	}
	if m.Role == gateway.MessageRoleTool {
		om.Role = "tool"
		om.ToolCallID = m.ToolCallID
	}
	return om
}

func mapOpenRouterToolCalls(toolCalls []gateway.ToolCall) []openrouterToolCall {
	calls := make([]openrouterToolCall, 0, len(toolCalls))
	for _, c := range toolCalls {
		args, err := json.Marshal(c.Arguments)
		if err != nil {
			args = []byte("{}")
		}
		calls = append(calls, openrouterToolCall{
			ID:   c.ID,
			Type: "function",
			Function: openrouterToolCallEntry{
				Name:      c.Name,
				Arguments: string(args),
			},
		})
	}
	return calls
}

func buildOpenRouterRequest(request *gateway.GenerateContentRequest, messages []openrouterMessage) openrouterRequest {
	messageList := messages
	reqBody := openrouterRequest{
		Model:     stringPtr(request.Model()),
		Messages:  &messageList,
		MaxTokens: request.MaxOutputTokens(),
	}

	applyOpenRouterSampling(&reqBody, request)
	applyOpenRouterPenalties(&reqBody, request)
	applyOpenRouterControlParams(&reqBody, request)
	applyOpenRouterResponseFormat(&reqBody, request)

	return reqBody
}

func stringPtr(value string) *string {
	return &value
}

func buildOpenRouterTools(tools []gateway.ToolDefinition) []openrouterTool {
	if len(tools) == 0 {
		return nil
	}

	result := make([]openrouterTool, 0, len(tools))
	for _, tool := range tools {
		result = append(result, openrouterTool{
			Type: "function",
			Function: openrouterToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}
	return result
}

func applyOpenRouterSampling(reqBody *openrouterRequest, request *gateway.GenerateContentRequest) {
	if temp := request.Temperature(); temp != nil {
		reqBody.Temperature = temp
	}
	if topP := request.TopP(); topP != nil {
		reqBody.TopP = topP
	}
	if topK := request.TopK(); topK != nil {
		reqBody.TopK = topK
	}
	if topA := request.TopA(); topA != nil {
		reqBody.TopA = topA
	}
	if minP := request.MinP(); minP != nil {
		reqBody.MinP = minP
	}
}

func applyOpenRouterPenalties(reqBody *openrouterRequest, request *gateway.GenerateContentRequest) {
	if fp := request.FrequencyPenalty(); fp != nil {
		reqBody.FrequencyPenalty = fp
	}
	if pp := request.PresencePenalty(); pp != nil {
		reqBody.PresencePenalty = pp
	}
	if rp := request.RepetitionPenalty(); rp != nil {
		reqBody.RepetitionPenalty = rp
	}
}

func applyOpenRouterControlParams(reqBody *openrouterRequest, request *gateway.GenerateContentRequest) {
	if seed := request.Seed(); seed != nil {
		reqBody.Seed = seed
	}
	if stop := request.Stop(); len(stop) > 0 {
		reqBody.Stop = &stop
	}
	if n := request.CandidateCount(); n != nil {
		reqBody.N = n
	}
}

func applyOpenRouterResponseFormat(reqBody *openrouterRequest, request *gateway.GenerateContentRequest) {
	responseSchema := request.ResponseSchema()
	responseFormat := request.ResponseFormat()

	// OpenRouter supports OpenAI-compatible response format
	if responseSchema != nil {
		// Extract schema name and description if present
		name := "response"
		if schemaName, ok := responseSchema["name"].(string); ok {
			name = schemaName
		}

		description := ""
		if schemaDesc, ok := responseSchema["description"].(string); ok {
			description = schemaDesc
		}

		// Build JSON schema response format (OpenAI-compatible)
		reqBody.ResponseFormat = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":        name,
				"description": description,
				"schema":      responseSchema,
				"strict":      true,
			},
		}
	} else if responseFormat != nil && *responseFormat != "" {
		// Simple response format (json_object or text)
		reqBody.ResponseFormat = map[string]any{
			"type": *responseFormat,
		}
	}
}

func parseOpenRouterResponse(resp *http.Response) (*gateway.GenerateContentResponse, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenRouter API returned status %d: %s", resp.StatusCode, string(body))
	}

	var openrouterResp openrouterResponse
	if err := json.Unmarshal(body, &openrouterResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if openrouterResp.Error != nil {
		return nil, fmt.Errorf("OpenRouter API error: %s (type: %s, code: %s)",
			openrouterResp.Error.Message,
			openrouterResp.Error.Type,
			openrouterResp.Error.Code,
		)
	}

	if len(openrouterResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from OpenRouter API")
	}

	message := openrouterResp.Choices[0].Message
	toolCalls, err := parseOpenRouterToolCalls(message.ToolCalls)
	if err != nil {
		return nil, err
	}

	blocks := make([]gateway.ContentBlock, 0, 2+len(toolCalls))
	if message.Reasoning != "" {
		blocks = append(blocks, gateway.ContentBlock{Kind: gateway.ContentBlockReasoning, Text: message.Reasoning})
	}
	if message.Content != "" {
		blocks = append(blocks, gateway.ContentBlock{Kind: gateway.ContentBlockText, Text: message.Content})
	}
	for i := range toolCalls {
		call := toolCalls[i]
		blocks = append(blocks, gateway.ContentBlock{Kind: gateway.ContentBlockToolCall, ToolCall: &call})
	}

	// Extract usage information
	var usage *gateway.UsageMetadata
	if openrouterResp.Usage != nil {
		usage = &gateway.UsageMetadata{
			PromptTokens:     openrouterResp.Usage.PromptTokens,
			CompletionTokens: openrouterResp.Usage.CompletionTokens,
			TotalTokens:      openrouterResp.Usage.TotalTokens,
		}
	}

	return &gateway.GenerateContentResponse{Blocks: blocks, Usage: usage}, nil
}

func parseOpenRouterToolCalls(calls []openrouterToolCall) ([]gateway.ToolCall, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	result := make([]gateway.ToolCall, 0, len(calls))
	for _, call := range calls {
		parsedArgs, err := parseOpenRouterArguments(call.Function.Arguments)
		if err != nil {
			return nil, err
		}
		result = append(result, gateway.ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: parsedArgs,
		})
	}

	return result, nil
}

func parseOpenRouterArguments(raw string) (map[string]any, error) {
	if raw == "" {
		return map[string]any{}, nil
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil, fmt.Errorf("failed to parse tool call arguments: %w", err)
	}

	return args, nil
}

// GenerateContentStream implements streaming for OpenRouter by delegating to GenerateContent
// and synthesizing stream events from the aggregated response.
// Full native SSE streaming will be added in a future phase.
func (g *openrouterGateway) GenerateContentStream(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
	callback gateway.StreamCallback,
) (*gateway.GenerateContentResponse, error) {
	resp, err := g.GenerateContent(ctx, request)
	if err != nil {
		if callback != nil {
			if cbErr := callback(gateway.StreamEvent{Kind: gateway.StreamEventError, Error: err.Error(), Recoverable: false}); cbErr != nil {
				return nil, fmt.Errorf("%w; stream callback error: %w", err, cbErr)
			}
		}
		return nil, err
	}
	return synthesizeStreamEvents(resp, callback)
}

// CountTokens estimates token count for OpenRouter models.
// Since OpenRouter is a proxy to various providers, we use character-based estimation:
// approximately (chars + 2) / 3 for better accuracy.
func (g *openrouterGateway) CountTokens(_ context.Context, _ string, texts []string) (int, error) {
	if g == nil {
		return 0, fmt.Errorf("openrouter gateway is nil")
	}

	if len(texts) == 0 {
		return 0, nil
	}

	// Count total characters across all texts
	totalChars := 0
	for _, text := range texts {
		totalChars += len(text)
	}

	// Estimate tokens: approximately (chars + 2) / 3
	estimatedTokens := (totalChars + 2) / 3

	return estimatedTokens, nil
}
