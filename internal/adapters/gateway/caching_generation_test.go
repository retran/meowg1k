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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
)

// mockCacheForCaching is a simple mock for caching tests
type mockCacheForCaching struct {
	data     map[string]string
	getCalls int
	setCalls int
	getError error
	setError error
}

func newMockCacheForCaching() *mockCacheForCaching {
	return &mockCacheForCaching{
		data: make(map[string]string),
	}
}

func (m *mockCacheForCaching) Get(ctx context.Context, key string) (string, bool, error) {
	m.getCalls++
	if m.getError != nil {
		return "", false, m.getError
	}
	value, found := m.data[key]
	return value, found, nil
}

func (m *mockCacheForCaching) Set(ctx context.Context, key, value string) error {
	m.setCalls++
	if m.setError != nil {
		return m.setError
	}
	m.data[key] = value
	return nil
}

func (m *mockCacheForCaching) Purge(ctx context.Context, ttl time.Duration) error {
	return nil
}

// mockGenGatewayForCaching is a simple mock for testing
type mockGenGatewayForCaching struct {
	response  string
	err       error
	callCount int
}

func (m *mockGenGatewayForCaching) GenerateContent(ctx context.Context, request *domainGateway.GenerateContentRequest) (string, error) {
	m.callCount++
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func TestCachingGenerationGateway_CacheHit(t *testing.T) {
	mockGateway := &mockGenGatewayForCaching{response: "fresh"}
	mockCache := newMockCacheForCaching()

	gateway := newCachingGenerationGateway(mockGateway, mockCache, false)

	request := domainGateway.NewGenerateContentRequest("model", "sys", "user", 100)

	// First call - should call gateway and cache result
	result1, err := gateway.GenerateContent(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, "fresh", result1)
	assert.Equal(t, 1, mockGateway.callCount)
	assert.Equal(t, 1, mockCache.setCalls)

	// Second call - should hit cache
	result2, err := gateway.GenerateContent(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, "fresh", result2)
	assert.Equal(t, 1, mockGateway.callCount, "Should not call gateway again")
	assert.Equal(t, 2, mockCache.getCalls)
}

func TestCachingGenerationGateway_UpdateCache(t *testing.T) {
	mockGateway := &mockGenGatewayForCaching{response: "fresh"}
	mockCache := newMockCacheForCaching()

	// Pre-populate cache
	ctx := context.Background()
	mockCache.data["test-key"] = "old"

	gateway := newCachingGenerationGateway(mockGateway, mockCache, true)

	request := domainGateway.NewGenerateContentRequest("model", "sys", "user", 100)

	// Should skip cache and call gateway
	result, err := gateway.GenerateContent(ctx, request)
	require.NoError(t, err)
	assert.Equal(t, "fresh", result)
	assert.Equal(t, 1, mockGateway.callCount)
	assert.Equal(t, 0, mockCache.getCalls, "Should not check cache when updateCache=true")
	assert.Equal(t, 1, mockCache.setCalls)
}

func TestCachingGenerationGateway_GatewayError(t *testing.T) {
	expectedErr := errors.New("gateway error")
	mockGateway := &mockGenGatewayForCaching{err: expectedErr}
	mockCache := newMockCacheForCaching()

	gateway := newCachingGenerationGateway(mockGateway, mockCache, false)

	request := domainGateway.NewGenerateContentRequest("model", "sys", "user", 100)

	result, err := gateway.GenerateContent(context.Background(), request)
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Empty(t, result)
	assert.Equal(t, 0, mockCache.setCalls, "Should not cache errors")
}

func TestCachingGenerationGateway_CacheGetError(t *testing.T) {
	mockGateway := &mockGenGatewayForCaching{response: "fresh"}
	mockCache := newMockCacheForCaching()
	mockCache.getError = errors.New("cache error")

	gateway := newCachingGenerationGateway(mockGateway, mockCache, false)

	request := domainGateway.NewGenerateContentRequest("model", "sys", "user", 100)

	// Should fallback to gateway on cache error
	result, err := gateway.GenerateContent(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, "fresh", result)
	assert.Equal(t, 1, mockGateway.callCount)
}

func TestCachingGenerationGateway_CacheSetError(t *testing.T) {
	mockGateway := &mockGenGatewayForCaching{response: "fresh"}
	mockCache := newMockCacheForCaching()
	mockCache.setError = errors.New("cache set error")

	gateway := newCachingGenerationGateway(mockGateway, mockCache, false)

	request := domainGateway.NewGenerateContentRequest("model", "sys", "user", 100)

	// Should still return result even if cache set fails
	result, err := gateway.GenerateContent(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, "fresh", result)
	assert.Equal(t, 1, mockCache.setCalls)
}

func TestCachingGenerationGateway_NilRequest(t *testing.T) {
	mockGateway := &mockGenGatewayForCaching{response: "fresh"}
	mockCache := newMockCacheForCaching()

	gateway := newCachingGenerationGateway(mockGateway, mockCache, false)

	result, err := gateway.GenerateContent(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "request cannot be nil")
	assert.Empty(t, result)
}

func TestCachingGenerationGateway_CreateCacheKey(t *testing.T) {
	mockGateway := &mockGenGatewayForCaching{}
	mockCache := newMockCacheForCaching()

	gateway := newCachingGenerationGateway(mockGateway, mockCache, false).(*cachingGenerationGateway)

	req1 := domainGateway.NewGenerateContentRequest("model", "sys", "user", 100)
	req2 := domainGateway.NewGenerateContentRequest("model", "sys", "user", 100)

	key1 := gateway.createCacheKey(req1)
	key2 := gateway.createCacheKey(req2)

	assert.Equal(t, key1, key2, "Same parameters should produce same cache key")
	assert.Contains(t, key1, "gen:", "Cache key should have prefix")

	// Different parameters should produce different keys
	req3 := domainGateway.NewGenerateContentRequest("different-model", "sys", "user", 100)
	key3 := gateway.createCacheKey(req3)
	assert.NotEqual(t, key1, key3, "Different parameters should produce different cache keys")
}
