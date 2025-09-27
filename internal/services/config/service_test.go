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

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/retran/meowg1k/internal/services/command"
	"github.com/spf13/cobra"
)

func TestNewServiceWithSpecificConfig(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")
	configContent := `profiles:
  test:
    provider: "openai"
    model: "gpt-3.5-turbo"
generate:
  default:
    profile: "test"
    systemPrompt: "You are a helpful assistant"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	// Create command with config flag
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().Set("config", configPath)

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	// Test NewService
	configSvc, err := NewService(commandSvc)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	if configSvc == nil {
		t.Fatal("Config service should not be nil")
	}

	// Test GetConfig
	config := configSvc.GetConfig()
	if config == nil {
		t.Fatal("Config should not be nil")
	}

	if config.Profiles == nil {
		t.Fatal("Profiles should not be nil")
	}

	testProfile, exists := config.Profiles["test"]
	if !exists {
		t.Fatal("Test profile should exist")
	}

	if testProfile.Provider != "openai" {
		t.Errorf("Expected provider 'openai', got '%s'", testProfile.Provider)
	}

	if testProfile.Model != "gpt-3.5-turbo" {
		t.Errorf("Expected model 'gpt-3.5-turbo', got '%s'", testProfile.Model)
	}

	if config.Generate == nil {
		t.Fatal("Generate config should not be nil")
	}

	if config.Generate.Default == nil {
		t.Fatal("Generate default should not be nil")
	}

	if config.Generate.Default.Profile != "test" {
		t.Errorf("Expected default profile 'test', got '%s'", config.Generate.Default.Profile)
	}
}

func TestNewServiceWithNonExistentConfig(t *testing.T) {
	// Create command with config flag pointing to non-existent file
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().Set("config", "/non/existent/config.yaml")

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	// Test NewService with non-existent config
	_, err = NewService(commandSvc)
	if err == nil {
		t.Error("Expected error when config file doesn't exist")
	}
}

func TestNewServiceWithInvalidConfig(t *testing.T) {
	// Create a temporary invalid config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid-config.yaml")
	invalidContent := `invalid: yaml: [unclosed bracket
`
	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	// Create command with config flag
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().Set("config", configPath)

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	// Test NewService with invalid config
	_, err = NewService(commandSvc)
	if err == nil {
		t.Error("Expected error when config file is invalid")
	}
}

func TestNewServiceWithoutConfig(t *testing.T) {
	// Create command without config flag set
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")
	// Don't set the config flag

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	// Test NewService without specific config (should look for standard locations)
	_, err = NewService(commandSvc)
	// This should fail because we don't have config files in standard locations
	if err == nil {
		t.Error("Expected error when no configuration file found")
	}
	
	// Check that error message is appropriate
	if err != nil && err.Error() != "no configuration file found in standard locations" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}