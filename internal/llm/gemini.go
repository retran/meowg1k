/*
Copyright © 2025 Andrew Vasilyev (me@retran.me)

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

package llm

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

var _ GenerationGateway = (*GeminiGenerationGateway)(nil)

type GeminiGenerationGateway struct {
	client *genai.Client
}

func NewGeminiGenerationGateway(ctx context.Context, apiKey string) (*GeminiGenerationGateway, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &GeminiGenerationGateway{
		client: client,
	}, nil
}

func (g *GeminiGenerationGateway) GenerateContent(ctx context.Context, request *GenerateContentRequest) (string, error) {
	systemPrompt := genai.Text(request.systemPrompt)[0]

	generationConfig := &genai.GenerateContentConfig{
		SystemInstruction: systemPrompt,
	}

	userPrompt := genai.Text(request.userPrompt)

	result, err := g.client.Models.GenerateContent(ctx, request.model, userPrompt, generationConfig)
	if err != nil {
		return "", fmt.Errorf("failed to fetch response from Gemini API: %w", err)
	}

	return result.Text(), nil
}
