// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

// cachingEmbeddingsGateway wraps an EmbeddingsGateway with caching functionality.
type cachingEmbeddingsGateway struct {
	gateway.ComputeDistanceMixin
	gateway     ports.EmbeddingsGateway
	cache       ports.CacheRepository
	updateCache bool // if true, skip cache lookup and always make fresh requests
}

// newCachingEmbeddingsGateway creates a new caching embeddings gateway.
func newCachingEmbeddingsGateway(gw ports.EmbeddingsGateway, cache ports.CacheRepository, updateCache bool) ports.EmbeddingsGateway {
	return &cachingEmbeddingsGateway{
		gateway:     gw,
		cache:       cache,
		updateCache: updateCache,
	}
}

// ComputeEmbeddings implements EmbeddingsGateway with caching.
func (g *cachingEmbeddingsGateway) ComputeEmbeddings(
	ctx context.Context,
	request *gateway.ComputeEmbeddingsRequest,
) ([]gateway.Embedding, error) {
	if g == nil {
		return nil, fmt.Errorf("caching embeddings gateway is nil")
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
			var embeddings []gateway.Embedding
			if err := json.Unmarshal([]byte(cachedValue), &embeddings); err == nil {
				return embeddings, nil
			}
		}
	}

	result, err := g.gateway.ComputeEmbeddings(ctx, request)
	if err != nil {
		return nil, err
	}

	if jsonData, err := json.Marshal(result); err == nil {
		if err := g.cache.Set(ctx, cacheKey, string(jsonData)); err != nil {
			// TODO: add logging when logger is available in this context
			_ = err
		}
	}

	return result, nil
}

// createCacheKey generates a deterministic cache key from the request parameters.
// Uses SHA256 to create a fixed-length key from all parameters.
func (g *cachingEmbeddingsGateway) createCacheKey(request *gateway.ComputeEmbeddingsRequest) string {
	chunksStr := strings.Join(request.Chunks(), "\x00")

	data := fmt.Sprintf(
		"emb:%s:%s:%s:%d",
		request.Model(),
		chunksStr,
		request.TaskType(),
		request.Dimensions(),
	)

	// Generate SHA256 hash
	hash := sha256.Sum256([]byte(data))
	return "emb:" + hex.EncodeToString(hash[:])
}
