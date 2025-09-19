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

	"github.com/retran/meowg1k/internal/llm/client/llama"
)

// Compile-time check for the LlamaGateway.
var _ GenerationGateway = (*LlamaGateway)(nil)

// var _ EmbeddingGateway = (*LlamaGateway)(nil) // Uncomment when embedding is implemented

// LlamaGateway is a unified client for a local LLM server compatible with the llama.cpp API.
type LlamaGateway struct {
	client *llama.CompletionClient
}

// NewLlamaGateway creates and initializes a new LlamaGateway.
func NewLlamaGateway(baseURL, apiKey string) (*LlamaGateway, error) {
	client, err := llama.NewCompletionClient(baseURL, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create local LLM client: %w", err)
	}

	return &LlamaGateway{
		client: client,
	}, nil
}

// GenerateContent sends a content generation request to the local LLM server.
func (g *LlamaGateway) GenerateContent(ctx context.Context, request *GenerateContentRequest) (string, error) {
	var promptBuilder strings.Builder

	if request.SystemPrompt() != "" {
		promptBuilder.WriteString("<|im_start|>system\n")
		promptBuilder.WriteString(request.SystemPrompt())
		promptBuilder.WriteString("<|im_end|>\n")
	}

	promptBuilder.WriteString("<|im_start|>user\n")
	promptBuilder.WriteString(request.UserPrompt())
	promptBuilder.WriteString("<|im_end|>\n")

	promptBuilder.WriteString("<|im_start|>assistant\n")

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
