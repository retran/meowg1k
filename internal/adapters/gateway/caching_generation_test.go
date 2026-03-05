// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

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

// mockCacheForCaching is a simple mock for caching tests.
type mockCacheForCaching struct {
	getError error
	setError error
	data     map[string]string
	getCalls int
	setCalls int
}

func newMockCacheForCaching() *mockCacheForCaching {
	return &mockCacheForCaching{
		data: make(map[string]string),
	}
}

func (m *mockCacheForCaching) Get(ctx context.Context, key string) (value string, found bool, err error) {
	m.getCalls++
	if m.getError != nil {
		return "", false, m.getError
	}
	value, found = m.data[key]
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

// mockGenGatewayForCaching is a simple mock for testing.
type mockGenGatewayForCaching struct {
	err       error
	response  string
	callCount int
}

func (m *mockGenGatewayForCaching) GenerateContent(ctx context.Context, request *domainGateway.GenerateContentRequest) (*domainGateway.GenerateContentResponse, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return &domainGateway.GenerateContentResponse{
		Blocks: []domainGateway.ContentBlock{{Kind: domainGateway.ContentBlockText, Text: m.response}},
	}, nil
}

func (m *mockGenGatewayForCaching) GenerateContentStream(ctx context.Context, request *domainGateway.GenerateContentRequest, callback domainGateway.StreamCallback) (*domainGateway.GenerateContentResponse, error) {
	resp, err := m.GenerateContent(ctx, request)
	if err != nil {
		return nil, err
	}
	return synthesizeStreamEvents(resp, callback)
}

func TestCachingGenerationGateway_CacheHit(t *testing.T) {
	mockGateway := &mockGenGatewayForCaching{response: "fresh"}
	mockCache := newMockCacheForCaching()

	gateway := newCachingGenerationGateway(mockGateway, mockCache, false)

	request := domainGateway.NewGenerateContentRequest("model", "sys", "user", 100)

	// First call - should call gateway and cache result
	result1, err := gateway.GenerateContent(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, "fresh", result1.Text())
	assert.Equal(t, 1, mockGateway.callCount)
	assert.Equal(t, 1, mockCache.setCalls)

	// Second call - should hit cache
	result2, err := gateway.GenerateContent(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, "fresh", result2.Text())
	assert.Equal(t, 1, mockGateway.callCount, "Should not call gateway again")
	assert.Equal(t, 2, mockCache.getCalls)
}

func TestCachingGenerationGateway_UpdateCache(t *testing.T) {
	mockGateway := &mockGenGatewayForCaching{response: "fresh"}
	mockCache := newMockCacheForCaching()

	gateway := newCachingGenerationGateway(mockGateway, mockCache, true)

	request := domainGateway.NewGenerateContentRequest("model", "sys", "user", 100)

	// Should skip cache and call gateway
	result, err := gateway.GenerateContent(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, "fresh", result.Text())
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
	assert.ErrorIs(t, err, expectedErr)
	assert.Empty(t, result.Text())
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
	assert.Equal(t, "fresh", result.Text())
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
	assert.Equal(t, "fresh", result.Text())
	assert.Equal(t, 1, mockCache.setCalls)
}

func TestCachingGenerationGateway_NilRequest(t *testing.T) {
	mockGateway := &mockGenGatewayForCaching{response: "fresh"}
	mockCache := newMockCacheForCaching()

	gateway := newCachingGenerationGateway(mockGateway, mockCache, false)

	result, err := gateway.GenerateContent(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "request cannot be nil")
	assert.Empty(t, result.Text())
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
