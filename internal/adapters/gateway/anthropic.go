// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"fmt"
	"net/http"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

// Compile-time interface satisfaction check
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
) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("context cannot be nil")
	}

	if g == nil {
		return "", fmt.Errorf("anthropic gateway is nil")
	}

	if request == nil {
		return "", fmt.Errorf("request cannot be nil")
	}

	model := request.Model()
	if model == "" {
		return "", fmt.Errorf("model is required for anthropic content generation")
	}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(request.UserPrompt())),
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		Messages:  messages,
		MaxTokens: int64(request.MaxOutputTokens()),
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
		return "", fmt.Errorf("failed to generate content from Anthropic for model %q: %w", model, err)
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("no content in response from Anthropic for model %q", model)
	}

	for i := range response.Content {
		if response.Content[i].Type == "text" {
			textBlock := response.Content[i].AsText()
			return textBlock.Text, nil
		}
	}

	return "", fmt.Errorf("no text content found in Anthropic response for model %q", model)
}
