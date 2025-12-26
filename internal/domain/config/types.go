// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package config defines domain types for application configuration including profiles, tasks, filters, and chunking settings.
package config

import (
	"time"
)

// Config represents the complete meowg1k configuration.
type Config struct {
	// Models define LLM API instances with their connection parameters and rate limits
	Models map[string]*ModelDefinition `yaml:"models" mapstructure:"models"`

	// Profiles define reusable LLM request configurations
	Profiles map[string]*ProfileDefinition `yaml:"profiles" mapstructure:"profiles"`

	// Generate command configuration
	Generate *GenerateConfig `yaml:"generate" mapstructure:"generate"`

	// Filter configuration for pre-analysis noise filtering
	Filter *FilterConfig `yaml:"filter" mapstructure:"filter"`

	// Summarize engine configuration ("Map" phase)
	Summarize *SummarizeConfig `yaml:"summarize" mapstructure:"summarize"`

	// Commit command configuration ("Reduce" phase)
	Commit *CommandConfig `yaml:"commit" mapstructure:"commit"`

	// PullRequest command configuration ("Reduce" phase)
	PullRequest *CommandConfig `yaml:"pullRequest" mapstructure:"pullRequest"`

	// Index configuration for document indexing
	Index *IndexConfig `yaml:"index" mapstructure:"index"`

	// Ask configuration for RAG-based question answering
	Ask *AskConfig `yaml:"ask" mapstructure:"ask"`

	// Cache configuration for LLM response caching
	Cache *CacheConfig `yaml:"cache" mapstructure:"cache"`
}

// CacheConfig defines configuration for LLM response caching.
type CacheConfig struct {
	// Enabled determines whether caching is enabled
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`

	// TTL defines how long cache entries should be kept before being purged
	TTL time.Duration `yaml:"ttl" mapstructure:"ttl"`
}

// ModelDefinition defines an LLM API instance with connection parameters.
// A model instance represents a specific API endpoint with its own rate limits.
// Multiple profiles can reference the same model instance to share rate limits.
type ModelDefinition struct {
	RateLimit       *ModelRateLimitConfig `yaml:"rateLimit" mapstructure:"rateLimit"`
	Provider        string                `yaml:"provider" mapstructure:"provider"`
	Model           string                `yaml:"model" mapstructure:"model"`
	BaseURL         string                `yaml:"baseURL" mapstructure:"baseURL"`
	APIKeyEnv       string                `yaml:"apiKeyEnv" mapstructure:"apiKeyEnv"`
	Tokenizer       string                `yaml:"tokenizer" mapstructure:"tokenizer"`
	MaxInputTokens  int                   `yaml:"maxInputTokens" mapstructure:"maxInputTokens"`
	MaxOutputTokens int                   `yaml:"maxOutputTokens" mapstructure:"maxOutputTokens"`
}

// ModelRateLimitConfig defines rate limiting for a model instance.
type ModelRateLimitConfig struct {
	// RequestsPerMinute sets the maximum requests per minute (0 = unlimited)
	RequestsPerMinute int `yaml:"requestsPerMinute" mapstructure:"requestsPerMinute"`

	// TokensPerMinute sets the maximum tokens per minute (0 = unlimited)
	TokensPerMinute int `yaml:"tokensPerMinute" mapstructure:"tokensPerMinute"`

	// RequestsPerDay sets the maximum requests per day (0 = unlimited)
	RequestsPerDay int `yaml:"requestsPerDay" mapstructure:"requestsPerDay"`
}

// ProfileDefinition defines request-specific parameters for using a model.
// Profiles reference a model instance and add request-specific settings like timeout and temperature.
type ProfileDefinition struct {
	CandidateCount    *int                   `yaml:"candidateCount" mapstructure:"candidateCount"`
	MirostatEta       *float64               `yaml:"mirostatEta" mapstructure:"mirostatEta"`
	Temperature       *float64               `yaml:"temperature" mapstructure:"temperature"`
	TopP              *float64               `yaml:"topP" mapstructure:"topP"`
	TopK              *int                   `yaml:"topK" mapstructure:"topK"`
	MaxTokens         *int                   `yaml:"maxTokens" mapstructure:"maxTokens"`
	FrequencyPenalty  *float64               `yaml:"frequencyPenalty" mapstructure:"frequencyPenalty"`
	PresencePenalty   *float64               `yaml:"presencePenalty" mapstructure:"presencePenalty"`
	Seed              *int                   `yaml:"seed" mapstructure:"seed"`
	Cache             *CacheConfig           `yaml:"cache" mapstructure:"cache"`
	ResponseFormat    *string                `yaml:"responseFormat" mapstructure:"responseFormat"`
	TopLogProbs       *int                   `yaml:"topLogProbs" mapstructure:"topLogProbs"`
	Grammar           *string                `yaml:"grammar" mapstructure:"grammar"`
	LogProbs          *bool                  `yaml:"logProbs" mapstructure:"logProbs"`
	ResponseSchema    map[string]interface{} `yaml:"responseSchema" mapstructure:"responseSchema"`
	LogitBias         map[string]int         `yaml:"logitBias" mapstructure:"logitBias"`
	ServiceTier       *string                `yaml:"serviceTier" mapstructure:"serviceTier"`
	User              *string                `yaml:"user" mapstructure:"user"`
	RepetitionPenalty *float64               `yaml:"repetitionPenalty" mapstructure:"repetitionPenalty"`
	MinP              *float64               `yaml:"minP" mapstructure:"minP"`
	TopA              *float64               `yaml:"topA" mapstructure:"topA"`
	TypicalP          *float64               `yaml:"typicalP" mapstructure:"typicalP"`
	Mirostat          *int                   `yaml:"mirostat" mapstructure:"mirostat"`
	MirostatTau       *float64               `yaml:"mirostatTau" mapstructure:"mirostatTau"`
	Model             string                 `yaml:"model" mapstructure:"model"`
	Stop              []string               `yaml:"stop" mapstructure:"stop"`
	Timeout           time.Duration          `yaml:"timeout" mapstructure:"timeout"`
}

// GenerateConfig holds configuration for the generate command.
type GenerateConfig struct {
	// Default settings used when no task is specified
	Default *GenerateDefault `yaml:"default" mapstructure:"default"`

	// Tasks define named generation tasks with specific prompts and settings
	Tasks map[string]*GenerateTask `yaml:"tasks" mapstructure:"tasks"`
}

// GenerateDefault defines default settings for the generate command.
type GenerateDefault struct {
	// Profile references a profile defined in the profiles section
	Profile string `yaml:"profile" mapstructure:"profile"`

	// SystemPrompt sets the default system prompt for all generation requests
	SystemPrompt string `yaml:"systemPrompt" mapstructure:"systemPrompt"`
}

// GenerateTask defines a specific generation task.
// Tasks allow predefined prompts and settings for common use cases.
type GenerateTask struct {
	// Profile references a profile defined in the profiles section (optional)
	Profile string `yaml:"profile" mapstructure:"profile"`

	// SystemPrompt overrides the default system prompt for this task (optional)
	SystemPrompt string `yaml:"systemPrompt" mapstructure:"systemPrompt"`

	// UserPrompt sets the task-specific user prompt
	UserPrompt string `yaml:"userPrompt" mapstructure:"userPrompt"`
}

// FilterConfig defines files to ignore during analysis.
type FilterConfig struct {
	// Ignore specifies glob patterns for files to exclude from analysis
	Ignore []string `yaml:"ignore" mapstructure:"ignore"`
}

// Strategy defines summarization strategy with its settings.
type Strategy struct {
	// Type specifies the summarization approach
	Type string `yaml:"type" mapstructure:"type"`

	// IncludeOriginalFile determines whether to send the original file content
	IncludeOriginalFile bool `yaml:"includeOriginalFile" mapstructure:"includeOriginalFile"`

	// IncludeChangedFile determines whether to send the changed file content
	IncludeChangedFile bool `yaml:"includeChangedFile" mapstructure:"includeChangedFile"`
}

// SummarizeConfig holds configuration for the summarization engine.
// Used during the "Map" phase of change analysis.
type SummarizeConfig struct {
	// Default summarization settings used when no rule matches
	Default *SummarizeDefault `yaml:"default" mapstructure:"default"`

	// Rules define file-specific summarization behavior
	Rules []*SummarizeRule `yaml:"rules" mapstructure:"rules"`
}

// SummarizeDefault defines default summarization settings.
type SummarizeDefault struct {
	// Profile references a profile defined in the profiles section
	Profile string `yaml:"profile" mapstructure:"profile"`

	// Strategy defines how files should be processed
	Strategy *Strategy `yaml:"strategy" mapstructure:"strategy"`

	// SystemPrompt sets the default system prompt for summarization
	SystemPrompt string `yaml:"systemPrompt" mapstructure:"systemPrompt"`
}

// SummarizeRule defines file-specific summarization rules.
// Rules allow customized processing based on file patterns.
type SummarizeRule struct {
	// Match specifies a gitignore-style pattern for files this rule applies to.
	// Supports glob patterns like *.go, **/*.go, internal/**, etc.
	Match string `yaml:"match" mapstructure:"match"`

	// Profile references a profile defined in the profiles section (optional)
	Profile string `yaml:"profile" mapstructure:"profile"`

	// Strategy defines how matching files should be processed (optional)
	Strategy *Strategy `yaml:"strategy" mapstructure:"strategy"`

	// SystemPrompt overrides the default system prompt for matching files (optional)
	SystemPrompt string `yaml:"systemPrompt" mapstructure:"systemPrompt"`

	// Skip indicates whether to skip processing matching files entirely
	Skip bool `yaml:"skip" mapstructure:"skip"`
}

// CommandConfig defines configuration for commit and PR commands.
// Used during the "Reduce" phase of change analysis.
type CommandConfig struct {
	// Profile references a profile defined in the profiles section
	Profile string `yaml:"profile" mapstructure:"profile"`

	// Strategy determines how changes are processed:
	// - "summarize" (default): uses Map-Reduce approach to summarize each file then compose the final message
	// - "flat": sends the entire diff directly to the model without summarization
	Strategy string `yaml:"strategy" mapstructure:"strategy"`

	// SystemPrompt sets the system prompt for the command
	SystemPrompt string `yaml:"systemPrompt" mapstructure:"systemPrompt"`
}

// IndexConfig defines configuration for document indexing.
type IndexConfig struct {
	Chunker   *ChunkerConfig `yaml:"chunker" mapstructure:"chunker"`
	Profile   string         `yaml:"profile" mapstructure:"profile"`
	BatchSize int            `yaml:"batchSize" mapstructure:"batchSize"`
}

// ChunkerConfig defines parameters for text chunking.
type ChunkerConfig struct {
	// MaxRunes is the maximum number of runes per chunk
	MaxRunes int `yaml:"maxRunes" mapstructure:"maxRunes"`

	// OverlapRunes is the number of runes to overlap between chunks
	OverlapRunes int `yaml:"overlapRunes" mapstructure:"overlapRunes"`
}

// AskConfig defines configuration for RAG-based question answering.
type AskConfig struct {
	Profile      string  `yaml:"profile" mapstructure:"profile"`
	SystemPrompt string  `yaml:"systemPrompt" mapstructure:"systemPrompt"`
	TopK         int     `yaml:"topK" mapstructure:"topK"`
	MinScore     float32 `yaml:"minScore" mapstructure:"minScore"`
}
