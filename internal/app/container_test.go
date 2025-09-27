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
	"runtime"
	"testing"

	"github.com/spf13/cobra"
)

func TestGetLogDir(t *testing.T) {
	// Test getLogDir function
	dir, err := getLogDir()
	if err != nil {
		t.Errorf("getLogDir returned error: %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get user home dir")
	}

	switch runtime.GOOS {
	case "darwin":
		expected := filepath.Join(home, "Library", "Logs", "meow")
		if dir != expected {
			t.Errorf("expected %s, got %s", expected, dir)
		}
	case "windows":
		expected := filepath.Join(home, "AppData", "Local", "meow", "logs")
		if dir != expected {
			t.Errorf("expected %s, got %s", expected, dir)
		}
	default:
		expected := filepath.Join(home, ".cache", "meow", "logs")
		if dir != expected {
			t.Errorf("expected %s, got %s", expected, dir)
		}
	}
}

func TestNewAppContainer(t *testing.T) {
	// Create a temporary config file for testing
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
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	// Create a test cobra command with required flags
	cmd := &cobra.Command{
		Use: "test",
	}
	// Add the flags that the command service expects
	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().String("task", "", "task name")
	cmd.Flags().String("user-prompt", "", "user prompt")
	cmd.Flags().Bool("silent", false, "silent mode")
	
	// Set the config flag to point to our temp config
	cmd.Flags().Set("config", configPath)

	// Call NewAppContainer
	container, err := NewAppContainer(cmd)
	if err != nil {
		t.Errorf("NewAppContainer returned error: %v", err)
	}

	if container == nil {
		t.Error("container is nil")
	}

	if container.Logger == nil {
		t.Error("Logger is nil")
	}

	if container.ShutdownService == nil {
		t.Error("ShutdownService is nil")
	}

	if container.CommandService == nil {
		t.Error("CommandService is nil")
	}

	if container.ConfigService == nil {
		t.Error("ConfigService is nil")
	}

	if container.Context == nil {
		t.Error("Context is nil")
	}

	// Check that AppContainerKey is set in context
	val := container.Context.Value(AppContainerKey)
	if val != container {
		t.Error("AppContainerKey not set correctly in context")
	}
}
