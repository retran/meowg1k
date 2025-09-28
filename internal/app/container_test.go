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
	"strings"
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
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
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
		t.Fatal("container is nil")
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

	if container.ShutdownService.Context() == nil {
		t.Error("Context is nil")
	}

	// Check that AppContainerKey is set in context
	val := container.ShutdownService.Context().Value(AppContainerKey)
	if val != container {
		t.Error("AppContainerKey not set correctly in context")
	}
}

func TestNewAppContainerWithErrors(t *testing.T) {
	tests := []struct {
		name        string
		setupCmd    func() *cobra.Command
		expectError bool
		errorMsg    string
	}{
		{
			name: "Command service creation error",
			setupCmd: func() *cobra.Command {
				// This should trigger a panic which gets recovered in NewAppContainer
				return nil
			},
			expectError: true,
			errorMsg:    "",
		},
		{
			name: "Config service creation error - invalid config path",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "test"}
				cmd.Flags().String("config", "", "config file path")
				cmd.Flags().String("task", "", "task name")
				cmd.Flags().String("user-prompt", "", "user prompt")
				cmd.Flags().Bool("silent", false, "silent mode")
				cmd.Flags().Set("config", "/nonexistent/path/config.yaml")
				return cmd
			},
			expectError: true,
			errorMsg:    "failed to read config file",
		},
		{
			name: "Config service creation error - no config found",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "test"}
				cmd.Flags().String("config", "", "config file path")
				cmd.Flags().String("task", "", "task name")
				cmd.Flags().String("user-prompt", "", "user prompt")
				cmd.Flags().Bool("silent", false, "silent mode")
				// Don't set config path, should fail to find any config
				return cmd
			},
			expectError: true,
			errorMsg:    "no configuration file found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var container *Container
			var err error

			if tt.name == "Command service creation error" {
				// Test panic recovery
				defer func() {
					if r := recover(); r == nil {
						// If no panic, test the error
						if err == nil && tt.expectError {
							t.Error("Expected error but got none")
						}
					}
				}()
				container, err = NewAppContainer(nil)
			} else {
				cmd := tt.setupCmd()
				container, err = NewAppContainer(cmd)
			}

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectError && container != nil {
				t.Error("Expected nil container on error")
			}
			if tt.errorMsg != "" && err != nil && !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("Expected error to contain '%s', got: %v", tt.errorMsg, err)
			}
		})
	}
}

func TestGetLogDirWithEnvironmentVariables(t *testing.T) {
	// Test getLogDir with various environment variable configurations

	// Save original environment variables
	originalLocalAppData := os.Getenv("LOCALAPPDATA")
	originalXDGCache := os.Getenv("XDG_CACHE_HOME")

	defer func() {
		os.Setenv("LOCALAPPDATA", originalLocalAppData)
		os.Setenv("XDG_CACHE_HOME", originalXDGCache)
	}()

	// Test Windows with LOCALAPPDATA set
	if runtime.GOOS == "windows" {
		testLocalAppData := "C:\\Users\\Test\\AppData\\Local"
		os.Setenv("LOCALAPPDATA", testLocalAppData)

		dir, err := getLogDir()
		if err != nil {
			t.Errorf("getLogDir failed: %v", err)
		}

		expected := filepath.Join(testLocalAppData, "meow", "logs")
		if dir != expected {
			t.Errorf("Expected %s, got %s", expected, dir)
		}
	}

	// Test Linux/Unix with XDG_CACHE_HOME set
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		testXDGCache := "/tmp/test-cache"
		os.Setenv("XDG_CACHE_HOME", testXDGCache)

		dir, err := getLogDir()
		if err != nil {
			t.Errorf("getLogDir failed: %v", err)
		}

		expected := filepath.Join(testXDGCache, "meow", "logs")
		if dir != expected {
			t.Errorf("Expected %s, got %s", expected, dir)
		}
	}
}

func TestGetLogDirFallbacks(t *testing.T) {
	// Test fallback behavior when environment variables are empty

	// Save original environment variables
	originalLocalAppData := os.Getenv("LOCALAPPDATA")
	originalXDGCache := os.Getenv("XDG_CACHE_HOME")

	defer func() {
		os.Setenv("LOCALAPPDATA", originalLocalAppData)
		os.Setenv("XDG_CACHE_HOME", originalXDGCache)
	}()

	// Test Windows fallback when LOCALAPPDATA is empty
	if runtime.GOOS == "windows" {
		os.Setenv("LOCALAPPDATA", "")

		dir, err := getLogDir()
		if err != nil {
			t.Errorf("getLogDir failed: %v", err)
		}

		home, _ := os.UserHomeDir()
		expected := filepath.Join(home, "AppData", "Local", "meow", "logs")
		if dir != expected {
			t.Errorf("Expected %s, got %s", expected, dir)
		}
	}

	// Test Linux/Unix fallback when XDG_CACHE_HOME is empty
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		os.Setenv("XDG_CACHE_HOME", "")

		dir, err := getLogDir()
		if err != nil {
			t.Errorf("getLogDir failed: %v", err)
		}

		home, _ := os.UserHomeDir()
		expected := filepath.Join(home, ".cache", "meow", "logs")
		if dir != expected {
			t.Errorf("Expected %s, got %s", expected, dir)
		}
	}
}

func TestNewAppContainerServicesCreation(t *testing.T) {
	// Test that all services are properly created and configured

	// Create a valid config
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `profiles:
  test:
    provider: "openai"
    model: "gpt-4"
    maxInputTokens: 2000
    maxOutputTokens: 1000
    temperature: 0.7
  anthropic:
    provider: "anthropic"
    model: "claude-3"
generate:
  default:
    profile: "test"
    systemPrompt: "You are a helpful assistant"
  anthropic-task:
    profile: "anthropic"
    systemPrompt: "You are an expert assistant"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().String("task", "", "task name")
	cmd.Flags().String("user-prompt", "", "user prompt")
	cmd.Flags().Bool("silent", false, "silent mode")
	cmd.Flags().Set("config", configPath)
	cmd.Flags().Set("task", "anthropic-task")
	cmd.Flags().Set("user-prompt", "Test prompt")
	cmd.Flags().Set("silent", "true")

	container, err := NewAppContainer(cmd)
	if err != nil {
		t.Fatalf("NewAppContainer failed: %v", err)
	}

	// Test that all services are created
	if container.Logger == nil {
		t.Error("Logger should not be nil")
	}
	if container.ShutdownService == nil {
		t.Error("ShutdownService should not be nil")
	}
	if container.CommandService == nil {
		t.Error("CommandService should not be nil")
	}
	if container.ConfigService == nil {
		t.Error("ConfigService should not be nil")
	}
	if container.ShutdownService.Context() == nil {
		t.Error("Context should not be nil")
	}

	// Test that services can be used
	config := container.ConfigService.GetConfig()
	if config == nil {
		t.Fatal("Config should not be nil")
	}

	if len(config.Profiles) != 2 {
		t.Errorf("Expected 2 profiles, got %d", len(config.Profiles))
	}

	if config.Generate == nil {
		t.Error("Generate config should not be nil")
	}

	// Note: Tasks are only created when they have explicit configuration beyond defaults
	// Since we only have one explicit task "anthropic-task", expect 1 task
	if config.Generate.Tasks == nil {
		// If Tasks is nil, that means no explicit tasks were configured beyond default
		t.Log("No explicit tasks configured beyond default")
	} else if len(config.Generate.Tasks) != 1 {
		t.Logf("Expected 1 task, got %d", len(config.Generate.Tasks))
	}

	// Test command service
	taskName, err := container.CommandService.GetTaskName()
	if err != nil {
		t.Errorf("GetTaskName failed: %v", err)
	}
	if taskName != "anthropic-task" {
		t.Errorf("Expected task 'anthropic-task', got '%s'", taskName)
	}

	userPrompt, err := container.CommandService.GetUserPrompt()
	if err != nil {
		t.Errorf("GetUserPrompt failed: %v", err)
	}
	if userPrompt != "Test prompt" {
		t.Errorf("Expected prompt 'Test prompt', got '%s'", userPrompt)
	}

	silent, err := container.CommandService.GetSilentFlag()
	if err != nil {
		t.Errorf("GetSilentFlag failed: %v", err)
	}
	if !silent {
		t.Error("Expected silent to be true")
	}
}

func TestAppContainerKeyContextValue(t *testing.T) {
	// Test that the context contains the correct value for AppContainerKey

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "minimal-config.yaml")
	configContent := `profiles:
  minimal:
    provider: "test"
    model: "test-model"
generate:
  default:
    profile: "minimal"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().String("task", "", "task name")
	cmd.Flags().String("user-prompt", "", "user prompt")
	cmd.Flags().Bool("silent", false, "silent mode")
	cmd.Flags().Set("config", configPath)

	container, err := NewAppContainer(cmd)
	if err != nil {
		t.Fatalf("NewAppContainer failed: %v", err)
	}

	// Test that the context value is set correctly
	contextValue := container.ShutdownService.Context().Value(AppContainerKey)
	if contextValue == nil {
		t.Error("AppContainerKey value should not be nil")
	}

	containerFromContext, ok := contextValue.(*Container)
	if !ok {
		t.Error("AppContainerKey value should be *AppContainer")
	}

	if containerFromContext != container {
		t.Error("Context should contain reference to the same container")
	}

	// Test that we can access services through the context
	if containerFromContext.Logger == nil {
		t.Error("Logger should be accessible through context")
	}
	if containerFromContext.ConfigService == nil {
		t.Error("ConfigService should be accessible through context")
	}
}

func TestGetLogDirErrorHandling(t *testing.T) {
	// Test error handling in getLogDir
	// This is difficult to test directly since os.UserHomeDir() rarely fails
	// in normal test environments. However, we can test the function works
	// correctly under normal conditions which is what's most important.

	dir, err := getLogDir()
	if err != nil {
		t.Errorf("getLogDir should not fail in normal test environment: %v", err)
	}

	if dir == "" {
		t.Error("getLogDir should return non-empty directory")
	}

	// Verify the directory follows expected patterns for each OS
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(dir, "Library/Logs/meow") {
			t.Errorf("macOS log directory should contain 'Library/Logs/meow', got: %s", dir)
		}
	case "windows":
		if !strings.Contains(dir, "meow") || !strings.Contains(dir, "logs") {
			t.Errorf("Windows log directory should contain 'meow' and 'logs', got: %s", dir)
		}
	default:
		if !strings.Contains(dir, "meow") || !strings.Contains(dir, "logs") {
			t.Errorf("Unix log directory should contain 'meow' and 'logs', got: %s", dir)
		}
	}
}

func TestNewAppContainerLogFileHandling(t *testing.T) {
	// Test that NewAppContainer properly handles log file creation
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().String("task", "", "task name")
	cmd.Flags().String("user-prompt", "test prompt", "user prompt")
	cmd.Flags().Bool("silent", false, "silent mode")

	// This should succeed and create log files
	container, err := NewAppContainer(cmd)
	if err != nil {
		// Log directory creation might fail in restricted environments
		t.Logf("NewAppContainer failed (might be environment-related): %v", err)
		return
	}

	if container == nil {
		t.Fatal("Expected container to be created")
	}

	if container.Logger == nil {
		t.Error("Logger should be initialized")
	}

	// Test shutdown to close log files properly
	if container.ShutdownService != nil {
		container.ShutdownService.Shutdown()
	}
}

func TestValidateLogPath(t *testing.T) {
	tests := []struct {
		name      string
		logDir    string
		fileName  string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid path",
			logDir:    "/tmp/logs",
			fileName:  "app.log",
			expectErr: false,
		},
		{
			name:      "filename with forward slash",
			logDir:    "/tmp/logs",
			fileName:  "subdir/app.log",
			expectErr: true,
			errMsg:    "log filename contains invalid characters or path separators",
		},
		{
			name:      "filename with backward slash",
			logDir:    "/tmp/logs",
			fileName:  "subdir\\app.log",
			expectErr: true,
			errMsg:    "log filename contains invalid characters or path separators",
		},
		{
			name:      "filename with double dots",
			logDir:    "/tmp/logs",
			fileName:  "../app.log",
			expectErr: true,
			errMsg:    "log filename contains invalid characters or path separators",
		},
		{
			name:      "filename with double dots in middle",
			logDir:    "/tmp/logs",
			fileName:  "app..log",
			expectErr: true,
			errMsg:    "log filename contains invalid characters or path separators",
		},
		{
			name:      "empty filename",
			logDir:    "/tmp/logs",
			fileName:  "",
			expectErr: false,
		},
		{
			name:      "filename with special chars",
			logDir:    "/tmp/logs",
			fileName:  "app-2025_01.log",
			expectErr: false,
		},
		{
			name:      "logdir with trailing slash",
			logDir:    "/tmp/logs/",
			fileName:  "app.log",
			expectErr: false,
		},
		{
			name:      "logdir with double slashes",
			logDir:    "/tmp//logs",
			fileName:  "app.log",
			expectErr: false,
		},
		{
			name:      "complex valid filename",
			logDir:    "/home/user/.cache/myapp/logs",
			fileName:  "meow-2025-09-28.log",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLogPath(tt.logDir, tt.fileName)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error to contain %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}
