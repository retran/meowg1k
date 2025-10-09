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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"

	"github.com/retran/meowg1k/internal/services/llm"
)

// Configuration errors
var (
	ErrSpecifiedConfigFileNotFound = errors.New("specified config file not found")
	ErrNoConfigFoundInStdLocations = errors.New("no configuration file found in standard locations")
	ErrFilePathResolverIsNil       = errors.New("config path resolver is nil")
	ErrServiceIsNil                = errors.New("service is nil")
)

const (
	projectName      = "meowg1k"
	projectConfigDir = "." + projectName
	configFileName   = "config"
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

	// PR command configuration ("Reduce" phase)
	PR *CommandConfig `yaml:"pr" mapstructure:"pr"`
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

	// TokenizerType specifies the tokenizer to use (optional, auto-detected from model)
	TokenizerType llm.TokenizerType `yaml:"tokenizerType" mapstructure:"tokenizerType"`

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

	// SystemPrompt sets the system prompt for the command
	SystemPrompt string `yaml:"systemPrompt" mapstructure:"systemPrompt"`
}

// FilePathResolver resolves the configuration file path.
type FilePathResolver interface {
	GetConfigPath() (string, error)
}

// Service loads and provides application configuration.
type Service struct {
	config *Config
}

// NewService creates a new configuration service and loads configuration at creation time.
func NewService(filePathResolver FilePathResolver) (*Service, error) {
	if filePathResolver == nil {
		return nil, ErrFilePathResolverIsNil
	}

	service := &Service{}
	v := viper.New()

	configPath, err := filePathResolver.GetConfigPath()
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to get config path from command: %w", err)
	}

	if configPath != "" {
		err = loadSpecificConfigFile(v, configPath)
	} else {
		err = loadDefaultConfigFiles(v)
	}

	if err != nil {
		// TODO proper error
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	service.config = &cfg

	return service, nil
}

// loadSpecificConfigFile loads a specific config file path.
func loadSpecificConfigFile(v *viper.Viper, configPath string) error {
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			// TODO proper error
			return fmt.Errorf("%w: %s", ErrSpecifiedConfigFileNotFound, configPath)
		}

		// TODO proper error
		return fmt.Errorf("failed to read config file: %w", err)
	}

	return nil
}

// loadDefaultConfigFiles loads configuration files from standard locations.
func loadDefaultConfigFiles(v *viper.Viper) error {
	v.SetConfigName(configFileName)
	v.SetConfigType("yaml")

	configPaths := getConfigPaths()
	foundAny := false

	for _, path := range configPaths {
		found, err := tryLoadConfigFromPath(v, path, !foundAny)
		if err != nil {
			// TODO proper error
			return err
		}

		if found {
			foundAny = true
		}
	}

	if !foundAny {
		return ErrNoConfigFoundInStdLocations
	}

	return nil
}

func tryLoadConfigFromPath(v *viper.Viper, path string, primary bool) (bool, error) {
	configFile := filepath.Join(path, configFileName+".yaml")

	if _, err := os.Stat(configFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}

		// TODO proper error
		return false, fmt.Errorf("failed to access config file %s: %w", configFile, err)
	}

	if primary {
		v.AddConfigPath(path)

		if err := v.ReadInConfig(); err != nil {
			// TODO proper error
			return false, fmt.Errorf("failed to read config from %s: %w", configFile, err)
		}

		return true, nil
	}

	v.SetConfigFile(configFile)

	if err := v.MergeInConfig(); err != nil {
		// TODO proper error
		return false, fmt.Errorf("failed to merge config from %s: %w", configFile, err)
	}

	return true, nil
}

// getConfigPaths returns the standard configuration file search paths.
func getConfigPaths() []string {
	var configPaths []string

	systemConfigDirs := os.Getenv("XDG_CONFIG_DIRS")
	if systemConfigDirs == "" {
		systemConfigDirs = "/etc/xdg"
	}

	configPaths = append(configPaths, filepath.Join(systemConfigDirs, projectName))

	userConfigDir := os.Getenv("XDG_CONFIG_HOME")
	if userConfigDir == "" {
		if home := os.Getenv("HOME"); home != "" {
			userConfigDir = filepath.Join(home, ".config")
		}
	}

	if userConfigDir != "" {
		configPaths = append(configPaths, filepath.Join(userConfigDir, projectName))
	}

	if cwd, err := os.Getwd(); err == nil {
		configPaths = append(configPaths, filepath.Join(cwd, projectConfigDir))
	}

	return configPaths
}

// GetConfig returns the loaded configuration.
func (s *Service) GetConfig() (*Config, error) {
	if s == nil {
		return nil, ErrServiceIsNil
	}

	return s.config, nil
}
