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

// Provider defines an enumeration for supported LLM providers.
type Provider string

const (
	// Llama identifies the Llama provider.
	Llama Provider = "llama"
	// Gemini identifies the Gemini provider.
	Gemini Provider = "gemini"
	// OpenAI identifies the OpenAI provider.
	OpenAI Provider = "openai"
	// OpenRouter identifies the OpenRouter provider.
	OpenRouter Provider = "openrouter"
	// OpenAICompatible identifies OpenAI-compatible providers with custom base URLs.
	OpenAICompatible Provider = "openai-compatible"
	// Anthropic identifies the Anthropic provider.
	Anthropic Provider = "anthropic"
	// Voyage identifies the Voyage AI provider (embeddings only).
	Voyage Provider = "voyage"
)

// GenerateContentRequest holds the parameters for a content generation request.
type GenerateContentRequest struct {
	model           string
	systemPrompt    string
	userPrompt      string
	maxOutputTokens int
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
