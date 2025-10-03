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

// Package task provides functionality to resolve and validate task configurations.
package task

import (
	"errors"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/services/command"
	"github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/profile"
)

// Task service errors
var (
	ErrTaskNotFoundInConfig            = errors.New("task not found in configuration")
	ErrNoDefaultConfigurationAvailable = errors.New("no default configuration available")
	ErrNoProfileConfigured             = errors.New("no profile configured")
	ErrUserPromptRequired              = errors.New("user prompt is required (use -p or --user-prompt)")
	ErrNoConfigurationAvailable        = errors.New("no configuration available")
)

// Service provides task configuration resolution capabilities.
type Service interface {
	// Get returns the resolved task configuration.
	Get() *Configuration
}

// Configuration represents a resolved task configuration.
type Configuration struct {
	Name         string
	Profile      *profile.ResolvedProfile
	SystemPrompt string
	UserPrompt   string
}

// serviceImpl is the concrete implementation of the task resolver service.
type serviceImpl struct {
	cachedConfig *Configuration
}

// Compile-time interface satisfaction check
var _ Service = (*serviceImpl)(nil)

// resolveTaskConfiguration resolves task configuration from the config and command-line inputs.
func resolveTaskConfiguration(
	taskName, cmdUserPrompt string,
	cfg *config.Config,
) (profileName, systemPrompt, userPrompt string, err error) {
	if taskName == "" || cfg.Generate == nil || cfg.Generate.Tasks == nil {
		return resolveDefaultConfiguration(cmdUserPrompt, cfg)
	}

	task, exists := cfg.Generate.Tasks[taskName]
	if !exists {
		return "", "", "", fmt.Errorf("%w: %s", ErrTaskNotFoundInConfig, taskName)
	}

	profileName = task.Profile
	systemPrompt = task.SystemPrompt

	if cmdUserPrompt != "" {
		userPrompt = cmdUserPrompt
	} else {
		userPrompt = task.UserPrompt
	}

	profileName, systemPrompt = applyDefaults(profileName, systemPrompt, cfg)

	return strings.TrimSpace(profileName), strings.TrimSpace(systemPrompt), strings.TrimSpace(userPrompt), nil
}

func resolveDefaultConfiguration(
	cmdUserPrompt string, cfg *config.Config,
) (profileName, systemPrompt, userPrompt string, err error) {
	if cfg.Generate == nil || cfg.Generate.Default == nil {
		err = ErrNoDefaultConfigurationAvailable
		return profileName, systemPrompt, userPrompt, err
	}

	profileName = strings.TrimSpace(cfg.Generate.Default.Profile)
	systemPrompt = strings.TrimSpace(cfg.Generate.Default.SystemPrompt)
	userPrompt = strings.TrimSpace(cmdUserPrompt)

	return profileName, systemPrompt, userPrompt, err
}

// applyDefaults applies default values for profile and system prompt if they are empty.
func applyDefaults(
	profileName, systemPrompt string, cfg *config.Config,
) (finalProfileName, finalSystemPrompt string) {
	finalProfileName = profileName
	finalSystemPrompt = systemPrompt

	if finalProfileName == "" && cfg.Generate.Default != nil {
		finalProfileName = cfg.Generate.Default.Profile
	}

	if finalSystemPrompt == "" && cfg.Generate.Default != nil {
		finalSystemPrompt = cfg.Generate.Default.SystemPrompt
	}

	return finalProfileName, finalSystemPrompt
}

// validateConfiguration validates the resolved configuration.
func validateConfiguration(taskName, profileName, userPrompt string) error {
	if profileName == "" {
		return ErrNoProfileConfigured
	}

	if taskName == "" && userPrompt == "" {
		return ErrUserPromptRequired
	}

	return nil
}

// NewService creates a new task resolver service.
func NewService(
	commandService command.Service,
	configService config.Service,
	profileService profile.Service,
) (Service, error) {
	service := &serviceImpl{}

	cfg := configService.GetConfig()
	if cfg == nil {
		return nil, ErrNoConfigurationAvailable
	}

	taskName, err := commandService.GetTaskName()
	if err != nil {
		return nil, fmt.Errorf("failed to get task name: %w", err)
	}

	taskName = strings.TrimSpace(taskName)

	cmdUserPrompt, err := commandService.GetUserPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get user prompt: %w", err)
	}

	profileName, systemPrompt, userPrompt, err := resolveTaskConfiguration(taskName, cmdUserPrompt, cfg)
	if err != nil {
		return nil, err
	}

	err = validateConfiguration(taskName, profileName, userPrompt)
	if err != nil {
		return nil, err
	}

	resolvedProfile, err := profileService.Get(profile.Profile(profileName))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve profile '%s': %w", profileName, err)
	}

	service.cachedConfig = &Configuration{
		Name:         taskName,
		Profile:      resolvedProfile,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}

	return service, nil
}

// Get returns the cached task configuration.
func (s *serviceImpl) Get() *Configuration {
	return s.cachedConfig
}
