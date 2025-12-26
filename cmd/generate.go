// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/app"
	"github.com/retran/meowg1k/pkg/executor"
)

var generateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"gen", "g"},
	Short:   "Generate any content based on input — code, text, or docs",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runFlowCommand(cmd, "GenerateFlow", func(container *app.Container) (executor.Flow, error) {
			return container.CreateGenerateFlow()
		})
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringP("task", "t", "", "Run a predefined task from config")
	generateCmd.Flags().StringP("system-prompt", "s", "", "System prompt for generation")
	generateCmd.Flags().StringP("user-prompt", "u", "", "User prompt for generation. Can be combined with stdin")
}
