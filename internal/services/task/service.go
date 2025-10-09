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
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/core/config"
	"github.com/retran/meowg1k/internal/core/ports"
	"github.com/retran/meowg1k/internal/core/profile"
	"github.com/retran/meowg1k/internal/core/task"
)

// Service resolves and caches task configurations.
type Service struct {
	resolvedConfig *task.ResolvedConfig
}

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
		return "", "", "", fmt.Errorf("task not found in configuration: %s", taskName)
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
	if cfg == nil || cfg.Generate == nil || cfg.Generate.Default == nil {
		err = fmt.Errorf("no default configuration available")
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

	if cfg != nil && cfg.Generate != nil && finalProfileName == "" && cfg.Generate.Default != nil {
		finalProfileName = cfg.Generate.Default.Profile
	}

	if cfg != nil && cfg.Generate != nil && finalSystemPrompt == "" && cfg.Generate.Default != nil {
		finalSystemPrompt = cfg.Generate.Default.SystemPrompt
	}

	return finalProfileName, finalSystemPrompt
}

// validateConfiguration validates the resolved configuration.
func validateConfiguration(taskName, profileName, userPrompt string) error {
	if profileName == "" {
		return fmt.Errorf("no profile configured")
	}

	if taskName == "" && userPrompt == "" {
		return fmt.Errorf("user prompt is required (use -p or --user-prompt)")
	}

	return nil
}

// NewService creates a new task configuration service.
func NewService(
	taskParametersReader ports.TaskParametersReader,
	configResolver ports.ConfigResolver,
	profileResolver ports.ProfileResolver,
) (*Service, error) {
	if taskParametersReader == nil {
		return nil, fmt.Errorf("task parameters reader is nil")
	}

	if configResolver == nil {
		return nil, fmt.Errorf("config resolver is nil")
	}

	if profileResolver == nil {
		return nil, fmt.Errorf("profile resolver is nil")
	}

	service := &Service{}

	cfg, err := configResolver.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get application config: %w", err)
	}
	if cfg == nil {
		return nil, fmt.Errorf("no configuration available")
	}

	taskName, err := taskParametersReader.GetTaskName()
	if err != nil {
		return nil, fmt.Errorf("failed to get task name: %w", err)
	}

	taskName = strings.TrimSpace(taskName)

	cmdUserPrompt, err := taskParametersReader.GetUserPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get user prompt: %w", err)
	}

	profileName, systemPrompt, userPrompt, err := resolveTaskConfiguration(taskName, cmdUserPrompt, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve task configuration: %w", err)
	}

	err = validateConfiguration(taskName, profileName, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to validate configuration: %w", err)
	}

	resolvedProfile, err := profileResolver.Get(profile.Profile(profileName))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve profile '%s': %w", profileName, err)
	}

	service.resolvedConfig = &task.ResolvedConfig{
		Name:         taskName,
		Profile:      resolvedProfile,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}

	return service, nil
}

// Get returns the cached task configuration.
func (s *Service) Get() (*task.ResolvedConfig, error) {
	if s == nil {
		return nil, fmt.Errorf("service is nil")
	}

	if s.resolvedConfig == nil {
		return nil, fmt.Errorf("no configuration available")
	}

	return s.resolvedConfig, nil
}
