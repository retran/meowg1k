// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGenerateContentRequest(t *testing.T) {
	model := "test-model"
	systemPrompt := "system prompt"
	userPrompt := "user prompt"
	maxTokens := 1000

	req := NewGenerateContentRequest(model, systemPrompt, userPrompt, maxTokens)

	assert.NotNil(t, req)
	assert.Equal(t, model, req.Model())
	assert.Equal(t, systemPrompt, req.SystemPrompt())
	assert.Equal(t, userPrompt, req.UserPrompt())
	assert.Equal(t, maxTokens, req.MaxOutputTokens())
}

func TestGenerateContentRequest_WithTemperature(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	temp := 0.7

	result := req.WithTemperature(&temp)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.Temperature())
	assert.Equal(t, temp, *req.Temperature())
}

func TestGenerateContentRequest_WithTopP(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	topP := 0.9

	result := req.WithTopP(&topP)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.TopP())
	assert.Equal(t, topP, *req.TopP())
}

func TestGenerateContentRequest_WithTopK(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	topK := 50

	result := req.WithTopK(&topK)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.TopK())
	assert.Equal(t, topK, *req.TopK())
}

func TestGenerateContentRequest_WithFrequencyPenalty(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	penalty := 0.5

	result := req.WithFrequencyPenalty(&penalty)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.FrequencyPenalty())
	assert.Equal(t, penalty, *req.FrequencyPenalty())
}

func TestGenerateContentRequest_WithPresencePenalty(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	penalty := 0.3

	result := req.WithPresencePenalty(&penalty)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.PresencePenalty())
	assert.Equal(t, penalty, *req.PresencePenalty())
}

func TestGenerateContentRequest_WithSeed(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	seed := 42

	result := req.WithSeed(&seed)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.Seed())
	assert.Equal(t, seed, *req.Seed())
}

func TestGenerateContentRequest_WithStop(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	stop := []string{"\n", "STOP"}

	result := req.WithStop(stop)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.Equal(t, stop, req.Stop())
}

func TestGenerateContentRequest_WithResponseFormat(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	format := "json"

	result := req.WithResponseFormat(&format)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.ResponseFormat())
	assert.Equal(t, format, *req.ResponseFormat())
}

func TestGenerateContentRequest_WithResponseSchema(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
		},
	}

	result := req.WithResponseSchema(schema)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.Equal(t, schema, req.ResponseSchema())
}

func TestGenerateContentRequest_WithCandidateCount(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	count := 3

	result := req.WithCandidateCount(&count)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.CandidateCount())
	assert.Equal(t, count, *req.CandidateCount())
}

func TestGenerateContentRequest_WithLogProbs(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	logProbs := true

	result := req.WithLogProbs(&logProbs)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.LogProbs())
	assert.Equal(t, logProbs, *req.LogProbs())
}

func TestGenerateContentRequest_WithTopLogProbs(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	topLogProbs := 5

	result := req.WithTopLogProbs(&topLogProbs)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.TopLogProbs())
	assert.Equal(t, topLogProbs, *req.TopLogProbs())
}

func TestGenerateContentRequest_WithLogitBias(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	bias := map[string]int{
		"token1": 10,
		"token2": -5,
	}

	result := req.WithLogitBias(bias)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.Equal(t, bias, req.LogitBias())
}

func TestGenerateContentRequest_WithServiceTier(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	tier := "auto"

	result := req.WithServiceTier(&tier)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.ServiceTier())
	assert.Equal(t, tier, *req.ServiceTier())
}

func TestGenerateContentRequest_WithUser(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	user := "user-123"

	result := req.WithUser(&user)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.User())
	assert.Equal(t, user, *req.User())
}

func TestGenerateContentRequest_WithRepetitionPenalty(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	penalty := 1.2

	result := req.WithRepetitionPenalty(&penalty)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.RepetitionPenalty())
	assert.Equal(t, penalty, *req.RepetitionPenalty())
}

func TestGenerateContentRequest_WithMinP(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	minP := 0.05

	result := req.WithMinP(&minP)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.MinP())
	assert.Equal(t, minP, *req.MinP())
}

func TestGenerateContentRequest_WithTopA(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	topA := 0.1

	result := req.WithTopA(&topA)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.TopA())
	assert.Equal(t, topA, *req.TopA())
}

func TestGenerateContentRequest_WithTypicalP(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	typicalP := 0.95

	result := req.WithTypicalP(&typicalP)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.TypicalP())
	assert.Equal(t, typicalP, *req.TypicalP())
}

func TestGenerateContentRequest_WithMirostat(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	mirostat := 2

	result := req.WithMirostat(&mirostat)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.Mirostat())
	assert.Equal(t, mirostat, *req.Mirostat())
}

func TestGenerateContentRequest_WithMirostatTau(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	tau := 5.0

	result := req.WithMirostatTau(&tau)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.MirostatTau())
	assert.Equal(t, tau, *req.MirostatTau())
}

func TestGenerateContentRequest_WithMirostatEta(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	eta := 0.1

	result := req.WithMirostatEta(&eta)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.MirostatEta())
	assert.Equal(t, eta, *req.MirostatEta())
}

func TestGenerateContentRequest_WithGrammar(t *testing.T) {
	req := NewGenerateContentRequest("model", "sys", "user", 100)
	grammar := "root ::= \"yes\" | \"no\""

	result := req.WithGrammar(&grammar)

	assert.Equal(t, req, result, "Should return self for chaining")
	assert.NotNil(t, req.Grammar())
	assert.Equal(t, grammar, *req.Grammar())
}

func TestGenerateContentRequest_Chaining(t *testing.T) {
	temp := 0.8
	topP := 0.95
	topK := 40
	seed := 123

	req := NewGenerateContentRequest("model", "sys", "user", 100).
		WithTemperature(&temp).
		WithTopP(&topP).
		WithTopK(&topK).
		WithSeed(&seed)

	assert.NotNil(t, req.Temperature())
	assert.Equal(t, temp, *req.Temperature())
	assert.NotNil(t, req.TopP())
	assert.Equal(t, topP, *req.TopP())
	assert.NotNil(t, req.TopK())
	assert.Equal(t, topK, *req.TopK())
	assert.NotNil(t, req.Seed())
	assert.Equal(t, seed, *req.Seed())
}
