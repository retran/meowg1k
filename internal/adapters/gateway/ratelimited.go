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

	"github.com/retran/meowg1k/internal/core/ratelimit"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

type rateLimitedGenerationGateway struct {
	gateway ports.GenerationGateway
	limiter ratelimit.Limiter
}

func newRateLimitedGenerationGateway(gateway ports.GenerationGateway, limiter ratelimit.Limiter) ports.GenerationGateway {
	return &rateLimitedGenerationGateway{
		gateway: gateway,
		limiter: limiter,
	}
}

func (g *rateLimitedGenerationGateway) GenerateContent(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
) (string, error) {
	if g == nil {
		return "", fmt.Errorf("rate limited generation gateway is nil")
	}

	if ctx == nil {
		return "", fmt.Errorf("context cannot be nil")
	}

	if request == nil {
		return "", fmt.Errorf("request cannot be nil")
	}

	tokenCount := estimateTokenCount(request.SystemPrompt() + request.UserPrompt())

	if err := g.limiter.Wait(ctx, tokenCount); err != nil {
		return "", fmt.Errorf("failed to acquire rate limit tokens: %w", err)
	}

	return g.gateway.GenerateContent(ctx, request)
}

func estimateTokenCount(text string) int {
	// TODO implement precise token counting
	return len(text) / 4
}

type rateLimitedEmbeddingsGateway struct {
	gateway ports.EmbeddingsGateway
	limiter ratelimit.Limiter
}

// newRateLimitedEmbeddingsGateway creates a new rate-limited embeddings gateway.
func newRateLimitedEmbeddingsGateway(gateway ports.EmbeddingsGateway, limiter ratelimit.Limiter) ports.EmbeddingsGateway {
	return &rateLimitedEmbeddingsGateway{
		gateway: gateway,
		limiter: limiter,
	}
}

func (g *rateLimitedEmbeddingsGateway) ComputeEmbeddings(
	ctx context.Context,
	request *gateway.ComputeEmbeddingsRequest,
) ([]gateway.Embedding, error) {
	if g == nil {
		return nil, fmt.Errorf("rate limited embeddings gateway is nil")
	}

	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// Estimate token count from the text chunks
	totalChars := 0
	for _, chunk := range request.Chunks() {
		totalChars += len(chunk)
	}
	tokenCount := totalChars / 4 // Rough estimate: ~4 chars per token

	if err := g.limiter.Wait(ctx, tokenCount); err != nil {
		return nil, fmt.Errorf("failed to acquire rate limit tokens: %w", err)
	}

	return g.gateway.ComputeEmbeddings(ctx, request)
}

func (g *rateLimitedEmbeddingsGateway) ComputeDistance(first, second gateway.Embedding) (float64, error) {
	if g == nil {
		return 0, fmt.Errorf("rate limited embeddings gateway is nil")
	}

	return g.gateway.ComputeDistance(first, second)
}
