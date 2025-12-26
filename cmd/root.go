// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package cmd implements the command-line interface for meow.
// It defines all CLI commands using the Cobra framework and manages
// application lifecycle (initialization and cleanup).
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/app"
)

func Execute() error {
	err := rootCmd.Execute()
	if err != nil {
		return fmt.Errorf("failed to execute root command: %w", err)
	}
	return nil
}

var rootCmd = &cobra.Command{
	Use:   "meow",
	Short: "'meow' — your fast, script-friendly AI companion",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd == nil {
			return fmt.Errorf("command cannot be nil")
		}

		if cmd.Name() == "version" || cmd.Name() == "help" || cmd.Name() == "meow" || cmd.Name() == "completion" || cmd.Name() == "init" {
			return nil
		}

		_, err := app.NewAppContainer(cmd)
		if err != nil {
			return fmt.Errorf("failed to initialize app: %w", err)
		}

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if cmd == nil {
			return fmt.Errorf("command cannot be nil")
		}

		if cmd.Name() == "version" || cmd.Name() == "help" || cmd.Name() == "meow" || cmd.Name() == "completion" || cmd.Name() == "init" {
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

func init() {
	rootCmd.PersistentFlags().String("config", "", "config file path (overrides project/user configs when specified)")
	rootCmd.PersistentFlags().String("workspace", "", "workspace root directory (overrides auto-detection)")
	rootCmd.PersistentFlags().Bool("silent", false, "silent mode - only output the result without progress indicators")
	rootCmd.PersistentFlags().Bool("no-cache", false, "disable LLM response caching")
	rootCmd.PersistentFlags().Bool("update-cache", false, "force cache refresh by making fresh requests and updating cache entries")
}
