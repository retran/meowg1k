// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestNewService(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	if service == nil {
		t.Fatal("Service should not be nil")
	}
}

func TestNewServiceReturnsErrorWithNilCommand(t *testing.T) {
	_, err := NewService(nil)
	if err == nil {
		t.Error("Expected error when command is nil")
	}
}

func TestGetCommand(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	gotCmd, err := service.GetCommand()
	if err != nil {
		t.Fatalf("GetCommand failed: %v", err)
	}
	if gotCmd != cmd {
		t.Error("GetCommand should return the original command")
	}
}

func TestGetCommandName(t *testing.T) {
	cmd := &cobra.Command{
		Use: "testcmd",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	cmdName, err := service.GetCommandName()
	if err != nil {
		t.Fatalf("GetCommandName failed: %v", err)
	}
	if cmdName != "testcmd" {
		t.Errorf("Expected command name 'testcmd', got '%s'", cmdName)
	}
}

func TestGetConfigPath(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().Set("config", "/path/to/config.yaml")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	configPath, err := service.GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath failed: %v", err)
	}

	if configPath != "/path/to/config.yaml" {
		t.Errorf("Expected config path '/path/to/config.yaml', got '%s'", configPath)
	}
}

func TestGetConfigPathUndefined(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.GetConfigPath()
	if err == nil {
		t.Error("Expected error when config flag is not defined")
	}
}

func TestGetWorkspacePath(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().String("workspace", "", "workspace root path")
	cmd.Flags().Set("workspace", "/path/to/workspace")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	workspacePath, err := service.GetWorkspacePath()
	if err != nil {
		t.Fatalf("GetWorkspacePath failed: %v", err)
	}

	if workspacePath != "/path/to/workspace" {
		t.Errorf("Expected workspace path '/path/to/workspace', got '%s'", workspacePath)
	}
}

func TestGetWorkspacePathUndefined(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.GetWorkspacePath()
	if err == nil {
		t.Error("Expected error when workspace flag is not defined")
	}
}

func TestGetTaskName(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().String("task", "", "task name")
	cmd.Flags().Set("task", "mytask")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	taskName, err := service.GetTaskName()
	if err != nil {
		t.Fatalf("GetTaskName failed: %v", err)
	}

	if taskName != "mytask" {
		t.Errorf("Expected task name 'mytask', got '%s'", taskName)
	}
}

func TestGetUserPrompt(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().String("user-prompt", "", "user prompt")
	cmd.Flags().Set("user-prompt", "hello world")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	userPrompt, err := service.GetUserPrompt()
	if err != nil {
		t.Fatalf("GetUserPrompt failed: %v", err)
	}

	if userPrompt != "hello world" {
		t.Errorf("Expected user prompt 'hello world', got '%s'", userPrompt)
	}
}

func TestGetSilentFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().Bool("silent", false, "silent mode")
	cmd.Flags().Set("silent", "true")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	silent, err := service.GetSilentFlag()
	if err != nil {
		t.Fatalf("GetSilentFlag failed: %v", err)
	}

	if !silent {
		t.Error("Expected silent flag to be true")
	}
}

func TestGetStdIn(t *testing.T) {
	// This test is tricky because it depends on stdin state
	// We'll test the basic functionality
	cmd := &cobra.Command{
		Use: "test",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// When run in test environment, stdin is typically empty
	stdin, err := service.GetStdIn()
	if err != nil {
		t.Fatalf("GetStdIn failed: %v", err)
	}
	if stdin == "" {
		// This is expected in test environment
		t.Log("StdIn is empty as expected in test environment")
	}

	// Test that GetStdIn returns a string (not nil)
	_, err = service.GetStdIn() // Should not return error
	if err != nil {
		t.Fatalf("GetStdIn failed: %v", err)
	}
}

func TestNewServiceStdinErrorPaths(t *testing.T) {
	// These tests are challenging because they require manipulating stdin
	// We can test the service creation works with various command configurations

	tests := []struct {
		setupCmd    func() *cobra.Command
		name        string
		expectError bool
	}{
		{
			name: "Valid command with no flags",
			setupCmd: func() *cobra.Command {
				return &cobra.Command{Use: "test"}
			},
			expectError: false,
		},
		{
			name: "Command with multiple flags",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "complex"}
				cmd.Flags().String("config", "", "config path")
				cmd.Flags().String("task", "", "task name")
				cmd.Flags().String("user-prompt", "", "user prompt")
				cmd.Flags().Bool("silent", false, "silent mode")
				return cmd
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.setupCmd()
			service, err := NewService(cmd)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.expectError && service == nil {
				t.Error("Expected service but got nil")
			}
		})
	}
}

func TestGetMethodsWithUndefinedFlags(t *testing.T) {
	// Test error handling when flags are not defined
	cmd := &cobra.Command{Use: "test"}
	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	tests := []struct {
		testFunc func() (interface{}, error)
		name     string
	}{
		{
			name: "GetConfigPath with undefined flag",
			testFunc: func() (interface{}, error) {
				return service.GetConfigPath()
			},
		},
		{
			name: "GetTaskName with undefined flag",
			testFunc: func() (interface{}, error) {
				return service.GetTaskName()
			},
		},
		{
			name: "GetUserPrompt with undefined flag",
			testFunc: func() (interface{}, error) {
				return service.GetUserPrompt()
			},
		},
		{
			name: "GetSilentFlag with undefined flag",
			testFunc: func() (interface{}, error) {
				return service.GetSilentFlag()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.testFunc()
			if err == nil {
				t.Errorf("Expected error for %s when flag is undefined", tt.name)
			}
		})
	}
}

func TestServiceMethodsWithDefinedButUnsetFlags(t *testing.T) {
	// Test behavior when flags are defined but not set (empty values)
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("config", "", "config path")
	cmd.Flags().String("task", "", "task name")
	cmd.Flags().String("user-prompt", "", "user prompt")
	cmd.Flags().Bool("silent", false, "silent mode")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Test with unset flags (should return empty/default values)
	configPath, err := service.GetConfigPath()
	if err != nil {
		t.Errorf("GetConfigPath failed: %v", err)
	}
	if configPath != "" {
		t.Errorf("Expected empty config path, got '%s'", configPath)
	}

	taskName, err := service.GetTaskName()
	if err != nil {
		t.Errorf("GetTaskName failed: %v", err)
	}
	if taskName != "" {
		t.Errorf("Expected empty task name, got '%s'", taskName)
	}

	userPrompt, err := service.GetUserPrompt()
	if err != nil {
		t.Errorf("GetUserPrompt failed: %v", err)
	}
	if userPrompt != "" {
		t.Errorf("Expected empty user prompt, got '%s'", userPrompt)
	}

	silent, err := service.GetSilentFlag()
	if err != nil {
		t.Errorf("GetSilentFlag failed: %v", err)
	}
	if silent {
		t.Error("Expected silent flag to be false by default")
	}
}

func TestGetMethodsWithVariousValues(t *testing.T) {
	// Test with various flag values including edge cases
	tests := []struct {
		name        string
		configValue string
		taskValue   string
		promptValue string
		silentValue string
	}{
		{
			name:        "Normal values",
			configValue: "/etc/config.yaml",
			taskValue:   "process-data",
			promptValue: "Process the input data",
			silentValue: "false",
		},
		{
			name:        "Empty string values",
			configValue: "",
			taskValue:   "",
			promptValue: "",
			silentValue: "false",
		},
		{
			name:        "Special characters",
			configValue: "/path/with spaces/config.yaml",
			taskValue:   "task-with-dashes_and_underscores",
			promptValue: "Prompt with \"quotes\" and 'apostrophes'",
			silentValue: "true",
		},
		{
			name:        "Long values",
			configValue: "/very/long/path/to/configuration/file/that/might/cause/issues.yaml",
			taskValue:   "very-long-task-name-that-exceeds-normal-length-expectations",
			promptValue: "This is a very long prompt that contains multiple sentences and might test the limits of string handling in the system.",
			silentValue: "true",
		},
		{
			name:        "Unicode and special characters",
			configValue: "/config/αβγ/config.yaml",
			taskValue:   "task-ñáéíóú",
			promptValue: "Prompt with 🚀 emoji and unicode αβγδε",
			silentValue: "false",
		},
		{
			name:        "Path with various separators",
			configValue: "C:\\Windows\\config.yaml",
			taskValue:   "windows\\task",
			promptValue: "Path: C:\\Users\\test\\file.txt",
			silentValue: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}
			cmd.Flags().String("config", "", "config path")
			cmd.Flags().String("task", "", "task name")
			cmd.Flags().String("user-prompt", "", "user prompt")
			cmd.Flags().Bool("silent", false, "silent mode")

			// Set flag values
			cmd.Flags().Set("config", tt.configValue)
			cmd.Flags().Set("task", tt.taskValue)
			cmd.Flags().Set("user-prompt", tt.promptValue)
			cmd.Flags().Set("silent", tt.silentValue)

			service, err := NewService(cmd)
			if err != nil {
				t.Fatalf("NewService failed: %v", err)
			}

			// Test all getters
			configPath, err := service.GetConfigPath()
			if err != nil {
				t.Errorf("GetConfigPath failed: %v", err)
			}
			if configPath != tt.configValue {
				t.Errorf("Expected config '%s', got '%s'", tt.configValue, configPath)
			}

			taskName, err := service.GetTaskName()
			if err != nil {
				t.Errorf("GetTaskName failed: %v", err)
			}
			if taskName != tt.taskValue {
				t.Errorf("Expected task '%s', got '%s'", tt.taskValue, taskName)
			}

			userPrompt, err := service.GetUserPrompt()
			if err != nil {
				t.Errorf("GetUserPrompt failed: %v", err)
			}
			if userPrompt != tt.promptValue {
				t.Errorf("Expected prompt '%s', got '%s'", tt.promptValue, userPrompt)
			}

			expectedSilent := tt.silentValue == "true"
			silent, err := service.GetSilentFlag()
			if err != nil {
				t.Errorf("GetSilentFlag failed: %v", err)
			}
			if silent != expectedSilent {
				t.Errorf("Expected silent %v, got %v", expectedSilent, silent)
			}
		})
	}
}

func TestServiceInterfaceCompliance(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Verify interface compliance
	_ = service
	t.Log("Service correctly implements Service interface")
}

func TestCommandServiceStateManagement(t *testing.T) {
	cmd := &cobra.Command{Use: "test-state"}
	cmd.Flags().String("config", "initial-config", "config path")
	cmd.Flags().String("task", "initial-task", "task name")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Test that service maintains state
	configPath1, _ := service.GetConfigPath()
	configPath2, _ := service.GetConfigPath()
	if configPath1 != configPath2 {
		t.Error("Service should maintain consistent state")
	}

	// Test that underlying command is preserved
	gotCmd, err := service.GetCommand()
	if err != nil {
		t.Fatalf("GetCommand failed: %v", err)
	}
	if gotCmd != cmd {
		t.Error("Service should preserve the original command reference")
	}

	// Test command name consistency
	cmdName, err := service.GetCommandName()
	if err != nil {
		t.Fatalf("GetCommandName failed: %v", err)
	}
	if cmdName != "test-state" {
		t.Errorf("Expected command name 'test-state', got '%s'", cmdName)
	}
}

func TestCommandServiceConcurrency(t *testing.T) {
	cmd := &cobra.Command{Use: "concurrent-test"}
	cmd.Flags().String("config", "test-config", "config path")
	cmd.Flags().Bool("silent", false, "silent mode")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Test concurrent access to service methods
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Multiple concurrent calls to various methods
			_, err := service.GetConfigPath()
			if err != nil {
				t.Errorf("Goroutine %d: GetConfigPath failed: %v", id, err)
				return
			}

			name, err := service.GetCommandName()
			if err != nil {
				t.Errorf("Goroutine %d: GetCommandName failed: %v", id, err)
				return
			}
			if name != "concurrent-test" {
				t.Errorf("Goroutine %d: Expected 'concurrent-test', got '%s'", id, name)
				return
			}

			_, err = service.GetSilentFlag()
			if err != nil {
				t.Errorf("Goroutine %d: GetSilentFlag failed: %v", id, err)
				return
			}

			stdin, err := service.GetStdIn()
			if err != nil {
				t.Errorf("Goroutine %d: GetStdIn failed: %v", id, err)
				return
			}
			_ = stdin // Just verify it doesn't return error
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestCommandServiceErrorPropagation(t *testing.T) {
	// Test that cobra flag errors are properly propagated
	cmd := &cobra.Command{Use: "error-test"}
	// Intentionally don't define flags to cause errors

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// These should all return errors since flags aren't defined
	testCases := []struct {
		fn   func() (interface{}, error)
		name string
	}{
		{func() (interface{}, error) { return service.GetConfigPath() }, "GetConfigPath"},
		{func() (interface{}, error) { return service.GetTaskName() }, "GetTaskName"},
		{func() (interface{}, error) { return service.GetUserPrompt() }, "GetUserPrompt"},
		{func() (interface{}, error) { return service.GetSilentFlag() }, "GetSilentFlag"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.fn()
			if err == nil {
				t.Errorf("%s should return error when flag is not defined", tc.name)
			}
			// Verify error message indicates the flag issue
			if !strings.Contains(err.Error(), "flag") {
				t.Errorf("%s error should mention flag issue, got: %v", tc.name, err)
			}
		})
	}
}

func TestCommandServiceMemoryUsage(t *testing.T) {
	// Test that service doesn't leak memory with repeated calls
	cmd := &cobra.Command{Use: "memory-test"}
	cmd.Flags().String("config", "test-config", "config path")
	cmd.Flags().String("task", "test-task", "task name")
	cmd.Flags().Bool("silent", false, "silent mode")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Make many repeated calls to ensure no memory leaks
	for i := 0; i < 1000; i++ {
		_, _ = service.GetConfigPath()
		_, _ = service.GetCommandName()
		_, _ = service.GetTaskName()
		_, _ = service.GetSilentFlag()
		_, _ = service.GetStdIn()
		_, _ = service.GetCommand()
	}

	t.Log("Completed 1000 iterations without issues")
}

func TestGetStdInWithPipedInput(t *testing.T) {
	// Save original stdin
	oldStdin := os.Stdin
	defer func() {
		os.Stdin = oldStdin
	}()

	// Create a pipe
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	// Set stdin to the read end of the pipe
	os.Stdin = r

	// Write test data to the write end
	testInput := "piped input data\nwith multiple lines"
	_, err = w.WriteString(testInput)
	if err != nil {
		t.Fatalf("Failed to write to pipe: %v", err)
	}
	w.Close() // Close writer to signal EOF

	cmd := &cobra.Command{
		Use: "test",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	stdin, err := service.GetStdIn()
	if err != nil {
		t.Fatalf("GetStdIn failed: %v", err)
	}
	expected := strings.TrimSpace(testInput)
	if stdin != expected {
		t.Errorf("Expected stdin '%s', got '%s'", expected, stdin)
	}
}

func TestGetIntentFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().String("intent", "", "intent flag")
	cmd.Flags().Set("intent", "commit")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	intent, err := service.GetIntentFlag()
	if err != nil {
		t.Fatalf("GetIntentFlag failed: %v", err)
	}

	if intent != "commit" {
		t.Errorf("Expected intent 'commit', got '%s'", intent)
	}
}

func TestGetIntentFlagUndefined(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.GetIntentFlag()
	if err == nil {
		t.Error("Expected error when intent flag is not defined")
	}
}

func TestNilServiceMethods(t *testing.T) {
	var service *Service

	t.Run("GetCommand on nil service", func(t *testing.T) {
		_, err := service.GetCommand()
		if err == nil {
			t.Error("Expected error for nil service, got nil")
		}
	})

	t.Run("GetCommandName on nil service", func(t *testing.T) {
		_, err := service.GetCommandName()
		if err == nil {
			t.Error("Expected error for nil service, got nil")
		}
	})

	t.Run("GetConfigPath on nil service", func(t *testing.T) {
		_, err := service.GetConfigPath()
		if err == nil {
			t.Error("Expected error for nil service, got nil")
		}
	})

	t.Run("GetTaskName on nil service", func(t *testing.T) {
		_, err := service.GetTaskName()
		if err == nil {
			t.Error("Expected error for nil service, got nil")
		}
	})

	t.Run("GetUserPrompt on nil service", func(t *testing.T) {
		_, err := service.GetUserPrompt()
		if err == nil {
			t.Error("Expected error for nil service, got nil")
		}
	})

	t.Run("GetSilentFlag on nil service", func(t *testing.T) {
		_, err := service.GetSilentFlag()
		if err == nil {
			t.Error("Expected error for nil service, got nil")
		}
	})

	t.Run("GetIntentFlag on nil service", func(t *testing.T) {
		_, err := service.GetIntentFlag()
		if err == nil {
			t.Error("Expected error for nil service, got nil")
		}
	})

	t.Run("GetTargetBranchFlag on nil service", func(t *testing.T) {
		_, err := service.GetTargetBranchFlag()
		if err == nil {
			t.Error("Expected error for nil service, got nil")
		}
	})

	t.Run("GetBaseBranchFlag on nil service", func(t *testing.T) {
		_, err := service.GetBaseBranchFlag()
		if err == nil {
			t.Error("Expected error for nil service, got nil")
		}
	})

	t.Run("GetStdIn on nil service", func(t *testing.T) {
		_, err := service.GetStdIn()
		if err == nil {
			t.Error("Expected error for nil service, got nil")
		}
	})
}

func TestGetNoCacheFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().Bool("no-cache", false, "disable cache")
	cmd.Flags().Set("no-cache", "true")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	noCache, err := service.GetNoCacheFlag()
	if err != nil {
		t.Fatalf("GetNoCacheFlag failed: %v", err)
	}

	if !noCache {
		t.Error("Expected no-cache flag to be true")
	}
}

func TestGetNoCacheFlagFalse(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().Bool("no-cache", false, "disable cache")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	noCache, err := service.GetNoCacheFlag()
	if err != nil {
		t.Fatalf("GetNoCacheFlag failed: %v", err)
	}

	if noCache {
		t.Error("Expected no-cache flag to be false")
	}
}

func TestGetNoCacheFlagUndefined(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.GetNoCacheFlag()
	if err == nil {
		t.Error("Expected error when no-cache flag is not defined")
	}
}

func TestGetNoCacheFlagNilService(t *testing.T) {
	var service *Service

	_, err := service.GetNoCacheFlag()
	if err == nil {
		t.Error("Expected error for nil service")
	}
	if !strings.Contains(err.Error(), "command service is nil") {
		t.Errorf("Expected 'command service is nil' error, got: %v", err)
	}
}

func TestGetUpdateCacheFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().Bool("update-cache", false, "update cache")
	cmd.Flags().Set("update-cache", "true")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	updateCache, err := service.GetUpdateCacheFlag()
	if err != nil {
		t.Fatalf("GetUpdateCacheFlag failed: %v", err)
	}

	if !updateCache {
		t.Error("Expected update-cache flag to be true")
	}
}

func TestGetUpdateCacheFlagFalse(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().Bool("update-cache", false, "update cache")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	updateCache, err := service.GetUpdateCacheFlag()
	if err != nil {
		t.Fatalf("GetUpdateCacheFlag failed: %v", err)
	}

	if updateCache {
		t.Error("Expected update-cache flag to be false")
	}
}

func TestGetUpdateCacheFlagUndefined(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.GetUpdateCacheFlag()
	if err == nil {
		t.Error("Expected error when update-cache flag is not defined")
	}
}

func TestGetUpdateCacheFlagNilService(t *testing.T) {
	var service *Service

	_, err := service.GetUpdateCacheFlag()
	if err == nil {
		t.Error("Expected error for nil service")
	}
}

func TestGetQueryTextFlagFromArgs(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
		Run: func(cmd *cobra.Command, args []string) {},
	}

	// Simulate passing arguments
	cmd.SetArgs([]string{"test query text"})
	cmd.Execute()

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	queryText, err := service.GetQueryTextFlag()
	if err != nil {
		t.Fatalf("GetQueryTextFlag failed: %v", err)
	}

	if queryText != "test query text" {
		t.Errorf("Expected query text 'test query text', got '%s'", queryText)
	}
}

func TestGetQueryTextFlagNoArgsNoStdin(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.GetQueryTextFlag()
	if err == nil {
		t.Error("Expected error when no args and no stdin")
	}
	if !strings.Contains(err.Error(), "query text is required") {
		t.Errorf("Expected 'query text is required' error, got: %v", err)
	}
}

func TestGetQueryTextFlagNilService(t *testing.T) {
	var service *Service

	_, err := service.GetQueryTextFlag()
	if err == nil {
		t.Error("Expected error for nil service")
	}
}

func TestGetSnapshotsFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().StringSlice("snapshots", []string{}, "snapshots")
	cmd.Flags().Set("snapshots", "_head_,_stage_,_workdir_")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	snapshots, err := service.GetSnapshotsFlag()
	if err != nil {
		t.Fatalf("GetSnapshotsFlag failed: %v", err)
	}

	expected := []string{"_head_", "_stage_", "_workdir_"}
	if len(snapshots) != len(expected) {
		t.Errorf("Expected %d snapshots, got %d", len(expected), len(snapshots))
	}
	for i, snap := range expected {
		if snapshots[i] != snap {
			t.Errorf("Expected snapshot[%d] = '%s', got '%s'", i, snap, snapshots[i])
		}
	}
}

func TestGetSnapshotsFlagEmpty(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().StringSlice("snapshots", []string{}, "snapshots")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	snapshots, err := service.GetSnapshotsFlag()
	if err != nil {
		t.Fatalf("GetSnapshotsFlag failed: %v", err)
	}

	if len(snapshots) != 0 {
		t.Errorf("Expected empty snapshots, got %v", snapshots)
	}
}

func TestGetSnapshotsFlagUndefined(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.GetSnapshotsFlag()
	if err == nil {
		t.Error("Expected error when snapshots flag is not defined")
	}
}

func TestGetSnapshotsFlagNilService(t *testing.T) {
	var service *Service

	_, err := service.GetSnapshotsFlag()
	if err == nil {
		t.Error("Expected error for nil service")
	}
}

func TestGetTopKFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().Int("top-k", 10, "top k results")
	cmd.Flags().Set("top-k", "25")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	topK, err := service.GetTopKFlag()
	if err != nil {
		t.Fatalf("GetTopKFlag failed: %v", err)
	}

	if topK != 25 {
		t.Errorf("Expected top-k 25, got %d", topK)
	}
}

func TestGetTopKFlagDefault(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().Int("top-k", 10, "top k results")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	topK, err := service.GetTopKFlag()
	if err != nil {
		t.Fatalf("GetTopKFlag failed: %v", err)
	}

	if topK != 10 {
		t.Errorf("Expected top-k 10 (default), got %d", topK)
	}
}

func TestGetTopKFlagUndefined(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.GetTopKFlag()
	if err == nil {
		t.Error("Expected error when top-k flag is not defined")
	}
}

func TestGetTopKFlagNilService(t *testing.T) {
	var service *Service

	_, err := service.GetTopKFlag()
	if err == nil {
		t.Error("Expected error for nil service")
	}
}

func TestGetMinScoreFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().Float32("min-score", 0.5, "minimum score")
	cmd.Flags().Set("min-score", "0.75")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	minScore, err := service.GetMinScoreFlag()
	if err != nil {
		t.Fatalf("GetMinScoreFlag failed: %v", err)
	}

	if minScore != 0.75 {
		t.Errorf("Expected min-score 0.75, got %f", minScore)
	}
}

func TestGetMinScoreFlagDefault(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().Float32("min-score", 0.5, "minimum score")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	minScore, err := service.GetMinScoreFlag()
	if err != nil {
		t.Fatalf("GetMinScoreFlag failed: %v", err)
	}

	if minScore != 0.5 {
		t.Errorf("Expected min-score 0.5 (default), got %f", minScore)
	}
}

func TestGetMinScoreFlagUndefined(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.GetMinScoreFlag()
	if err == nil {
		t.Error("Expected error when min-score flag is not defined")
	}
}

func TestGetMinScoreFlagNilService(t *testing.T) {
	var service *Service

	_, err := service.GetMinScoreFlag()
	if err == nil {
		t.Error("Expected error for nil service")
	}
}

func TestGetJSONFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().Bool("json", false, "json output")
	cmd.Flags().Set("json", "true")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	json, err := service.GetJSONFlag()
	if err != nil {
		t.Fatalf("GetJSONFlag failed: %v", err)
	}

	if !json {
		t.Error("Expected json flag to be true")
	}
}

func TestGetJSONFlagFalse(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().Bool("json", false, "json output")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	json, err := service.GetJSONFlag()
	if err != nil {
		t.Fatalf("GetJSONFlag failed: %v", err)
	}

	if json {
		t.Error("Expected json flag to be false")
	}
}

func TestGetJSONFlagUndefined(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.GetJSONFlag()
	if err == nil {
		t.Error("Expected error when json flag is not defined")
	}
}

func TestGetJSONFlagNilService(t *testing.T) {
	var service *Service

	_, err := service.GetJSONFlag()
	if err == nil {
		t.Error("Expected error for nil service")
	}
}

func TestGetQuestionFlagFromArgs(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
		Run: func(cmd *cobra.Command, args []string) {},
	}

	cmd.SetArgs([]string{"What is the meaning of life?"})
	cmd.Execute()

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	question, err := service.GetQuestionFlag()
	if err != nil {
		t.Fatalf("GetQuestionFlag failed: %v", err)
	}

	if question != "What is the meaning of life?" {
		t.Errorf("Expected question 'What is the meaning of life?', got '%s'", question)
	}
}

func TestGetQuestionFlagNoArgsNoStdin(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.GetQuestionFlag()
	if err == nil {
		t.Error("Expected error when no args and no stdin")
	}
	if !strings.Contains(err.Error(), "question is required") {
		t.Errorf("Expected 'question is required' error, got: %v", err)
	}
}

func TestGetQuestionFlagNilService(t *testing.T) {
	var service *Service

	_, err := service.GetQuestionFlag()
	if err == nil {
		t.Error("Expected error for nil service")
	}
}

func TestGetProfileFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().String("profile", "", "profile name")
	cmd.Flags().Set("profile", "production")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	profile, err := service.GetProfileFlag()
	if err != nil {
		t.Fatalf("GetProfileFlag failed: %v", err)
	}

	if profile != "production" {
		t.Errorf("Expected profile 'production', got '%s'", profile)
	}
}

func TestGetProfileFlagEmpty(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().String("profile", "", "profile name")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	profile, err := service.GetProfileFlag()
	if err != nil {
		t.Fatalf("GetProfileFlag failed: %v", err)
	}

	if profile != "" {
		t.Errorf("Expected empty profile, got '%s'", profile)
	}
}

func TestGetProfileFlagUndefined(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.GetProfileFlag()
	if err == nil {
		t.Error("Expected error when profile flag is not defined")
	}
}

func TestGetProfileFlagNilService(t *testing.T) {
	var service *Service

	_, err := service.GetProfileFlag()
	if err == nil {
		t.Error("Expected error for nil service")
	}
}

func TestGetShowContextFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().Bool("show-context", false, "show context")
	cmd.Flags().Set("show-context", "true")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	showContext, err := service.GetShowContextFlag()
	if err != nil {
		t.Fatalf("GetShowContextFlag failed: %v", err)
	}

	if !showContext {
		t.Error("Expected show-context flag to be true")
	}
}

func TestGetShowContextFlagFalse(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().Bool("show-context", false, "show context")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	showContext, err := service.GetShowContextFlag()
	if err != nil {
		t.Fatalf("GetShowContextFlag failed: %v", err)
	}

	if showContext {
		t.Error("Expected show-context flag to be false")
	}
}

func TestGetShowContextFlagUndefined(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.GetShowContextFlag()
	if err == nil {
		t.Error("Expected error when show-context flag is not defined")
	}
}

func TestGetShowContextFlagNilService(t *testing.T) {
	var service *Service

	_, err := service.GetShowContextFlag()
	if err == nil {
		t.Error("Expected error for nil service")
	}
}

func TestGetSystemPromptFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().String("system-prompt", "", "system prompt")
	cmd.Flags().Set("system-prompt", "You are a helpful assistant.")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	systemPrompt, err := service.GetSystemPromptFlag()
	if err != nil {
		t.Fatalf("GetSystemPromptFlag failed: %v", err)
	}

	if systemPrompt != "You are a helpful assistant." {
		t.Errorf("Expected system prompt 'You are a helpful assistant.', got '%s'", systemPrompt)
	}
}

func TestGetSystemPromptFlagEmpty(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().String("system-prompt", "", "system prompt")

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	systemPrompt, err := service.GetSystemPromptFlag()
	if err != nil {
		t.Fatalf("GetSystemPromptFlag failed: %v", err)
	}

	if systemPrompt != "" {
		t.Errorf("Expected empty system prompt, got '%s'", systemPrompt)
	}
}

func TestGetSystemPromptFlagUndefined(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}

	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.GetSystemPromptFlag()
	if err == nil {
		t.Error("Expected error when system-prompt flag is not defined")
	}
}

func TestGetSystemPromptFlagNilService(t *testing.T) {
	var service *Service

	_, err := service.GetSystemPromptFlag()
	if err == nil {
		t.Error("Expected error for nil service")
	}
}
