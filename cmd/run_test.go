// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunCmd(t *testing.T) {
	t.Run("Command properties", func(t *testing.T) {
	if doCmd.Use != "do <task>" {
		t.Errorf("Expected Use to be 'do <task>', got '%s'", doCmd.Use)
	}

	if doCmd.Short == "" {
		t.Error("Expected Short description to be non-empty")
	}

	if !strings.Contains(strings.ToLower(doCmd.Short), "agent") {
		t.Error("Short description should mention 'agent'")
	}
	})

	t.Run("Command flags", func(t *testing.T) {
		flagNames := []struct {
			name      string
			shorthand string
		}{
			{"profile", ""},
			{"system-prompt", ""},
			{"snapshots", "s"},
			{"top-k", "k"},
			{"min-score", ""},
		}

		for _, flag := range flagNames {
		lookup := doCmd.Flags().Lookup(flag.name)
			if lookup == nil {
				t.Fatalf("Expected '%s' flag to be defined", flag.name)
			}
			if flag.shorthand != "" && lookup.Shorthand != flag.shorthand {
				t.Errorf("Expected %s flag shorthand to be '%s', got '%s'", flag.name, flag.shorthand, lookup.Shorthand)
			}
		}
	})

	t.Run("RunE function exists", func(t *testing.T) {
	if doCmd.RunE == nil {
		t.Fatal("Expected RunE function to be defined")
	}
	})
}

func TestRunCmdRunE(t *testing.T) {
	t.Run("Run without app container", func(t *testing.T) {
		ctx := context.Background()

		testCmd := &cobra.Command{Use: "test-run"}
		testCmd.SetContext(ctx)

	err := doCmd.RunE(testCmd, []string{})

		if err == nil {
			t.Fatal("Expected error when app container is not initialized")
		}

		if !strings.Contains(err.Error(), "application not initialized") {
			t.Errorf("Expected 'application not initialized' error, got: %v", err)
		}
	})

	t.Run("Run with nil context", func(t *testing.T) {
		testCmd := &cobra.Command{Use: "test-run"}
		testCmd.SetContext(context.Background())

	err := doCmd.RunE(testCmd, []string{})

		if err == nil {
			t.Fatal("Expected error when context is nil")
		}

		if !strings.Contains(err.Error(), "application not initialized") {
			t.Errorf("Expected 'application not initialized' error, got: %v", err)
		}
	})
}

func TestRunCmdIntegration(t *testing.T) {
	t.Run("Command is added to root", func(t *testing.T) {
		found := false
		for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "do" {
			found = true
			break
		}
		}

		if !found {
			t.Error("Run command should be added to root command")
		}
	})

	t.Run("Help text formatting", func(t *testing.T) {
	if doCmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	if len(doCmd.Short) > 80 {
		t.Error("Short description should be concise (under 80 characters)")
	}

		requiredTerms := []string{"agent", "task"}
	shortLower := strings.ToLower(doCmd.Short)

		for _, term := range requiredTerms {
			if !strings.Contains(shortLower, term) {
				t.Errorf("Short description should contain '%s'", term)
			}
		}
	})
}

func TestRunCmdFlags(t *testing.T) {
	t.Run("Flag setting and retrieval", func(t *testing.T) {
	if err := doCmd.Flags().Set("profile", "default"); err != nil {
		t.Fatalf("Failed to set profile flag: %v", err)
	}
	if value, err := doCmd.Flags().GetString("profile"); err != nil {
		t.Fatalf("Failed to get profile flag: %v", err)
	} else if value != "default" {
		t.Errorf("Expected profile flag to be 'default', got '%s'", value)
	}

	if err := doCmd.Flags().Set("snapshots", "_workdir_"); err != nil {
		t.Fatalf("Failed to set snapshots flag: %v", err)
	}
	if value, err := doCmd.Flags().GetStringSlice("snapshots"); err != nil {
		t.Fatalf("Failed to get snapshots flag: %v", err)
	} else if len(value) != 1 || value[0] != "_workdir_" {
		t.Errorf("Expected snapshots flag to be [_workdir_], got %v", value)
	}
	})
}
