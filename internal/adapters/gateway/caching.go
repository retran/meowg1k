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
	"crypto/sha256"
	"encoding/hex"
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
func newCachingGenerationGateway(gateway ports.GenerationGateway, cache ports.CacheRepository, updateCache bool) ports.GenerationGateway {
	return &cachingGenerationGateway{
		gateway:     gateway,
		cache:       cache,
		updateCache: updateCache,
	}
}

// GenerateContent implements GenerationGateway with caching.
func (g *cachingGenerationGateway) GenerateContent(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
) (string, error) {
	if g == nil {
		return "", fmt.Errorf("caching generation gateway is nil")
	}

	if ctx == nil {
		return "", fmt.Errorf("context cannot be nil")
	}

	if request == nil {
		return "", fmt.Errorf("request cannot be nil")
	}

	cacheKey := g.createCacheKey(request)

	if !g.updateCache {
		if cachedValue, found, err := g.cache.Get(ctx, cacheKey); err == nil && found {
			return cachedValue, nil
		}
	}

	result, err := g.gateway.GenerateContent(ctx, request)
	if err != nil {
		return "", err
	}

	if err := g.cache.Set(ctx, cacheKey, result); err != nil {
		// Log error but don't fail the request
		// TODO: add logging when logger is available in this context
		_ = err
	}

	return result, nil
}

// createCacheKey generates a deterministic cache key from the request parameters.
// Uses SHA256 to create a fixed-length key from all parameters.
func (g *cachingGenerationGateway) createCacheKey(request *gateway.GenerateContentRequest) string {
	data := fmt.Sprintf(
		"gen:%s:%s:%s:%d",
		request.Model(),
		request.SystemPrompt(),
		request.UserPrompt(),
		request.MaxOutputTokens(),
	)

	hash := sha256.Sum256([]byte(data))
	return "gen:" + hex.EncodeToString(hash[:])
}
