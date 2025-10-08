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

	"github.com/retran/meowg1k/internal/services/task"
)

// StandardInputReader reads content from standard input.
type StandardInputReader interface {
	GetStdIn() string
}

// TaskConfigurationProvider provides task configuration.
type TaskConfigurationProvider interface {
	Get() *task.Configuration
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
	systemPrompt := buildSystemPrompt(taskConfigProvider)
	userPrompt := buildUserPrompt(stdInReader, taskConfigProvider)

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

func buildSystemPrompt(taskConfigProvider TaskConfigurationProvider) string {
	return taskConfigProvider.Get().SystemPrompt
}

func buildUserPrompt(stdInReader StandardInputReader, taskConfigProvider TaskConfigurationProvider) string {
	sb := strings.Builder{}

	userPrompt := taskConfigProvider.Get().UserPrompt

	contents := stdInReader.GetStdIn()

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

	return sb.String()
}
