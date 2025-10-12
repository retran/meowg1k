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

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

// Compile-time interface satisfaction check
var _ ports.GenerationGateway = (*anthropicGateway)(nil)

// anthropicGateway wraps the Anthropic SDK client for content generation.
type anthropicGateway struct {
	client anthropic.Client
}

// NewAnthropicGateway creates a new Anthropic
func newAnthropicGateway(apiKey string) (ports.GenerationGateway, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic API key is required")
	}

	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)

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
