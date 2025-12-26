// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/app"
	"github.com/retran/meowg1k/pkg/executor"
)

var doCmd = &cobra.Command{
	Use:   "do <task>",
	Short: "Run a multi-step agent to complete a task",
	Long: `Run a multi-step agent workflow (research → plan → execute → verify).

The agent can call tools for workspace operations, search, git, and more,
based on step-specific configuration.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runFlowCommand(cmd, "AgentFlow", func(container *app.Container) (executor.Flow, error) {
			return container.CreateAgentFlow()
		})
	},
}

func init() {
	rootCmd.AddCommand(doCmd)
	doCmd.Flags().String("profile", "", "Profile to use for agent defaults (overrides config)")
	doCmd.Flags().String("system-prompt", "", "System prompt for agent defaults (overrides config)")
	doCmd.Flags().StringSliceP("snapshots", "s", []string{"_workdir_", "_stage_", "_head_"}, "Snapshots to search (workdir, stage, head)")
	doCmd.Flags().IntP("top-k", "k", 0, "Number of top results to retrieve (0 = use config default)")
	doCmd.Flags().Float32("min-score", 0.0, "Minimum similarity score (0.0 = use config default)")
}
