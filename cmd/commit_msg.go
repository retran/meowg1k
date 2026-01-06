// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/app"
	"github.com/retran/meowg1k/pkg/executor"
)

var draftCommitCmd = &cobra.Command{
	Use:     "commit",
	Aliases: []string{"commit-msg"},
	Short:   "Draft a commit message based on a diff",
	Long: `Draft a commit message based on a diff.

By default, the tool analyzes staged files and generates a commit message.

For branch diff commits, set --diff branch and specify --base to compare against:

   meow draft commit --diff branch --base dev

You can provide your intent or context in two ways:

1. Via command line flag:
   meow draft commit --intent "Fix user authentication bug"

2. Via stdin (pipe):
   echo "Add new user registration feature" | meow draft commit

	The intent will be included in the prompt to help write a more accurate commit message.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		return runFlowCommand(cmd, "CommitMsgFlow", func(container *app.Container) (executor.Flow, error) {
			return container.CreateCommitMsgFlow()
		})
	},
}

func init() {
	draftCmd.AddCommand(draftCommitCmd)
	draftCommitCmd.Flags().StringP("intent", "i", "", "Developer intent for the commit message (can also be provided via stdin)")
	draftCommitCmd.Flags().String("diff", "staged", "Diff source: staged or branch")
	draftCommitCmd.Flags().StringP("base", "b", "", "Base branch to compare against when diff=branch")
}
