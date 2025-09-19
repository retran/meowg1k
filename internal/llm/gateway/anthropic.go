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

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicGateway wraps the Anthropic SDK client for content generation.
type AnthropicGateway struct {
	client anthropic.Client
}

// NewAnthropicGateway creates a new Anthropic gateway.
func NewAnthropicGateway(apiKey string) (*AnthropicGateway, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic API key is required")
	}

	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)

	return &AnthropicGateway{
		client: client,
	}, nil
}

// GenerateContent generates content using Anthropic's API.
func (g *AnthropicGateway) GenerateContent(ctx context.Context, request *GenerateContentRequest) (string, error) {
	model := request.Model()
	if model == "" {
		return "", fmt.Errorf("model is required")
	}

	// Prepare messages for the API
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(request.UserPrompt())),
	}

	// Create the request parameters
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		Messages:  messages,
		MaxTokens: int64(request.MaxOutputTokens()),
	}

	// Add system prompt if provided
	if systemPrompt := request.SystemPrompt(); systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemPrompt},
		}
	}

	// Make the API call
	response, err := g.client.Messages.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to generate content with Anthropic: %w", err)
	}

	// Extract text content from response
	if len(response.Content) == 0 {
		return "", fmt.Errorf("no content in response from Anthropic")
	}

	// Find the first text block in the response
	for _, content := range response.Content {
		if content.Type == "text" {
			textBlock := content.AsText()
			return textBlock.Text, nil
		}
	}

	return "", fmt.Errorf("no text content found in Anthropic response")
}
