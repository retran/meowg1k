// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestWriteCmd(t *testing.T) {
	t.Run("Command properties", func(t *testing.T) {
		if writeCmd.Use != "write" {
			t.Errorf("Expected Use to be 'write', got '%s'", writeCmd.Use)
		}

		expectedAliases := []string{"gen", "g"}
		if len(writeCmd.Aliases) != len(expectedAliases) {
			t.Errorf("Expected %d aliases, got %d", len(expectedAliases), len(writeCmd.Aliases))
		}

		for i, alias := range expectedAliases {
			if i >= len(writeCmd.Aliases) || writeCmd.Aliases[i] != alias {
				t.Errorf("Expected alias '%s' at position %d", alias, i)
			}
		}

		if writeCmd.Short == "" {
			t.Error("Expected Short description to be non-empty")
		}

		if !strings.Contains(writeCmd.Short, "Generate") {
			t.Error("Short description should contain 'Generate'")
		}
	})

	t.Run("Command flags", func(t *testing.T) {
		taskFlag := writeCmd.Flags().Lookup("task")
		if taskFlag == nil {
			t.Fatal("Expected 'task' flag to be defined")
		}
		if taskFlag.Shorthand != "t" {
			t.Errorf("Expected task flag shorthand to be 't', got '%s'", taskFlag.Shorthand)
		}

		systemPromptFlag := writeCmd.Flags().Lookup("system-prompt")
		if systemPromptFlag == nil {
			t.Fatal("Expected 'system-prompt' flag to be defined")
		}
		if systemPromptFlag.Shorthand != "s" {
			t.Errorf("Expected system-prompt flag shorthand to be 's', got '%s'", systemPromptFlag.Shorthand)
		}

		userPromptFlag := writeCmd.Flags().Lookup("user-prompt")
		if userPromptFlag == nil {
			t.Fatal("Expected 'user-prompt' flag to be defined")
		}
		if userPromptFlag.Shorthand != "u" {
			t.Errorf("Expected user-prompt flag shorthand to be 'u', got '%s'", userPromptFlag.Shorthand)
		}
	})

	t.Run("RunE function exists", func(t *testing.T) {
		if writeCmd.RunE == nil {
			t.Fatal("Expected RunE function to be defined")
		}
	})
}

func TestWriteCmdRunE(t *testing.T) {
	t.Run("Run without app container", func(t *testing.T) {
		ctx := context.Background()

		testCmd := &cobra.Command{Use: "test-write"}
		testCmd.SetContext(ctx)

		err := writeCmd.RunE(testCmd, []string{})

		if err == nil {
			t.Fatal("Expected error when app container is not initialized")
		}

		if !strings.Contains(err.Error(), "application not initialized") {
			t.Errorf("Expected 'application not initialized' error, got: %v", err)
		}
	})

	t.Run("Run with nil context", func(t *testing.T) {
		testCmd := &cobra.Command{Use: "test-write"}
		testCmd.SetContext(context.Background())

		err := writeCmd.RunE(testCmd, []string{})

		if err == nil {
			t.Fatal("Expected error when context is nil")
		}

		if !strings.Contains(err.Error(), "application not initialized") {
			t.Errorf("Expected 'application not initialized' error, got: %v", err)
		}
	})

	t.Run("Flag setting and retrieval", func(t *testing.T) {
		testCases := []struct {
			flagName  string
			flagValue string
		}{
			{"task", "test-task"},
			{"system-prompt", "You are a helpful assistant"},
			{"user-prompt", "Generate a test response"},
		}

		for _, tc := range testCases {
			t.Run("Flag_"+tc.flagName, func(t *testing.T) {
				err := writeCmd.Flags().Set(tc.flagName, tc.flagValue)
				if err != nil {
					t.Errorf("Failed to set flag %s: %v", tc.flagName, err)
				}

				value, err := writeCmd.Flags().GetString(tc.flagName)
				if err != nil {
					t.Errorf("Failed to get flag %s: %v", tc.flagName, err)
				}

				if value != tc.flagValue {
					t.Errorf("Expected flag %s to be '%s', got '%s'", tc.flagName, tc.flagValue, value)
				}
			})
		}
	})
}

func TestWriteCmdIntegration(t *testing.T) {
	t.Run("Command is added to root", func(t *testing.T) {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == "write" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Write command should be added to root command")
		}
	})

	t.Run("Aliases work", func(t *testing.T) {
		aliases := writeCmd.Aliases
		expectedAliases := []string{"gen", "g"}

		if len(aliases) != len(expectedAliases) {
			t.Errorf("Expected %d aliases, got %d", len(expectedAliases), len(aliases))
		}

		for i, expected := range expectedAliases {
			if i >= len(aliases) || aliases[i] != expected {
				t.Errorf("Expected alias '%s' at position %d, got '%s'", expected, i, aliases[i])
			}
		}
	})

	t.Run("Help text formatting", func(t *testing.T) {
		if writeCmd.Short == "" {
			t.Error("Short description should not be empty")
		}

		if len(writeCmd.Short) > 80 {
			t.Error("Short description should be concise (under 80 characters)")
		}

		requiredTerms := []string{"write", "content"}
		shortLower := strings.ToLower(writeCmd.Short)

		for _, term := range requiredTerms {
			if !strings.Contains(shortLower, term) {
				t.Errorf("Short description should contain '%s'", term)
			}
		}
	})
}

func TestWriteCmdFlags(t *testing.T) {
	t.Run("Task flag properties", func(t *testing.T) {
		flag := writeCmd.Flags().Lookup("task")
		if flag == nil {
			t.Fatal("Task flag should exist")
		}

		if flag.DefValue != "" {
			t.Errorf("Task flag default should be empty, got '%s'", flag.DefValue)
		}

		if !strings.Contains(flag.Usage, "task") {
			t.Error("Task flag usage should mention 'task'")
		}
	})

	t.Run("System prompt flag properties", func(t *testing.T) {
		flag := writeCmd.Flags().Lookup("system-prompt")
		if flag == nil {
			t.Fatal("System prompt flag should exist")
		}

		if flag.DefValue != "" {
			t.Errorf("System prompt flag default should be empty, got '%s'", flag.DefValue)
		}

		if !strings.Contains(strings.ToLower(flag.Usage), "system") || !strings.Contains(strings.ToLower(flag.Usage), "prompt") {
			t.Errorf("System prompt flag usage should mention 'system' and 'prompt', got: %s", flag.Usage)
		}
	})

	t.Run("User prompt flag properties", func(t *testing.T) {
		flag := writeCmd.Flags().Lookup("user-prompt")
		if flag == nil {
			t.Fatal("User prompt flag should exist")
		}

		if flag.DefValue != "" {
			t.Errorf("User prompt flag default should be empty, got '%s'", flag.DefValue)
		}

		if !strings.Contains(strings.ToLower(flag.Usage), "user") || !strings.Contains(strings.ToLower(flag.Usage), "prompt") {
			t.Errorf("User prompt flag usage should mention 'user' and 'prompt', got: %s", flag.Usage)
		}

		if !strings.Contains(strings.ToLower(flag.Usage), "stdin") {
			t.Errorf("User prompt flag usage should mention 'stdin', got: %s", flag.Usage)
		}
	})

	t.Run("Flag combinations", func(t *testing.T) {
		flagSettings := map[string]string{
			"task":          "comprehensive-test",
			"system-prompt": "You are an expert software tester",
			"user-prompt":   "Create comprehensive test cases",
		}

		for name, value := range flagSettings {
			err := writeCmd.Flags().Set(name, value)
			if err != nil {
				t.Errorf("Failed to set flag %s to '%s': %v", name, value, err)
			}
		}

		for name, expectedValue := range flagSettings {
			actualValue, err := writeCmd.Flags().GetString(name)
			if err != nil {
				t.Errorf("Failed to get flag %s: %v", name, err)
			}
			if actualValue != expectedValue {
				t.Errorf("Flag %s: expected '%s', got '%s'", name, expectedValue, actualValue)
			}
		}
	})
}

func TestWriteCmdEdgeCases(t *testing.T) {
	t.Run("Empty flag values", func(t *testing.T) {
		flags := []string{"task", "system-prompt", "user-prompt"}

		for _, flagName := range flags {
			err := writeCmd.Flags().Set(flagName, "")
			if err != nil {
				t.Errorf("Should be able to set flag %s to empty value: %v", flagName, err)
			}

			value, err := writeCmd.Flags().GetString(flagName)
			if err != nil {
				t.Errorf("Failed to get flag %s after setting to empty: %v", flagName, err)
			}

			if value != "" {
				t.Errorf("Flag %s should be empty after setting to empty, got '%s'", flagName, value)
			}
		}
	})

	t.Run("Special characters in flag values", func(t *testing.T) {
		specialValues := map[string]string{
			"task":          "task-with-special_chars@123",
			"system-prompt": "System prompt with \"quotes\" and 'apostrophes'",
			"user-prompt":   "User prompt with unicode: αβγδε and emoji: 🚀",
		}

		for flagName, specialValue := range specialValues {
			err := writeCmd.Flags().Set(flagName, specialValue)
			if err != nil {
				t.Errorf("Should be able to set flag %s to special value: %v", flagName, err)
			}

			value, err := writeCmd.Flags().GetString(flagName)
			if err != nil {
				t.Errorf("Failed to get flag %s with special characters: %v", flagName, err)
			}

			if value != specialValue {
				t.Errorf("Flag %s with special chars: expected '%s', got '%s'", flagName, specialValue, value)
			}
		}
	})

	t.Run("Very long flag values", func(t *testing.T) {
		longValue := strings.Repeat("This is a very long string for testing purposes. ", 100)

		flags := []string{"task", "system-prompt", "user-prompt"}

		for _, flagName := range flags {
			err := writeCmd.Flags().Set(flagName, longValue)
			if err != nil {
				t.Errorf("Should be able to set flag %s to long value: %v", flagName, err)
			}

			value, err := writeCmd.Flags().GetString(flagName)
			if err != nil {
				t.Errorf("Failed to get long flag %s: %v", flagName, err)
			}

			if value != longValue {
				t.Errorf("Flag %s: long value not preserved correctly", flagName)
			}
		}
	})
}
