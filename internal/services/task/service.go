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

	mdProfile "github.com/retran/meowg1k/internal/models/profile"
	"github.com/retran/meowg1k/internal/services/command"
	"github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/profile"
)

// Service provides task configuration resolution capabilities.
type Service interface {
	// Get returns the resolved task configuration.
	Get() *TaskConfiguration
}

// TaskConfiguration represents a resolved task configuration.
type TaskConfiguration struct {
	Profile      *mdProfile.ResolvedProfile
	SystemPrompt string
	UserPrompt   string
}

// serviceImpl is the concrete implementation of the task resolver service.
type serviceImpl struct {
	Service
	profileService profile.Service
	cachedConfig   *TaskConfiguration
}

// Compile-time interface satisfaction check
var _ Service = (*serviceImpl)(nil)

// NewService creates a new task resolver service.
// It loads and validates the task configuration at creation time.
func NewService(commandService command.Service, configService config.Service, profileService profile.Service) (Service, error) {
	service := &serviceImpl{}

	cfg := configService.GetConfig()
	if cfg == nil {
		return nil, fmt.Errorf("no configuration available")
	}

	taskName, err := commandService.GetTaskName()
	if err != nil {
		return nil, fmt.Errorf("failed to get task name: %w", err)
	}

	cmdUserPrompt, err := commandService.GetUserPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get user prompt: %w", err)
	}

	var profileName, systemPrompt, userPrompt string
	if taskName != "" && cfg.Generate != nil && cfg.Generate.Tasks != nil {
		task, exists := cfg.Generate.Tasks[taskName]
		if !exists {
			return nil, fmt.Errorf("task '%s' not found in configuration", taskName)
		}

		profileName = task.Profile
		systemPrompt = task.SystemPrompt
		if cmdUserPrompt != "" {
			userPrompt = cmdUserPrompt
		} else {
			userPrompt = task.UserPrompt
		}

		if profileName == "" && cfg.Generate.Default != nil {
			profileName = cfg.Generate.Default.Profile
		}

		if systemPrompt == "" && cfg.Generate.Default != nil {
			systemPrompt = cfg.Generate.Default.SystemPrompt
		}
	} else {
		if cfg.Generate == nil || cfg.Generate.Default == nil {
			return nil, fmt.Errorf("no default configuration available")
		}

		profileName = cfg.Generate.Default.Profile
		systemPrompt = cfg.Generate.Default.SystemPrompt
		userPrompt = cmdUserPrompt
	}

	profileName = strings.TrimSpace(profileName)
	systemPrompt = strings.TrimSpace(systemPrompt)
	userPrompt = strings.TrimSpace(userPrompt)

	if profileName == "" {
		return nil, fmt.Errorf("no profile configured")
	}

	if taskName == "" && userPrompt == "" {
		return nil, fmt.Errorf("user prompt is required (use -p or --user-prompt)")
	}

	resolvedProfile, err := profileService.Get(mdProfile.Profile(profileName))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve profile '%s': %w", profileName, err)
	}

	service.cachedConfig = &TaskConfiguration{
		Profile:      resolvedProfile,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}

	return service, nil
}

// Get returns the cached task configuration.
func (s *serviceImpl) Get() *TaskConfiguration {
	return s.cachedConfig
}
