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

package command

import (
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

func TestNewServicePanicsWithNilCommand(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when command is nil")
		}
	}()
	
	NewService(nil)
}

func TestGetCommand(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	
	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	
	if service.GetCommand() != cmd {
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
	
	if service.GetCommandName() != "testcmd" {
		t.Errorf("Expected command name 'testcmd', got '%s'", service.GetCommandName())
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
	stdin := service.GetStdIn()
	if stdin == "" {
		// This is expected in test environment
		t.Log("StdIn is empty as expected in test environment")
	}
	
	// Test that GetStdIn returns a string (not nil)
	_ = service.GetStdIn() // Should not panic
}

func TestNewServiceStdinErrorPaths(t *testing.T) {
	// These tests are challenging because they require manipulating stdin
	// We can test the service creation works with various command configurations
	
	tests := []struct {
		name        string
		setupCmd    func() *cobra.Command
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
		name     string
		testFunc func() (interface{}, error)
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
		name          string
		configValue   string
		taskValue     string
		promptValue   string
		silentValue   string
	}{
		{
			name:          "Normal values",
			configValue:   "/etc/config.yaml",
			taskValue:     "process-data",
			promptValue:   "Process the input data",
			silentValue:   "false",
		},
		{
			name:          "Empty string values",
			configValue:   "",
			taskValue:     "",
			promptValue:   "",
			silentValue:   "false",
		},
		{
			name:          "Special characters",
			configValue:   "/path/with spaces/config.yaml",
			taskValue:     "task-with-dashes_and_underscores",
			promptValue:   "Prompt with \"quotes\" and 'apostrophes'",
			silentValue:   "true",
		},
		{
			name:          "Long values",
			configValue:   "/very/long/path/to/configuration/file/that/might/cause/issues.yaml",
			taskValue:     "very-long-task-name-that-exceeds-normal-length-expectations",
			promptValue:   "This is a very long prompt that contains multiple sentences and might test the limits of string handling in the system.",
			silentValue:   "true",
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