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

// Package config provides configuration models for the meowg1k application.
package config

import (
	"time"

	mdGateway "github.com/retran/meowg1k/internal/models/gateway"
	mdLLM "github.com/retran/meowg1k/internal/models/llm"
)

// Config represents the complete meowg1k configuration.
type Config struct {
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

	// PR command configuration ("Reduce" phase)
	PR *CommandConfig `yaml:"pr" mapstructure:"pr"`
}

// ProfileDefinition defines a reusable LLM configuration that can be referenced by different commands.
// Profiles allow you to define provider settings once and reuse them across tasks.
type ProfileDefinition struct {
	// Provider specifies the LLM provider to use
	Provider string `yaml:"provider" mapstructure:"provider"`

	// Model specifies the model name (optional, uses smart defaults if omitted)
	Model string `yaml:"model" mapstructure:"model"`

	// MaxInputTokens sets the maximum input token limit (optional, defaults to model limits)
	MaxInputTokens int `yaml:"maxInputTokens" mapstructure:"maxInputTokens"`

	// MaxOutputTokens sets the maximum output token limit (optional, defaults to model limits)
	MaxOutputTokens int `yaml:"maxOutputTokens" mapstructure:"maxOutputTokens"`

	// Timeout sets the request timeout duration (optional, defaults to 5m)
	Timeout time.Duration `yaml:"timeout" mapstructure:"timeout"`

	// BaseURL sets the API base URL (required for "llama" and "openai-compatible" providers)
	BaseURL string `yaml:"baseURL" mapstructure:"baseURL"`

	// APIKeyEnv specifies the environment variable containing the API key (optional, uses smart defaults)
	APIKeyEnv string `yaml:"apiKeyEnv" mapstructure:"apiKeyEnv"`

	// TokenizerType specifies the tokenizer to use (optional, auto-detected from model)
	TokenizerType mdLLM.TokenizerType `yaml:"tokenizerType" mapstructure:"tokenizerType"`
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

	// IncludeFullFile determines whether to send the entire file content
	IncludeFullFile bool `yaml:"includeFullFile" mapstructure:"includeFullFile"`
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
	// Match specifies a glob pattern for files this rule applies to
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

	// SystemPrompt sets the system prompt for the command
	SystemPrompt string `yaml:"systemPrompt" mapstructure:"systemPrompt"`
}

// ProviderDefinition defines the characteristics of a provider.
type ProviderDefinition struct {
	Type            mdGateway.Provider  `json:"type"`
	Name            string              `json:"name"`
	DefaultModel    string              `json:"default_model"`
	DefaultBaseURL  string              `json:"default_base_url"`
	DefaultEnvVar   string              `json:"default_env_var"`
	RequiresAPIKey  bool                `json:"requires_api_key"`
	RequiresBaseURL bool                `json:"requires_base_url"`
	TokenizerType   mdLLM.TokenizerType `json:"tokenizer_type"`
	MaxInputTokens  int                 `json:"max_input_tokens"`
	MaxOutputTokens int                 `json:"max_output_tokens"`
	DefaultTimeout  time.Duration       `json:"default_timeout"`
}
