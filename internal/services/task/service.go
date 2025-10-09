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
	ErrTaskParametersReaderIsNil       = errors.New("task parameters reader is nil")
	ErrConfigReaderIsNil               = errors.New("config reader is nil")
	ErrProfileResolverIsNil            = errors.New("profile resolver is nil")
	ErrServiceIsNil                    = errors.New("service is nil")
)

// Configuration represents a resolved task configuration.
type Configuration struct {
	Name         string
	Profile      *profile.ResolvedProfile
	SystemPrompt string
	UserPrompt   string
}

// TaskParametersReader reads task parameters from command line.
type TaskParametersReader interface {
	GetTaskName() (string, error)
	GetUserPrompt() (string, error)
}

// ConfigReader reads the application configuration.
type ConfigReader interface {
	GetConfig() (*config.Config, error)
}

// ProfileResolver resolves profile configurations.
type ProfileResolver interface {
	Get(profile profile.Profile) (*profile.ResolvedProfile, error)
}

// Service resolves and caches task configurations.
type Service struct {
	cachedConfig *Configuration
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
		// TODO proper error
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
	if cfg == nil || cfg.Generate == nil || cfg.Generate.Default == nil {
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
		return ErrNoProfileConfigured
	}

	if taskName == "" && userPrompt == "" {
		return ErrUserPromptRequired
	}

	return nil
}

// NewService creates a new task configuration service.
func NewService(
	taskParametersReader TaskParametersReader,
	configReader ConfigReader,
	profileResolver ProfileResolver,
) (*Service, error) {
	if taskParametersReader == nil {
		return nil, ErrTaskParametersReaderIsNil
	}

	if configReader == nil {
		return nil, ErrConfigReaderIsNil
	}

	if profileResolver == nil {
		return nil, ErrProfileResolverIsNil
	}

	service := &Service{}

	cfg, err := configReader.GetConfig()
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to get application config: %w", err)
	}
	if cfg == nil {
		return nil, ErrNoConfigurationAvailable
	}

	taskName, err := taskParametersReader.GetTaskName()
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to get task name: %w", err)
	}

	taskName = strings.TrimSpace(taskName)

	cmdUserPrompt, err := taskParametersReader.GetUserPrompt()
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to get user prompt: %w", err)
	}

	profileName, systemPrompt, userPrompt, err := resolveTaskConfiguration(taskName, cmdUserPrompt, cfg)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	err = validateConfiguration(taskName, profileName, userPrompt)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	resolvedProfile, err := profileResolver.Get(profile.Profile(profileName))
	if err != nil {
		// TODO proper error
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
func (s *Service) Get() (*Configuration, error) {
	if s == nil {
		return nil, ErrServiceIsNil
	}

	if s.cachedConfig == nil {
		return nil, ErrNoConfigurationAvailable
	}

	return s.cachedConfig, nil
}
