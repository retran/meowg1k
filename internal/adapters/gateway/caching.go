// Copyright © 2025 The meowg1k Authors.
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
			// TODO: add logging when logger is available in this context
			_ = err
		}
	}
	if err != nil {
		// TODO: add logging when logger is available in this context
		_ = err
	}

	return result, nil
}

// GenerateContentStream implements GenerationGateway with caching for streaming.
// It calls the inner gateway's stream, buffers all events, caches the aggregated
// response, and on cache hit replays events instantly through the callback.
func (g *cachingGenerationGateway) GenerateContentStream(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
	callback gateway.StreamCallback,
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
		if cached, hit := g.lookupCachedStream(ctx, cacheKey); hit {
			return cached, g.replayCachedStream(cached, callback)
		}
	}

	result, err := g.gateway.GenerateContentStream(ctx, request, callback)
	if err != nil {
		return nil, fmt.Errorf("failed to stream content: %w", err)
	}

	g.storeCache(ctx, cacheKey, result)

	return result, nil
}

// lookupCachedStream retrieves and deserializes a cached stream response.
// Returns (resp, true) on a valid cache hit, or (nil, false) on miss or error.
func (g *cachingGenerationGateway) lookupCachedStream(ctx context.Context, cacheKey string) (*gateway.GenerateContentResponse, bool) {
	cachedValue, found, err := g.cache.Get(ctx, cacheKey)
	if err != nil || !found {
		return nil, false
	}

	var cachedResp gateway.GenerateContentResponse
	if jsonErr := json.Unmarshal([]byte(cachedValue), &cachedResp); jsonErr != nil {
		return nil, false
	}

	return &cachedResp, true
}

// replayCachedStream fires stream callbacks for a cached response.
// A nil callback is a no-op; returns the first callback error encountered.
func (g *cachingGenerationGateway) replayCachedStream(cached *gateway.GenerateContentResponse, callback gateway.StreamCallback) error {
	if callback == nil {
		return nil
	}

	for _, block := range cached.Blocks {
		if cbErr := g.replayBlock(block, callback); cbErr != nil {
			return cbErr
		}
	}

	var usage *gateway.UsageMetadata
	if cached.Usage != nil {
		u := *cached.Usage
		usage = &u
	}

	return callback(gateway.StreamEvent{Kind: gateway.StreamEventDone, Usage: usage})
}

// replayBlock fires the appropriate stream event for a single cached content block.
func (g *cachingGenerationGateway) replayBlock(block gateway.ContentBlock, callback gateway.StreamCallback) error {
	switch block.Kind {
	case gateway.ContentBlockText:
		return callback(gateway.StreamEvent{Kind: gateway.StreamEventText, Delta: block.Text})
	case gateway.ContentBlockReasoning:
		return callback(gateway.StreamEvent{Kind: gateway.StreamEventThinking, Delta: block.Text})
	case gateway.ContentBlockToolCall:
		// Tool call blocks are not streamed as incremental events;
		// they are available in the final response.
	}

	return nil
}

// storeCache serializes result and writes it to the cache, silently ignoring errors.
func (g *cachingGenerationGateway) storeCache(ctx context.Context, cacheKey string, result *gateway.GenerateContentResponse) {
	encoded, marshalErr := json.Marshal(result)
	if marshalErr == nil {
		if setErr := g.cache.Set(ctx, cacheKey, string(encoded)); setErr != nil {
			_ = setErr
		}
	}
}

// createCacheKey generates a deterministic cache key from the request parameters.
// Uses SHA256 to create a fixed-length key from all parameters.
func (g *cachingGenerationGateway) createCacheKey(request *gateway.GenerateContentRequest) string {
	toolsJSON, err := request.ToolsJSON()
	if err != nil {
		toolsJSON = ""
	}

	// Include message history in the key so each conversation turn gets a distinct entry.
	messagesJSON := ""
	if msgs := request.Messages(); len(msgs) > 0 {
		if encoded, encErr := json.Marshal(msgs); encErr == nil {
			messagesJSON = string(encoded)
		}
	}

	data := fmt.Sprintf(
		"gen:%s:%s:%s:%s:%s:%d",
		request.Model(),
		request.SystemPrompt(),
		request.UserPrompt(),
		toolsJSON,
		messagesJSON,
		request.MaxOutputTokens(),
	)

	hash := sha256.Sum256([]byte(data))
	return "gen:" + hex.EncodeToString(hash[:])
}
