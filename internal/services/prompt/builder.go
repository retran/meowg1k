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

	configservice "github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/utils/io"
)

// Builder provides prompt composition capabilities from various sources.
type Builder interface {
	// BuildUserPrompt combines base prompt with stdin content using the specified wrapper.
	// If basePrompt is empty but stdin has content, uses stdin as the prompt.
	// If basePrompt exists and stdin has content, appends stdin using the wrapper format.
	BuildUserPrompt(basePrompt string, stdinWrapper string) (string, error)

	// CombinePrompts merges multiple prompt parts with proper spacing.
	// Skips empty parts and ensures proper newline separation.
	CombinePrompts(parts ...string) string

	// ResolvePrompt resolves a prompt configuration using the current config.
	ResolvePrompt(promptName string) (string, error)
}

// builderImpl is the concrete implementation of the prompt builder service.
type builderImpl struct {
	configService configservice.Service
}

// NewBuilder creates a new prompt builder service.
func NewBuilder(configService configservice.Service) Builder {
	return &builderImpl{
		configService: configService,
	}
}

// BuildUserPrompt combines base prompt with stdin content using the specified wrapper.
func (b *builderImpl) BuildUserPrompt(basePrompt string, stdinWrapper string) (string, error) {
	// Read stdin content
	stdinContent, err := io.ReadFromStdin()
	if err != nil {
		return "", fmt.Errorf("failed to read stdin: %w", err)
	}

	// If no stdin content, return base prompt as-is
	if stdinContent == "" {
		return basePrompt, nil
	}

	// If no base prompt but stdin has content, use stdin as the prompt
	if basePrompt == "" {
		return stdinContent, nil
	}

	// Both base prompt and stdin exist - combine them using wrapper
	wrappedStdin := strings.ReplaceAll(stdinWrapper, "%s", stdinContent)
	return basePrompt + wrappedStdin, nil
}

// CombinePrompts merges multiple prompt parts with proper spacing.
func (b *builderImpl) CombinePrompts(parts ...string) string {
	var nonEmptyParts []string

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			nonEmptyParts = append(nonEmptyParts, trimmed)
		}
	}

	if len(nonEmptyParts) == 0 {
		return ""
	}

	return strings.Join(nonEmptyParts, "\n\n")
}

// ResolvePrompt resolves a prompt configuration using the current config.
func (b *builderImpl) ResolvePrompt(promptName string) (string, error) {
	cfg := b.configService.GetConfig()
	// No need to check for nil since manager service guarantees a loaded config

	// Look for prompts in the generate tasks
	if cfg.Generate != nil && cfg.Generate.Tasks != nil {
		if task, exists := cfg.Generate.Tasks[promptName]; exists {
			if task.UserPrompt != "" {
				return task.UserPrompt, nil
			}
		}
	}

	return "", fmt.Errorf("prompt '%s' not found", promptName)
}
