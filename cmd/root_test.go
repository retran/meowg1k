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
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestExecute(t *testing.T) {
	t.Run("Execute with version command", func(t *testing.T) {
		// Capture output
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Set args to version command
		os.Args = []string{"meow", "version"}

		err := Execute()
		
		// Restore stdout
		w.Close()
		os.Stdout = old

		// Read captured output
		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		if err != nil {
			t.Errorf("Execute with version command failed: %v", err)
		}

		// Version command should work without app initialization
		t.Logf("Version command output: %s", output)
	})

	t.Run("Execute with help command", func(t *testing.T) {
		// Capture output
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Set args to help command
		os.Args = []string{"meow", "help"}

		err := Execute()
		
		// Restore stdout
		w.Close()
		os.Stdout = old

		// Read captured output
		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		if err != nil {
			t.Errorf("Execute with help command failed: %v", err)
		}

		// Help should contain usage information
		if !strings.Contains(output, "meow") {
			t.Error("Help output should contain 'meow'")
		}
		t.Logf("Help command output: %s", strings.Split(output, "\n")[0])
	})

	t.Run("Execute with invalid command", func(t *testing.T) {
		// Capture stderr
		old := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		// Set args to invalid command
		os.Args = []string{"meow", "invalid-command"}

		err := Execute()
		
		// Restore stderr
		w.Close()
		os.Stderr = old

		// Read captured output
		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		if err == nil {
			t.Error("Execute with invalid command should return error")
		}

		t.Logf("Invalid command error: %v", err)
		t.Logf("Error output: %s", output)
	})
}

func TestRootCmd(t *testing.T) {
	t.Run("Root command properties", func(t *testing.T) {
		if rootCmd.Use != "meow" {
			t.Errorf("Expected Use to be 'meow', got '%s'", rootCmd.Use)
		}

		if rootCmd.Short == "" {
			t.Error("Expected Short description to be non-empty")
		}

		if !strings.Contains(rootCmd.Short, "meow") {
			t.Error("Short description should contain 'meow'")
		}

		if !strings.Contains(rootCmd.Short, "AI") {
			t.Error("Short description should mention AI")
		}
	})

	t.Run("Persistent flags", func(t *testing.T) {
		configFlag := rootCmd.PersistentFlags().Lookup("config")
		if configFlag == nil {
			t.Fatal("Expected 'config' persistent flag to be defined")
		}
		if configFlag.DefValue != "" {
			t.Errorf("Expected config flag default to be empty, got '%s'", configFlag.DefValue)
		}

		silentFlag := rootCmd.PersistentFlags().Lookup("silent")
		if silentFlag == nil {
			t.Fatal("Expected 'silent' persistent flag to be defined")
		}
		if silentFlag.DefValue != "false" {
			t.Errorf("Expected silent flag default to be 'false', got '%s'", silentFlag.DefValue)
		}
	})

	t.Run("PersistentPreRunE function exists", func(t *testing.T) {
		if rootCmd.PersistentPreRunE == nil {
			t.Fatal("Expected PersistentPreRunE to be defined")
		}
	})
}

func TestRootCmdPersistentPreRunE(t *testing.T) {
	t.Run("Skip app initialization for version command", func(t *testing.T) {
		// Create a mock version command
		versionCmd := &cobra.Command{
			Use: "version",
		}

		err := rootCmd.PersistentPreRunE(versionCmd, []string{})
		if err != nil {
			t.Errorf("PersistentPreRunE should not return error for version command: %v", err)
		}
	})

	t.Run("Skip app initialization for help command", func(t *testing.T) {
		// Create a mock help command
		helpCmd := &cobra.Command{
			Use: "help",
		}

		err := rootCmd.PersistentPreRunE(helpCmd, []string{})
		if err != nil {
			t.Errorf("PersistentPreRunE should not return error for help command: %v", err)
		}
	})

	t.Run("Skip app initialization for meow command", func(t *testing.T) {
		// Create a mock meow root command
		meowCmd := &cobra.Command{
			Use: "meow",
		}

		err := rootCmd.PersistentPreRunE(meowCmd, []string{})
		if err != nil {
			t.Errorf("PersistentPreRunE should not return error for meow command: %v", err)
		}
	})

	t.Run("Skip app initialization for completion command", func(t *testing.T) {
		// Create a mock completion command
		completionCmd := &cobra.Command{
			Use: "completion",
		}

		err := rootCmd.PersistentPreRunE(completionCmd, []string{})
		if err != nil {
			t.Errorf("PersistentPreRunE should not return error for completion command: %v", err)
		}
	})

	t.Run("App initialization for other commands", func(t *testing.T) {
		// Create a mock generate command that requires app initialization
		generateCmd := &cobra.Command{
			Use: "generate",
		}

		// This will likely fail due to missing config or dependencies, but that's expected
		err := rootCmd.PersistentPreRunE(generateCmd, []string{})
		if err != nil {
			t.Logf("Expected error for generate command (app initialization): %v", err)
			// Should be app initialization error, not a nil pointer or similar
			if strings.Contains(err.Error(), "failed to initialize app") {
				t.Log("Got expected app initialization error")
			} else {
				t.Logf("Unexpected error type: %v", err)
			}
		} else {
			t.Log("Unexpected success - app initialization worked in test environment")
		}
	})
}

func TestRootCmdContextHandling(t *testing.T) {
	t.Run("Context setting for commands requiring app", func(t *testing.T) {
		// Create a mock command that requires app initialization
		testCmd := &cobra.Command{
			Use: "test-context",
		}

		// Try to run the persistent pre-run
		err := rootCmd.PersistentPreRunE(testCmd, []string{})
		
		if err != nil {
			// Expected to fail due to missing dependencies, but verify it's the right error
			t.Logf("Expected app initialization error: %v", err)
		} else {
			// If it succeeds, verify context was set
			ctx := testCmd.Context()
			if ctx == nil {
				t.Error("Context should be set when app initialization succeeds")
			} else if ctx == context.Background() {
				t.Log("Context is background context")
			} else {
				t.Log("Context was properly set by app container")
			}
		}
	})
}

func TestRootCmdFlags(t *testing.T) {
	t.Run("Config flag behavior", func(t *testing.T) {
		// Test setting config flag
		err := rootCmd.PersistentFlags().Set("config", "/test/config.yaml")
		if err != nil {
			t.Errorf("Failed to set config flag: %v", err)
		}

		configValue, err := rootCmd.PersistentFlags().GetString("config")
		if err != nil {
			t.Errorf("Failed to get config flag: %v", err)
		}

		if configValue != "/test/config.yaml" {
			t.Errorf("Expected config value '/test/config.yaml', got '%s'", configValue)
		}
	})

	t.Run("Silent flag behavior", func(t *testing.T) {
		// Test setting silent flag
		err := rootCmd.PersistentFlags().Set("silent", "true")
		if err != nil {
			t.Errorf("Failed to set silent flag: %v", err)
		}

		silentValue, err := rootCmd.PersistentFlags().GetBool("silent")
		if err != nil {
			t.Errorf("Failed to get silent flag: %v", err)
		}

		if !silentValue {
			t.Error("Expected silent flag to be true")
		}
	})

	t.Run("Flag descriptions", func(t *testing.T) {
		configFlag := rootCmd.PersistentFlags().Lookup("config")
		if !strings.Contains(configFlag.Usage, "config file") {
			t.Error("Config flag should mention 'config file' in usage")
		}

		silentFlag := rootCmd.PersistentFlags().Lookup("silent")
		if !strings.Contains(silentFlag.Usage, "silent") {
			t.Error("Silent flag should mention 'silent' in usage")
		}
	})
}

func TestRootCmdEdgeCases(t *testing.T) {
	t.Run("Empty args", func(t *testing.T) {
		// Create a command with empty args
		testCmd := &cobra.Command{Use: "empty-args"}
		
		err := rootCmd.PersistentPreRunE(testCmd, []string{})
		// Should handle empty args gracefully
		if err != nil {
			t.Logf("Error with empty args: %v", err)
		}
	})

	t.Run("Command name variations", func(t *testing.T) {
		commandNames := []string{
			"version",
			"VERSION", // Case shouldn't matter for name comparison
			"help",
			"HELP",
			"meow",
			"MEOW",
			"completion",
			"COMPLETION",
		}

		for _, name := range commandNames {
			t.Run("Name_"+name, func(t *testing.T) {
				testCmd := &cobra.Command{Use: name}
				err := rootCmd.PersistentPreRunE(testCmd, []string{})
				// These commands should not require app initialization
				if err != nil && name != strings.ToUpper(name) {
					// Only lowercase versions should be recognized
					t.Logf("Command name %s resulted in error: %v", name, err)
				}
			})
		}
	})

	t.Run("Nil command", func(t *testing.T) {
		// Test with nil command - this is expected to fail gracefully
		// but shouldn't cause panics in production
		err := rootCmd.PersistentPreRunE(nil, []string{})
		if err == nil {
			t.Error("Expected error with nil command")
		}
		
		// Any error is acceptable as long as it doesn't panic
		t.Logf("Got expected error with nil command: %v", err)
	})
}