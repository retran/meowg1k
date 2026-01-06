// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

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

func newRateLimitedGenerationGateway(innerGateway ports.GenerationGateway, limiter ratelimit.Limiter) ports.GenerationGateway {
	return &rateLimitedGenerationGateway{
		gateway: innerGateway,
		limiter: limiter,
	}
}

func (g *rateLimitedGenerationGateway) GenerateContent(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
) (*gateway.GenerateContentResponse, error) {
	if g == nil {
		return nil, fmt.Errorf("rate limited generation gateway is nil")
	}

	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	promptText := request.SystemPrompt() + request.UserPrompt()
	if msgs := request.Messages(); len(msgs) > 0 {
		promptText = ""
		for _, m := range msgs {
			promptText += string(m.Role) + ":" + m.Content + "\n"
		}
	}

	tokenCount := estimateTokenCount(promptText)

	if err := g.limiter.Wait(ctx, tokenCount); err != nil {
		return nil, fmt.Errorf("failed to acquire rate limit tokens: %w", err)
	}

	content, err := g.gateway.GenerateContent(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to write content: %w", err)
	}
	return content, nil
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
func newRateLimitedEmbeddingsGateway(innerGateway ports.EmbeddingsGateway, limiter ratelimit.Limiter) ports.EmbeddingsGateway {
	return &rateLimitedEmbeddingsGateway{
		gateway: innerGateway,
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

	embs, err := g.gateway.ComputeEmbeddings(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to compute embeddings: %w", err)
	}
	return embs, nil
}

func (g *rateLimitedEmbeddingsGateway) ComputeDistance(first, second gateway.Embedding) (float64, error) {
	if g == nil {
		return 0, fmt.Errorf("rate limited embeddings gateway is nil")
	}

	dist, err := g.gateway.ComputeDistance(first, second)
	if err != nil {
		return 0, fmt.Errorf("failed to compute distance: %w", err)
	}
	return dist, nil
}
