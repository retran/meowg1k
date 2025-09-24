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
	"strings"

	"github.com/retran/meowg1k/internal/services/command"
	"github.com/retran/meowg1k/internal/services/task"
)

// GeneratePromptService is a service that provides prompts for the generate command.
type GeneratePromptService struct {
	systemPrompt string
	userPrompt   string
}

// Compile-time interface satisfaction check
var _ SystemPromptProvider = (*GeneratePromptService)(nil)
var _ SystemPromptProvider = (*GeneratePromptService)(nil)

// NewGeneratePromptService creates a new instance of the prompt service for generate command.
func NewGeneratePromptService(commandService command.Service, taskService task.Service) (*GeneratePromptService, error) {
	systemPrompt, err := buildSystemPrompt(taskService)
	if err != nil {
		return nil, err
	}

	userPrompt, err := buildUserPrompt(commandService, taskService)
	if err != nil {
		return nil, err
	}

	return &GeneratePromptService{
		systemPrompt: systemPrompt,
		userPrompt:   userPrompt,
	}, nil
}

// GetSystemPrompt returns the system prompt.
func (g *GeneratePromptService) GetSystemPrompt() (string, error) {
	return g.systemPrompt, nil
}

// GetUserPrompt returns the user prompt.
func (g *GeneratePromptService) GetUserPrompt() (string, error) {
	return g.userPrompt, nil
}

func buildSystemPrompt(taskService task.Service) (string, error) {
	return taskService.Get().SystemPrompt, nil
}

func buildUserPrompt(commandService command.Service, taskService task.Service) (string, error) {
	sb := strings.Builder{}

	userPrompt := taskService.Get().UserPrompt

	contents := commandService.GetStdIn()

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
