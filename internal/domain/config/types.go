// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package config defines domain types for application configuration including providers, models, presets, and flows.
package config

import "time"

// Config represents the complete meowg1k configuration.
type Config struct {
	Filter        *FilterConfig              `yaml:"filter" mapstructure:"filter"`
	Providers     map[string]*ProviderConfig `yaml:"providers" mapstructure:"providers"`
	Models        map[string]*ModelConfig    `yaml:"models" mapstructure:"models"`
	Presets       map[string]*PresetConfig   `yaml:"presets" mapstructure:"presets"`
	Activities    *ActivitiesConfig          `yaml:"activities" mapstructure:"activities"`
	Flows         *FlowsConfig               `yaml:"flows" mapstructure:"flows"`
	SchemaVersion int                        `yaml:"schema_version" mapstructure:"schema_version"`
}

// CacheConfig defines configuration for LLM response caching.
type CacheConfig struct {
	Enabled *bool         `yaml:"enabled" mapstructure:"enabled"`
	TTL     time.Duration `yaml:"ttl" mapstructure:"ttl"`
}

// FilterConfig defines files to ignore during analysis.
type FilterConfig struct {
	Ignore []string `yaml:"ignore" mapstructure:"ignore"`
}

// ProviderConfig defines shared provider settings used by models.
type ProviderConfig struct {
	Limits               *ModelLimits     `yaml:"limits" mapstructure:"limits"`
	RateLimit            *RateLimitConfig `yaml:"rate_limit" mapstructure:"rate_limit"`
	Type                 string           `yaml:"type" mapstructure:"type"`
	BaseURL              string           `yaml:"base_url" mapstructure:"base_url"`
	APIKey               string           `yaml:"-" mapstructure:"-"` // Direct API key (not serialized)
	Tokenizer            string           `yaml:"tokenizer" mapstructure:"tokenizer"`
	AppID                string           `yaml:"-" mapstructure:"-"` // Not serialized — passed at runtime
	EditorVersion        string           `yaml:"-" mapstructure:"-"`
	EditorPluginVersion  string           `yaml:"-" mapstructure:"-"`
	UserAgent            string           `yaml:"-" mapstructure:"-"`
	CopilotIntegrationID string           `yaml:"-" mapstructure:"-"`
	OpenAIOrganization   string           `yaml:"-" mapstructure:"-"`
	RetryCount           int              `yaml:"retry_count" mapstructure:"retry_count"`
}

// ModelConfig defines an LLM API instance with connection parameters.
type ModelConfig struct {
	Limits    *ModelLimits     `yaml:"limits" mapstructure:"limits"`
	RateLimit *RateLimitConfig `yaml:"rate_limit" mapstructure:"rate_limit"`
	Metadata  map[string]any   `yaml:"metadata" mapstructure:"metadata"`
	Provider  string           `yaml:"provider" mapstructure:"provider"`
	Model     string           `yaml:"model" mapstructure:"model"`
	BaseURL   string           `yaml:"base_url" mapstructure:"base_url"`
	APIKey    string           `yaml:"-" mapstructure:"-"` // Direct API key (not serialized)
	Tokenizer string           `yaml:"tokenizer" mapstructure:"tokenizer"`
}

// ModelLimits defines model token limits.
type ModelLimits struct {
	MaxInputTokens  int `yaml:"max_input_tokens" mapstructure:"max_input_tokens"`
	MaxOutputTokens int `yaml:"max_output_tokens" mapstructure:"max_output_tokens"`
}

// RateLimitConfig defines rate limiting for a model instance.
type RateLimitConfig struct {
	RequestsPerMinute int `yaml:"requests_per_minute" mapstructure:"requests_per_minute"`
	TokensPerMinute   int `yaml:"tokens_per_minute" mapstructure:"tokens_per_minute"`
	RequestsPerDay    int `yaml:"requests_per_day" mapstructure:"requests_per_day"`
}

// PresetConfig defines a reusable runtime configuration.
type PresetConfig struct {
	Cache   *CacheConfig   `yaml:"cache" mapstructure:"cache"`
	Request *RequestConfig `yaml:"request" mapstructure:"request"`
	Labels  map[string]any `yaml:"labels" mapstructure:"labels"`
	Extends string         `yaml:"extends" mapstructure:"extends"`
	Model   string         `yaml:"model" mapstructure:"model"`
	Timeout time.Duration  `yaml:"timeout" mapstructure:"timeout"`
}

// RequestConfig defines request-level generation parameters.
type RequestConfig struct {
	CandidateCount    *int           `yaml:"candidate_count" mapstructure:"candidate_count"`
	Temperature       *float64       `yaml:"temperature" mapstructure:"temperature"`
	TopP              *float64       `yaml:"top_p" mapstructure:"top_p"`
	TopK              *int           `yaml:"top_k" mapstructure:"top_k"`
	MaxTokens         *int           `yaml:"max_tokens" mapstructure:"max_tokens"`
	FrequencyPenalty  *float64       `yaml:"frequency_penalty" mapstructure:"frequency_penalty"`
	PresencePenalty   *float64       `yaml:"presence_penalty" mapstructure:"presence_penalty"`
	Seed              *int           `yaml:"seed" mapstructure:"seed"`
	Cache             *CacheConfig   `yaml:"cache" mapstructure:"cache"`
	TopLogProbs       *int           `yaml:"top_log_probs" mapstructure:"top_log_probs"`
	Grammar           *string        `yaml:"grammar" mapstructure:"grammar"`
	LogProbs          *bool          `yaml:"log_probs" mapstructure:"log_probs"`
	LogitBias         map[string]int `yaml:"logit_bias" mapstructure:"logit_bias"`
	ServiceTier       *string        `yaml:"service_tier" mapstructure:"service_tier"`
	User              *string        `yaml:"user" mapstructure:"user"`
	RepetitionPenalty *float64       `yaml:"repetition_penalty" mapstructure:"repetition_penalty"`
	MinP              *float64       `yaml:"min_p" mapstructure:"min_p"`
	TopA              *float64       `yaml:"top_a" mapstructure:"top_a"`
	TypicalP          *float64       `yaml:"typical_p" mapstructure:"typical_p"`
	Mirostat          *int           `yaml:"mirostat" mapstructure:"mirostat"`
	MirostatTau       *float64       `yaml:"mirostat_tau" mapstructure:"mirostat_tau"`
	MirostatEta       *float64       `yaml:"mirostat_eta" mapstructure:"mirostat_eta"`
	Stop              []string       `yaml:"stop" mapstructure:"stop"`
}

// FlowsConfig groups user-facing workflows (legacy, will be deprecated).
type FlowsConfig struct {
	Write  *WriteFlowConfig  `yaml:"write" mapstructure:"write"`
	Index  *IndexFlowConfig  `yaml:"index" mapstructure:"index"`
	Answer *AnswerFlowConfig `yaml:"answer" mapstructure:"answer"`
	Draft  *DraftFlowConfig  `yaml:"draft" mapstructure:"draft"`
}

// ActivitiesConfig groups reusable building blocks used by flows.
type ActivitiesConfig struct {
	Summarize *SummarizeActivityConfig `yaml:"summarize" mapstructure:"summarize"`
}

// WriteFlowConfig holds configuration for the write command.
type WriteFlowConfig struct {
	Tasks        map[string]*WriteTask `yaml:"tasks" mapstructure:"tasks"`
	Metadata     map[string]any        `yaml:"metadata" mapstructure:"metadata"`
	Preset       string                `yaml:"preset" mapstructure:"preset"`
	SystemPrompt string                `yaml:"system_prompt" mapstructure:"system_prompt"`
}

// WriteTask defines a specific generation task.
type WriteTask struct {
	Preset       string `yaml:"preset" mapstructure:"preset"`
	SystemPrompt string `yaml:"system_prompt" mapstructure:"system_prompt"`
	UserPrompt   string `yaml:"user_prompt" mapstructure:"user_prompt"`
}

// IndexFlowConfig defines configuration for document indexing.
type IndexFlowConfig struct {
	Chunker   *ChunkerConfig `yaml:"chunker" mapstructure:"chunker"`
	Preset    string         `yaml:"preset" mapstructure:"preset"`
	BatchSize int            `yaml:"batch_size" mapstructure:"batch_size"`
}

// ChunkerConfig defines parameters for text chunking.
type ChunkerConfig struct {
	MaxRunes     int `yaml:"max_runes" mapstructure:"max_runes"`
	OverlapRunes int `yaml:"overlap_runes" mapstructure:"overlap_runes"`
}

// AnswerFlowConfig defines configuration for RAG-based question answering.
type AnswerFlowConfig struct {
	Retrieval    *RetrievalConfig `yaml:"retrieval" mapstructure:"retrieval"`
	Preset       string           `yaml:"preset" mapstructure:"preset"`
	SystemPrompt string           `yaml:"system_prompt" mapstructure:"system_prompt"`
}

// RetrievalConfig defines retrieval settings for RAG.
type RetrievalConfig struct {
	TopK     int     `yaml:"top_k" mapstructure:"top_k"`
	MinScore float32 `yaml:"min_score" mapstructure:"min_score"`
}

// AgentConfig holds configuration for agent mode.
type AgentConfig struct {
	Tools        *AgentToolsConfig               `yaml:"tools" mapstructure:"tools"`
	Pipelines    map[string]*AgentPipelineConfig `yaml:"pipelines" mapstructure:"pipelines"`
	Personas     map[string]*PersonaConfig       `yaml:"personas" mapstructure:"personas"`
	Safety       *AgentSafetyConfig              `yaml:"safety" mapstructure:"safety"`
	SystemPrompt string                          `yaml:"system_prompt" mapstructure:"system_prompt"`
}

// AgentPipelineConfig defines a named pipeline with shared prompt and step order.
type AgentPipelineConfig struct {
	Instructions string   `yaml:"instructions" mapstructure:"instructions"`
	Steps        []string `yaml:"steps" mapstructure:"steps"`
}

// PersonaConfig defines a reusable agent persona.
type PersonaConfig struct {
	Role             string   `yaml:"role" mapstructure:"role"`
	Preset           string   `yaml:"preset" mapstructure:"preset"`
	Tools            []string `yaml:"tools" mapstructure:"tools"`
	SystemPersona    string   `yaml:"system_persona" mapstructure:"system_persona"`
	UserInstructions string   `yaml:"user_instructions" mapstructure:"user_instructions"`
	AllowedDelegates []string `yaml:"allowed_delegates" mapstructure:"allowed_delegates"`
	AllowedTasks     []string `yaml:"allowed_tasks" mapstructure:"allowed_tasks"`
}

// AgentSafetyConfig defines safety limits for the agent.
type AgentSafetyConfig struct {
	CircuitBreaker *CircuitBreakerConfig `yaml:"circuit_breaker" mapstructure:"circuit_breaker"`
	MaxSteps       int                   `yaml:"max_steps" mapstructure:"max_steps"`
	DryRun         bool                  `yaml:"dry_run" mapstructure:"dry_run"`
}

// CircuitBreakerConfig defines circuit breaker settings.
type CircuitBreakerConfig struct {
	MaxRestarts int `yaml:"max_restarts" mapstructure:"max_restarts"`
}

// AgentToolsConfig defines tool defaults for agent mode.
type AgentToolsConfig struct {
	SearchDefaults   *AgentSearchDefaults `yaml:"search_defaults" mapstructure:"search_defaults"`
	ToolDescriptions map[string]string    `yaml:"tool_descriptions" mapstructure:"tool_descriptions"`
}

// AgentSearchDefaults defines defaults for embeddings search.
type AgentSearchDefaults struct {
	Snapshots []string `yaml:"snapshots" mapstructure:"snapshots"`
	TopK      int      `yaml:"top_k" mapstructure:"top_k"`
	MinScore  float32  `yaml:"min_score" mapstructure:"min_score"`
}

// DraftFlowConfig groups commit/pr draft commands.
type DraftFlowConfig struct {
	Commit *CommandFlowConfig `yaml:"commit" mapstructure:"commit"`
	Pr     *CommandFlowConfig `yaml:"pr" mapstructure:"pr"`
}

// SummarizeActivityConfig holds configuration for the summarization engine.
type SummarizeActivityConfig struct {
	Preset       string           `yaml:"preset" mapstructure:"preset"`
	Strategy     *StrategyConfig  `yaml:"strategy" mapstructure:"strategy"`
	SystemPrompt string           `yaml:"system_prompt" mapstructure:"system_prompt"`
	Rules        []*SummarizeRule `yaml:"rules" mapstructure:"rules"`
}

// StrategyConfig defines summarization strategy with its settings.
type StrategyConfig struct {
	Type                string `yaml:"type" mapstructure:"type"`
	IncludeOriginalFile bool   `yaml:"include_original_file" mapstructure:"include_original_file"`
	IncludeChangedFile  bool   `yaml:"include_changed_file" mapstructure:"include_changed_file"`
}

// SummarizeRule defines file-specific summarization rules.
type SummarizeRule struct {
	Match        string          `yaml:"match" mapstructure:"match"`
	Preset       string          `yaml:"preset" mapstructure:"preset"`
	Strategy     *StrategyConfig `yaml:"strategy" mapstructure:"strategy"`
	SystemPrompt string          `yaml:"system_prompt" mapstructure:"system_prompt"`
	Skip         bool            `yaml:"skip" mapstructure:"skip"`
}

// CommandFlowConfig defines configuration for commit and PR commands.
type CommandFlowConfig struct {
	Preset       string `yaml:"preset" mapstructure:"preset"`
	Strategy     string `yaml:"strategy" mapstructure:"strategy"`
	SystemPrompt string `yaml:"system_prompt" mapstructure:"system_prompt"`
}
