package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mdGateway "github.com/retran/meowg1k/internal/models/gateway"
)

func TestNewGenerateContentRequest(t *testing.T) {
	model := "gpt-4"
	systemPrompt := "You are a helpful assistant"
	userPrompt := "Hello, world!"
	maxTokens := 100

	req := mdGateway.NewGenerateContentRequest(model, systemPrompt, userPrompt, maxTokens)

	assert.NotNil(t, req)
	assert.Equal(t, model, req.Model())
	assert.Equal(t, systemPrompt, req.SystemPrompt())
	assert.Equal(t, userPrompt, req.UserPrompt())
	assert.Equal(t, maxTokens, req.MaxOutputTokens())
}

func TestGenerateContentRequestGetters(t *testing.T) {
	req := mdGateway.NewGenerateContentRequest("claude-3", "System instructions", "User question", 500)

	assert.Equal(t, "claude-3", req.Model())
	assert.Equal(t, "System instructions", req.SystemPrompt())
	assert.Equal(t, "User question", req.UserPrompt())
	assert.Equal(t, 500, req.MaxOutputTokens())
}

func TestNewComputeEmbeddingsRequest(t *testing.T) {
	model := "text-embedding-ada-002"
	chunks := []string{"hello", "world", "test"}
	taskType := mdGateway.SemanticSimilarity

	req := mdGateway.NewComputeEmbeddingsRequest(model, chunks, taskType)

	assert.NotNil(t, req)
	assert.Equal(t, model, req.Model())
	assert.Equal(t, chunks, req.Chunks())
	assert.Equal(t, taskType, req.TaskType())
	// Dimensions should be set from registry (we can't predict the exact value)
	assert.GreaterOrEqual(t, req.Dimensions(), 0)
}

func TestNewComputeEmbeddingsRequestWithDimensions(t *testing.T) {
	model := "text-embedding-ada-002"
	chunks := []string{"hello", "world"}
	taskType := mdGateway.Classification
	dimensions := 512

	req := mdGateway.NewComputeEmbeddingsRequestWithDimensions(model, chunks, taskType, dimensions)

	assert.NotNil(t, req)
	assert.Equal(t, model, req.Model())
	assert.Equal(t, chunks, req.Chunks())
	assert.Equal(t, taskType, req.TaskType())
	assert.Equal(t, dimensions, req.Dimensions())
}

func TestComputeEmbeddingsRequestGetters(t *testing.T) {
	req := mdGateway.NewComputeEmbeddingsRequestWithDimensions("voyage-large-2", []string{"text1", "text2"}, mdGateway.RetrievalDocument, 1024)

	assert.Equal(t, "voyage-large-2", req.Model())
	assert.Equal(t, []string{"text1", "text2"}, req.Chunks())
	assert.Equal(t, mdGateway.RetrievalDocument, req.TaskType())
	assert.Equal(t, 1024, req.Dimensions())
}

func TestComputeDistanceMixin(t *testing.T) {
	mixin := &ComputeDistanceMixin{}

	tests := []struct {
		name        string
		a           mdGateway.Embedding
		b           mdGateway.Embedding
		expected    float64
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Identical vectors",
			a:           mdGateway.Embedding{1.0, 2.0, 3.0},
			b:           mdGateway.Embedding{1.0, 2.0, 3.0},
			expected:    1.0,
			expectError: false,
		},
		{
			name:        "Orthogonal vectors",
			a:           mdGateway.Embedding{1.0, 0.0},
			b:           mdGateway.Embedding{0.0, 1.0},
			expected:    0.0,
			expectError: false,
		},
		{
			name:        "Opposite vectors",
			a:           mdGateway.Embedding{1.0, 0.0},
			b:           mdGateway.Embedding{-1.0, 0.0},
			expected:    -1.0,
			expectError: false,
		},
		{
			name:        "Different length vectors",
			a:           mdGateway.Embedding{1.0, 2.0},
			b:           mdGateway.Embedding{1.0, 2.0, 3.0},
			expectError: true,
			errorMsg:    "vectors must have the same length",
		},
		{
			name:        "Empty vectors",
			a:           mdGateway.Embedding{},
			b:           mdGateway.Embedding{},
			expectError: true,
			errorMsg:    "vectors must not be empty",
		},
		{
			name:        "Zero magnitude vector a",
			a:           mdGateway.Embedding{0.0, 0.0},
			b:           mdGateway.Embedding{1.0, 0.0},
			expected:    0.0,
			expectError: false,
		},
		{
			name:        "Zero magnitude vector b",
			a:           mdGateway.Embedding{1.0, 0.0},
			b:           mdGateway.Embedding{0.0, 0.0},
			expected:    0.0,
			expectError: false,
		},
		{
			name:        "Regular similarity calculation",
			a:           mdGateway.Embedding{1.0, 2.0, 3.0},
			b:           mdGateway.Embedding{2.0, 4.0, 6.0},
			expected:    1.0, // These are parallel vectors
			expectError: false,
		},
		{
			name:        "Partial similarity",
			a:           mdGateway.Embedding{1.0, 1.0},
			b:           mdGateway.Embedding{1.0, 0.0},
			expected:    0.7071067811865475, // cos(45°) ≈ 0.707
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mixin.ComputeDistance(tt.a, tt.b)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.InDelta(t, tt.expected, result, 1e-10) // Allow for floating point precision
			}
		})
	}
}

func TestTaskTypeConstants(t *testing.T) {
	// Test that all task type constants are properly defined
	taskTypes := []mdGateway.TaskType{
		mdGateway.SemanticSimilarity,
		mdGateway.Classification,
		mdGateway.Clustering,
		mdGateway.RetrievalDocument,
		mdGateway.RetrievalQuery,
		mdGateway.CodeRetrievalQuery,
		mdGateway.QuestionAnswering,
		mdGateway.FactVerification,
	}

	for _, taskType := range taskTypes {
		assert.NotEmpty(t, string(taskType), "TaskType should not be empty")
	}

	// Test specific values
	assert.Equal(t, "SEMANTIC_SIMILARITY", string(mdGateway.SemanticSimilarity))
	assert.Equal(t, "CLASSIFICATION", string(mdGateway.Classification))
	assert.Equal(t, "CLUSTERING", string(mdGateway.Clustering))
	assert.Equal(t, "RETRIEVAL_DOCUMENT", string(mdGateway.RetrievalDocument))
	assert.Equal(t, "RETRIEVAL_QUERY", string(mdGateway.RetrievalQuery))
	assert.Equal(t, "CODE_RETRIEVAL_QUERY", string(mdGateway.CodeRetrievalQuery))
	assert.Equal(t, "QUESTION_ANSWERING", string(mdGateway.QuestionAnswering))
	assert.Equal(t, "FACT_VERIFICATION", string(mdGateway.FactVerification))
}

func TestNewGenerateContentRequestGetters(t *testing.T) {
	model := "gpt-4"
	systemPrompt := "You are a helpful assistant"
	userPrompt := "Hello, world!"
	maxTokens := 100

	req := mdGateway.NewGenerateContentRequest(model, systemPrompt, userPrompt, maxTokens)

	// Test all getter methods
	assert.Equal(t, model, req.Model())
	assert.Equal(t, systemPrompt, req.SystemPrompt())
	assert.Equal(t, userPrompt, req.UserPrompt())
	assert.Equal(t, maxTokens, req.MaxOutputTokens())
}

func TestNewComputeEmbeddingsRequestGetters(t *testing.T) {
	model := "text-embedding-ada-002"
	chunks := []string{"hello", "world", "test"}
	taskType := mdGateway.SemanticSimilarity

	req := mdGateway.NewComputeEmbeddingsRequest(model, chunks, taskType)

	// Test all getter methods
	assert.Equal(t, model, req.Model())
	assert.Equal(t, chunks, req.Chunks())
	assert.Equal(t, taskType, req.TaskType())
	assert.GreaterOrEqual(t, req.Dimensions(), 0)
}

func TestNewComputeEmbeddingsRequestWithDimensionsGetters(t *testing.T) {
	model := "text-embedding-ada-002"
	chunks := []string{"hello", "world"}
	taskType := mdGateway.Classification
	dimensions := 512

	req := mdGateway.NewComputeEmbeddingsRequestWithDimensions(model, chunks, taskType, dimensions)

	// Test all getter methods
	assert.Equal(t, model, req.Model())
	assert.Equal(t, chunks, req.Chunks())
	assert.Equal(t, taskType, req.TaskType())
	assert.Equal(t, dimensions, req.Dimensions())
}

func TestComputeDistance(t *testing.T) {
	mixin := &ComputeDistanceMixin{}

	embedding1 := mdGateway.Embedding{1.0, 0.0, 0.0}
	embedding2 := mdGateway.Embedding{0.0, 1.0, 0.0}
	embedding3 := mdGateway.Embedding{1.0, 0.0, 0.0}

	// Test distance between different vectors
	distance12, err := mixin.ComputeDistance(embedding1, embedding2)
	require.NoError(t, err)
	assert.InDelta(t, 0.0, distance12, 0.001) // Should be 0 for orthogonal vectors

	// Test distance between identical vectors
	distance13, err := mixin.ComputeDistance(embedding1, embedding3)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, distance13, 0.001) // Should be 1 for identical vectors

	// Test empty vectors
	empty1 := mdGateway.Embedding{}
	empty2 := mdGateway.Embedding{}
	_, err = mixin.ComputeDistance(empty1, empty2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vectors must not be empty")

	// Test vectors of different lengths
	short := mdGateway.Embedding{1.0}
	long := mdGateway.Embedding{1.0, 0.0, 0.0}
	_, err = mixin.ComputeDistance(short, long)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vectors must have the same length")
}
