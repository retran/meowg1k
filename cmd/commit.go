// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/app"
	"github.com/retran/meowg1k/pkg/executor"
)

var commitCmd = &cobra.Command{
	Use:     "commit",
	Aliases: []string{"c"},
	Short:   "Generate commit message based on staged changes or branch diff",
	Long: `Generate a commit message based on staged changes or branch diff.

By default, the tool analyzes staged files and generates a commit message.

For squash commits, specify the target branch to generate a commit message
based on the diff between the current branch and the target branch:

   meow commit --target-branch dev

You can provide your intent or context in two ways:

1. Via command line flag:
   meow commit --intent "Fix user authentication bug"

2. Via stdin (pipe):
   echo "Add new user registration feature" | meow commit

The intent will be included in the prompt to help generate a more accurate commit message.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runFlowCommand(cmd, "CommitFlow", func(container *app.Container) (executor.Flow, error) {
			return container.CreateCommitFlow()
		})
	},
}

func init() {
	rootCmd.AddCommand(commitCmd)
	commitCmd.Flags().StringP("intent", "i", "", "Developer intent for the commit (can also be provided via stdin)")
	commitCmd.Flags().StringP("target-branch", "t", "", "Target branch to compare against for squash commit (enables squash mode)")
}
