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

var _ GenerationGateway = (*LlamaGenerationGateway)(nil)

// LlamaGenerationGateway is an implementation of GenerationGateway that uses a local LLM server
type LlamaGenerationGateway struct {
	client *llama.CompletionClient
}

// NewLlamaGenerationGateway creates and initializes a new LlamaGenerationGateway.
func NewLlamaGenerationGateway(baseURL string) (*LlamaGenerationGateway, error) {
	client, err := llama.NewCompletionClient(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create local LLM client: %w", err)
	}

	return &LlamaGenerationGateway{
		client: client,
	}, nil
}

// GenerateContent sends a content generation request to the local LLM server.
func (g *LlamaGenerationGateway) GenerateContent(ctx context.Context, request *GenerateContentRequest) (string, error) {
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
		return "", fmt.Errorf("failed to fetch response from local LLM API: %w", err)
	}

	return resp.Content, nil
}
