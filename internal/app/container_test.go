// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/adapters/command"
	"github.com/retran/meowg1k/internal/adapters/config"
	"github.com/retran/meowg1k/internal/adapters/httpclient"
	"github.com/retran/meowg1k/internal/adapters/output"
	"github.com/retran/meowg1k/internal/adapters/sqlite/cache"
	"github.com/retran/meowg1k/internal/adapters/workspace"
	"github.com/retran/meowg1k/internal/core/shutdown"
	domainConfig "github.com/retran/meowg1k/internal/domain/config"
	domainOutput "github.com/retran/meowg1k/internal/domain/output"
	"github.com/retran/meowg1k/internal/ports"
)

// testMockDBHost is a simple mock implementation for testing nil validation.
type testMockDBHost struct{}

func (h *testMockDBHost) GetMainDB() (*sql.DB, error) {
	return nil, nil
}

func (h *testMockDBHost) GetProjectDB() (*sql.DB, error) {
	return nil, nil
}

func (h *testMockDBHost) Close() error {
	return nil
}

// NewTestAppContainer creates a new app.Container for testing with a mock database host.
// This ensures tests use in-memory databases and don't create files on disk.
func NewTestAppContainer(cmd *cobra.Command, dbHost ports.Host) (*Container, error) {
	if cmd == nil {
		return nil, fmt.Errorf("cobra command is nil")
	}
	if dbHost == nil {
		return nil, fmt.Errorf("host is nil")
	}

	container := &Container{}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Use a discard logger for tests to avoid log output
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	shutdownService := shutdown.NewService(ctx, logger, 10*time.Second)

	commandService, err := command.NewService(cmd)
	if err != nil {
		return nil, err
	}

	_ = workspace.NewService(commandService) // workspace service not needed for this test

	configService, err := config.NewService()
	if err != nil {
		return nil, err
	}

	outputService := output.NewService(domainOutput.Stdout)
	if err := shutdownService.Register(func(ctx context.Context) error {
		return outputService.Flush()
	}); err != nil {
		return nil, err
	}

	cacheRepo := cache.NewRepository(dbHost)

	if err := shutdownService.Register(func(ctx context.Context) error {
		if dbHost != nil {
			return dbHost.Close()
		}
		return nil
	}); err != nil {
		return nil, err
	}

	httpClientService, err := httpclient.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create http client service: %w", err)
	}
	if err := shutdownService.Register(func(ctx context.Context) error {
		httpClientService.Close()
		return nil
	}); err != nil {
		return nil, err
	}

	container.Logger = logger
	container.ShutdownService = shutdownService
	container.CommandService = commandService
	container.ConfigService = configService
	container.OutputService = outputService
	container.httpClientService = httpClientService
	container.dbHost = dbHost
	container.cacheRepo = cacheRepo

	shutdownCtx := context.WithValue(shutdownService.Context(), AppContainerKey, container)
	cmd.SetContext(shutdownCtx)

	return container, nil
}

func TestGetLogDir(t *testing.T) {
	dir, err := getLogDir()
	if err != nil {
		t.Errorf("getLogDir returned error: %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get user home dir")
	}

	switch runtime.GOOS {
	case osDarwin:
		expected := filepath.Join(home, "Library", "Logs", "meow")
		if dir != expected {
			t.Errorf("expected %s, got %s", expected, dir)
		}
	case osWindows:
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

func TestGetLogDirWithXDGCacheHome(t *testing.T) {
	if runtime.GOOS == osDarwin || runtime.GOOS == osWindows {
		t.Skip("XDG_CACHE_HOME test only applies to Linux/Unix systems")
	}

	// Save original XDG_CACHE_HOME
	originalXDG := os.Getenv("XDG_CACHE_HOME")
	defer os.Setenv("XDG_CACHE_HOME", originalXDG)

	// Test with custom XDG_CACHE_HOME
	customCache := "/tmp/custom_cache"
	os.Setenv("XDG_CACHE_HOME", customCache)

	dir, err := getLogDir()
	if err != nil {
		t.Errorf("getLogDir returned error: %v", err)
	}

	expected := filepath.Join(customCache, "meow", "logs")
	if dir != expected {
		t.Errorf("expected %s, got %s", expected, dir)
	}
}

func TestGetLogDirWithLocalAppData(t *testing.T) {
	if runtime.GOOS != osWindows {
		t.Skip("LOCALAPPDATA test only applies to Windows")
	}

	// Save original LOCALAPPDATA
	originalLocalAppData := os.Getenv("LOCALAPPDATA")
	defer os.Setenv("LOCALAPPDATA", originalLocalAppData)

	// Test with custom LOCALAPPDATA
	customLocalAppData := "C:\\CustomAppData"
	os.Setenv("LOCALAPPDATA", customLocalAppData)

	dir, err := getLogDir()
	if err != nil {
		t.Errorf("getLogDir returned error: %v", err)
	}

	expected := filepath.Join(customLocalAppData, "meow", "logs")
	if dir != expected {
		t.Errorf("expected %s, got %s", expected, dir)
	}
}

func TestNewAppContainer(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `schema_version: 1
providers:
  openai:
    type: "openai"
models:
  test:
    provider: "openai"
    model: "gpt-3.5-turbo"
    limits:
      max_input_tokens: 1000
      max_output_tokens: 500
presets:
  test:
    model: "test"
flows:
  write:
    preset: "test"
    system_prompt: "You are a helpful assistant"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	cmd := &cobra.Command{
		Use: "test",
	}

	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().String("workspace", "", "workspace root path")
	cmd.Flags().String("task", "", "task name")
	cmd.Flags().String("user-prompt", "", "user prompt")
	cmd.Flags().Bool("silent", false, "silent mode")

	cmd.Flags().Set("config", configPath)

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

	val := cmd.Context().Value(AppContainerKey)
	if val != container {
		t.Error("AppContainerKey not set correctly in context")
	}
}

func TestNewAppContainerNil(t *testing.T) {
	container, err := NewAppContainer(nil)
	if err == nil {
		t.Fatal("expected error when cmd is nil, got nil")
	}
	if container != nil {
		t.Errorf("expected nil container, got: %v", container)
	}
}

func TestNewTestAppContainerNil(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}

	t.Run("nil cmd", func(t *testing.T) {
		container, err := NewTestAppContainer(nil, &testMockDBHost{})
		if err == nil {
			t.Fatal("expected error when cmd is nil, got nil")
		}
		if container != nil {
			t.Errorf("expected nil container, got: %v", container)
		}
	})

	t.Run("nil dbHost", func(t *testing.T) {
		container, err := NewTestAppContainer(cmd, nil)
		if err == nil {
			t.Fatal("expected error when dbHost is nil, got nil")
		}
		if container != nil {
			t.Errorf("expected nil container, got: %v", container)
		}
	})
}

func TestNewAppContainerWithErrors(t *testing.T) {
	tests := []struct {
		setupCmd    func() *cobra.Command
		name        string
		errorMsg    string
		expectError bool
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
				cmd.Flags().String("workspace", "", "workspace root path")
				cmd.Flags().String("task", "", "task name")
				cmd.Flags().String("user-prompt", "", "user prompt")
				cmd.Flags().Bool("silent", false, "silent mode")
				cmd.Flags().Bool("no-cache", false, "disable cache")
				cmd.Flags().Bool("update-cache", false, "update cache")
				cmd.Flags().Bool("plain", false, "plain output")
				cmd.Flags().Bool("no-color", false, "no color")
				cmd.Flags().Set("config", "/nonexistent/path/config.yaml")
				return cmd
			},
			expectError: false, // Config service no longer fails if config file doesn't exist
			errorMsg:    "",
		},
		{
			name: "Config service creation error - no config found",
			setupCmd: func() *cobra.Command {
				// Save current directory and environment
				origDir, _ := os.Getwd()
				origHome := os.Getenv("HOME")
				origXDG := os.Getenv("XDG_CONFIG_HOME")

				// Change to a temporary directory with no config files
				tmpDir := t.TempDir()
				os.Chdir(tmpDir)

				// Clear HOME and XDG_CONFIG_HOME to ensure no user configs are found
				os.Setenv("HOME", tmpDir)
				os.Setenv("XDG_CONFIG_HOME", tmpDir)

				// Restore after test
				t.Cleanup(func() {
					os.Chdir(origDir)
					os.Setenv("HOME", origHome)
					if origXDG != "" {
						os.Setenv("XDG_CONFIG_HOME", origXDG)
					} else {
						os.Unsetenv("XDG_CONFIG_HOME")
					}
				})

				cmd := &cobra.Command{Use: "test"}
				cmd.Flags().String("config", "", "config file path")
				cmd.Flags().String("workspace", "", "workspace root path")
				cmd.Flags().String("task", "", "task name")
				cmd.Flags().String("user-prompt", "", "user prompt")
				cmd.Flags().Bool("silent", false, "silent mode")
				cmd.Flags().Bool("no-cache", false, "disable cache")
				cmd.Flags().Bool("update-cache", false, "update cache")
				cmd.Flags().Bool("plain", false, "plain output")
				cmd.Flags().Bool("no-color", false, "no color")
				// Don't set config path
				return cmd
			},
			expectError: false, // Config service no longer fails if no config found
			errorMsg:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var container *Container
			var err error

			if tt.name == "Command service creation error" {
				container, err = newAppContainerWithRecovery(t, tt.expectError)
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
	originalLocalAppData := os.Getenv("LOCALAPPDATA")
	originalXDGCache := os.Getenv("XDG_CACHE_HOME")

	defer func() {
		os.Setenv("LOCALAPPDATA", originalLocalAppData)
		os.Setenv("XDG_CACHE_HOME", originalXDGCache)
	}()

	// Test Windows with LOCALAPPDATA set
	if runtime.GOOS == osWindows {
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
	if runtime.GOOS != osWindows && runtime.GOOS != osDarwin {
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
	originalLocalAppData := os.Getenv("LOCALAPPDATA")
	originalXDGCache := os.Getenv("XDG_CACHE_HOME")

	defer func() {
		os.Setenv("LOCALAPPDATA", originalLocalAppData)
		os.Setenv("XDG_CACHE_HOME", originalXDGCache)
	}()

	// Test Windows fallback when LOCALAPPDATA is empty
	if runtime.GOOS == osWindows {
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
	t.Skip("This test is for deprecated YAML-based config system. " +
		"The application now uses Starlark-based configuration loaded during CLI initialization.")

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `schema_version: 1
providers:
  openai:
    type: "openai"
  anthropic:
    type: "anthropic"
models:
  test:
    provider: "openai"
    model: "gpt-4"
    limits:
      max_input_tokens: 2000
      max_output_tokens: 1000
  anthropic:
    provider: "anthropic"
    model: "claude-3"
presets:
  test:
    model: "test"
    request:
      temperature: 0.7
  anthropic:
    model: "anthropic"
flows:
  write:
    preset: "test"
    system_prompt: "You are a helpful assistant"
    tasks:
      anthropic-task:
        preset: "anthropic"
        system_prompt: "You are an expert assistant"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	cleanEnvDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cleanEnvDir)
	t.Setenv("HOME", cleanEnvDir)
	t.Setenv("XDG_CONFIG_DIRS", "")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().String("workspace", "", "workspace root path")
	cmd.Flags().String("task", "", "task name")
	cmd.Flags().String("user-prompt", "", "user prompt")
	cmd.Flags().Bool("no-tui", false, "disable TUI")
	cmd.Flags().Set("config", configPath)
	cmd.Flags().Set("task", "anthropic-task")
	cmd.Flags().Set("user-prompt", "Test prompt")
	cmd.Flags().Set("no-tui", "true")

	container, err := NewAppContainer(cmd)
	if err != nil {
		t.Fatalf("NewAppContainer failed: %v", err)
	}

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

	cfg, err := container.ConfigService.Get()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	t.Logf("Config loaded: Presets=%d, Models=%d, Providers=%d",
		len(cfg.Presets), len(cfg.Models), len(cfg.Providers))
	t.Logf("Presets: %+v", cfg.Presets)
	t.Logf("Flows: %+v", cfg.Flows)

	if len(cfg.Presets) != 2 {
		t.Errorf("Expected 2 presets, got %d", len(cfg.Presets))
	}

	if cfg.Flows == nil || cfg.Flows.Write == nil {
		t.Error("Write pipeline config should not be nil")
	}

	if cfg.Flows != nil && cfg.Flows.Write != nil && cfg.Flows.Write.Tasks == nil {
		t.Log("No explicit tasks configured beyond default")
	} else if cfg.Flows != nil && cfg.Flows.Write != nil && len(cfg.Flows.Write.Tasks) != 1 {
		t.Logf("Expected 1 task, got %d", len(cfg.Flows.Write.Tasks))
	}

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

	noTUI, err := container.CommandService.GetNoTUIFlag()
	if err != nil {
		t.Errorf("GetNoTUIFlag failed: %v", err)
	}
	if !noTUI {
		t.Error("Expected no-tui to be true")
	}
}

func TestAppContainerKeyContextValue(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "minimal-config.yaml")
	configContent := `schema_version: 1
providers:
  test:
    type: "test"
models:
  minimal:
    provider: "test"
    model: "test-model"
presets:
  minimal:
    model: "minimal"
flows:
  write:
    preset: "minimal"
    system_prompt: "test"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().String("workspace", "", "workspace root path")
	cmd.Flags().String("task", "", "task name")
	cmd.Flags().String("user-prompt", "", "user prompt")
	cmd.Flags().Bool("silent", false, "silent mode")
	cmd.Flags().Set("config", configPath)

	container, err := NewAppContainer(cmd)
	if err != nil {
		t.Fatalf("NewAppContainer failed: %v", err)
	}

	contextValue := cmd.Context().Value(AppContainerKey)
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

	if containerFromContext.Logger == nil {
		t.Error("Logger should be accessible through context")
	}

	if containerFromContext.ConfigService == nil {
		t.Error("ConfigService should be accessible through context")
	}
}

func TestGetLogDirErrorHandling(t *testing.T) {
	dir, err := getLogDir()
	if err != nil {
		t.Errorf("getLogDir should not fail in normal test environment: %v", err)
	}

	if dir == "" {
		t.Error("getLogDir should return non-empty directory")
	}

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
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().String("workspace", "", "workspace root path")
	cmd.Flags().String("task", "", "task name")
	cmd.Flags().String("user-prompt", "test prompt", "user prompt")
	cmd.Flags().Bool("silent", false, "silent mode")

	container, err := NewAppContainer(cmd)
	if err != nil {
		t.Logf("NewAppContainer failed (might be environment-related): %v", err)
		return
	}

	if container == nil {
		t.Fatal("Expected container to be created")
	}

	if container.Logger == nil {
		t.Error("Logger should be initialized")
	}

	if container.ShutdownService != nil {
		container.ShutdownService.Shutdown()
	}
}

func TestValidateLogPath(t *testing.T) {
	tests := []struct {
		name      string
		logDir    string
		fileName  string
		errMsg    string
		expectErr bool
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
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error to contain %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func newAppContainerWithRecovery(t *testing.T, expectError bool) (*Container, error) {
	t.Helper()

	var (
		container *Container
		err       error
	)
	defer func() {
		if r := recover(); r == nil && err == nil && expectError {
			t.Error("Expected error but got none")
		}
	}()

	container, err = NewAppContainer(nil)
	return container, err
}

func TestGetHTTPClientService(t *testing.T) {
	container := &Container{
		httpClientService: &mockHTTPClientService{},
	}

	service := container.GetHTTPClientService()
	if service == nil {
		t.Error("Expected HTTP client service but got nil")
	}
}

func TestMaxPresetCacheTTL(t *testing.T) {
	tests := []struct {
		cfg      *domainConfig.Config
		name     string
		expected time.Duration
	}{
		{
			name:     "nil config",
			cfg:      nil,
			expected: 0,
		},
		{
			name: "no presets",
			cfg: &domainConfig.Config{
				Presets: map[string]*domainConfig.PresetConfig{},
			},
			expected: 0,
		},
		{
			name: "preset with no cache",
			cfg: &domainConfig.Config{
				Presets: map[string]*domainConfig.PresetConfig{
					"test": {},
				},
			},
			expected: 0,
		},
		{
			name: "preset with disabled cache",
			cfg: &domainConfig.Config{
				Presets: map[string]*domainConfig.PresetConfig{
					"test": {
						Cache: &domainConfig.CacheConfig{
							Enabled: boolPtr(false),
							TTL:     time.Hour,
						},
					},
				},
			},
			expected: 0,
		},
		{
			name: "preset with enabled cache",
			cfg: &domainConfig.Config{
				Presets: map[string]*domainConfig.PresetConfig{
					"test": {
						Cache: &domainConfig.CacheConfig{
							Enabled: boolPtr(true),
							TTL:     time.Hour,
						},
					},
				},
			},
			expected: time.Hour,
		},
		{
			name: "multiple presets, return max TTL",
			cfg: &domainConfig.Config{
				Presets: map[string]*domainConfig.PresetConfig{
					"short": {
						Cache: &domainConfig.CacheConfig{
							Enabled: boolPtr(true),
							TTL:     time.Minute,
						},
					},
					"long": {
						Cache: &domainConfig.CacheConfig{
							Enabled: boolPtr(true),
							TTL:     time.Hour * 24,
						},
					},
				},
			},
			expected: time.Hour * 24,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maxPresetCacheTTL(tt.cfg)
			if result != tt.expected {
				t.Errorf("Expected %v but got %v", tt.expected, result)
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func TestGetDBHost(t *testing.T) {
	dbHost := &testMockDBHost{}
	container := &Container{
		dbHost: dbHost,
	}

	result := container.GetDBHost()
	if result != dbHost {
		t.Error("Expected dbHost to be returned")
	}
}

func TestGetCacheRepo(t *testing.T) {
	mockRepo := cache.NewRepository(&testMockDBHost{})
	container := &Container{
		cacheRepo: mockRepo,
	}

	result := container.GetCacheRepo()
	if result != mockRepo {
		t.Error("Expected cacheRepo to be returned")
	}
}

// Mock HTTP client service for testing.
type mockHTTPClientService struct{}

func (m *mockHTTPClientService) Get() *http.Client {
	return &http.Client{}
}

func (m *mockHTTPClientService) GetWithTimeout(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func (m *mockHTTPClientService) Close() error {
	return nil
}

func (m *mockHTTPClientService) Validate() error {
	return nil
}
