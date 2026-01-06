// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/app"
	"github.com/retran/meowg1k/pkg/executor"
)

var draftPrCmd = &cobra.Command{
	Use:   "pr",
	Short: "Draft a pull request description based on a diff",
	Long: `Draft a Pull Request description based on the diff between the current branch and a base branch.

The tool analyzes all files changed between your current branch and the specified base branch,
then generates a comprehensive PR title and description.

You must specify the base branch to compare against:

   meow draft pr --base main
   meow draft pr --base dev

You can provide your intent or context in two ways:

1. Via command line flag:
   meow draft pr --base main --intent "Implement new user authentication feature"

2. Via stdin (pipe):
   echo "Add payment integration with Stripe" | meow draft pr --base main

	The intent will be included in the prompt to help write a more accurate PR description.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		return runFlowCommand(cmd, "PrFlow", func(container *app.Container) (executor.Flow, error) {
			return container.CreatePrFlow()
		})
	},
}

func init() {
	draftCmd.AddCommand(draftPrCmd)
	draftPrCmd.Flags().StringP("intent", "i", "", "Developer intent for the Pull Request (can also be provided via stdin)")
	draftPrCmd.Flags().String("diff", "branch", "Diff source: branch (staged is not supported for PR drafts)")
	draftPrCmd.Flags().StringP("base", "b", "", "Base branch to compare against when diff=branch")
}
