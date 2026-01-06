// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package prompt provides services for generating prompts from templates with variable substitution.
package prompt

import (
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/domain/task"
)

// StandardInputReader reads content from standard input.
type StandardInputReader interface {
	GetStdIn() (string, error)
}

// TaskConfigurationProvider provides task configuration.
type TaskConfigurationProvider interface {
	Get() (*task.ResolvedConfig, error)
}

// GeneratePromptService constructs prompts for the write command.
type GeneratePromptService struct {
	systemPrompt string
	userPrompt   string
}

// NewGeneratePromptService creates a prompt service for the write command.
func NewGeneratePromptService(
	stdInReader StandardInputReader,
	taskConfigProvider TaskConfigurationProvider,
) (*GeneratePromptService, error) {
	if stdInReader == nil {
		return nil, fmt.Errorf("standard input reader is nil")
	}

	if taskConfigProvider == nil {
		return nil, fmt.Errorf("task configuration provider is nil")
	}

	systemPrompt, err := buildSystemPrompt(taskConfigProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to build system prompt: %w", err)
	}

	userPrompt, err := buildUserPrompt(stdInReader, taskConfigProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to build user prompt: %w", err)
	}

	return &GeneratePromptService{
		systemPrompt: systemPrompt,
		userPrompt:   userPrompt,
	}, nil
}

// GetSystemPrompt returns the system prompt.
func (g *GeneratePromptService) GetSystemPrompt() (string, error) {
	if g == nil {
		return "", fmt.Errorf("prompt service is nil")
	}

	return g.systemPrompt, nil
}

// GetUserPrompt returns the user prompt.
func (g *GeneratePromptService) GetUserPrompt() (string, error) {
	if g == nil {
		return "", fmt.Errorf("prompt service is nil")
	}

	return g.userPrompt, nil
}

func buildSystemPrompt(taskConfigProvider TaskConfigurationProvider) (string, error) {
	cfg, err := taskConfigProvider.Get()
	if err != nil {
		return "", fmt.Errorf("failed to get task configuration: %w", err)
	}

	return cfg.SystemPrompt, nil
}

func buildUserPrompt(stdInReader StandardInputReader, taskConfigProvider TaskConfigurationProvider) (string, error) {
	sb := strings.Builder{}

	cfg, err := taskConfigProvider.Get()
	if err != nil {
		return "", fmt.Errorf("failed to get task configuration: %w", err)
	}

	userPrompt := cfg.UserPrompt

	contents, err := stdInReader.GetStdIn()
	if err != nil {
		return "", fmt.Errorf("failed to read stdin: %w", err)
	}

	if userPrompt != "" {
		sb.WriteString(userPrompt)
		sb.WriteString("\n")
	}

	if contents != "" {
		if userPrompt != "" {
			sb.WriteString("```\n")
		}

		sb.WriteString(contents)

		if userPrompt != "" {
			sb.WriteString("\n```")
		}

		sb.WriteString("\n")
	}

	return sb.String(), nil
}
