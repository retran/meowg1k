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
