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

// GeneratePromptService constructs prompts for the generate command.
type GeneratePromptService struct {
	systemPrompt string
	userPrompt   string
}

// NewGeneratePromptService creates a prompt service for the generate command.
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
