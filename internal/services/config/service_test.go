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
	os.MkdirAll(configDir, 0755)
	
	configPath := filepath.Join(configDir, "config.yaml")
	configContent := `profiles:
  default:
    provider: "openai"
    model: "gpt-4"
generate:
  default:
    profile: "default"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
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

	config := configSvc.GetConfig()
	if config.Profiles["default"].Provider != "openai" {
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
	os.MkdirAll(configDir, 0755)
	
	configPath := filepath.Join(configDir, "config.yaml")
	configContent := `profiles:
  user:
    provider: "anthropic"
    model: "claude-3"
generate:
  default:
    profile: "user"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
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

	config := configSvc.GetConfig()
	if config.Profiles["user"].Provider != "anthropic" {
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
	os.MkdirAll(homeConfigDir, 0755)
	
	configPath := filepath.Join(homeConfigDir, "config.yaml")
	configContent := `profiles:
  home:
    provider: "google"
    model: "gemini-pro"
generate:
  default:
    profile: "home"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Test with HOME fallback (no XDG_CONFIG_HOME set)
	os.Setenv("XDG_CONFIG_DIRS", "/etc/xdg") // Default, no config here
	os.Setenv("XDG_CONFIG_HOME", "")        // Empty, should use HOME fallback
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

	config := configSvc.GetConfig()
	if config.Profiles["home"].Provider != "google" {
		t.Error("Failed to load config from HOME/.config location")
	}
}

func TestNewServiceWithCurrentDirectoryConfig(t *testing.T) {
	// Create temporary current directory with config
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".meowg1k")
	os.MkdirAll(configDir, 0755)
	
	configPath := filepath.Join(configDir, "config.yaml")
	configContent := `profiles:
  local:
    provider: "local"
    model: "llama-local"
generate:
  default:
    profile: "local"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
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

	config := configSvc.GetConfig()
	if config.Profiles["local"].Provider != "local" {
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
	os.MkdirAll(systemConfigDir, 0755)
	systemConfigPath := filepath.Join(systemConfigDir, "config.yaml")
	systemConfigContent := `profiles:
  system:
    provider: "system"
    model: "system-model"
generate:
  default:
    profile: "system"
`
	os.WriteFile(systemConfigPath, []byte(systemConfigContent), 0644)
	
	// Create user config  
	userConfigDir := filepath.Join(tempDir, "user", "meowg1k")
	os.MkdirAll(userConfigDir, 0755)
	userConfigPath := filepath.Join(userConfigDir, "config.yaml")
	userConfigContent := `profiles:
  user:
    provider: "user"
    model: "user-model"
  system:
    # Override system config
    model: "user-override-model"
`
	os.WriteFile(userConfigPath, []byte(userConfigContent), 0644)

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

	config := configSvc.GetConfig()
	
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