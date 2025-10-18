// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"

	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
)

// TestGenerateContentRequestAllMethods tests all methods of domainGateway.GenerateContentRequest
func TestGenerateContentRequestAllMethods(t *testing.T) {
	model := "test-model"
	system := "system prompt"
	user := "user prompt"
	maxTokens := 1000

	req := domainGateway.NewGenerateContentRequest(model, system, user, maxTokens)

	// Test constructor
	assert.NotNil(t, req)

	// Test all getter methods
	assert.Equal(t, model, req.Model())
	assert.Equal(t, system, req.SystemPrompt())
	assert.Equal(t, user, req.UserPrompt())
	assert.Equal(t, maxTokens, req.MaxOutputTokens())
}

// TestComputeEmbeddingsRequestAllMethods tests all methods of ComputeEmbeddingsRequest
func TestComputeEmbeddingsRequestAllMethods(t *testing.T) {
	model := "embedding-model"
	chunks := []string{"chunk1", "chunk2"}
	taskType := domainGateway.SemanticSimilarity

	// Test basic constructor
	req1 := domainGateway.NewComputeEmbeddingsRequest(model, chunks, taskType)
	assert.NotNil(t, req1)
	assert.Equal(t, model, req1.Model())
	assert.Equal(t, chunks, req1.Chunks())
	assert.Equal(t, taskType, req1.TaskType())
	assert.GreaterOrEqual(t, req1.Dimensions(), 0)

	// Test constructor with dimensions
	dimensions := 512
	req2 := domainGateway.NewComputeEmbeddingsRequestWithDimensions(model, chunks, taskType, dimensions)
	assert.NotNil(t, req2)
	assert.Equal(t, model, req2.Model())
	assert.Equal(t, chunks, req2.Chunks())
	assert.Equal(t, taskType, req2.TaskType())
	assert.Equal(t, dimensions, req2.Dimensions())
}

// TestComputeDistanceMixinAllCases tests ComputeDistance with various scenarios
func TestComputeDistanceMixinAllCases(t *testing.T) {
	mixin := &domainGateway.ComputeDistanceMixin{}

	// Test identical vectors
	e1 := domainGateway.Embedding{1.0, 2.0, 3.0}
	e2 := domainGateway.Embedding{1.0, 2.0, 3.0}
	dist, err := mixin.ComputeDistance(e1, e2)
	assert.NoError(t, err)
	assert.InDelta(t, 1.0, dist, 0.0001)

	// Test orthogonal vectors
	e3 := domainGateway.Embedding{1.0, 0.0}
	e4 := domainGateway.Embedding{0.0, 1.0}
	dist, err = mixin.ComputeDistance(e3, e4)
	assert.NoError(t, err)
	assert.InDelta(t, 0.0, dist, 0.0001)

	// Test different length vectors
	e5 := domainGateway.Embedding{1.0}
	e6 := domainGateway.Embedding{1.0, 2.0}
	_, err = mixin.ComputeDistance(e5, e6)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "same length")

	// Test empty vectors
	e7 := domainGateway.Embedding{}
	e8 := domainGateway.Embedding{}
	_, err = mixin.ComputeDistance(e7, e8)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not be empty")
}
