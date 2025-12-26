// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"runtime"

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
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		container, ok := ctx.Value(app.AppContainerKey).(*app.Container)
		if !ok || container == nil {
			return fmt.Errorf("application not initialized")
		}

		flow, err := container.CreatePullRequestFlow()
		if err != nil {
			return fmt.Errorf("failed to create pull request flow: %w", err)
		}

		concurrency := runtime.NumCPU() * 2
		orchestrator, err := executor.NewOrchestrator(container.OutputService, container.TraceLogger, concurrency)
		if err != nil {
			return fmt.Errorf("failed to create flow runner: %w", err)
		}

		silent, err := container.CommandService.GetSilentFlag()
		if err != nil {
			return fmt.Errorf("failed to get command silent flag: %w", err)
		}

		return orchestrator.Execute(ctx, "PullRequestFlow", flow, silent)
	},
}

func init() {
	rootCmd.AddCommand(prCmd)
	prCmd.Flags().StringP("intent", "i", "", "Developer intent for the Pull Request (can also be provided via stdin)")
	prCmd.Flags().StringP("base", "b", "", "Base branch to compare against (required)")
	_ = prCmd.MarkFlagRequired("base") //nolint:errcheck // Flag definition errors are caught at startup
}
