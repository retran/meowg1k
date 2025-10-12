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
	"fmt"
	"math"
)

// GenerateContentRequest holds the parameters for a content generation request.
type GenerateContentRequest struct {
	model            string
	systemPrompt     string
	userPrompt       string
	maxOutputTokens  int
	temperature      *float64
	topP             *float64
	topK             *int
	frequencyPenalty *float64
	presencePenalty  *float64
	seed             *int
	stop             []string
}

// NewGenerateContentRequest creates and returns a new GenerateContentRequest.
func NewGenerateContentRequest(model, systemPrompt, userPrompt string, maxOutputTokens int) *GenerateContentRequest {
	return &GenerateContentRequest{
		model:           model,
		systemPrompt:    systemPrompt,
		userPrompt:      userPrompt,
		maxOutputTokens: maxOutputTokens,
	}
}

// WithTemperature sets the temperature parameter and returns the request.
func (r *GenerateContentRequest) WithTemperature(temperature *float64) *GenerateContentRequest {
	r.temperature = temperature
	return r
}

// WithTopP sets the topP parameter and returns the request.
func (r *GenerateContentRequest) WithTopP(topP *float64) *GenerateContentRequest {
	r.topP = topP
	return r
}

// WithTopK sets the topK parameter and returns the request.
func (r *GenerateContentRequest) WithTopK(topK *int) *GenerateContentRequest {
	r.topK = topK
	return r
}

// WithFrequencyPenalty sets the frequencyPenalty parameter and returns the request.
func (r *GenerateContentRequest) WithFrequencyPenalty(frequencyPenalty *float64) *GenerateContentRequest {
	r.frequencyPenalty = frequencyPenalty
	return r
}

// WithPresencePenalty sets the presencePenalty parameter and returns the request.
func (r *GenerateContentRequest) WithPresencePenalty(presencePenalty *float64) *GenerateContentRequest {
	r.presencePenalty = presencePenalty
	return r
}

// WithSeed sets the seed parameter and returns the request.
func (r *GenerateContentRequest) WithSeed(seed *int) *GenerateContentRequest {
	r.seed = seed
	return r
}

// WithStop sets the stop sequences and returns the request.
func (r *GenerateContentRequest) WithStop(stop []string) *GenerateContentRequest {
	r.stop = stop
	return r
}

// Model returns the model name for the content generation request.
func (r *GenerateContentRequest) Model() string {
	return r.model
}

// SystemPrompt returns the system prompt for the content generation request.
func (r *GenerateContentRequest) SystemPrompt() string {
	return r.systemPrompt
}

// UserPrompt returns the user prompt for the content generation request.
func (r *GenerateContentRequest) UserPrompt() string {
	return r.userPrompt
}

// MaxOutputTokens returns the maximum output tokens for the content generation request.
func (r *GenerateContentRequest) MaxOutputTokens() int {
	return r.maxOutputTokens
}

// Temperature returns the temperature parameter for the content generation request.
func (r *GenerateContentRequest) Temperature() *float64 {
	return r.temperature
}

// TopP returns the topP parameter for the content generation request.
func (r *GenerateContentRequest) TopP() *float64 {
	return r.topP
}

// TopK returns the topK parameter for the content generation request.
func (r *GenerateContentRequest) TopK() *int {
	return r.topK
}

// FrequencyPenalty returns the frequencyPenalty parameter for the content generation request.
func (r *GenerateContentRequest) FrequencyPenalty() *float64 {
	return r.frequencyPenalty
}

// PresencePenalty returns the presencePenalty parameter for the content generation request.
func (r *GenerateContentRequest) PresencePenalty() *float64 {
	return r.presencePenalty
}

// Seed returns the seed parameter for the content generation request.
func (r *GenerateContentRequest) Seed() *int {
	return r.seed
}

// Stop returns the stop sequences for the content generation request.
func (r *GenerateContentRequest) Stop() []string {
	return r.stop
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
func NewComputeEmbeddingsRequest(model string, chunks []string, taskType TaskType) *ComputeEmbeddingsRequest {
	return &ComputeEmbeddingsRequest{
		model:      model,
		chunks:     chunks,
		taskType:   taskType,
		dimensions: 0, // Will be set by the service if needed
	}
}

// NewComputeEmbeddingsRequestWithDimensions creates a new ComputeEmbeddingsRequest with custom dimensions.
func NewComputeEmbeddingsRequestWithDimensions(
	model string,
	chunks []string,
	taskType TaskType,
	dimensions int,
) *ComputeEmbeddingsRequest {
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

// ComputeDistanceMixin provides a default implementation of ComputeDistance.
type ComputeDistanceMixin struct{}

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
