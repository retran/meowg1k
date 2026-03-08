// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

// Package preset defines domain types for LLM provider presets with rate limits and cost tracking.
package preset

import (
	"time"

	"github.com/retran/meowg1k/internal/domain/model"
	"github.com/retran/meowg1k/internal/domain/provider"
)

// Preset defines an enumeration for configured preset names.
type Preset string

// ResolvedPreset represents a preset with all values resolved from both model and preset config.
type ResolvedPreset struct {
	ServiceTier          *string
	LogitBias            map[string]int
	MirostatEta          *float64
	MirostatTau          *float64
	Mirostat             *int
	TypicalP             *float64
	TopA                 *float64
	MinP                 *float64
	RepetitionPenalty    *float64
	Temperature          *float64
	Grammar              *string
	LogProbs             *bool
	User                 *string
	TopP                 *float64
	TopK                 *int
	FrequencyPenalty     *float64
	PresencePenalty      *float64
	Seed                 *int
	TopLogProbs          *int
	CandidateCount       *int
	TokenizerType        model.Tokenizer
	BaseURL              string
	ModelID              string
	Provider             provider.Provider
	Name                 string
	APIKeyEnv            string
	APIKey               string //nolint:gosec // API key field for preset configuration, not a hardcoded credential
	AppID                string
	EditorVersion        string
	EditorPluginVersion  string
	UserAgent            string
	CopilotIntegrationID string
	OpenAIOrganization   string
	Model                string
	Stop                 []string
	RateLimit            model.RateLimitConfig
	MaxOutputTokens      int
	MaxInputTokens       int
	Timeout              time.Duration
	CacheTTL             time.Duration
	CacheEnabled         bool
}
