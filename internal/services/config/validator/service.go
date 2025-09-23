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

package validator

import (
	"errors"
	"fmt"
	"time"

	"github.com/retran/meowg1k/internal/config"
	"github.com/retran/meowg1k/internal/services/config/registry"
	"github.com/retran/meowg1k/internal/services/gateway"
)

// Service provides configuration validation capabilities.
type Service interface {
	// ValidateConfig validates the entire configuration.
	ValidateConfig(cfg *config.Config) error

	// ValidateProfile validates a specific profile.
	ValidateProfile(profile *config.Profile, profileName string) error

	// ValidateResolvedProfile validates a resolved profile.
	ValidateResolvedProfile(resolved *config.ResolvedProfile) error
}

// serviceImpl is the concrete implementation of the validator service.
type serviceImpl struct {
	registryService registry.Service
}

// Compile-time interface satisfaction check
var _ Service = (*serviceImpl)(nil)

// NewService creates a new configuration validator service.
func NewService(registryService registry.Service) Service {
	return &serviceImpl{
		registryService: registryService,
	}
}

// ValidateConfig validates the entire configuration.
func (s *serviceImpl) ValidateConfig(cfg *config.Config) error {
	var errs []error

	if cfg == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	// Validate profiles
	if len(cfg.Profiles) == 0 {
		errs = append(errs, fmt.Errorf("profiles: at least one profile must be defined"))
	} else {
		for name, profile := range cfg.Profiles {
			if err := s.ValidateProfile(profile, name); err != nil {
				errs = append(errs, fmt.Errorf("profiles.%s: %w", name, err))
			}
		}
	}

	// Validate generate configuration
	if cfg.Generate != nil {
		if err := s.validateGenerateConfig(cfg.Generate); err != nil {
			errs = append(errs, fmt.Errorf("generate: %w", err))
		}
	}

	// Validate filter configuration
	if cfg.Filter != nil {
		if err := s.validateFilterConfig(cfg.Filter); err != nil {
			errs = append(errs, fmt.Errorf("filter: %w", err))
		}
	}

	// Validate summarize configuration
	if cfg.Summarize != nil {
		if err := s.validateSummarizeConfig(cfg.Summarize); err != nil {
			errs = append(errs, fmt.Errorf("summarize: %w", err))
		}
	}

	// Validate commit configuration
	if cfg.Commit != nil {
		if err := s.validateCommandConfig(cfg.Commit, "commit"); err != nil {
			errs = append(errs, fmt.Errorf("commit: %w", err))
		}
	}

	// Validate PR configuration
	if cfg.PR != nil {
		if err := s.validateCommandConfig(cfg.PR, "pr"); err != nil {
			errs = append(errs, fmt.Errorf("pr: %w", err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// ValidateProfile validates a specific profile.
func (s *serviceImpl) ValidateProfile(profile *config.Profile, profileName string) error {
	if profile == nil {
		return fmt.Errorf("profile cannot be nil")
	}

	var errs []error

	// Validate provider
	if profile.Provider == "" {
		errs = append(errs, fmt.Errorf("profile '%s': provider is required", profileName))
	} else if !s.registryService.HasProvider(profile.Provider) {
		errs = append(errs, fmt.Errorf("profile '%s': invalid provider '%s'", profileName, profile.Provider))
	}

	// Validate model
	if profile.Model == "" {
		// If no model is specified, check if the provider has a default model
		if profile.Provider != "" && s.registryService.HasProvider(profile.Provider) {
			providerType := gateway.Provider(profile.Provider)
			defaultProfile := s.registryService.GetDefaultProfile(providerType)
			if defaultProfile.Model == "" {
				errs = append(errs, fmt.Errorf("profile '%s': model is required (provider '%s' has no default model)", profileName, profile.Provider))
			}
			// If default model exists, validation passes
		} else {
			errs = append(errs, fmt.Errorf("profile '%s': model is required", profileName))
		}
	} else {
		// Enhanced model validation
		if err := s.validateModelName(profile.Model, profileName); err != nil {
			errs = append(errs, err)
		}
	}

	// Enhanced timeout validation
	if profile.Timeout != 0 {
		if profile.Timeout < time.Second {
			errs = append(errs, fmt.Errorf("profile '%s': timeout must be at least 1 second, got %v", profileName, profile.Timeout))
		} else if profile.Timeout > 30*time.Minute {
			errs = append(errs, fmt.Errorf("profile '%s': timeout is too large (max 30 minutes), got %v", profileName, profile.Timeout))
		}
	}

	// Enhanced token validation
	if profile.MaxInputTokens != 0 {
		if profile.MaxInputTokens <= 0 {
			errs = append(errs, fmt.Errorf("profile '%s': max input tokens must be positive, got %d", profileName, profile.MaxInputTokens))
		} else if profile.MaxInputTokens > 2000000 {
			errs = append(errs, fmt.Errorf("profile '%s': max input tokens is too large (max 2,000,000), got %d", profileName, profile.MaxInputTokens))
		}
	}

	if profile.MaxOutputTokens != 0 {
		if profile.MaxOutputTokens <= 0 {
			errs = append(errs, fmt.Errorf("profile '%s': max output tokens must be positive, got %d", profileName, profile.MaxOutputTokens))
		} else if profile.MaxOutputTokens > 200000 {
			errs = append(errs, fmt.Errorf("profile '%s': max output tokens is too large (max 200,000), got %d", profileName, profile.MaxOutputTokens))
		}
	}

	// Validate BaseURL for providers that require it
	if err := s.validateBaseURL(profile, profileName); err != nil {
		errs = append(errs, err)
	}

	// Validate API key environment variable
	if err := s.validateAPIKeyEnv(profile, profileName); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// ValidateResolvedProfile validates a resolved profile.
func (s *serviceImpl) ValidateResolvedProfile(resolved *config.ResolvedProfile) error {
	if resolved == nil {
		return fmt.Errorf("resolved profile cannot be nil")
	}

	// Check timeout
	if resolved.Timeout == 0 {
		resolved.Timeout = 5 * time.Minute // Apply default
	} else if resolved.Timeout < time.Second {
		return fmt.Errorf("timeout must be at least 1 second, got %v", resolved.Timeout)
	}

	// Check output tokens
	if resolved.MaxOutputTokens <= 0 {
		resolved.MaxOutputTokens = 4096 // Apply default
	} else if resolved.MaxOutputTokens > 200000 {
		return fmt.Errorf("max output tokens too large: %d (max 200000)", resolved.MaxOutputTokens)
	}

	// Check input tokens
	if resolved.MaxInputTokens <= 0 {
		resolved.MaxInputTokens = 128000 // Apply default
	} else if resolved.MaxInputTokens > 2000000 {
		return fmt.Errorf("max input tokens too large: %d (max 2000000)", resolved.MaxInputTokens)
	}

	// Check model
	if resolved.Model == "" {
		return fmt.Errorf("model name is required")
	}

	return nil
}

// Helper validation methods

func (s *serviceImpl) validateGenerateConfig(generate *config.GenerateConfig) error {
	if generate == nil {
		return nil
	}

	var errs []error

	// Validate default configuration
	if generate.Default != nil {
		if err := s.validateGenerateDefault(generate.Default); err != nil {
			errs = append(errs, err)
		}
	}

	// Validate tasks
	if generate.Tasks != nil {
		for name, task := range generate.Tasks {
			if err := s.validateGenerateTask(task, name); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (s *serviceImpl) validateGenerateDefault(defaultConfig *config.GenerateDefault) error {
	if defaultConfig == nil {
		return nil
	}

	if defaultConfig.Profile == "" {
		return fmt.Errorf("generate.default.profile: profile reference is required")
	}

	return nil
}

func (s *serviceImpl) validateGenerateTask(task *config.GenerateTask, taskName string) error {
	if task == nil {
		return fmt.Errorf("generate.tasks.%s: task cannot be nil", taskName)
	}

	if task.UserPrompt == "" {
		return fmt.Errorf("generate.tasks.%s.user_prompt: user prompt is required for tasks", taskName)
	}

	return nil
}

func (s *serviceImpl) validateFilterConfig(filter *config.FilterConfig) error {
	if filter == nil {
		return nil
	}

	if len(filter.Ignore) == 0 {
		return fmt.Errorf("filter.ignore: at least one ignore pattern should be specified")
	}

	return nil
}

func (s *serviceImpl) validateSummarizeConfig(summarize *config.SummarizeConfig) error {
	if summarize == nil {
		return nil
	}

	var errs []error

	// Validate default configuration
	if summarize.Default != nil {
		if err := s.validateSummarizeDefault(summarize.Default); err != nil {
			errs = append(errs, err)
		}
	}

	// Validate rules
	if summarize.Rules != nil {
		for i, rule := range summarize.Rules {
			if err := s.validateSummarizeRule(rule, i); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (s *serviceImpl) validateSummarizeDefault(defaultConfig *config.SummarizeDefault) error {
	if defaultConfig == nil {
		return nil
	}

	var errs []error

	if defaultConfig.Profile == "" {
		errs = append(errs, fmt.Errorf("summarize.default.profile: profile reference is required"))
	}

	if defaultConfig.Strategy != nil {
		if err := s.validateStrategy(defaultConfig.Strategy, "summarize.default.strategy"); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (s *serviceImpl) validateSummarizeRule(rule *config.SummarizeRule, index int) error {
	if rule == nil {
		return fmt.Errorf("summarize.rules[%d]: rule cannot be nil", index)
	}

	var errs []error

	if rule.Match == "" {
		errs = append(errs, fmt.Errorf("summarize.rules[%d].match: match pattern is required", index))
	}

	if rule.Strategy != nil {
		if err := s.validateStrategy(rule.Strategy, fmt.Sprintf("summarize.rules[%d].strategy", index)); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (s *serviceImpl) validateStrategy(strategy *config.Strategy, fieldPath string) error {
	if strategy == nil {
		return nil
	}

	validTypes := []string{"plaintext", "diff", "structured"}
	if strategy.Type == "" {
		return fmt.Errorf("%s.type: strategy type is required", fieldPath)
	} else {
		valid := false
		for _, validType := range validTypes {
			if strategy.Type == validType {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("%s.type: invalid strategy type '%s' (valid: %v)", fieldPath, strategy.Type, validTypes)
		}
	}

	return nil
}

func (s *serviceImpl) validateCommandConfig(cmd *config.CommandConfig, cmdName string) error {
	if cmd == nil {
		return nil
	}

	var errs []error

	if cmd.Profile == "" {
		errs = append(errs, fmt.Errorf("%s.profile: profile reference is required", cmdName))
	}

	if cmd.SystemPrompt == "" {
		errs = append(errs, fmt.Errorf("%s.system_prompt: system prompt is required", cmdName))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// validateModelName validates the model name format and content
func (s *serviceImpl) validateModelName(model, profileName string) error {
	if len(model) == 0 {
		return fmt.Errorf("profile '%s': model name cannot be empty", profileName)
	}

	if len(model) > 100 {
		return fmt.Errorf("profile '%s': model name is too long (max 100 characters), got %d", profileName, len(model))
	}

	// Check for common invalid characters in model names
	for _, char := range model {
		if char < 32 || char > 126 { // Only printable ASCII characters
			return fmt.Errorf("profile '%s': model name contains invalid character (only printable ASCII allowed)", profileName)
		}
	}

	return nil
}

// validateBaseURL validates base URL requirements for specific providers
func (s *serviceImpl) validateBaseURL(profile *config.Profile, profileName string) error {
	switch profile.Provider {
	case "llama", "openai-compatible":
		if profile.BaseURL == "" {
			return fmt.Errorf("profile '%s': baseURL is required for provider '%s'", profileName, profile.Provider)
		}

		// Basic URL format validation
		if len(profile.BaseURL) < 7 { // Minimum for "http://"
			return fmt.Errorf("profile '%s': baseURL is too short to be valid", profileName)
		}

		if !(profile.BaseURL[:7] == "http://" || (len(profile.BaseURL) >= 8 && profile.BaseURL[:8] == "https://")) {
			return fmt.Errorf("profile '%s': baseURL must start with http:// or https://", profileName)
		}

	default:
		// For other providers, baseURL should be empty or ignored
		if profile.BaseURL != "" {
			// This is a warning case, not an error - some users might set it anyway
			// We could log a warning here if we had access to a logger
		}
	}

	return nil
}

// validateAPIKeyEnv validates API key environment variable names
func (s *serviceImpl) validateAPIKeyEnv(profile *config.Profile, profileName string) error {
	if profile.APIKeyEnv == "" {
		// Empty is fine, defaults will be used
		return nil
	}

	// Validate environment variable name format
	if len(profile.APIKeyEnv) == 0 {
		return fmt.Errorf("profile '%s': apiKeyEnv cannot be empty string (omit field for defaults)", profileName)
	}

	if len(profile.APIKeyEnv) > 100 {
		return fmt.Errorf("profile '%s': apiKeyEnv is too long (max 100 characters)", profileName)
	}

	// Basic validation for environment variable name format
	// Should start with letter or underscore, contain only alphanumeric and underscore
	firstChar := profile.APIKeyEnv[0]
	if !((firstChar >= 'A' && firstChar <= 'Z') || (firstChar >= 'a' && firstChar <= 'z') || firstChar == '_') {
		return fmt.Errorf("profile '%s': apiKeyEnv must start with letter or underscore", profileName)
	}

	for i, char := range profile.APIKeyEnv {
		if !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_') {
			return fmt.Errorf("profile '%s': apiKeyEnv contains invalid character at position %d (only letters, numbers, underscore allowed)", profileName, i)
		}
	}

	return nil
}
