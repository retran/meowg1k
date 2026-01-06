// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/app"
	"github.com/retran/meowg1k/pkg/executor"
)

var writeCmd = &cobra.Command{
	Use:     "write",
	Aliases: []string{"gen", "g"},
	Short:   "Generate any content based on input - code, text, or docs",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		return runFlowCommand(cmd, "WriteFlow", func(container *app.Container) (executor.Flow, error) {
			return container.CreateWriteFlow()
		})
	},
}

func init() {
	rootCmd.AddCommand(writeCmd)
	writeCmd.Flags().StringP("task", "t", "", "Run a predefined task from config")
	writeCmd.Flags().StringP("system-prompt", "s", "", "System prompt for generation")
	writeCmd.Flags().StringP("user-prompt", "u", "", "User prompt for generation. Can be combined with stdin")
}
