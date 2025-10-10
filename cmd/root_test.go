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
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		os.Args = []string{"meow", "version"}

		err := Execute()

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		if err != nil {
			t.Errorf("Execute with version command failed: %v", err)
		}

		t.Logf("Version command output: %s", output)
	})

	t.Run("Execute with help command", func(t *testing.T) {
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		os.Args = []string{"meow", "help"}

		err := Execute()

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		if err != nil {
			t.Errorf("Execute with help command failed: %v", err)
		}

		if !strings.Contains(output, "meow") {
			t.Error("Help output should contain 'meow'")
		}
		t.Logf("Help command output: %s", strings.Split(output, "\n")[0])
	})

	t.Run("Execute with invalid command", func(t *testing.T) {
		old := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		os.Args = []string{"meow", "invalid-command"}

		err := Execute()

		w.Close()
		os.Stderr = old

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
		localVersionCmd := &cobra.Command{
			Use: "version",
		}

		err := rootCmd.PersistentPreRunE(localVersionCmd, []string{})
		if err != nil {
			t.Errorf("PersistentPreRunE should not return error for version command: %v", err)
		}
	})

	t.Run("Skip app initialization for help command", func(t *testing.T) {
		helpCmd := &cobra.Command{
			Use: "help",
		}

		err := rootCmd.PersistentPreRunE(helpCmd, []string{})
		if err != nil {
			t.Errorf("PersistentPreRunE should not return error for help command: %v", err)
		}
	})

	t.Run("Skip app initialization for meow command", func(t *testing.T) {
		meowCmd := &cobra.Command{
			Use: "meow",
		}

		err := rootCmd.PersistentPreRunE(meowCmd, []string{})
		if err != nil {
			t.Errorf("PersistentPreRunE should not return error for meow command: %v", err)
		}
	})

	t.Run("Skip app initialization for completion command", func(t *testing.T) {
		completionCmd := &cobra.Command{
			Use: "completion",
		}

		err := rootCmd.PersistentPreRunE(completionCmd, []string{})
		if err != nil {
			t.Errorf("PersistentPreRunE should not return error for completion command: %v", err)
		}
	})

	t.Run("App initialization for other commands", func(t *testing.T) {
		generateCmd := &cobra.Command{
			Use: "generate",
		}

		err := rootCmd.PersistentPreRunE(generateCmd, []string{})
		if err != nil {
			t.Logf("Expected error for generate command (app initialization): %v", err)
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
		testCmd := &cobra.Command{
			Use: "test-context",
		}

		err := rootCmd.PersistentPreRunE(testCmd, []string{})

		if err != nil {
			t.Logf("Expected app initialization error: %v", err)
		} else {
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
		testCmd := &cobra.Command{Use: "empty-args"}

		err := rootCmd.PersistentPreRunE(testCmd, []string{})
		if err != nil {
			t.Logf("Error with empty args: %v", err)
		}
	})

	t.Run("Command name variations", func(t *testing.T) {
		commandNames := []string{
			"version",
			"VERSION",
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
				if err != nil && name != strings.ToUpper(name) {
					t.Logf("Command name %s resulted in error: %v", name, err)
				}
			})
		}
	})

	t.Run("Nil command", func(t *testing.T) {
		err := rootCmd.PersistentPreRunE(nil, []string{})
		if err == nil {
			t.Error("Expected error with nil command")
		}

		t.Logf("Got expected error with nil command: %v", err)
	})
}

func TestRootCmdPersistentPostRunE(t *testing.T) {
	t.Run("Skip shutdown for version command", func(t *testing.T) {
		versionCmd := &cobra.Command{
			Use: "version",
		}

		err := rootCmd.PersistentPostRunE(versionCmd, []string{})
		if err != nil {
			t.Errorf("PersistentPostRunE should not return error for version command: %v", err)
		}
	})

	t.Run("Skip shutdown for help command", func(t *testing.T) {
		helpCmd := &cobra.Command{
			Use: "help",
		}

		err := rootCmd.PersistentPostRunE(helpCmd, []string{})
		if err != nil {
			t.Errorf("PersistentPostRunE should not return error for help command: %v", err)
		}
	})

	t.Run("Skip shutdown for meow command", func(t *testing.T) {
		meowCmd := &cobra.Command{
			Use: "meow",
		}

		err := rootCmd.PersistentPostRunE(meowCmd, []string{})
		if err != nil {
			t.Errorf("PersistentPostRunE should not return error for meow command: %v", err)
		}
	})

	t.Run("Skip shutdown for completion command", func(t *testing.T) {
		completionCmd := &cobra.Command{
			Use: "completion",
		}

		err := rootCmd.PersistentPostRunE(completionCmd, []string{})
		if err != nil {
			t.Errorf("PersistentPostRunE should not return error for completion command: %v", err)
		}
	})

	t.Run("Nil command", func(t *testing.T) {
		err := rootCmd.PersistentPostRunE(nil, []string{})
		if err == nil {
			t.Error("Expected error with nil command")
		}

		t.Logf("Got expected error with nil command: %v", err)
	})

	t.Run("Command without context", func(t *testing.T) {
		testCmd := &cobra.Command{
			Use: "test-no-context",
		}

		err := rootCmd.PersistentPostRunE(testCmd, []string{})
		if err != nil {
			t.Errorf("PersistentPostRunE should handle nil context gracefully: %v", err)
		}
	})

	t.Run("Command without app container in context", func(t *testing.T) {
		testCmd := &cobra.Command{
			Use: "test-no-container",
		}
		testCmd.SetContext(context.Background())

		err := rootCmd.PersistentPostRunE(testCmd, []string{})
		if err != nil {
			t.Errorf("PersistentPostRunE should handle missing app container gracefully: %v", err)
		}
	})
}
