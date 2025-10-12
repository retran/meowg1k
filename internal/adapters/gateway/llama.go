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

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/llama"
)

var _ ports.GenerationGateway = (*llamaGateway)(nil)

// TODO Uncomment when implemented
// var _ EmbeddingGateway = (*llamaGateway)(nil)

// llamaGateway is a unified client for a local LLM server compatible with the llama.cpp API.
type llamaGateway struct {
	client *llama.Client
}

const (
	systemPromptStart = "<|im_start|>system\n"
	systemPromptEnd   = "<|im_end|>\n"
	userPromptStart   = "<|im_start|>user\n"
	userPromptEnd     = "<|im_end|>\n"
	assistantStart    = "<|im_start|>assistant\n"
)

// NewLlamaGateway creates and initializes a new LlamaGateway.
func newLlamaGateway(baseURL, apiKey string) (ports.GenerationGateway, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("base URL is required for llama gateway")
	}

	client, err := llama.NewClient(baseURL, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create llama client with base URL %q: %w", baseURL, err)
	}

	return &llamaGateway{
		client: client,
	}, nil
}

// GenerateContent sends a content generation request to the local LLM server.
func (g *llamaGateway) GenerateContent(ctx context.Context, request *gateway.GenerateContentRequest) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("context cannot be nil")
	}

	if g == nil {
		return "", fmt.Errorf("llama gateway is nil")
	}

	if request == nil {
		return "", fmt.Errorf("request cannot be nil")
	}

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
		Prompt:   prompt,
		NPredict: -1,
	}

	// Set stop sequences - merge defaults with user-provided ones
	defaultStops := []string{"<|endoftext|>", "<|im_end|>"}
	if stop := request.Stop(); len(stop) > 0 {
		req.Stop = append(defaultStops, stop...)
	} else {
		req.Stop = defaultStops
	}

	// Set generation parameters if provided, otherwise use defaults
	if temperature := request.Temperature(); temperature != nil {
		req.Temperature = *temperature
	} else {
		req.Temperature = 0.8 // default
	}

	if topK := request.TopK(); topK != nil {
		req.TopK = *topK
	} else {
		req.TopK = 40 // default
	}

	if topP := request.TopP(); topP != nil {
		req.TopP = *topP
	} else {
		req.TopP = 0.95 // default
	}

	if frequencyPenalty := request.FrequencyPenalty(); frequencyPenalty != nil {
		req.FrequencyPenalty = *frequencyPenalty
	}

	if presencePenalty := request.PresencePenalty(); presencePenalty != nil {
		req.PresencePenalty = *presencePenalty
	}

	if seed := request.Seed(); seed != nil {
		req.Seed = *seed
	}

	resp, err := g.client.Complete(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to generate content from local LLM API: %w", err)
	}

	return resp.Content, nil
}
