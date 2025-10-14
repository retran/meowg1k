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
	// Provider specifies the LLM provider (required)
	Provider string `yaml:"provider" mapstructure:"provider"`

	// Model specifies the model name (optional, uses provider defaults if omitted)
	Model string `yaml:"model" mapstructure:"model"`

	// BaseURL sets the API base URL (required for "llama" and "openai-compatible" providers)
	BaseURL string `yaml:"baseURL" mapstructure:"baseURL"`

	// APIKeyEnv specifies the environment variable containing the API key (optional)
	APIKeyEnv string `yaml:"apiKeyEnv" mapstructure:"apiKeyEnv"`

	// MaxInputTokens sets the maximum input token limit (optional, defaults to model limits)
	MaxInputTokens int `yaml:"maxInputTokens" mapstructure:"maxInputTokens"`

	// MaxOutputTokens sets the maximum output token limit (optional, defaults to model limits)
	MaxOutputTokens int `yaml:"maxOutputTokens" mapstructure:"maxOutputTokens"`

	// Tokenizer specifies the tokenizer to use (optional, auto-detected from model)
	Tokenizer string `yaml:"tokenizer" mapstructure:"tokenizer"`

	// RateLimit defines rate limiting for this model instance
	RateLimit *ModelRateLimitConfig `yaml:"rateLimit" mapstructure:"rateLimit"`
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
	// Model references a model defined in the models section (required)
	Model string `yaml:"model" mapstructure:"model"`

	// Timeout sets the request timeout duration (optional, defaults to 5m)
	Timeout time.Duration `yaml:"timeout" mapstructure:"timeout"`

	// Temperature controls randomness in generation (optional, model-specific defaults)
	Temperature *float64 `yaml:"temperature" mapstructure:"temperature"`

	// TopP controls nucleus sampling (optional, model-specific defaults)
	TopP *float64 `yaml:"topP" mapstructure:"topP"`

	// TopK controls top-k sampling (optional, model-specific defaults)
	TopK *int `yaml:"topK" mapstructure:"topK"`

	// MaxTokens overrides the model's default max output tokens for this profile (optional)
	MaxTokens *int `yaml:"maxTokens" mapstructure:"maxTokens"`

	// FrequencyPenalty penalizes tokens based on their frequency in the response (optional, -2.0 to 2.0)
	FrequencyPenalty *float64 `yaml:"frequencyPenalty" mapstructure:"frequencyPenalty"`

	// PresencePenalty penalizes tokens based on their presence in the response (optional, -2.0 to 2.0)
	PresencePenalty *float64 `yaml:"presencePenalty" mapstructure:"presencePenalty"`

	// Seed sets a random seed for deterministic sampling (optional)
	Seed *int `yaml:"seed" mapstructure:"seed"`

	// Stop specifies sequences where the model will stop generating (optional)
	Stop []string `yaml:"stop" mapstructure:"stop"`

	// ResponseFormat specifies the format of the response (e.g., "text", "json_object", "json_schema")
	// Supported by: OpenAI, Anthropic (as responseMimeType)
	ResponseFormat *string `yaml:"responseFormat" mapstructure:"responseFormat"`

	// ResponseSchema specifies a JSON schema for structured output (optional)
	// When provided, the model will generate output matching this schema
	// Supported by: OpenAI (with response_format), Gemini, Anthropic
	ResponseSchema map[string]interface{} `yaml:"responseSchema" mapstructure:"responseSchema"`

	// CandidateCount specifies the number of response candidates to generate (optional)
	// Supported by: Gemini (candidateCount), OpenAI (n)
	CandidateCount *int `yaml:"candidateCount" mapstructure:"candidateCount"`

	// LogProbs enables returning log probabilities of output tokens (optional)
	// Supported by: OpenAI, Gemini (responseLogprobs)
	LogProbs *bool `yaml:"logProbs" mapstructure:"logProbs"`

	// TopLogProbs specifies how many top log probabilities to return per token (optional, 0-20)
	// Only used when LogProbs is true
	// Supported by: OpenAI (top_logprobs), Gemini (logprobs)
	TopLogProbs *int `yaml:"topLogProbs" mapstructure:"topLogProbs"`

	// LogitBias modifies the likelihood of specified tokens appearing (optional)
	// Map of token IDs to bias values (-100 to 100)
	// Supported by: OpenAI
	LogitBias map[string]int `yaml:"logitBias" mapstructure:"logitBias"`

	// ServiceTier specifies the service tier for the request (optional, e.g., "auto", "default")
	// Supported by: OpenAI
	ServiceTier *string `yaml:"serviceTier" mapstructure:"serviceTier"`

	// User specifies a unique identifier for the end-user (optional)
	// Used for abuse monitoring and tracking
	// Supported by: OpenAI
	User *string `yaml:"user" mapstructure:"user"`

	// RepetitionPenalty reduces repetition of tokens from input (optional, 0.0 to 2.0)
	// Higher values make repetition less likely. Token penalty scales based on original token's probability.
	// Supported by: OpenRouter, Llama.cpp
	RepetitionPenalty *float64 `yaml:"repetitionPenalty" mapstructure:"repetitionPenalty"`

	// MinP represents minimum probability for a token relative to the most likely token (optional, 0.0 to 1.0)
	// If set to 0.1, only tokens that are at least 1/10th as probable as the best option are considered.
	// Supported by: OpenRouter, Llama.cpp
	MinP *float64 `yaml:"minP" mapstructure:"minP"`

	// TopA filters tokens based on "sufficiently high" probabilities (optional, 0.0 to 1.0)
	// A dynamic filtering mechanism similar to Top-P
	// Supported by: OpenRouter
	TopA *float64 `yaml:"topA" mapstructure:"topA"`

	// TypicalP (typical sampling) parameter (optional, 0.0 to 1.0)
	// Locally typical sampling implementation, balancing creativity and coherence
	// Supported by: Llama.cpp
	TypicalP *float64 `yaml:"typicalP" mapstructure:"typicalP"`

	// Mirostat enables Mirostat sampling algorithm (optional, 0, 1, or 2)
	// 0 = disabled, 1 = Mirostat v1, 2 = Mirostat v2
	// Supported by: Llama.cpp
	Mirostat *int `yaml:"mirostat" mapstructure:"mirostat"`

	// MirostatTau is the target entropy for Mirostat (optional, default 5.0)
	// Controls the balance of coherence/creativity in Mirostat sampling
	// Supported by: Llama.cpp
	MirostatTau *float64 `yaml:"mirostatTau" mapstructure:"mirostatTau"`

	// MirostatEta is the learning rate for Mirostat (optional, default 0.1)
	// Controls how quickly Mirostat adjusts
	// Supported by: Llama.cpp
	MirostatEta *float64 `yaml:"mirostatEta" mapstructure:"mirostatEta"`

	// Grammar specifies a grammar string for constrained generation (optional)
	// Uses GBNF (GGML BNF) format for grammar rules
	// Supported by: Llama.cpp
	Grammar *string `yaml:"grammar" mapstructure:"grammar"`

	// Cache overrides global cache settings for this profile (optional)
	Cache *CacheConfig `yaml:"cache" mapstructure:"cache"`
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
	// Profile references a profile defined in the profiles section for computing embeddings
	Profile string `yaml:"profile" mapstructure:"profile"`

	// Chunker defines chunking parameters for document processing
	Chunker *ChunkerConfig `yaml:"chunker" mapstructure:"chunker"`
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
	// Profile references a profile defined in the profiles section for generating answers
	Profile string `yaml:"profile" mapstructure:"profile"`

	// TopK is the number of top results to retrieve from vector search
	TopK int `yaml:"topK" mapstructure:"topK"`

	// MinScore is the minimum similarity score for retrieved chunks (0.0 to 1.0)
	MinScore float32 `yaml:"minScore" mapstructure:"minScore"`

	// SystemPrompt is the system prompt for the answer generation
	SystemPrompt string `yaml:"systemPrompt" mapstructure:"systemPrompt"`
}
