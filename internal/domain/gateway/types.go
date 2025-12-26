// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package gateway defines domain types for LLM gateway interactions including requests, responses, and embeddings.
package gateway

import (
	"fmt"
	"math"
)

// GenerateContentRequest holds the parameters for a content generation request.
type GenerateContentRequest struct {
	responseSchema    map[string]interface{}
	mirostatTau       *float64
	grammar           *string
	mirostatEta       *float64
	temperature       *float64
	topP              *float64
	topK              *int
	frequencyPenalty  *float64
	presencePenalty   *float64
	seed              *int
	candidateCount    *int
	logProbs          *bool
	mirostat          *int
	typicalP          *float64
	responseFormat    *string
	topLogProbs       *int
	logitBias         map[string]int
	serviceTier       *string
	user              *string
	repetitionPenalty *float64
	minP              *float64
	topA              *float64
	model             string
	systemPrompt      string
	userPrompt        string
	stop              []string
	maxOutputTokens   int
}

// ErrToolCallingNotSupported indicates the gateway does not support tool calling.
var ErrToolCallingNotSupported = fmt.Errorf("tool calling not supported")

// ToolDefinition describes a tool/function that the model can call.
type ToolDefinition struct {
	Parameters  map[string]any `json:"parameters,omitempty"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
}

// ToolCall represents a model-emitted tool call.
type ToolCall struct {
	Arguments map[string]any `json:"arguments"`
	Name      string         `json:"name"`
	ID        string         `json:"id,omitempty"`
}

// GenerateContentResponse represents a model response that may include tool calls.
type GenerateContentResponse struct {
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
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

// WithResponseFormat sets the response format and returns the request.
func (r *GenerateContentRequest) WithResponseFormat(responseFormat *string) *GenerateContentRequest {
	r.responseFormat = responseFormat
	return r
}

// WithResponseSchema sets the response schema and returns the request.
func (r *GenerateContentRequest) WithResponseSchema(responseSchema map[string]interface{}) *GenerateContentRequest {
	r.responseSchema = responseSchema
	return r
}

// WithCandidateCount sets the number of candidates to generate and returns the request.
func (r *GenerateContentRequest) WithCandidateCount(candidateCount *int) *GenerateContentRequest {
	r.candidateCount = candidateCount
	return r
}

// WithLogProbs sets whether to return log probabilities and returns the request.
func (r *GenerateContentRequest) WithLogProbs(logProbs *bool) *GenerateContentRequest {
	r.logProbs = logProbs
	return r
}

// WithTopLogProbs sets the number of top log probabilities to return and returns the request.
func (r *GenerateContentRequest) WithTopLogProbs(topLogProbs *int) *GenerateContentRequest {
	r.topLogProbs = topLogProbs
	return r
}

// WithLogitBias sets the logit bias map and returns the request.
func (r *GenerateContentRequest) WithLogitBias(logitBias map[string]int) *GenerateContentRequest {
	r.logitBias = logitBias
	return r
}

// WithServiceTier sets the service tier and returns the request.
func (r *GenerateContentRequest) WithServiceTier(serviceTier *string) *GenerateContentRequest {
	r.serviceTier = serviceTier
	return r
}

// WithUser sets the user identifier and returns the request.
func (r *GenerateContentRequest) WithUser(user *string) *GenerateContentRequest {
	r.user = user
	return r
}

// WithRepetitionPenalty sets the repetition penalty and returns the request.
func (r *GenerateContentRequest) WithRepetitionPenalty(repetitionPenalty *float64) *GenerateContentRequest {
	r.repetitionPenalty = repetitionPenalty
	return r
}

// WithMinP sets the minimum probability threshold and returns the request.
func (r *GenerateContentRequest) WithMinP(minP *float64) *GenerateContentRequest {
	r.minP = minP
	return r
}

// WithTopA sets the top-A filtering parameter and returns the request.
func (r *GenerateContentRequest) WithTopA(topA *float64) *GenerateContentRequest {
	r.topA = topA
	return r
}

// WithTypicalP sets the typical sampling parameter and returns the request.
func (r *GenerateContentRequest) WithTypicalP(typicalP *float64) *GenerateContentRequest {
	r.typicalP = typicalP
	return r
}

// WithMirostat sets the Mirostat sampling mode and returns the request.
func (r *GenerateContentRequest) WithMirostat(mirostat *int) *GenerateContentRequest {
	r.mirostat = mirostat
	return r
}

// WithMirostatTau sets the Mirostat target entropy and returns the request.
func (r *GenerateContentRequest) WithMirostatTau(mirostatTau *float64) *GenerateContentRequest {
	r.mirostatTau = mirostatTau
	return r
}

// WithMirostatEta sets the Mirostat learning rate and returns the request.
func (r *GenerateContentRequest) WithMirostatEta(mirostatEta *float64) *GenerateContentRequest {
	r.mirostatEta = mirostatEta
	return r
}

// WithGrammar sets the grammar constraints and returns the request.
func (r *GenerateContentRequest) WithGrammar(grammar *string) *GenerateContentRequest {
	r.grammar = grammar
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

// ResponseFormat returns the response format for the content generation request.
func (r *GenerateContentRequest) ResponseFormat() *string {
	return r.responseFormat
}

// ResponseSchema returns the response schema for the content generation request.
func (r *GenerateContentRequest) ResponseSchema() map[string]interface{} {
	return r.responseSchema
}

// CandidateCount returns the number of candidates to generate for the content generation request.
func (r *GenerateContentRequest) CandidateCount() *int {
	return r.candidateCount
}

// LogProbs returns whether to return log probabilities for the content generation request.
func (r *GenerateContentRequest) LogProbs() *bool {
	return r.logProbs
}

// TopLogProbs returns the number of top log probabilities to return for the content generation request.
func (r *GenerateContentRequest) TopLogProbs() *int {
	return r.topLogProbs
}

// LogitBias returns the logit bias map for the content generation request.
func (r *GenerateContentRequest) LogitBias() map[string]int {
	return r.logitBias
}

// ServiceTier returns the service tier for the content generation request.
func (r *GenerateContentRequest) ServiceTier() *string {
	return r.serviceTier
}

// User returns the user identifier for the content generation request.
func (r *GenerateContentRequest) User() *string {
	return r.user
}

// RepetitionPenalty returns the repetition penalty for the content generation request.
func (r *GenerateContentRequest) RepetitionPenalty() *float64 {
	return r.repetitionPenalty
}

// MinP returns the minimum probability threshold for the content generation request.
func (r *GenerateContentRequest) MinP() *float64 {
	return r.minP
}

// TopA returns the top-A filtering parameter for the content generation request.
func (r *GenerateContentRequest) TopA() *float64 {
	return r.topA
}

// TypicalP returns the typical sampling parameter for the content generation request.
func (r *GenerateContentRequest) TypicalP() *float64 {
	return r.typicalP
}

// Mirostat returns the Mirostat sampling mode for the content generation request.
func (r *GenerateContentRequest) Mirostat() *int {
	return r.mirostat
}

// MirostatTau returns the Mirostat target entropy for the content generation request.
func (r *GenerateContentRequest) MirostatTau() *float64 {
	return r.mirostatTau
}

// MirostatEta returns the Mirostat learning rate for the content generation request.
func (r *GenerateContentRequest) MirostatEta() *float64 {
	return r.mirostatEta
}

// Grammar returns the grammar constraints for the content generation request.
func (r *GenerateContentRequest) Grammar() *string {
	return r.grammar
}

// Embedding represents a text embedding as a vector of floating-point numbers.
type Embedding []float64

// TaskType specifies the intended use case for the text embedding. This allows the model
// to produce higher-quality embeddings tailored to the specific task.
type TaskType string

// TaskType values for embedding requests.
const (
	// SemanticSimilarity indicates embeddings optimized for similarity search.
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
	taskType   TaskType
	chunks     []string
	dimensions int
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
