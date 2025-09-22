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

package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
)

func TestGenerateCommand(t *testing.T) {
	// Test that the generate command is properly configured
	if generateCmd == nil {
		t.Fatal("generateCmd should not be nil")
	}

	if generateCmd.Use != "generate" {
		t.Errorf("Expected Use to be 'generate', got %s", generateCmd.Use)
	}

	expectedAliases := []string{"gen", "g"}
	if len(generateCmd.Aliases) != len(expectedAliases) {
		t.Errorf("Expected %d aliases, got %d", len(expectedAliases), len(generateCmd.Aliases))
	}

	for i, alias := range expectedAliases {
		if i >= len(generateCmd.Aliases) || generateCmd.Aliases[i] != alias {
			t.Errorf("Expected alias %d to be %s, got %s", i, alias, generateCmd.Aliases[i])
		}
	}

	// Test that flags are set up correctly
	taskFlag := generateCmd.Flags().Lookup("task")
	if taskFlag == nil {
		t.Error("Expected 'task' flag to be defined")
	}

	userPromptFlag := generateCmd.Flags().Lookup("user-prompt")
	if userPromptFlag == nil {
		t.Error("Expected 'user-prompt' flag to be defined")
	}

	silentFlag := generateCmd.Flags().Lookup("silent")
	if silentFlag == nil {
		t.Error("Expected 'silent' flag to be defined")
	}
}

func TestRunGenerateWithMissingConfig(t *testing.T) {
	// Create a mock command
	cmd := &cobra.Command{}
	cmd.Flags().Bool("silent", false, "Silent mode")

	// Set context
	ctx := context.Background()
	cmd.SetContext(ctx)

	// Capture stderr
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)

	// This should fail because appConfig is not set
	err := runGenerate(cmd)
	if err == nil {
		t.Error("Expected error when appConfig is nil, got nil")
	}
}

func TestGenerateCommandInit(t *testing.T) {
	// Test that init function properly adds the command to root
	// Since init() is called automatically, we just verify the command exists
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "generate" {
			found = true
			break
		}
	}
	if !found {
		t.Error("generate command not found in root command")
	}
}

func TestGenerateCommandFlags(t *testing.T) {
	// Test flag parsing
	generateCmd.Flags().Set("task", "test-task")
	generateCmd.Flags().Set("user-prompt", "test prompt")
	generateCmd.Flags().Set("silent", "true")

	taskFlag, _ := generateCmd.Flags().GetString("task")
	if taskFlag != "test-task" {
		t.Errorf("Expected task flag to be 'test-task', got %s", taskFlag)
	}

	promptFlag, _ := generateCmd.Flags().GetString("user-prompt")
	if promptFlag != "test prompt" {
		t.Errorf("Expected user-prompt flag to be 'test prompt', got %s", promptFlag)
	}

	silentFlag, _ := generateCmd.Flags().GetBool("silent")
	if !silentFlag {
		t.Error("Expected silent flag to be true")
	}
}
