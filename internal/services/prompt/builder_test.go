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
	"os"
	"testing"

	"github.com/retran/meowg1k/internal/models/config"
	configservice "github.com/retran/meowg1k/internal/services/config"
	"github.com/stretchr/testify/mock"
)

// Mock manager service for testing
type mockManagerService struct {
	mock.Mock
}

func (m *mockManagerService) GetConfig() *config.Config {
	args := m.Called()
	return args.Get(0).(*config.Config)
}

func (m *mockManagerService) LoadConfig() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockManagerService) LoadConfigFromPath(configPath string) error {
	args := m.Called(configPath)
	return args.Error(0)
}

func (m *mockManagerService) LoadFromSources(sources ...configservice.ConfigSource) error {
	args := m.Called(sources)
	return args.Error(0)
}

func TestBuilder_CombinePrompts(t *testing.T) {
	mockManager := &mockManagerService{}
	builder := NewBuilder(mockManager)

	tests := []struct {
		name     string
		parts    []string
		expected string
	}{
		{
			name:     "empty parts",
			parts:    []string{},
			expected: "",
		},
		{
			name:     "single part",
			parts:    []string{"Hello"},
			expected: "Hello",
		},
		{
			name:     "multiple parts",
			parts:    []string{"Hello", "World", "!"},
			expected: "Hello\n\nWorld\n\n!",
		},
		{
			name:     "parts with empty strings",
			parts:    []string{"Hello", "", "World", "   ", "!"},
			expected: "Hello\n\nWorld\n\n!",
		},
		{
			name:     "parts with whitespace",
			parts:    []string{" Hello ", " World "},
			expected: "Hello\n\nWorld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.CombinePrompts(tt.parts...)
			if result != tt.expected {
				t.Errorf("CombinePrompts() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBuilder_BuildUserPrompt_NoStdin(t *testing.T) {
	mockManager := &mockManagerService{}
	builder := NewBuilder(mockManager)

	// Test with base prompt but no stdin (since we can't easily mock stdin in tests)
	result, err := builder.BuildUserPrompt("base prompt", "\n\n```\n%s\n```")
	if err != nil {
		t.Errorf("BuildUserPrompt() error = %v", err)
	}

	// Without stdin, should return base prompt unchanged
	expected := "base prompt"
	if result != expected {
		t.Errorf("BuildUserPrompt() = %q, want %q", result, expected)
	}
}

func TestBuilder_BuildUserPrompt_WithStdin(t *testing.T) {
	mockManager := &mockManagerService{}
	builder := NewBuilder(mockManager)

	// Save original stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	// Create a pipe and replace stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdin = r

	// Write content to stdin in a goroutine
	stdinContent := "content from stdin"
	go func() {
		defer w.Close()
		w.WriteString(stdinContent)
	}()

	// Test BuildUserPrompt with stdin content
	result, err := builder.BuildUserPrompt("base prompt", "\n\n```\n%s\n```")
	if err != nil {
		t.Errorf("BuildUserPrompt() error = %v", err)
	}

	// Should combine base prompt with stdin content using the template
	expected := "base prompt\n\n```\ncontent from stdin\n```"
	if result != expected {
		t.Errorf("BuildUserPrompt() = %q, want %q", result, expected)
	}
}

func TestBuilder_ResolvePrompt_Success(t *testing.T) {
	mockManager := &mockManagerService{}
	builder := NewBuilder(mockManager)

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Tasks: map[string]*config.GenerateTask{
				"test-task": {
					UserPrompt: "Test user prompt",
				},
			},
		},
	}

	mockManager.On("GetConfig").Return(cfg)

	// Execute
	result, err := builder.ResolvePrompt("test-task")

	// Assert
	if err != nil {
		t.Errorf("ResolvePrompt() error = %v", err)
	}
	if result != "Test user prompt" {
		t.Errorf("ResolvePrompt() = %q, want %q", result, "Test user prompt")
	}

	mockManager.AssertExpectations(t)
}

func TestBuilder_ResolvePrompt_NotFound(t *testing.T) {
	mockManager := &mockManagerService{}
	builder := NewBuilder(mockManager)

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Tasks: map[string]*config.GenerateTask{},
		},
	}

	mockManager.On("GetConfig").Return(cfg)

	// Execute
	result, err := builder.ResolvePrompt("nonexistent-task")

	// Assert
	if err == nil {
		t.Error("ResolvePrompt() expected error, got nil")
	}
	if result != "" {
		t.Errorf("ResolvePrompt() = %q, want empty string", result)
	}

	mockManager.AssertExpectations(t)
}
