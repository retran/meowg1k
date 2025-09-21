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
	"fmt"
	"math"

	"github.com/retran/meowg1k/internal/services/llm/registry"
)

// EmbeddingsGateway defines the contract for a client that computes text embeddings
// and measures the distance between them.
type EmbeddingsGateway interface {
	// ComputeEmbeddings computes the vector embedding for the given text.
	ComputeEmbeddings(ctx context.Context, request *ComputeEmbeddingsRequest) ([]Embedding, error)
	// ComputeDistance calculates a similarity or distance score between two embeddings.
	// The exact metric (e.g., cosine similarity) depends on the implementation.
	ComputeDistance(first, second Embedding) (float64, error)
}

// Embedding represents a text embedding as a vector of floating-point numbers.
type Embedding []float64

// TaskType specifies the intended use case for the text embedding. This allows the model
// to produce higher-quality embeddings tailored to the specific task.
type TaskType string

const (
	SemanticSimilarity TaskType = "SEMANTIC_SIMILARITY"
	Classification     TaskType = "CLASSIFICATION"
	Clustering         TaskType = "CLUSTERING"
	RetrievalDocument  TaskType = "RETRIEVAL_DOCUMENT"
	RetrievalQuery     TaskType = "RETRIEVAL_QUERY"
	CodeRetrievalQuery TaskType = "CODE_RETRIEVAL_QUERY"
	QuestionAnswering  TaskType = "QUESTION_ANSWERING"
	FactVerification   TaskType = "FACT_VERIFICATION"
)

// ComputeEmbeddingsRequest holds the parameters for a text embedding request.
type ComputeEmbeddingsRequest struct {
	model      string
	chunks     []string
	taskType   TaskType
	dimensions int // Output dimensionality (optional, uses model default if 0)
}

// NewComputeEmbeddingsRequest creates and returns a new ComputeEmbeddingsRequest.
// If the model has a default embedding dimension in the registry, it will be automatically used.
func NewComputeEmbeddingsRequest(model string, chunks []string, taskType TaskType) *ComputeEmbeddingsRequest {
	// Get the default embedding dimension from the model registry
	defaultDim := registry.DefaultService.GetDefaultEmbedDimension(model)
	return &ComputeEmbeddingsRequest{
		model:      model,
		chunks:     chunks,
		taskType:   taskType,
		dimensions: defaultDim,
	}
}

// NewComputeEmbeddingsRequestWithDimensions creates a new ComputeEmbeddingsRequest with custom dimensions.
func NewComputeEmbeddingsRequestWithDimensions(model string, chunks []string, taskType TaskType, dimensions int) *ComputeEmbeddingsRequest {
	return &ComputeEmbeddingsRequest{
		model:      model,
		chunks:     chunks,
		taskType:   taskType,
		dimensions: dimensions,
	}
}

// Model returns the model name for the embedding request.
func (r *ComputeEmbeddingsRequest) Model() string {
	return r.model
}

// Chunks returns the text chunks to be embedded.
func (r *ComputeEmbeddingsRequest) Chunks() []string {
	return r.chunks
}

// TaskType returns the specified task type for the embedding.
func (r *ComputeEmbeddingsRequest) TaskType() TaskType {
	return r.taskType
}

// Dimensions returns the output dimensionality for the embedding.
func (r *ComputeEmbeddingsRequest) Dimensions() int {
	return r.dimensions
}

type ComputeDistanceMixin struct {
}

// ComputeDistance calculates the cosine similarity between two embeddings.
// It returns a value between -1 (opposite) and 1 (identical), where 0 indicates orthogonality.
func (g *ComputeDistanceMixin) ComputeDistance(a, b Embedding) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("vectors must have the same length")
	}

	if len(a) == 0 || len(b) == 0 {
		return 0, fmt.Errorf("vectors must not be empty")
	}

	var dotProduct, aMagnitude, bMagnitude float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		aMagnitude += float64(a[i]) * float64(a[i])
		bMagnitude += float64(b[i]) * float64(b[i])
	}

	if aMagnitude == 0 || bMagnitude == 0 {
		return 0, nil
	}

	return dotProduct / (math.Sqrt(aMagnitude) * math.Sqrt(bMagnitude)), nil
}
