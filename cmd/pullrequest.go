// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/app"
	"github.com/retran/meowg1k/pkg/executor"
)

var prCmd = &cobra.Command{
	Use:     "pullrequest",
	Aliases: []string{"pr"},
	Short:   "Generate Pull Request description based on branch diff",
	Long: `Generate a Pull Request description based on the diff between the current branch and a base branch.

The tool analyzes all files changed between your current branch and the specified base branch,
then generates a comprehensive PR title and description.

You must specify the base branch to compare against:

   meow pr --base main
   meow pr --base dev

You can provide your intent or context in two ways:

1. Via command line flag:
   meow pr --base main --intent "Implement new user authentication feature"

2. Via stdin (pipe):
   echo "Add payment integration with Stripe" | meow pr --base main

The intent will be included in the prompt to help generate a more accurate PR description.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runFlowCommand(cmd, "PullRequestFlow", func(container *app.Container) (executor.Flow, error) {
			return container.CreatePullRequestFlow()
		})
	},
}

func init() {
	rootCmd.AddCommand(prCmd)
	prCmd.Flags().StringP("intent", "i", "", "Developer intent for the Pull Request (can also be provided via stdin)")
	prCmd.Flags().StringP("base", "b", "", "Base branch to compare against (required)")
	_ = prCmd.MarkFlagRequired("base") //nolint:errcheck // Flag definition errors are caught at startup
}
