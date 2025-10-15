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
	"runtime"

	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/app"
	"github.com/retran/meowg1k/pkg/executor"
)

var askCmd = &cobra.Command{
	Use:     "ask <question>",
	Aliases: []string{"a"},
	Short:   "Ask a question about your codebase using RAG",
	Long: `Ask a question about your codebase and get an AI-generated answer.

This command uses Retrieval-Augmented Generation (RAG) to answer questions
about your code. It first searches for relevant code chunks using vector
similarity, then uses an LLM to generate an answer based on that context.

The question can be provided as an argument or via stdin. You can customize
the retrieval parameters (top-k, min-score) and the generation profile.

Examples:
  # Ask a question as argument
  meow ask "How does authentication work?"

  # Ask a question from stdin
  echo "What's the error handling strategy?" | meow ask

  # Use a different profile and show retrieved context
  meow ask "Explain the database layer" --profile smart --show-context

  # Search more thoroughly with higher k and lower threshold
  meow ask "Where are the API routes defined?" --top-k 10 --min-score 0.5`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if cmd == nil {
			return fmt.Errorf("command is nil")
		}

		ctx := cmd.Context()

		container, ok := ctx.Value(app.AppContainerKey).(*app.Container)
		if !ok || container == nil {
			return fmt.Errorf("application not initialized")
		}

		flow, err := container.CreateAskFlow()
		if err != nil {
			return fmt.Errorf("failed to create ask flow: %w", err)
		}

		// Limit concurrency to prevent database lock contention
		concurrency := runtime.NumCPU() * 2
		orchestrator, err := executor.NewOrchestrator(container.OutputService, container.TraceLogger, concurrency)
		if err != nil {
			return fmt.Errorf("failed to create flow runner: %w", err)
		}

		silent, err := container.CommandService.GetSilentFlag()
		if err != nil {
			return fmt.Errorf("failed to get command silent flag: %w", err)
		}

		return orchestrator.Execute(ctx, "AskFlow", flow, silent)
	},
}

func init() {
	rootCmd.AddCommand(askCmd)
	askCmd.Flags().String("profile", "", "Profile to use for answer generation (overrides config)")
	askCmd.Flags().IntP("top-k", "k", 0, "Number of top results to retrieve (0 = use config default)")
	askCmd.Flags().Float32("min-score", 0.0, "Minimum similarity score (0.0 = use config default)")
	askCmd.Flags().Bool("show-context", false, "Show retrieved code context before the answer")
	askCmd.Flags().String("system-prompt", "", "System prompt for answer generation (overrides config)")
	askCmd.Flags().StringSliceP("snapshots", "s", []string{"_workdir_", "_stage_", "_head_"}, "Snapshots to search (workdir, stage, head)")
}
