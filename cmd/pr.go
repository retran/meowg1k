/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/app"
)

var prCmd = &cobra.Command{
	Use:     "pr",
	Aliases: []string{"p"},
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
		if cmd == nil {
			return ErrCommandIsNil
		}

		ctx := cmd.Context()

		appContainer, ok := ctx.Value(app.AppContainerKey).(*app.Container)
		if !ok || appContainer == nil {
			return ErrAppNotInitialized
		}

		flow, err := appContainer.CreatePRFlow()
		if err != nil {
			return err
		}

		runner, err := app.NewFlowRunner(appContainer)
		if err != nil {
			return err
		}
		return runner.RunFlow(ctx, "GeneratePR", flow)
	},
}

func init() {
	rootCmd.AddCommand(prCmd)
	prCmd.Flags().StringP("intent", "i", "", "Developer intent for the PR (can also be provided via stdin)")
	prCmd.Flags().StringP("base", "b", "", "Base branch to compare against (required)")
	// This should never fail as we're just marking a flag as required
	// If it does fail, it's a programming error in the command setup
	_ = prCmd.MarkFlagRequired("base")
}
