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
	"fmt"

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
	RunE: func(cmd *cobra.Command, args []string) error {
		if cmd == nil {
			return fmt.Errorf("command is nil")
		}

		ctx := cmd.Context()

		container, ok := ctx.Value(app.AppContainerKey).(*app.Container)
		if !ok || container == nil {
			return fmt.Errorf("application not initialized")
		}

		flow, err := container.CreateCommitFlow()
		if err != nil {
			return fmt.Errorf("failed to create commit flow: %w", err)
		}

		orchestrator, err := executor.NewOrchestrator(container.OutputService, container.TraceLogger)
		if err != nil {
			return fmt.Errorf("failed to create flow runner: %w", err)
		}

		silent, err := container.CommandService.GetSilentFlag()
		if err != nil {
			return fmt.Errorf("failed to get command silent flag: %w", err)
		}

		return orchestrator.Execute(ctx, "CommitFlow", flow, silent)
	},
}

func init() {
	rootCmd.AddCommand(commitCmd)
	commitCmd.Flags().StringP("intent", "i", "", "Developer intent for the commit (can also be provided via stdin)")
	commitCmd.Flags().StringP("target-branch", "t", "", "Target branch to compare against for squash commit (enables squash mode)")
}
