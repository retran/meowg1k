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

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestCreateCommitFlow(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `profiles:
  test:
    provider: "openai"
    model: "gpt-3.5-turbo"
    maxInputTokens: 1000
    maxOutputTokens: 500
generate:
  default:
    profile: "test"
    systemPrompt: "You are a helpful assistant"
filter:
  ignore:
    - "*.tmp"
    - ".git/**"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	cmd := &cobra.Command{
		Use: "test",
	}

	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().String("task", "", "task name")
	cmd.Flags().String("user-prompt", "", "user prompt")
	cmd.Flags().Bool("silent", false, "silent mode")

	cmd.Flags().Set("config", configPath)

	container, err := NewAppContainer(cmd)
	if err != nil {
		t.Fatalf("NewAppContainer returned error: %v", err)
	}

	flow := container.CreateCommitFlow()
	if flow == nil {
		t.Error("CreateCommitFlow returned nil")
	}

	// Note: We don't execute the flow here as it requires a full git environment
	// and proper executor context. The fact that it was created without panic is sufficient.
}

func TestCreateGenerateFlow(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `profiles:
  test:
    provider: "openai"
    model: "gpt-3.5-turbo"
    maxInputTokens: 1000
    maxOutputTokens: 500
generate:
  default:
    profile: "test"
    systemPrompt: "You are a helpful assistant"
  tasks:
    test:
      profile: "test"
      systemPrompt: "Test system prompt"
      userPrompt: "Test user prompt"
filter:
  ignore:
    - "*.tmp"
    - ".git/**"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	cmd := &cobra.Command{
		Use: "test",
	}

	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().String("task", "test", "task name")
	cmd.Flags().String("user-prompt", "", "user prompt")
	cmd.Flags().Bool("silent", false, "silent mode")

	cmd.Flags().Set("config", configPath)

	container, err := NewAppContainer(cmd)
	if err != nil {
		t.Fatalf("NewAppContainer returned error: %v", err)
	}

	flow, err := container.CreateGenerateFlow()
	if err != nil {
		t.Errorf("CreateGenerateFlow returned error: %v", err)
	}
	if flow == nil {
		t.Error("CreateGenerateFlow returned nil")
	}
}

func TestCreateGenerateFlowWithUserPrompt(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `profiles:
  test:
    provider: "openai"
    model: "gpt-3.5-turbo"
    maxInputTokens: 1000
    maxOutputTokens: 500
generate:
  default:
    profile: "test"
    systemPrompt: "You are a helpful assistant"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	cmd := &cobra.Command{
		Use: "test",
	}

	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().String("task", "", "task name")
	cmd.Flags().String("user-prompt", "Custom prompt", "user prompt")
	cmd.Flags().Bool("silent", false, "silent mode")

	cmd.Flags().Set("config", configPath)
	cmd.Flags().Set("user-prompt", "Custom prompt")

	container, err := NewAppContainer(cmd)
	if err != nil {
		t.Fatalf("NewAppContainer returned error: %v", err)
	}

	flow, err := container.CreateGenerateFlow()
	if err != nil {
		t.Errorf("CreateGenerateFlow returned error: %v", err)
	}
	if flow == nil {
		t.Error("CreateGenerateFlow returned nil")
	}
}
