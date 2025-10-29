// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/adapters/command"
)

// mockWorkspaceDirResolver is a mock implementation of WorkspaceDirResolver for testing.
type mockWorkspaceDirResolver struct {
	dir string
	err error
}

func (m *mockWorkspaceDirResolver) Get() (string, error) {
	return m.dir, m.err
}

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

	cleanEnvDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cleanEnvDir)
	t.Setenv("HOME", cleanEnvDir)
	t.Setenv("XDG_CONFIG_DIRS", "")

	// Create command with config flag
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().Set("config", configPath)

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	// Test NewService
	configSvc, err := NewService(commandSvc, &mockWorkspaceDirResolver{})
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
	_, err = NewService(commandSvc, &mockWorkspaceDirResolver{})
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
	_, err = NewService(commandSvc, &mockWorkspaceDirResolver{})
	if err == nil {
		t.Error("Expected error when config file is invalid")
	}
}

func TestNewServiceWithoutConfig(t *testing.T) {
	// Create command without config flag set
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")
	// Don't set the config flag

	cleanEnvDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cleanEnvDir)
	t.Setenv("HOME", cleanEnvDir)
	t.Setenv("XDG_CONFIG_DIRS", "")

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	// Test NewService without specific config (should look for standard locations and workspace)
	// Since we don't have config files anywhere, it should fail
	_, err = NewService(commandSvc, &mockWorkspaceDirResolver{})
	if err == nil {
		t.Error("Expected error when no configuration file found")
	}

	// Check that error message is appropriate
	expectedMsg := "no configuration file found"
	if err != nil && !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain %q, got: %v", expectedMsg, err)
	}
}

func TestNewServiceWithConfigHome(t *testing.T) {
	// Test with XDG_CONFIG_HOME

	// Save original environment variables
	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")

	defer func() {
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

	// Test with custom XDG_CONFIG_HOME
	os.Setenv("XDG_CONFIG_HOME", tempDir)
	os.Setenv("HOME", "")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")
	// Don't set specific config path

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	configSvc, err := NewService(commandSvc, &mockWorkspaceDirResolver{})
	if err != nil {
		t.Fatalf("NewService failed with XDG_CONFIG_HOME: %v", err)
	}

	if configSvc == nil {
		t.Fatal("Config service should not be nil")
	}

	config, err := configSvc.Get()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if config.Profiles["default"].Model != "gpt4" || config.Models["gpt4"].Provider != "openai" {
		t.Error("Failed to load config from XDG_CONFIG_HOME location")
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

	configSvc, err := NewService(commandSvc, &mockWorkspaceDirResolver{})
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

	configSvc, err := NewService(commandSvc, &mockWorkspaceDirResolver{})
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

func TestNewServiceWithWorkspaceConfig(t *testing.T) {
	// Create temporary workspace directory with config
	tempDir := t.TempDir()

	// Create workspace config file (.meowg1k.yaml in root)
	configPath := filepath.Join(tempDir, ".meowg1k.yaml")
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

	// Save and reset environment variables to avoid other config locations
	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")

	defer func() {
		os.Setenv("XDG_CONFIG_HOME", originalConfigHome)
		os.Setenv("HOME", originalHome)
	}()

	os.Setenv("XDG_CONFIG_HOME", "/nonexistent")
	os.Setenv("HOME", "/nonexistent")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	// Create mock workspace resolver that returns our temp directory
	workspaceResolver := &mockWorkspaceDirResolver{dir: tempDir}

	configSvc, err := NewService(commandSvc, workspaceResolver)
	if err != nil {
		t.Fatalf("NewService failed with workspace config: %v", err)
	}

	config, err := configSvc.Get()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if config.Profiles["local"].Model != "llama-local" || config.Models["llama-local"].Provider != "local" {
		t.Error("Failed to load config from workspace directory")
	}
}

func TestNewServiceConfigMerging(t *testing.T) {
	// Test configuration merging between user config and workspace config

	// Save original environment variables
	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")

	defer func() {
		os.Setenv("XDG_CONFIG_HOME", originalConfigHome)
		os.Setenv("HOME", originalHome)
	}()

	// Create temporary directories
	tempDir := t.TempDir()

	// Create user config
	userConfigDir := filepath.Join(tempDir, "user", "meowg1k")
	os.MkdirAll(userConfigDir, 0o755)
	userConfigPath := filepath.Join(userConfigDir, "config.yaml")
	userConfigContent := `models:
  base-model:
    provider: "openai"
    model: "gpt-4"
profiles:
  user:
    model: "base-model"
  shared:
    model: "base-model"
generate:
  default:
    profile: "user"
`
	os.WriteFile(userConfigPath, []byte(userConfigContent), 0o644)

	// Create workspace config
	workspaceDir := filepath.Join(tempDir, "workspace")
	os.MkdirAll(workspaceDir, 0o755)
	workspaceConfigPath := filepath.Join(workspaceDir, ".meowg1k.yaml")
	workspaceConfigContent := `models:
  workspace-model:
    provider: "anthropic"
    model: "claude-3"
profiles:
  workspace:
    model: "workspace-model"
  shared:
    # Override user config
    model: "workspace-model"
`
	os.WriteFile(workspaceConfigPath, []byte(workspaceConfigContent), 0o644)

	// Set environment to use user config
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, "user"))
	os.Setenv("HOME", "")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	// Use workspace resolver pointing to workspace dir
	workspaceResolver := &mockWorkspaceDirResolver{dir: workspaceDir}

	configSvc, err := NewService(commandSvc, workspaceResolver)
	if err != nil {
		t.Fatalf("NewService failed with config merging: %v", err)
	}

	config, err := configSvc.Get()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	// Should have profiles from both user and workspace configs
	if _, exists := config.Profiles["user"]; !exists {
		t.Error("User profile should exist")
	}
	if _, exists := config.Profiles["workspace"]; !exists {
		t.Error("Workspace profile should exist")
	}
	if _, exists := config.Profiles["shared"]; !exists {
		t.Error("Shared profile should exist")
	}

	// Workspace config should override user config for shared profile
	if config.Profiles["shared"].Model != "workspace-model" {
		t.Errorf("Expected workspace override, got %s", config.Profiles["shared"].Model)
	}

	// Should have models from both configs
	if _, exists := config.Models["base-model"]; !exists {
		t.Error("Base model from user config should exist")
	}
	if _, exists := config.Models["workspace-model"]; !exists {
		t.Error("Workspace model should exist")
	}
}

func TestNewServiceWithInvalidYAMLInUserConfig(t *testing.T) {
	// Test handling of invalid YAML in user config location

	// Save original environment variables
	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")

	defer func() {
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
	os.Setenv("XDG_CONFIG_HOME", tempDir)
	os.Setenv("HOME", "/nonexistent")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	_, err = NewService(commandSvc, &mockWorkspaceDirResolver{})
	if err == nil {
		t.Error("Expected error when user config has invalid YAML")
	}
}

func TestNewServiceWithInvalidYAMLInWorkspaceConfig(t *testing.T) {
	// Test handling of invalid YAML in workspace config (merge scenario)

	// Save original environment variables
	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")

	defer func() {
		os.Setenv("XDG_CONFIG_HOME", originalConfigHome)
		os.Setenv("HOME", originalHome)
	}()

	// Create temporary directories
	tempDir := t.TempDir()

	// Create valid user config
	userDir := filepath.Join(tempDir, "user", "meowg1k")
	os.MkdirAll(userDir, 0o755)
	userPath := filepath.Join(userDir, "config.yaml")
	userContent := `profiles:
  user:
    provider: "openai"
    model: "gpt-4"
`
	os.WriteFile(userPath, []byte(userContent), 0o644)

	// Create invalid workspace config
	workspaceDir := filepath.Join(tempDir, "workspace")
	os.MkdirAll(workspaceDir, 0o755)
	workspacePath := filepath.Join(workspaceDir, ".meowg1k.yaml")
	invalidContent := `profiles:
  workspace:
    provider: "anthropic"
	bad_indentation: "broken"
`
	os.WriteFile(workspacePath, []byte(invalidContent), 0o644)

	// Set environment to load user config
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, "user"))
	os.Setenv("HOME", "")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	// Use workspace resolver that points to workspace with invalid config
	workspaceResolver := &mockWorkspaceDirResolver{dir: workspaceDir}

	_, err = NewService(commandSvc, workspaceResolver)
	if err == nil {
		t.Error("Expected error when workspace config has invalid YAML")
	}
}

func TestNewServiceWithNilFilePathResolver(t *testing.T) {
	_, err := NewService(nil, &mockWorkspaceDirResolver{})
	if err == nil {
		t.Error("Expected error when file path resolver is nil")
	}
	if err != nil && !strings.Contains(err.Error(), "config path resolver is nil") {
		t.Errorf("Expected error about nil resolver, got: %v", err)
	}
}

func TestNewServiceWithNilWorkspaceDirResolver(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	_, err = NewService(commandSvc, nil)
	if err == nil {
		t.Error("Expected error when workspace dir resolver is nil")
	}
	if err != nil && !strings.Contains(err.Error(), "workspace dir resolver is nil") {
		t.Errorf("Expected error about nil resolver, got: %v", err)
	}
}

func TestServiceGetWithNilService(t *testing.T) {
	var service *Service
	_, err := service.Get()
	if err == nil {
		t.Error("Expected error when calling Get on nil service")
	}
	if err != nil && !strings.Contains(err.Error(), "config service is nil") {
		t.Errorf("Expected error about nil service, got: %v", err)
	}
}

func TestNewServiceWithWorkspaceDirResolverError(t *testing.T) {
	// Save original environment variables
	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")

	defer func() {
		os.Setenv("XDG_CONFIG_HOME", originalConfigHome)
		os.Setenv("HOME", originalHome)
	}()

	// Create a valid user config so we don't fail on missing config
	tempDir := t.TempDir()
	userDir := filepath.Join(tempDir, "meowg1k")
	os.MkdirAll(userDir, 0o755)
	userPath := filepath.Join(userDir, "config.yaml")
	userContent := `models:
  test:
    provider: "test"
profiles:
  test:
    model: "test"
`
	os.WriteFile(userPath, []byte(userContent), 0o644)
	os.Setenv("XDG_CONFIG_HOME", tempDir)
	os.Setenv("HOME", "")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	// Create workspace resolver that returns an error
	workspaceResolver := &mockWorkspaceDirResolver{
		dir: "",
		err: errors.New("workspace resolver error"),
	}

	_, err = NewService(commandSvc, workspaceResolver)
	if err == nil {
		t.Error("Expected error when workspace resolver returns error")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to get workspace directory") {
		t.Errorf("Expected workspace directory error, got: %v", err)
	}
}

func TestNewServiceWithYMLExtension(t *testing.T) {
	// Test that .yml extension is also recognized (not just .yaml)

	// Save original environment variables
	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")

	defer func() {
		os.Setenv("XDG_CONFIG_HOME", originalConfigHome)
		os.Setenv("HOME", originalHome)
	}()

	// Create temporary directory with .yml config file
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "meowg1k")
	os.MkdirAll(configDir, 0o755)

	// Use .yml extension instead of .yaml
	configPath := filepath.Join(configDir, "config.yml")
	configContent := `models:
  yml-test:
    provider: "openai"
    model: "gpt-3.5-turbo"
profiles:
  yml:
    model: "yml-test"
generate:
  default:
    profile: "yml"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	os.Setenv("XDG_CONFIG_HOME", tempDir)
	os.Setenv("HOME", "")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	configSvc, err := NewService(commandSvc, &mockWorkspaceDirResolver{})
	if err != nil {
		t.Fatalf("NewService failed with .yml extension: %v", err)
	}

	config, err := configSvc.Get()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if config.Profiles["yml"].Model != "yml-test" {
		t.Errorf("Failed to load config with .yml extension")
	}
}

func TestNewServiceConfigPrecedence(t *testing.T) {
	// Test that config file specified via flag takes precedence over default locations

	// Save original environment variables
	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")

	defer func() {
		os.Setenv("XDG_CONFIG_HOME", originalConfigHome)
		os.Setenv("HOME", originalHome)
	}()

	tempDir := t.TempDir()

	// Create user config
	userDir := filepath.Join(tempDir, "user", "meowg1k")
	os.MkdirAll(userDir, 0o755)
	userPath := filepath.Join(userDir, "config.yaml")
	userContent := `models:
  user-model:
    provider: "openai"
    model: "gpt-3.5"
profiles:
  default:
    model: "user-model"
    temperature: 0.5
`
	os.WriteFile(userPath, []byte(userContent), 0o644)

	// Create flag-specified config with override
	flagPath := filepath.Join(tempDir, "flag-config.yaml")
	flagContent := `profiles:
  default:
    # Override temperature from user config
    temperature: 0.9
  flag-profile:
    model: "user-model"
    temperature: 0.7
`
	os.WriteFile(flagPath, []byte(flagContent), 0o644)

	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, "user"))
	os.Setenv("HOME", "")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().Set("config", flagPath)

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	configSvc, err := NewService(commandSvc, &mockWorkspaceDirResolver{})
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	config, err := configSvc.Get()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	// Check that flag config overrode user config
	if config.Profiles["default"].Temperature == nil || *config.Profiles["default"].Temperature != 0.9 {
		t.Errorf("Expected temperature 0.9 from flag config, got %v", config.Profiles["default"].Temperature)
	}

	// Check that flag config added new profile
	if _, exists := config.Profiles["flag-profile"]; !exists {
		t.Error("Flag profile should exist")
	}

	// Check that user config model is still there (merged, not replaced)
	if config.Profiles["default"].Model != "user-model" {
		t.Error("Model from user config should still be present")
	}
}

func TestNewServiceEmptyWorkspaceDir(t *testing.T) {
	// Test that empty workspace dir doesn't cause issues

	// Save original environment variables
	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")

	defer func() {
		os.Setenv("XDG_CONFIG_HOME", originalConfigHome)
		os.Setenv("HOME", originalHome)
	}()

	// Create user config
	tempDir := t.TempDir()
	userDir := filepath.Join(tempDir, "meowg1k")
	os.MkdirAll(userDir, 0o755)
	userPath := filepath.Join(userDir, "config.yaml")
	userContent := `models:
  test:
    provider: "test"
profiles:
  test:
    model: "test"
`
	os.WriteFile(userPath, []byte(userContent), 0o644)
	os.Setenv("XDG_CONFIG_HOME", tempDir)
	os.Setenv("HOME", "")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	// Empty workspace dir should not cause error
	workspaceResolver := &mockWorkspaceDirResolver{dir: ""}

	configSvc, err := NewService(commandSvc, workspaceResolver)
	if err != nil {
		t.Fatalf("NewService should not fail with empty workspace dir: %v", err)
	}

	config, err := configSvc.Get()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if config.Profiles["test"].Model != "test" {
		t.Error("Should load user config even with empty workspace dir")
	}
}

func TestNewServiceGetCalledMultipleTimes(t *testing.T) {
	// Test that Get() can be called multiple times and returns the same config

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")
	configContent := `models:
  test:
    provider: "test"
profiles:
  test:
    model: "test"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().Set("config", configPath)

	commandSvc, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	configSvc, err := NewService(commandSvc, &mockWorkspaceDirResolver{})
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Call Get multiple times
	config1, err1 := configSvc.Get()
	config2, err2 := configSvc.Get()
	config3, err3 := configSvc.Get()

	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("Get should not fail: %v, %v, %v", err1, err2, err3)
	}

	// All should return the same config instance
	if config1 != config2 || config2 != config3 {
		t.Error("Get should return the same config instance on multiple calls")
	}

	// Verify config content
	if config1.Profiles["test"].Model != "test" {
		t.Error("Config should have test profile")
	}
}
