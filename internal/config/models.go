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

// Package config provides configuration structure and loading for meowg1k.
package config

import (
	"time"

	"github.com/retran/meowg1k/internal/models"
)

// Config represents the complete meowg1k configuration.
type Config struct {
	// Profiles define reusable LLM request configurations
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

// Profile defines a reusable LLM configuration that can be referenced by different commands.
type Profile struct {
	Provider        string               `yaml:"provider" mapstructure:"provider"`
	Model           string               `yaml:"model" mapstructure:"model"`
	MaxInputTokens  int                  `yaml:"maxInputTokens" mapstructure:"maxInputTokens"`
	MaxOutputTokens int                  `yaml:"maxOutputTokens" mapstructure:"maxOutputTokens"`
	Timeout         time.Duration        `yaml:"timeout" mapstructure:"timeout"`
	BaseURL         string               `yaml:"baseURL" mapstructure:"baseURL"`
	APIKeyEnv       string               `yaml:"apiKeyEnv" mapstructure:"apiKeyEnv"`
	TokenizerType   models.TokenizerType `yaml:"tokenizerType" mapstructure:"tokenizerType"`
}

// GenerateConfig holds configuration for the generate command.
type GenerateConfig struct {
	Default *GenerateDefault         `yaml:"default" mapstructure:"default"`
	Tasks   map[string]*GenerateTask `yaml:"tasks" mapstructure:"tasks"`
}

// GenerateDefault defines default settings for the generate command.
type GenerateDefault struct {
	Profile      string `yaml:"profile" mapstructure:"profile"`
	SystemPrompt string `yaml:"systemPrompt" mapstructure:"systemPrompt"`
}

// GenerateTask defines a specific generation task.
type GenerateTask struct {
	Profile      string `yaml:"profile" mapstructure:"profile"`
	SystemPrompt string `yaml:"systemPrompt" mapstructure:"systemPrompt"`
	UserPrompt   string `yaml:"userPrompt" mapstructure:"userPrompt"`
}

// FilterConfig defines files to ignore during analysis.
type FilterConfig struct {
	Ignore []string `yaml:"ignore" mapstructure:"ignore"`
}

// Strategy defines summarization strategy with its settings.
type Strategy struct {
	Type         string `yaml:"type" mapstructure:"type"`
	SendFullFile bool   `yaml:"sendFullFile" mapstructure:"sendFullFile"`
}

// SummarizeConfig holds configuration for the summarization engine.
type SummarizeConfig struct {
	Default *SummarizeDefault `yaml:"default" mapstructure:"default"`
	Rules   []*SummarizeRule  `yaml:"rules" mapstructure:"rules"`
}

// SummarizeDefault defines default summarization settings.
type SummarizeDefault struct {
	Profile      string    `yaml:"profile" mapstructure:"profile"`
	Strategy     *Strategy `yaml:"strategy" mapstructure:"strategy"`
	SystemPrompt string    `yaml:"systemPrompt" mapstructure:"systemPrompt"`
}

// SummarizeRule defines file-specific summarization rules.
type SummarizeRule struct {
	Match        string    `yaml:"match" mapstructure:"match"`
	Profile      string    `yaml:"profile" mapstructure:"profile"`
	Strategy     *Strategy `yaml:"strategy" mapstructure:"strategy"`
	SystemPrompt string    `yaml:"systemPrompt" mapstructure:"systemPrompt"`
	Skip         bool      `yaml:"skip" mapstructure:"skip"`
}

// CommandConfig defines configuration for commit and PR commands.
type CommandConfig struct {
	Profile      string `yaml:"profile" mapstructure:"profile"`
	SystemPrompt string `yaml:"systemPrompt" mapstructure:"systemPrompt"`
}
