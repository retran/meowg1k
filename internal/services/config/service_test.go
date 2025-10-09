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
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/services/command"
)

func TestNewServiceWithSpecificConfig(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")
	configContent := `models:
  gpt-35-turbo:
    provider: "openai"
    model: "gpt-3.5-turbo"
profiles:
  test:
    model: "gpt-35-turbo"
generate:
  default:
    profile: "test"
    systemPrompt: "You are a helpful assistant"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
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

	// Test Get
	config, err := configSvc.Get()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

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

	if testProfile.Model != "gpt-35-turbo" {
		t.Errorf("Expected model reference 'gpt-35-turbo', got '%s'", testProfile.Model)
	}

	// Check model definition
	testModel, exists := config.Models["gpt-35-turbo"]
	if !exists {
		t.Fatal("Model 'gpt-35-turbo' should exist")
	}

	if testModel.Provider != "openai" {
		t.Errorf("Expected provider 'openai', got '%s'", testModel.Provider)
	}

	if testModel.Model != "gpt-3.5-turbo" {
		t.Errorf("Expected model 'gpt-3.5-turbo', got '%s'", testModel.Model)
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
	err := os.WriteFile(configPath, []byte(invalidContent), 0o644)
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
	expectedMsg := "no configuration file found in standard locations"
	if err != nil && !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain %q, got: %v", expectedMsg, err)
	}
}

func TestNewServiceWithSystemConfigDirs(t *testing.T) {
	// Test with various XDG_CONFIG_DIRS and XDG_CONFIG_HOME scenarios

	// Save original environment variables
	originalConfigDirs := os.Getenv("XDG_CONFIG_DIRS")
	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")

	defer func() {
		os.Setenv("XDG_CONFIG_DIRS", originalConfigDirs)
		os.Setenv("XDG_CONFIG_HOME", originalConfigHome)
		os.Setenv("HOME", originalHome)
	}()

	// Create temporary directories and config file
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "meowg1k")
	os.MkdirAll(configDir, 0o755)

	configPath := filepath.Join(configDir, "config.yaml")
	configContent := `models:
  gpt4:
    provider: "openai"
    model: "gpt-4"
profiles:
  default:
    model: "gpt4"
generate:
  default:
    profile: "default"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Test with custom XDG_CONFIG_DIRS
	os.Setenv("XDG_CONFIG_DIRS", tempDir)
	os.Setenv("XDG_CONFIG_HOME", "")
	os.Setenv("HOME", "")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")
	// Don't set specific config path

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	configSvc, err := NewService(commandSvc)
	if err != nil {
		t.Fatalf("NewService failed with XDG_CONFIG_DIRS: %v", err)
	}

	if configSvc == nil {
		t.Fatal("Config service should not be nil")
	}

	config, err := configSvc.Get()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if config.Profiles["default"].Model != "gpt4" || config.Models["gpt4"].Provider != "openai" {
		t.Error("Failed to load config from XDG_CONFIG_DIRS location")
	}
}

func TestNewServiceWithUserConfigHome(t *testing.T) {
	// Save original environment variables
	originalConfigDirs := os.Getenv("XDG_CONFIG_DIRS")
	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")

	defer func() {
		os.Setenv("XDG_CONFIG_DIRS", originalConfigDirs)
		os.Setenv("XDG_CONFIG_HOME", originalConfigHome)
		os.Setenv("HOME", originalHome)
	}()

	// Create temporary directories and config file
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "meowg1k")
	os.MkdirAll(configDir, 0o755)

	configPath := filepath.Join(configDir, "config.yaml")
	configContent := `models:
  claude3:
    provider: "anthropic"
    model: "claude-3"
profiles:
  user:
    model: "claude3"
generate:
  default:
    profile: "user"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Test with custom XDG_CONFIG_HOME
	os.Setenv("XDG_CONFIG_DIRS", "/etc/xdg") // Default, no config here
	os.Setenv("XDG_CONFIG_HOME", tempDir)
	os.Setenv("HOME", "")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	configSvc, err := NewService(commandSvc)
	if err != nil {
		t.Fatalf("NewService failed with XDG_CONFIG_HOME: %v", err)
	}

	config, err := configSvc.Get()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if config.Profiles["user"].Model != "claude3" || config.Models["claude3"].Provider != "anthropic" {
		t.Error("Failed to load config from XDG_CONFIG_HOME location")
	}
}

func TestNewServiceWithHomeConfigFallback(t *testing.T) {
	// Save original environment variables
	originalConfigDirs := os.Getenv("XDG_CONFIG_DIRS")
	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")

	defer func() {
		os.Setenv("XDG_CONFIG_DIRS", originalConfigDirs)
		os.Setenv("XDG_CONFIG_HOME", originalConfigHome)
		os.Setenv("HOME", originalHome)
	}()

	// Create temporary directories and config file
	tempDir := t.TempDir()
	homeConfigDir := filepath.Join(tempDir, ".config", "meowg1k")
	os.MkdirAll(homeConfigDir, 0o755)

	configPath := filepath.Join(homeConfigDir, "config.yaml")
	configContent := `models:
  gemini:
    provider: "google"
    model: "gemini-pro"
profiles:
  home:
    model: "gemini"
generate:
  default:
    profile: "home"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Test with HOME fallback (no XDG_CONFIG_HOME set)
	os.Setenv("XDG_CONFIG_DIRS", "/etc/xdg") // Default, no config here
	os.Setenv("XDG_CONFIG_HOME", "")         // Empty, should use HOME fallback
	os.Setenv("HOME", tempDir)

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	configSvc, err := NewService(commandSvc)
	if err != nil {
		t.Fatalf("NewService failed with HOME fallback: %v", err)
	}

	config, err := configSvc.Get()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if config.Profiles["home"].Model != "gemini" || config.Models["gemini"].Provider != "google" {
		t.Error("Failed to load config from HOME/.config location")
	}
}

func TestNewServiceWithCurrentDirectoryConfig(t *testing.T) {
	// Create temporary current directory with config
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".meowg1k")
	os.MkdirAll(configDir, 0o755)

	configPath := filepath.Join(configDir, "config.yaml")
	configContent := `models:
  llama-local:
    provider: "local"
    model: "llama-local"
profiles:
  local:
    model: "llama-local"
generate:
  default:
    profile: "local"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Change to temp directory
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	// Save and reset environment variables to avoid other config locations
	originalConfigDirs := os.Getenv("XDG_CONFIG_DIRS")
	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")

	defer func() {
		os.Setenv("XDG_CONFIG_DIRS", originalConfigDirs)
		os.Setenv("XDG_CONFIG_HOME", originalConfigHome)
		os.Setenv("HOME", originalHome)
	}()

	os.Setenv("XDG_CONFIG_DIRS", "/nonexistent")
	os.Setenv("XDG_CONFIG_HOME", "/nonexistent")
	os.Setenv("HOME", "/nonexistent")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	configSvc, err := NewService(commandSvc)
	if err != nil {
		t.Fatalf("NewService failed with current directory config: %v", err)
	}

	config, err := configSvc.Get()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if config.Profiles["local"].Model != "llama-local" || config.Models["llama-local"].Provider != "local" {
		t.Error("Failed to load config from current directory")
	}
}

func TestNewServiceConfigMerging(t *testing.T) {
	// Test configuration merging when multiple config files exist

	// Save original environment variables
	originalConfigDirs := os.Getenv("XDG_CONFIG_DIRS")
	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")

	defer func() {
		os.Setenv("XDG_CONFIG_DIRS", originalConfigDirs)
		os.Setenv("XDG_CONFIG_HOME", originalConfigHome)
		os.Setenv("HOME", originalHome)
	}()

	// Create temporary directories
	tempDir := t.TempDir()

	// Create system config
	systemConfigDir := filepath.Join(tempDir, "system", "meowg1k")
	os.MkdirAll(systemConfigDir, 0o755)
	systemConfigPath := filepath.Join(systemConfigDir, "config.yaml")
	systemConfigContent := `profiles:
  system:
    provider: "system"
    model: "system-model"
generate:
  default:
    profile: "system"
`
	os.WriteFile(systemConfigPath, []byte(systemConfigContent), 0o644)

	// Create user config
	userConfigDir := filepath.Join(tempDir, "user", "meowg1k")
	os.MkdirAll(userConfigDir, 0o755)
	userConfigPath := filepath.Join(userConfigDir, "config.yaml")
	userConfigContent := `profiles:
  user:
    provider: "user"
    model: "user-model"
  system:
    # Override system config
    model: "user-override-model"
`
	os.WriteFile(userConfigPath, []byte(userConfigContent), 0o644)

	// Set environment to use both configs
	os.Setenv("XDG_CONFIG_DIRS", filepath.Join(tempDir, "system"))
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, "user"))
	os.Setenv("HOME", "")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	configSvc, err := NewService(commandSvc)
	if err != nil {
		t.Fatalf("NewService failed with config merging: %v", err)
	}

	config, err := configSvc.Get()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	// Should have both system and user profiles
	if _, exists := config.Profiles["system"]; !exists {
		t.Error("System profile should exist")
	}
	if _, exists := config.Profiles["user"]; !exists {
		t.Error("User profile should exist")
	}

	// User config should override system config
	if config.Profiles["system"].Model != "user-override-model" {
		t.Errorf("Expected user override, got %s", config.Profiles["system"].Model)
	}
}

func TestNewServiceWithInvalidYAMLInPrimaryConfig(t *testing.T) {
	// Test handling of invalid YAML in primary config location

	// Save original environment variables
	originalConfigDirs := os.Getenv("XDG_CONFIG_DIRS")
	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")

	defer func() {
		os.Setenv("XDG_CONFIG_DIRS", originalConfigDirs)
		os.Setenv("XDG_CONFIG_HOME", originalConfigHome)
		os.Setenv("HOME", originalHome)
	}()

	// Create temporary directory and invalid config file
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "meowg1k")
	os.MkdirAll(configDir, 0o755)

	configPath := filepath.Join(configDir, "config.yaml")
	invalidContent := `profiles:
  test:
    provider: "openai"
	invalid_indent: "broken yaml"
`
	os.WriteFile(configPath, []byte(invalidContent), 0o644)

	// Set environment to use this config
	os.Setenv("XDG_CONFIG_DIRS", tempDir)
	os.Setenv("XDG_CONFIG_HOME", "/nonexistent")
	os.Setenv("HOME", "/nonexistent")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	_, err = NewService(commandSvc)
	if err == nil {
		t.Error("Expected error when primary config has invalid YAML")
	}
}

func TestNewServiceWithInvalidYAMLInSecondaryConfig(t *testing.T) {
	// Test handling of invalid YAML in secondary config (merge scenario)

	// Save original environment variables
	originalConfigDirs := os.Getenv("XDG_CONFIG_DIRS")
	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")

	defer func() {
		os.Setenv("XDG_CONFIG_DIRS", originalConfigDirs)
		os.Setenv("XDG_CONFIG_HOME", originalConfigHome)
		os.Setenv("HOME", originalHome)
	}()

	// Create temporary directories
	tempDir := t.TempDir()

	// Create valid primary config
	primaryDir := filepath.Join(tempDir, "primary", "meowg1k")
	os.MkdirAll(primaryDir, 0o755)
	primaryPath := filepath.Join(primaryDir, "config.yaml")
	primaryContent := `profiles:
  primary:
    provider: "openai"
    model: "gpt-4"
`
	os.WriteFile(primaryPath, []byte(primaryContent), 0o644)

	// Create invalid secondary config
	secondaryDir := filepath.Join(tempDir, "secondary", "meowg1k")
	os.MkdirAll(secondaryDir, 0o755)
	secondaryPath := filepath.Join(secondaryDir, "config.yaml")
	invalidContent := `profiles:
  secondary:
    provider: "anthropic"
	bad_indentation: "broken"
`
	os.WriteFile(secondaryPath, []byte(invalidContent), 0o644)

	// Set environment to load primary first, then try to merge secondary
	os.Setenv("XDG_CONFIG_DIRS", filepath.Join(tempDir, "primary"))
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, "secondary"))
	os.Setenv("HOME", "")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	_, err = NewService(commandSvc)
	if err == nil {
		t.Error("Expected error when secondary config has invalid YAML")
	}
}
