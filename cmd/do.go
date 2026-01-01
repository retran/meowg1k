// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/app"
	"github.com/retran/meowg1k/pkg/executor"
)

func newDoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "do [goal]",
		Short: "Run an autonomous agent flow to accomplish a goal",
		Long: `The 'do' command initiates a multi-agent workflow (Discover, Plan, Execute, Verify)
to accomplish a complex task. It can read the goal from arguments or standard input.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFlowCommand(cmd, "DoFlow", func(container *app.Container) (executor.Flow, error) {
				return container.CreateDoFlow()
			})
		},
	}

	cmd.Flags().Bool("dry-run", false, "Simulate changes without writing to disk")

	return cmd
}

func init() {
	rootCmd.AddCommand(newDoCmd())
}
