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
	"strings"

	"github.com/retran/meowg1k/internal/models/gateway"
	"github.com/retran/meowg1k/internal/services/llm/llama"
)

var _ GenerationGateway = (*llamaGateway)(nil)

// var _ EmbeddingGateway = (*llamaGateway)(nil) // Uncomment when embedding is implemented

// llamaGateway is a unified client for a local LLM server compatible with the llama.cpp API.
type llamaGateway struct {
	client llama.Service
}

const (
	systemPromptStart = "<|im_start|>system\n"
	systemPromptEnd   = "<|im_end|>\n"
	userPromptStart   = "<|im_start|>user\n"
	userPromptEnd     = "<|im_end|>\n"
	assistantStart    = "<|im_start|>assistant\n"
)

// NewLlamaGateway creates and initializes a new LlamaGateway.
func newLlamaGateway(baseURL, apiKey string) (GenerationGateway, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	client, err := llama.NewService(baseURL, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create llama client: %w", err)
	}

	return &llamaGateway{
		client: client,
	}, nil
}

// GenerateContent sends a content generation request to the local LLM server.
func (g *llamaGateway) GenerateContent(ctx context.Context, request *gateway.GenerateContentRequest) (string, error) {
	var promptBuilder strings.Builder

	if request.SystemPrompt() != "" {
		promptBuilder.WriteString(systemPromptStart)
		promptBuilder.WriteString(request.SystemPrompt())
		promptBuilder.WriteString(systemPromptEnd)
	}

	promptBuilder.WriteString(userPromptStart)
	promptBuilder.WriteString(request.UserPrompt())
	promptBuilder.WriteString(userPromptEnd)

	promptBuilder.WriteString(assistantStart)

	prompt := promptBuilder.String()

	req := &llama.CompletionRequest{
		Prompt:      prompt,
		Temperature: 0.8,
		TopK:        40,
		TopP:        0.95,
		NPredict:    -1,
		Stop:        []string{"<|endoftext|>", "<|im_end|>"},
	}

	resp, err := g.client.Complete(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: failed to fetch response from local LLM API: %w", err)
	}

	return resp.Content, nil
}
