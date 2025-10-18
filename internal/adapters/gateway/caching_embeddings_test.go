// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
)

// mockEmbGatewayForCaching is a simple mock for testing embeddings
type mockEmbGatewayForCaching struct {
	embeddings []domainGateway.Embedding
	err        error
	callCount  int
}

func (m *mockEmbGatewayForCaching) ComputeEmbeddings(ctx context.Context, request *domainGateway.ComputeEmbeddingsRequest) ([]domainGateway.Embedding, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return m.embeddings, nil
}

func (m *mockEmbGatewayForCaching) ComputeDistance(emb1, emb2 domainGateway.Embedding) (float64, error) {
	return 0.5, nil
}

func TestCachingEmbeddingsGateway_CacheHit(t *testing.T) {
	mockGateway := &mockEmbGatewayForCaching{
		embeddings: []domainGateway.Embedding{{0.1, 0.2, 0.3}},
	}
	mockCache := newMockCacheForCaching()

	gateway := newCachingEmbeddingsGateway(mockGateway, mockCache, false)

	request := domainGateway.NewComputeEmbeddingsRequest("model", []string{"text1"}, domainGateway.RetrievalDocument)

	// First call - should call gateway and cache result
	result1, err := gateway.ComputeEmbeddings(context.Background(), request)
	require.NoError(t, err)
	assert.Len(t, result1, 1)
	assert.Equal(t, 1, mockGateway.callCount)
	assert.Equal(t, 1, mockCache.setCalls)

	// Second call - should hit cache
	result2, err := gateway.ComputeEmbeddings(context.Background(), request)
	require.NoError(t, err)
	assert.Len(t, result2, 1)
	assert.Equal(t, 1, mockGateway.callCount, "Should not call gateway again")
	assert.Equal(t, 2, mockCache.getCalls)
}

func TestCachingEmbeddingsGateway_UpdateCache(t *testing.T) {
	mockGateway := &mockEmbGatewayForCaching{
		embeddings: []domainGateway.Embedding{{0.1, 0.2, 0.3}},
	}
	mockCache := newMockCacheForCaching()

	gateway := newCachingEmbeddingsGateway(mockGateway, mockCache, true)

	request := domainGateway.NewComputeEmbeddingsRequest("model", []string{"text1"}, domainGateway.RetrievalDocument)

	// Should skip cache and call gateway
	result, err := gateway.ComputeEmbeddings(context.Background(), request)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, 1, mockGateway.callCount)
	assert.Equal(t, 0, mockCache.getCalls, "Should not check cache when updateCache=true")
	assert.Equal(t, 1, mockCache.setCalls)
}

func TestCachingEmbeddingsGateway_GatewayError(t *testing.T) {
	expectedErr := errors.New("gateway error")
	mockGateway := &mockEmbGatewayForCaching{err: expectedErr}
	mockCache := newMockCacheForCaching()

	gateway := newCachingEmbeddingsGateway(mockGateway, mockCache, false)

	request := domainGateway.NewComputeEmbeddingsRequest("model", []string{"text1"}, domainGateway.RetrievalDocument)

	result, err := gateway.ComputeEmbeddings(context.Background(), request)
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, result)
	assert.Equal(t, 0, mockCache.setCalls, "Should not cache errors")
}

func TestCachingEmbeddingsGateway_NilRequest(t *testing.T) {
	mockGateway := &mockEmbGatewayForCaching{
		embeddings: []domainGateway.Embedding{{0.1, 0.2, 0.3}},
	}
	mockCache := newMockCacheForCaching()

	gateway := newCachingEmbeddingsGateway(mockGateway, mockCache, false)

	result, err := gateway.ComputeEmbeddings(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "request cannot be nil")
	assert.Nil(t, result)
}

func TestCachingEmbeddingsGateway_CreateCacheKey(t *testing.T) {
	mockGateway := &mockEmbGatewayForCaching{}
	mockCache := newMockCacheForCaching()

	gateway := newCachingEmbeddingsGateway(mockGateway, mockCache, false).(*cachingEmbeddingsGateway)

	req1 := domainGateway.NewComputeEmbeddingsRequest("model", []string{"text1"}, domainGateway.RetrievalDocument)
	req2 := domainGateway.NewComputeEmbeddingsRequest("model", []string{"text1"}, domainGateway.RetrievalDocument)

	key1 := gateway.createCacheKey(req1)
	key2 := gateway.createCacheKey(req2)

	assert.Equal(t, key1, key2, "Same parameters should produce same cache key")
	assert.Contains(t, key1, "emb:", "Cache key should have prefix")

	// Different parameters should produce different keys
	req3 := domainGateway.NewComputeEmbeddingsRequest("different-model", []string{"text1"}, domainGateway.RetrievalDocument)
	key3 := gateway.createCacheKey(req3)
	assert.NotEqual(t, key1, key3, "Different parameters should produce different cache keys")
}

func TestCachingEmbeddingsGateway_NilContext(t *testing.T) {
	mockGateway := &mockEmbGatewayForCaching{
		embeddings: []domainGateway.Embedding{{0.1, 0.2, 0.3}},
	}
	mockCache := newMockCacheForCaching()

	gateway := newCachingEmbeddingsGateway(mockGateway, mockCache, false)

	request := domainGateway.NewComputeEmbeddingsRequest("model", []string{"text1"}, domainGateway.RetrievalDocument)

	//nolint:staticcheck // intentionally testing nil context handling
	result, err := gateway.ComputeEmbeddings(nil, request)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cannot be nil")
	assert.Nil(t, result)
}

func TestCachingEmbeddingsGateway_NilGateway(t *testing.T) {
	var gateway *cachingEmbeddingsGateway = nil

	request := domainGateway.NewComputeEmbeddingsRequest("model", []string{"text1"}, domainGateway.RetrievalDocument)

	result, err := gateway.ComputeEmbeddings(context.Background(), request)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "caching embeddings gateway is nil")
	assert.Nil(t, result)
}

func TestCachingEmbeddingsGateway_CacheKeyDifferentChunks(t *testing.T) {
	mockGateway := &mockEmbGatewayForCaching{}
	mockCache := newMockCacheForCaching()

	gateway := newCachingEmbeddingsGateway(mockGateway, mockCache, false).(*cachingEmbeddingsGateway)

	req1 := domainGateway.NewComputeEmbeddingsRequest("model", []string{"text1", "text2"}, domainGateway.RetrievalDocument)
	req2 := domainGateway.NewComputeEmbeddingsRequest("model", []string{"text1"}, domainGateway.RetrievalDocument)

	key1 := gateway.createCacheKey(req1)
	key2 := gateway.createCacheKey(req2)

	assert.NotEqual(t, key1, key2, "Different chunks should produce different cache keys")
}

func TestCachingEmbeddingsGateway_CacheKeyDifferentTaskType(t *testing.T) {
	mockGateway := &mockEmbGatewayForCaching{}
	mockCache := newMockCacheForCaching()

	gateway := newCachingEmbeddingsGateway(mockGateway, mockCache, false).(*cachingEmbeddingsGateway)

	req1 := domainGateway.NewComputeEmbeddingsRequest("model", []string{"text1"}, domainGateway.RetrievalDocument)
	req2 := domainGateway.NewComputeEmbeddingsRequest("model", []string{"text1"}, domainGateway.RetrievalQuery)

	key1 := gateway.createCacheKey(req1)
	key2 := gateway.createCacheKey(req2)

	assert.NotEqual(t, key1, key2, "Different task types should produce different cache keys")
}

func TestCachingEmbeddingsGateway_CacheKeyDifferentDimensions(t *testing.T) {
	mockGateway := &mockEmbGatewayForCaching{}
	mockCache := newMockCacheForCaching()

	gateway := newCachingEmbeddingsGateway(mockGateway, mockCache, false).(*cachingEmbeddingsGateway)

	req1 := domainGateway.NewComputeEmbeddingsRequestWithDimensions("model", []string{"text1"}, domainGateway.RetrievalDocument, 512)
	req2 := domainGateway.NewComputeEmbeddingsRequestWithDimensions("model", []string{"text1"}, domainGateway.RetrievalDocument, 1024)

	key1 := gateway.createCacheKey(req1)
	key2 := gateway.createCacheKey(req2)

	assert.NotEqual(t, key1, key2, "Different dimensions should produce different cache keys")
}

func TestCachingEmbeddingsGateway_CacheGetError(t *testing.T) {
	mockGateway := &mockEmbGatewayForCaching{
		embeddings: []domainGateway.Embedding{{0.1, 0.2, 0.3}},
	}
	mockCache := newMockCacheForCaching()
	mockCache.getError = errors.New("cache get error")

	gateway := newCachingEmbeddingsGateway(mockGateway, mockCache, false)

	request := domainGateway.NewComputeEmbeddingsRequest("model", []string{"text1"}, domainGateway.RetrievalDocument)

	// Should fall back to gateway when cache get fails
	result, err := gateway.ComputeEmbeddings(context.Background(), request)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, 1, mockGateway.callCount)
}

func TestCachingEmbeddingsGateway_CacheSetError(t *testing.T) {
	mockGateway := &mockEmbGatewayForCaching{
		embeddings: []domainGateway.Embedding{{0.1, 0.2, 0.3}},
	}
	mockCache := newMockCacheForCaching()
	mockCache.setError = errors.New("cache set error")

	gateway := newCachingEmbeddingsGateway(mockGateway, mockCache, false)

	request := domainGateway.NewComputeEmbeddingsRequest("model", []string{"text1"}, domainGateway.RetrievalDocument)

	// Should succeed even if cache set fails
	result, err := gateway.ComputeEmbeddings(context.Background(), request)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, 1, mockGateway.callCount)
}

func TestCachingEmbeddingsGateway_MultipleChunks(t *testing.T) {
	mockGateway := &mockEmbGatewayForCaching{
		embeddings: []domainGateway.Embedding{
			{0.1, 0.2, 0.3},
			{0.4, 0.5, 0.6},
			{0.7, 0.8, 0.9},
		},
	}
	mockCache := newMockCacheForCaching()

	gateway := newCachingEmbeddingsGateway(mockGateway, mockCache, false)

	request := domainGateway.NewComputeEmbeddingsRequest("model", []string{"text1", "text2", "text3"}, domainGateway.RetrievalDocument)

	result, err := gateway.ComputeEmbeddings(context.Background(), request)
	require.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, 1, mockGateway.callCount)

	// Second call should hit cache
	result2, err := gateway.ComputeEmbeddings(context.Background(), request)
	require.NoError(t, err)
	assert.Len(t, result2, 3)
	assert.Equal(t, 1, mockGateway.callCount, "Should not call gateway again")
}
