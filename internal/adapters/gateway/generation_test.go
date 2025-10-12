package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
)

func TestNewGenerateContentRequest(t *testing.T) {
	model := "gpt-4"
	systemPrompt := "You are a helpful assistant"
	userPrompt := "Hello, world!"
	maxTokens := 100

	req := domainGateway.NewGenerateContentRequest(model, systemPrompt, userPrompt, maxTokens)

	assert.NotNil(t, req)
	assert.Equal(t, model, req.Model())
	assert.Equal(t, systemPrompt, req.SystemPrompt())
	assert.Equal(t, userPrompt, req.UserPrompt())
	assert.Equal(t, maxTokens, req.MaxOutputTokens())
}

func TestGenerateContentRequestGetters(t *testing.T) {
	req := domainGateway.NewGenerateContentRequest("claude-3", "System instructions", "User question", 500)

	assert.Equal(t, "claude-3", req.Model())
	assert.Equal(t, "System instructions", req.SystemPrompt())
	assert.Equal(t, "User question", req.UserPrompt())
	assert.Equal(t, 500, req.MaxOutputTokens())
}

func TestNewComputeEmbeddingsRequest(t *testing.T) {
	model := "text-embedding-ada-002"
	chunks := []string{"hello", "world", "test"}
	taskType := domainGateway.SemanticSimilarity

	req := domainGateway.NewComputeEmbeddingsRequest(model, chunks, taskType)

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
	taskType := domainGateway.Classification
	dimensions := 512

	req := domainGateway.NewComputeEmbeddingsRequestWithDimensions(model, chunks, taskType, dimensions)

	assert.NotNil(t, req)
	assert.Equal(t, model, req.Model())
	assert.Equal(t, chunks, req.Chunks())
	assert.Equal(t, taskType, req.TaskType())
	assert.Equal(t, dimensions, req.Dimensions())
}

func TestComputeEmbeddingsRequestGetters(t *testing.T) {
	req := domainGateway.NewComputeEmbeddingsRequestWithDimensions("voyage-large-2", []string{"text1", "text2"}, domainGateway.RetrievalDocument, 1024)

	assert.Equal(t, "voyage-large-2", req.Model())
	assert.Equal(t, []string{"text1", "text2"}, req.Chunks())
	assert.Equal(t, domainGateway.RetrievalDocument, req.TaskType())
	assert.Equal(t, 1024, req.Dimensions())
}

func TestComputeDistanceMixin(t *testing.T) {
	mixin := &domainGateway.ComputeDistanceMixin{}

	tests := []struct {
		name        string
		a           domainGateway.Embedding
		b           domainGateway.Embedding
		expected    float64
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Identical vectors",
			a:           domainGateway.Embedding{1.0, 2.0, 3.0},
			b:           domainGateway.Embedding{1.0, 2.0, 3.0},
			expected:    1.0,
			expectError: false,
		},
		{
			name:        "Orthogonal vectors",
			a:           domainGateway.Embedding{1.0, 0.0},
			b:           domainGateway.Embedding{0.0, 1.0},
			expected:    0.0,
			expectError: false,
		},
		{
			name:        "Opposite vectors",
			a:           domainGateway.Embedding{1.0, 0.0},
			b:           domainGateway.Embedding{-1.0, 0.0},
			expected:    -1.0,
			expectError: false,
		},
		{
			name:        "Different length vectors",
			a:           domainGateway.Embedding{1.0, 2.0},
			b:           domainGateway.Embedding{1.0, 2.0, 3.0},
			expectError: true,
			errorMsg:    "vectors must have the same length",
		},
		{
			name:        "Empty vectors",
			a:           domainGateway.Embedding{},
			b:           domainGateway.Embedding{},
			expectError: true,
			errorMsg:    "vectors must not be empty",
		},
		{
			name:        "Zero magnitude vector a",
			a:           domainGateway.Embedding{0.0, 0.0},
			b:           domainGateway.Embedding{1.0, 0.0},
			expected:    0.0,
			expectError: false,
		},
		{
			name:        "Zero magnitude vector b",
			a:           domainGateway.Embedding{1.0, 0.0},
			b:           domainGateway.Embedding{0.0, 0.0},
			expected:    0.0,
			expectError: false,
		},
		{
			name:        "Regular similarity calculation",
			a:           domainGateway.Embedding{1.0, 2.0, 3.0},
			b:           domainGateway.Embedding{2.0, 4.0, 6.0},
			expected:    1.0, // These are parallel vectors
			expectError: false,
		},
		{
			name:        "Partial similarity",
			a:           domainGateway.Embedding{1.0, 1.0},
			b:           domainGateway.Embedding{1.0, 0.0},
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
	taskTypes := []domainGateway.TaskType{
		domainGateway.SemanticSimilarity,
		domainGateway.Classification,
		domainGateway.Clustering,
		domainGateway.RetrievalDocument,
		domainGateway.RetrievalQuery,
		domainGateway.CodeRetrievalQuery,
		domainGateway.QuestionAnswering,
		domainGateway.FactVerification,
	}

	for _, taskType := range taskTypes {
		assert.NotEmpty(t, string(taskType), "TaskType should not be empty")
	}

	// Test specific values
	assert.Equal(t, "SEMANTIC_SIMILARITY", string(domainGateway.SemanticSimilarity))
	assert.Equal(t, "CLASSIFICATION", string(domainGateway.Classification))
	assert.Equal(t, "CLUSTERING", string(domainGateway.Clustering))
	assert.Equal(t, "RETRIEVAL_DOCUMENT", string(domainGateway.RetrievalDocument))
	assert.Equal(t, "RETRIEVAL_QUERY", string(domainGateway.RetrievalQuery))
	assert.Equal(t, "CODE_RETRIEVAL_QUERY", string(domainGateway.CodeRetrievalQuery))
	assert.Equal(t, "QUESTION_ANSWERING", string(domainGateway.QuestionAnswering))
	assert.Equal(t, "FACT_VERIFICATION", string(domainGateway.FactVerification))
}

func TestNewGenerateContentRequestGetters(t *testing.T) {
	model := "gpt-4"
	systemPrompt := "You are a helpful assistant"
	userPrompt := "Hello, world!"
	maxTokens := 100

	req := domainGateway.NewGenerateContentRequest(model, systemPrompt, userPrompt, maxTokens)

	// Test all getter methods
	assert.Equal(t, model, req.Model())
	assert.Equal(t, systemPrompt, req.SystemPrompt())
	assert.Equal(t, userPrompt, req.UserPrompt())
	assert.Equal(t, maxTokens, req.MaxOutputTokens())
	// Parameters should be nil by default
	assert.Nil(t, req.Temperature())
	assert.Nil(t, req.TopP())
	assert.Nil(t, req.TopK())
}

func TestGenerateContentRequestWithParameters(t *testing.T) {
	model := "gpt-4"
	systemPrompt := "You are a helpful assistant"
	userPrompt := "Hello, world!"
	maxTokens := 100
	temperature := 0.7
	topP := 0.9
	topK := 40

	req := domainGateway.NewGenerateContentRequest(model, systemPrompt, userPrompt, maxTokens).
		WithTemperature(&temperature).
		WithTopP(&topP).
		WithTopK(&topK)

	// Test all getter methods including parameters
	assert.Equal(t, model, req.Model())
	assert.Equal(t, systemPrompt, req.SystemPrompt())
	assert.Equal(t, userPrompt, req.UserPrompt())
	assert.Equal(t, maxTokens, req.MaxOutputTokens())
	assert.NotNil(t, req.Temperature())
	assert.Equal(t, temperature, *req.Temperature())
	assert.NotNil(t, req.TopP())
	assert.Equal(t, topP, *req.TopP())
	assert.NotNil(t, req.TopK())
	assert.Equal(t, topK, *req.TopK())
}

func TestGenerateContentRequestWithPartialParameters(t *testing.T) {
	model := "claude-3"
	systemPrompt := "System instructions"
	userPrompt := "User question"
	maxTokens := 500
	temperature := 0.5

	// Test with only temperature set
	req := domainGateway.NewGenerateContentRequest(model, systemPrompt, userPrompt, maxTokens).
		WithTemperature(&temperature)

	assert.Equal(t, model, req.Model())
	assert.NotNil(t, req.Temperature())
	assert.Equal(t, temperature, *req.Temperature())
	assert.Nil(t, req.TopP())
	assert.Nil(t, req.TopK())
}

func TestGenerateContentRequestWithAllParameters(t *testing.T) {
	model := "gpt-4"
	systemPrompt := "You are a helpful assistant"
	userPrompt := "Hello, world!"
	maxTokens := 100
	temperature := 0.7
	topP := 0.9
	topK := 40
	frequencyPenalty := 0.5
	presencePenalty := 0.3
	seed := 12345
	stop := []string{"STOP", "END"}

	req := domainGateway.NewGenerateContentRequest(model, systemPrompt, userPrompt, maxTokens).
		WithTemperature(&temperature).
		WithTopP(&topP).
		WithTopK(&topK).
		WithFrequencyPenalty(&frequencyPenalty).
		WithPresencePenalty(&presencePenalty).
		WithSeed(&seed).
		WithStop(stop)

	// Test all getter methods including new parameters
	assert.Equal(t, model, req.Model())
	assert.Equal(t, systemPrompt, req.SystemPrompt())
	assert.Equal(t, userPrompt, req.UserPrompt())
	assert.Equal(t, maxTokens, req.MaxOutputTokens())
	
	assert.NotNil(t, req.Temperature())
	assert.Equal(t, temperature, *req.Temperature())
	
	assert.NotNil(t, req.TopP())
	assert.Equal(t, topP, *req.TopP())
	
	assert.NotNil(t, req.TopK())
	assert.Equal(t, topK, *req.TopK())
	
	assert.NotNil(t, req.FrequencyPenalty())
	assert.Equal(t, frequencyPenalty, *req.FrequencyPenalty())
	
	assert.NotNil(t, req.PresencePenalty())
	assert.Equal(t, presencePenalty, *req.PresencePenalty())
	
	assert.NotNil(t, req.Seed())
	assert.Equal(t, seed, *req.Seed())
	
	assert.Equal(t, stop, req.Stop())
}

func TestGenerateContentRequestWithPenaltyParameters(t *testing.T) {
	model := "gpt-4"
	systemPrompt := "System"
	userPrompt := "User"
	maxTokens := 100
	frequencyPenalty := 1.5
	presencePenalty := -0.5

	req := domainGateway.NewGenerateContentRequest(model, systemPrompt, userPrompt, maxTokens).
		WithFrequencyPenalty(&frequencyPenalty).
		WithPresencePenalty(&presencePenalty)

	assert.NotNil(t, req.FrequencyPenalty())
	assert.Equal(t, frequencyPenalty, *req.FrequencyPenalty())
	assert.NotNil(t, req.PresencePenalty())
	assert.Equal(t, presencePenalty, *req.PresencePenalty())
	// Other parameters should be nil
	assert.Nil(t, req.Temperature())
	assert.Nil(t, req.TopP())
	assert.Nil(t, req.Seed())
}

func TestNewComputeEmbeddingsRequestGetters(t *testing.T) {
	model := "text-embedding-ada-002"
	chunks := []string{"hello", "world", "test"}
	taskType := domainGateway.SemanticSimilarity

	req := domainGateway.NewComputeEmbeddingsRequest(model, chunks, taskType)

	// Test all getter methods
	assert.Equal(t, model, req.Model())
	assert.Equal(t, chunks, req.Chunks())
	assert.Equal(t, taskType, req.TaskType())
	assert.GreaterOrEqual(t, req.Dimensions(), 0)
}

func TestNewComputeEmbeddingsRequestWithDimensionsGetters(t *testing.T) {
	model := "text-embedding-ada-002"
	chunks := []string{"hello", "world"}
	taskType := domainGateway.Classification
	dimensions := 512

	req := domainGateway.NewComputeEmbeddingsRequestWithDimensions(model, chunks, taskType, dimensions)

	// Test all getter methods
	assert.Equal(t, model, req.Model())
	assert.Equal(t, chunks, req.Chunks())
	assert.Equal(t, taskType, req.TaskType())
	assert.Equal(t, dimensions, req.Dimensions())
}

func TestComputeDistance(t *testing.T) {
	mixin := &domainGateway.ComputeDistanceMixin{}

	embedding1 := domainGateway.Embedding{1.0, 0.0, 0.0}
	embedding2 := domainGateway.Embedding{0.0, 1.0, 0.0}
	embedding3 := domainGateway.Embedding{1.0, 0.0, 0.0}

	// Test distance between different vectors
	distance12, err := mixin.ComputeDistance(embedding1, embedding2)
	require.NoError(t, err)
	assert.InDelta(t, 0.0, distance12, 0.001) // Should be 0 for orthogonal vectors

	// Test distance between identical vectors
	distance13, err := mixin.ComputeDistance(embedding1, embedding3)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, distance13, 0.001) // Should be 1 for identical vectors

	// Test empty vectors
	empty1 := domainGateway.Embedding{}
	empty2 := domainGateway.Embedding{}
	_, err = mixin.ComputeDistance(empty1, empty2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vectors must not be empty")

	// Test vectors of different lengths
	short := domainGateway.Embedding{1.0}
	long := domainGateway.Embedding{1.0, 0.0, 0.0}
	_, err = mixin.ComputeDistance(short, long)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vectors must have the same length")
}
