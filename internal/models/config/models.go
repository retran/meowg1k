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

	"github.com/retran/meowg1k/internal/models/gateway"
	"github.com/retran/meowg1k/internal/models/llm"
)

// Config represents the complete meowg1k configuration.
// Configuration files are read in order of precedence:
// 1. Explicit config via --config flag (overrides all others)
// 2. Project config: ./.meowg1k/config.yaml
// 3. User config: ~/.config/meowg1k/config.yaml
type Config struct {
	// Logging configuration for controlling verbosity and output
	Logging *LoggingConfig `yaml:"logging" mapstructure:"logging"`

	// Profiles define reusable LLM request configurations
	// Key: profile name (string), Value: Profile configuration
	// Example: profiles.fast, profiles.smart, profiles.local
	Profiles map[string]*Profile `yaml:"profiles" mapstructure:"profiles"`

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

// LoggingConfig defines logging behavior and verbosity levels.
type LoggingConfig struct {
	// Level sets the minimum log level to output
	// Accepted values: "debug", "info", "warn", "error"
	// CLI flags take precedence over this configuration
	Level string `yaml:"level" mapstructure:"level"`

	// Quiet suppresses all log output except errors
	// CLI --quiet flag takes precedence over this configuration
	Quiet bool `yaml:"quiet" mapstructure:"quiet"`
}

// Profile defines a reusable LLM configuration that can be referenced by different commands.
// Profiles allow you to define provider settings once and reuse them across tasks.
type Profile struct {
	// Provider specifies the LLM provider to use
	// Accepted values: "openai", "anthropic", "gemini", "openrouter", "nebius", "voyage", "llama", "openai-compatible"
	Provider string `yaml:"provider" mapstructure:"provider"`

	// Model specifies the model name (optional, uses smart defaults if omitted)
	// Examples: "gpt-4o", "claude-3-5-sonnet-20241022", "gemini-2.5-flash"
	Model string `yaml:"model" mapstructure:"model"`

	// MaxInputTokens sets the maximum input token limit (optional, defaults to model limits)
	// Range: 1-2000000 depending on model capabilities
	MaxInputTokens int `yaml:"maxInputTokens" mapstructure:"maxInputTokens"`

	// MaxOutputTokens sets the maximum output token limit (optional, defaults to model limits)
	// Range: 1-128000 depending on model capabilities
	MaxOutputTokens int `yaml:"maxOutputTokens" mapstructure:"maxOutputTokens"`

	// Timeout sets the request timeout duration (optional, defaults to 5m)
	// Format: "30s", "5m", "1h" (Go duration format)
	Timeout time.Duration `yaml:"timeout" mapstructure:"timeout"`

	// BaseURL sets the API base URL (required for "llama" and "openai-compatible" providers)
	// Examples: "http://localhost:8080", "https://api.custom-llm.com/v1"
	BaseURL string `yaml:"baseURL" mapstructure:"baseURL"`

	// APIKeyEnv specifies the environment variable containing the API key (optional, uses smart defaults)
	// Examples: "CUSTOM_API_KEY", "OPENAI_API_KEY"
	APIKeyEnv string `yaml:"apiKeyEnv" mapstructure:"apiKeyEnv"`

	// TokenizerType specifies the tokenizer to use (optional, auto-detected from model)
	// Used internally for token counting and validation
	TokenizerType llm.TokenizerType `yaml:"tokenizerType" mapstructure:"tokenizerType"`
}

// GenerateConfig holds configuration for the generate command.
// Defines default settings and task-specific overrides for content generation.
type GenerateConfig struct {
	// Default settings used when no task is specified
	Default *GenerateDefault `yaml:"default" mapstructure:"default"`

	// Tasks define named generation tasks with specific prompts and settings
	// Key: task name (string), Value: GenerateTask configuration
	// Tasks can be invoked via: meow generate -t <task_name>
	Tasks map[string]*GenerateTask `yaml:"tasks" mapstructure:"tasks"`
}

// GenerateDefault defines default settings for the generate command.
type GenerateDefault struct {
	// Profile references a profile defined in the profiles section
	// Must match a key from the profiles map
	Profile string `yaml:"profile" mapstructure:"profile"`

	// SystemPrompt sets the default system prompt for all generation requests
	// Used when no task-specific system prompt is provided
	SystemPrompt string `yaml:"systemPrompt" mapstructure:"systemPrompt"`
}

// GenerateTask defines a specific generation task.
// Tasks allow predefined prompts and settings for common use cases.
type GenerateTask struct {
	// Profile references a profile defined in the profiles section (optional)
	// If omitted, uses the default profile from GenerateDefault
	Profile string `yaml:"profile" mapstructure:"profile"`

	// SystemPrompt overrides the default system prompt for this task (optional)
	SystemPrompt string `yaml:"systemPrompt" mapstructure:"systemPrompt"`

	// UserPrompt sets the task-specific user prompt
	// This prompt is sent along with any stdin input
	UserPrompt string `yaml:"userPrompt" mapstructure:"userPrompt"`
}

// FilterConfig defines files to ignore during analysis.
// Used by summarization and other analysis commands.
type FilterConfig struct {
	// Ignore specifies glob patterns for files to exclude from analysis
	// Examples: ["*.log", "dist/**", ".git/**", "node_modules/**"]
	Ignore []string `yaml:"ignore" mapstructure:"ignore"`
}

// Strategy defines summarization strategy with its settings.
type Strategy struct {
	// Type specifies the summarization approach
	// Accepted values: "plaintext", "diff", "structured"
	Type string `yaml:"type" mapstructure:"type"`

	// SendFullFile determines whether to send the entire file content
	// true: send complete file, false: send only changes/excerpts
	SendFullFile bool `yaml:"sendFullFile" mapstructure:"sendFullFile"`
}

// SummarizeConfig holds configuration for the summarization engine.
// Used during the "Map" phase of change analysis.
type SummarizeConfig struct {
	// Default summarization settings used when no rule matches
	Default *SummarizeDefault `yaml:"default" mapstructure:"default"`

	// Rules define file-specific summarization behavior
	// Rules are evaluated in order; first match wins
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
	// Examples: "*.go", "**/*.test.js", "README.md"
	Match string `yaml:"match" mapstructure:"match"`

	// Profile references a profile defined in the profiles section (optional)
	// If omitted, uses the default profile
	Profile string `yaml:"profile" mapstructure:"profile"`

	// Strategy defines how matching files should be processed (optional)
	// If omitted, uses the default strategy
	Strategy *Strategy `yaml:"strategy" mapstructure:"strategy"`

	// SystemPrompt overrides the default system prompt for matching files (optional)
	SystemPrompt string `yaml:"systemPrompt" mapstructure:"systemPrompt"`

	// Skip indicates whether to skip processing matching files entirely
	// When true, matching files are excluded from summarization
	Skip bool `yaml:"skip" mapstructure:"skip"`
}

// CommandConfig defines configuration for commit and PR commands.
// Used during the "Reduce" phase of change analysis.
type CommandConfig struct {
	// Profile references a profile defined in the profiles section
	Profile string `yaml:"profile" mapstructure:"profile"`

	// SystemPrompt sets the system prompt for the command
	// Should provide instructions specific to the command's purpose
	SystemPrompt string `yaml:"systemPrompt" mapstructure:"systemPrompt"`
}

// ProviderDefinition defines the characteristics of a provider.
type ProviderDefinition struct {
	Type            gateway.Provider  `json:"type"`
	Name            string            `json:"name"`
	DefaultModel    string            `json:"default_model"`
	DefaultBaseURL  string            `json:"default_base_url"`
	DefaultEnvVar   string            `json:"default_env_var"`
	RequiresAPIKey  bool              `json:"requires_api_key"`
	RequiresBaseURL bool              `json:"requires_base_url"`
	TokenizerType   llm.TokenizerType `json:"tokenizer_type"`
	MaxInputTokens  int               `json:"max_input_tokens"`
	MaxOutputTokens int               `json:"max_output_tokens"`
	DefaultTimeout  time.Duration     `json:"default_timeout"`
}

// ResolvedProfile represents a profile with all values resolved.
type ResolvedProfile struct {
	Provider        gateway.Provider
	Model           string
	MaxInputTokens  int
	MaxOutputTokens int
	Timeout         time.Duration
	BaseURL         string
	APIKey          string
	TokenizerType   llm.TokenizerType
}
