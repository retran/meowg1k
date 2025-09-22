/*package resolver

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

package resolver

import (
	"fmt"
	"os"

	"github.com/retran/meowg1k/internal/config"
	"github.com/retran/meowg1k/internal/services/config/command"
	"github.com/retran/meowg1k/internal/services/config/manager"
	"github.com/retran/meowg1k/internal/services/config/registry"
	"github.com/retran/meowg1k/internal/services/config/validator"
)

// Service provides configuration resolution capabilities.
type Service interface {
	// ResolveProfile resolves a profile with validation using the current config.
	ResolveProfile(profileName string) (*config.ResolvedProfile, error)

	// ResolvePrompt resolves a prompt configuration using the current config.
	ResolvePrompt(promptName string) (string, error)

	// ResolveTaskConfiguration resolves profile and prompts based on task flag or defaults using current command and config.
	ResolveTaskConfiguration() (profileName, systemPrompt, userPrompt string, err error)
}

// serviceImpl is the concrete implementation of the resolver service.
type serviceImpl struct {
	registryService  registry.Service
	validatorService validator.Service
	commandService   command.Service
	managerService   manager.Service
}

// Compile-time interface satisfaction check
var _ Service = (*serviceImpl)(nil)

// NewService creates a new configuration resolver service.
func NewService(registryService registry.Service, validatorService validator.Service, commandService command.Service, managerService manager.Service) Service {
	return &serviceImpl{
		registryService:  registryService,
		validatorService: validatorService,
		commandService:   commandService,
		managerService:   managerService,
	}
}

// ResolveProfile resolves a profile with validation using the current config.
func (s *serviceImpl) ResolveProfile(profileName string) (*config.ResolvedProfile, error) {
	cfg := s.managerService.GetConfig()
	// No need to check for nil since manager service guarantees a loaded config

	if cfg.Profiles == nil {
		return nil, fmt.Errorf("no profiles defined in configuration")
	}

	profile, exists := cfg.Profiles[profileName]
	if !exists {
		return nil, fmt.Errorf("profile '%s' not found in configuration", profileName)
	}

	// Get provider definition
	providerDef, err := s.registryService.GetProvider(profile.Provider)
	if err != nil {
		return nil, fmt.Errorf("unknown provider '%s' in profile '%s': %w", profile.Provider, profileName, err)
	}

	// Apply defaults for missing values
	resolved := &config.ResolvedProfile{
		Provider:        providerDef.Type,
		Model:           profile.Model,
		MaxInputTokens:  profile.MaxInputTokens,
		MaxOutputTokens: profile.MaxOutputTokens,
		Timeout:         profile.Timeout,
		BaseURL:         profile.BaseURL,
		TokenizerType:   profile.TokenizerType,
	}

	// Apply provider defaults if values are not set
	if resolved.Model == "" {
		resolved.Model = providerDef.DefaultModel
	}

	if resolved.MaxInputTokens == 0 {
		resolved.MaxInputTokens = providerDef.MaxInputTokens
	}

	if resolved.MaxOutputTokens == 0 {
		resolved.MaxOutputTokens = providerDef.MaxOutputTokens
	}

	if resolved.Timeout == 0 {
		resolved.Timeout = providerDef.DefaultTimeout
	}

	if resolved.TokenizerType == "" {
		resolved.TokenizerType = providerDef.TokenizerType
	}

	if resolved.BaseURL == "" {
		resolved.BaseURL = providerDef.DefaultBaseURL
	}

	// Resolve API key from environment
	apiKeyEnv := profile.APIKeyEnv
	if apiKeyEnv == "" && providerDef.DefaultEnvVar != "" {
		// Use default environment variable name from provider definition
		apiKeyEnv = providerDef.DefaultEnvVar
	}

	if apiKeyEnv != "" {
		resolved.APIKey = os.Getenv(apiKeyEnv)
	}

	// Validate the resolved profile
	if err := s.validatorService.ValidateResolvedProfile(resolved); err != nil {
		return nil, fmt.Errorf("profile validation failed: %w", err)
	}

	return resolved, nil
}

// ResolvePrompt resolves a prompt configuration using the current config.
func (s *serviceImpl) ResolvePrompt(promptName string) (string, error) {
	cfg := s.managerService.GetConfig()
	// No need to check for nil since manager service guarantees a loaded config

	// Look for prompts in the generate tasks
	if cfg.Generate != nil && cfg.Generate.Tasks != nil {
		if task, exists := cfg.Generate.Tasks[promptName]; exists {
			if task.UserPrompt != "" {
				return task.UserPrompt, nil
			}
		}
	}

	return "", fmt.Errorf("prompt '%s' not found", promptName)
}

// ResolveTaskConfiguration resolves profile and prompts based on task flag or defaults using current command and config.
func (s *serviceImpl) ResolveTaskConfiguration() (profileName, systemPrompt, userPrompt string, err error) {
	cmd := s.commandService.GetCommand()
	// No need to check for nil since command service guarantees a command

	cfg := s.managerService.GetConfig()
	// No need to check for nil since manager service guarantees a loaded config

	// Check if a specific task is requested
	taskName, err := cmd.Flags().GetString("task")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse task flag: %w", err)
	}

	if taskName != "" {
		// Use task-specific configuration
		if cfg.Generate == nil || cfg.Generate.Tasks == nil {
			return "", "", "", fmt.Errorf("no tasks configured in generate.tasks")
		}

		task, exists := cfg.Generate.Tasks[taskName]
		if !exists {
			return "", "", "", fmt.Errorf("task '%s' not found in configuration", taskName)
		}

		// Use task's profile or fall back to default
		if task.Profile != "" {
			profileName = task.Profile
		} else if cfg.Generate != nil && cfg.Generate.Default != nil && cfg.Generate.Default.Profile != "" {
			profileName = cfg.Generate.Default.Profile
		} else {
			return "", "", "", fmt.Errorf("no profile configured for task '%s' and no default profile", taskName)
		}

		// Use task's system prompt, fall back to default system prompt if task doesn't define one
		systemPrompt = task.SystemPrompt
		if systemPrompt == "" && cfg.Generate != nil && cfg.Generate.Default != nil {
			systemPrompt = cfg.Generate.Default.SystemPrompt
		}

		// Use task's user prompt as base, but allow command-line override
		cmdUserPrompt, err := cmd.Flags().GetString("user-prompt")
		if err != nil {
			return "", "", "", fmt.Errorf("failed to parse user-prompt flag: %w", err)
		}
		if cmdUserPrompt != "" {
			// Command-line user prompt overrides task's user prompt
			userPrompt = cmdUserPrompt
		} else {
			// Use task's predefined user prompt
			userPrompt = task.UserPrompt
		}
	} else {
		// Use default configuration
		if cfg.Generate == nil || cfg.Generate.Default == nil || cfg.Generate.Default.Profile == "" {
			return "", "", "", fmt.Errorf("no default profile configured in generate.default.profile")
		}

		profileName = cfg.Generate.Default.Profile

		// Get system prompt from config default
		if cfg.Generate.Default != nil {
			systemPrompt = cfg.Generate.Default.SystemPrompt
		}

		// User prompt is required from command line when not using a task
		cmdUserPrompt, err := cmd.Flags().GetString("user-prompt")
		if err != nil {
			return "", "", "", fmt.Errorf("failed to parse user-prompt flag: %w", err)
		}
		if cmdUserPrompt == "" {
			return "", "", "", fmt.Errorf("user prompt is required (use -p or --user-prompt)")
		}
		userPrompt = cmdUserPrompt
	}

	return profileName, systemPrompt, userPrompt, nil
}
