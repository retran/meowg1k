// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/app"
	"github.com/retran/meowg1k/pkg/executor"
)

var askCmd = &cobra.Command{
	Use:     "ask <question>",
	Aliases: []string{"a"},
	Short:   "Ask a question about your codebase using RAG",
	Long: `Ask a question about your codebase and get an AI-generated answer.

Uses Retrieval-Augmented Generation (RAG) to search for relevant code chunks
using vector similarity, then generates an answer with an LLM.

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
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runFlowCommand(cmd, "AskFlow", func(container *app.Container) (executor.Flow, error) {
			return container.CreateAskFlow()
		})
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
