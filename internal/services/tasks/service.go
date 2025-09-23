/*
Copyright © 2025 Andrew Vasilyev <me@retran.m// ResolveTaskConfiguration resolves profile and prompts based on task flag or defaults using current command and config.
func (s *serviceImpl) ResolveTaskConfiguration() (profileName, systemPrompt, userPrompt string, err error) {
	cfg := s.configService.GetConfig()
	// No need to check for nil since config service guarantees a loaded config

	// Check if a specific task is requested
	taskName, err := s.commandService.GetTaskName()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse task flag: %w", err)
	}under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tasks

import (
	"fmt"

	"github.com/retran/meowg1k/internal/services/command"
	configservice "github.com/retran/meowg1k/internal/services/config"
)

// Service provides task configuration resolution capabilities.
type Service interface {
	// ResolveTaskConfiguration resolves profile and prompts based on task flag or defaults using current command and config.
	ResolveTaskConfiguration() (profileName, systemPrompt, userPrompt string, err error)
}

// serviceImpl is the concrete implementation of the task resolver service.
type serviceImpl struct {
	commandService command.Service
	configService  configservice.Service
}

// Compile-time interface satisfaction check
var _ Service = (*serviceImpl)(nil)

// NewService creates a new task resolver service.
func NewService(commandService command.Service, configService configservice.Service) Service {
	return &serviceImpl{
		commandService: commandService,
		configService:  configService,
	}
}

// ResolveTaskConfiguration resolves profile and prompts based on task flag or defaults using current command and config.
func (s *serviceImpl) ResolveTaskConfiguration() (profileName, systemPrompt, userPrompt string, err error) {
	cfg := s.configService.GetConfig()
	// No need to check for nil since manager service guarantees a loaded config

	// Check if a specific task is requested
	taskName, err := s.commandService.GetTaskName()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get task name: %w", err)
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
		cmdUserPrompt, err := s.commandService.GetUserPrompt()
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
		cmdUserPrompt, err := s.commandService.GetUserPrompt()
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
