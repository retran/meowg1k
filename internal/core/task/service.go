// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package task provides services for managing predefined tasks with prompts and configurations.
package task

import (
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/preset"
	task2 "github.com/retran/meowg1k/internal/domain/task"
	"github.com/retran/meowg1k/internal/ports"
)

// ParametersReader reads task parameters from command line.
type ParametersReader interface {
	GetTaskName() (string, error)
	GetUserPrompt() (string, error)
}

// Service resolves and caches task configurations.
type Service struct {
	resolvedConfig *task2.ResolvedConfig
}

// resolveTaskConfiguration resolves task configuration from the config and command-line inputs.
func resolveTaskConfiguration(
	taskName, cmdUserPrompt string,
	cfg *config.Config,
) (presetName, systemPrompt, userPrompt string, err error) {
	if taskName == "" || cfg.Flows == nil || cfg.Flows.Write == nil || cfg.Flows.Write.Tasks == nil {
		return resolveDefaultConfiguration(cmdUserPrompt, cfg)
	}

	task, exists := cfg.Flows.Write.Tasks[taskName]
	if !exists {
		return "", "", "", fmt.Errorf("task not found in configuration: %s", taskName)
	}

	presetName = task.Preset
	systemPrompt = task.SystemPrompt

	if cmdUserPrompt != "" {
		userPrompt = cmdUserPrompt
	} else {
		userPrompt = task.UserPrompt
	}

	presetName, systemPrompt = applyDefaults(presetName, systemPrompt, cfg)

	return strings.TrimSpace(presetName), strings.TrimSpace(systemPrompt), strings.TrimSpace(userPrompt), nil
}

func resolveDefaultConfiguration(
	cmdUserPrompt string, cfg *config.Config,
) (presetName, systemPrompt, userPrompt string, err error) {
	if cfg == nil || cfg.Flows == nil || cfg.Flows.Write == nil {
		err = fmt.Errorf("no default write configuration available")
		return presetName, systemPrompt, userPrompt, err
	}

	presetName = strings.TrimSpace(cfg.Flows.Write.Preset)
	systemPrompt = strings.TrimSpace(cfg.Flows.Write.SystemPrompt)
	userPrompt = strings.TrimSpace(cmdUserPrompt)

	return presetName, systemPrompt, userPrompt, err
}

// applyDefaults applies default values for preset and system prompt if they are empty.
func applyDefaults(
	presetName, systemPrompt string, cfg *config.Config,
) (finalPresetName, finalSystemPrompt string) {
	finalPresetName = presetName
	finalSystemPrompt = systemPrompt

	if cfg != nil && cfg.Flows != nil && cfg.Flows.Write != nil && finalPresetName == "" {
		finalPresetName = cfg.Flows.Write.Preset
	}

	if cfg != nil && cfg.Flows != nil && cfg.Flows.Write != nil && finalSystemPrompt == "" {
		finalSystemPrompt = cfg.Flows.Write.SystemPrompt
	}

	return finalPresetName, finalSystemPrompt
}

// validateConfiguration validates the resolved configuration.
func validateConfiguration(taskName, presetName, userPrompt string) error {
	if presetName == "" {
		return fmt.Errorf("no preset configured")
	}

	if taskName == "" && userPrompt == "" {
		return fmt.Errorf("user prompt is required (use -p or --user-prompt)")
	}

	return nil
}

// NewService creates a new task configuration service.
func NewService(
	taskParametersReader ParametersReader,
	configResolver ports.ConfigResolver,
	presetResolver ports.PresetResolver,
) (*Service, error) {
	if taskParametersReader == nil {
		return nil, fmt.Errorf("task parameters reader is nil")
	}

	if configResolver == nil {
		return nil, fmt.Errorf("config resolver is nil")
	}

	if presetResolver == nil {
		return nil, fmt.Errorf("preset resolver is nil")
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

	presetName, systemPrompt, userPrompt, err := resolveTaskConfiguration(taskName, cmdUserPrompt, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve task configuration: %w", err)
	}

	err = validateConfiguration(taskName, presetName, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to validate configuration: %w", err)
	}

	resolvedPreset, err := presetResolver.Get(preset.Preset(presetName))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve preset %q: %w", presetName, err)
	}

	service.resolvedConfig = &task2.ResolvedConfig{
		Name:         taskName,
		Preset:       resolvedPreset,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}

	return service, nil
}

// Get returns the cached task configuration.
func (s *Service) Get() (*task2.ResolvedConfig, error) {
	if s == nil {
		return nil, fmt.Errorf("service is nil")
	}

	if s.resolvedConfig == nil {
		return nil, fmt.Errorf("no configuration available")
	}

	return s.resolvedConfig, nil
}
