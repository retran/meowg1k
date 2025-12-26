// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package profile defines domain types for LLM provider profiles with rate limits and cost tracking.
package profile

import (
	"time"

	"github.com/retran/meowg1k/internal/domain/model"
	"github.com/retran/meowg1k/internal/domain/provider"
)

// Profile defines an enumeration for configured profile names.
type Profile string

// ResolvedProfile represents a profile with all values resolved from both model and profile config.
type ResolvedProfile struct {
	ServiceTier       *string
	LogitBias         map[string]int
	MirostatEta       *float64
	MirostatTau       *float64
	Mirostat          *int
	TypicalP          *float64
	TopA              *float64
	MinP              *float64
	RepetitionPenalty *float64
	Temperature       *float64
	Grammar           *string
	LogProbs          *bool
	User              *string
	TopP              *float64
	TopK              *int
	FrequencyPenalty  *float64
	PresencePenalty   *float64
	Seed              *int
	TopLogProbs       *int
	ResponseFormat    *string
	ResponseSchema    map[string]interface{}
	CandidateCount    *int
	TokenizerType     model.Tokenizer
	BaseURL           string
	ModelID           string
	Provider          provider.Provider
	Name              string
	APIKeyEnv         string
	APIKey            string
	Model             string
	Stop              []string
	RateLimit         model.RateLimitConfig
	MaxOutputTokens   int
	MaxInputTokens    int
	Timeout           time.Duration
	CacheTTL          time.Duration
	CacheEnabled      bool
}
