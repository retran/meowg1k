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

	"github.com/retran/meowg1k/pkg/ratelimit"
)

// rateLimitedGenerationGateway wraps a GenerationGateway with rate limiting.
type rateLimitedGenerationGateway struct {
	gateway GenerationGateway
	limiter ratelimit.Limiter
}

// newRateLimitedGenerationGateway creates a new rate-limited generation
func newRateLimitedGenerationGateway(gateway GenerationGateway, limiter ratelimit.Limiter) GenerationGateway {
	return &rateLimitedGenerationGateway{
		gateway: gateway,
		limiter: limiter,
	}
}

// GenerateContent implements GenerationGateway with rate limiting.
func (g *rateLimitedGenerationGateway) GenerateContent(
	ctx context.Context,
	request *GenerateContentRequest,
) (string, error) {
	tokenCount := estimateTokenCount(request.SystemPrompt() + request.UserPrompt())

	if err := g.limiter.Wait(ctx, tokenCount); err != nil {
		return "", err
	}

	return g.gateway.GenerateContent(ctx, request)
}

// estimateTokenCount provides a rough estimate of token count.
// Typically ~4 characters per token for English text.
func estimateTokenCount(text string) int {
	// TODO implement precise token counting
	return len(text) / 4
}

// rateLimitedEmbeddingsGateway wraps an EmbeddingsGateway with rate limiting.
type rateLimitedEmbeddingsGateway struct {
	gateway EmbeddingsGateway
	limiter ratelimit.Limiter
}

// newRateLimitedEmbeddingsGateway creates a new rate-limited embeddings gateway.
func newRateLimitedEmbeddingsGateway(gateway EmbeddingsGateway, limiter ratelimit.Limiter) EmbeddingsGateway {
	return &rateLimitedEmbeddingsGateway{
		gateway: gateway,
		limiter: limiter,
	}
}

// ComputeEmbeddings implements EmbeddingsGateway with rate limiting.
func (g *rateLimitedEmbeddingsGateway) ComputeEmbeddings(
	ctx context.Context,
	request *ComputeEmbeddingsRequest,
) ([]Embedding, error) {
	// Estimate token count from the text chunks
	totalChars := 0
	for _, chunk := range request.Chunks() {
		totalChars += len(chunk)
	}
	tokenCount := totalChars / 4 // Rough estimate: ~4 chars per token

	if err := g.limiter.Wait(ctx, tokenCount); err != nil {
		return nil, err
	}

	return g.gateway.ComputeEmbeddings(ctx, request)
}

// ComputeDistance implements EmbeddingsGateway by delegating to the wrapped gateway.
func (g *rateLimitedEmbeddingsGateway) ComputeDistance(first, second Embedding) (float64, error) {
	return g.gateway.ComputeDistance(first, second)
}
