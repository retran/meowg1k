// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

// cachingGenerationGateway wraps a GenerationGateway with caching functionality.
type cachingGenerationGateway struct {
	gateway     ports.GenerationGateway
	cache       ports.CacheRepository
	updateCache bool // if true, skip cache lookup and always make fresh requests
}

// newCachingGenerationGateway creates a new caching generation gateway.
func newCachingGenerationGateway(innerGateway ports.GenerationGateway, cache ports.CacheRepository, updateCache bool) ports.GenerationGateway {
	return &cachingGenerationGateway{
		gateway:     innerGateway,
		cache:       cache,
		updateCache: updateCache,
	}
}

// GenerateContent implements GenerationGateway with caching.
func (g *cachingGenerationGateway) GenerateContent(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
) (*gateway.GenerateContentResponse, error) {
	if g == nil {
		return nil, fmt.Errorf("caching generation gateway is nil")
	}

	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	cacheKey := g.createCacheKey(request)

	if !g.updateCache {
		if cachedValue, found, err := g.cache.Get(ctx, cacheKey); err == nil && found {
			var cachedResp gateway.GenerateContentResponse
			if err := json.Unmarshal([]byte(cachedValue), &cachedResp); err == nil {
				return &cachedResp, nil
			}
		}
	}

	result, err := g.gateway.GenerateContent(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to write content: %w", err)
	}

	encoded, err := json.Marshal(result)
	if err == nil {
		if err := g.cache.Set(ctx, cacheKey, string(encoded)); err != nil {
			// Log error but don't fail the request
			// TODO: add logging when logger is available in this context
			_ = err
		}
	}
	if err != nil {
		// Log error but don't fail the request
		// TODO: add logging when logger is available in this context
		_ = err
	}

	return result, nil
}

// createCacheKey generates a deterministic cache key from the request parameters.
// Uses SHA256 to create a fixed-length key from all parameters.
func (g *cachingGenerationGateway) createCacheKey(request *gateway.GenerateContentRequest) string {
	toolsJSON, _ := request.ToolsJSON()
	data := fmt.Sprintf(
		"gen:%s:%s:%s:%s:%d",
		request.Model(),
		request.SystemPrompt(),
		request.UserPrompt(),
		toolsJSON,
		request.MaxOutputTokens(),
	)

	hash := sha256.Sum256([]byte(data))
	return "gen:" + hex.EncodeToString(hash[:])
}
