// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

// Package cmd implements the command-line interface for meow.
// It defines all CLI commands using the Cobra framework and manages.
// application lifecycle (initialization and cleanup).
package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/app"
)

// Execute runs the root command.
func Execute() error {
	// Load Starlark commands before executing
	if err := loadStarlarkCommands(); err != nil {
		// Log error but don't fail - Starlark scripts are optional
		if _, printErr := fmt.Fprintf(rootCmd.ErrOrStderr(), "Warning: Failed to load Starlark scripts: %v\n", err); printErr != nil {
			return fmt.Errorf("failed to write warning: %w", printErr)
		}
	}

	err := rootCmd.Execute()
	if err != nil {
		// Suppress cancellation — user already saw "Cancelling…" in the TUI.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("failed to execute command: %w", err)
		}
		if _, printErr := fmt.Fprintf(rootCmd.ErrOrStderr(), "Error: %v\n", err); printErr != nil {
			return fmt.Errorf("failed to write error: %w", printErr)
		}
		return fmt.Errorf("failed to execute command: %w", err)
	}
	return nil
}

// loadStarlarkCommands loads user-defined commands from Starlark scripts.
// This is called once during application startup, before Execute().
func loadStarlarkCommands() error {
	// Create a container for loading Starlark scripts
	// We don't use rootCmd here to avoid flag parsing issues
	container, workspaceRoot, err := app.NewAppContainerForStarlark()
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	defer container.ShutdownService.Shutdown()

	// Load Starlark commands with workspace root
	if err := BuildStarlarkCommands(container, workspaceRoot); err != nil {
		return fmt.Errorf("failed to build Starlark commands: %w", err)
	}

	return nil
}

var rootCmd = &cobra.Command{
	Use:           "meow",
	Short:         "'meow' — your fast, script-friendly AI companion",
	SilenceUsage:  true, // never print usage on RunE errors
	SilenceErrors: true, // we print errors ourselves in Execute()
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		if cmd == nil {
			return fmt.Errorf("command cannot be nil")
		}

		if cmd.Name() == "version" || cmd.Name() == "help" || cmd.Name() == commandMeow || cmd.Name() == "completion" || cmd.Name() == commandInit || isAuthCommand(cmd) {
			return nil
		}

		_, err := app.NewAppContainer(cmd)
		if err != nil {
			return fmt.Errorf("failed to initialize app: %w", err)
		}

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, _ []string) error {
		if cmd == nil {
			return fmt.Errorf("command cannot be nil")
		}

		if cmd.Name() == "version" || cmd.Name() == "help" || cmd.Name() == commandMeow || cmd.Name() == "completion" || cmd.Name() == commandInit || isAuthCommand(cmd) {
			return nil
		}

		ctx := cmd.Context()
		if ctx == nil {
			return nil
		}

		appContainer, ok := ctx.Value(app.AppContainerKey).(*app.Container)
		if !ok || appContainer == nil {
			return nil
		}

		appContainer.ShutdownService.Shutdown()
		return nil
	},
}

const (
	commandInit = "init"
	commandMeow = "meow"
)

// isAuthCommand returns true if cmd or any of its ancestors is the "auth" command.
// This handles subcommands like "auth copilot" where cmd.Name() == "copilot".
func isAuthCommand(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == commandAuth {
			return true
		}
	}
	return false
}

func init() {
	rootCmd.PersistentFlags().String("workspace", "", "workspace root directory (overrides auto-detection)")
	rootCmd.PersistentFlags().Bool("no-cache", false, "disable LLM response caching")
	rootCmd.PersistentFlags().Bool("update-cache", false, "force cache refresh by making fresh requests and updating cache entries")
	rootCmd.PersistentFlags().Bool("no-tui", false, "disable the interactive TUI (same behaviour as running in a non-TTY environment)")
}
