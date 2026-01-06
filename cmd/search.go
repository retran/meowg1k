// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/app"
	"github.com/retran/meowg1k/pkg/executor"
)

var searchCmd = &cobra.Command{
	Use:     "search <text>",
	Aliases: []string{"s"},
	Short:   "Search for code chunks similar to the query text",
	Long: `Search for code chunks similar to the query text using vector similarity.

This command performs semantic search over indexed code chunks and returns
the most relevant results. The query text can be provided as an argument
or via stdin.

The command searches across specified snapshots (workdir, stage, head by default)
and returns chunks with similarity scores above the minimum threshold.

Examples:
  # Search with query text as argument
  meow search "authentication logic"

  # Search with query text from stdin
  echo "error handling" | meow search

  # Search only in workdir and stage snapshots
  meow search "database connection" --snapshots workdir,stage

  # Get top 20 results in JSON format
  meow search "API endpoints" --top-k 20 --json`,
	Args: validateInputOrStdin,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		return runFlowCommand(cmd, "SearchFlow", func(container *app.Container) (executor.Flow, error) {
			return container.CreateSearchFlow()
		})
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().IntP("top-k", "k", 10, "Number of top results to return")
	searchCmd.Flags().StringSliceP("snapshots", "s", []string{"_workdir_", "_stage_", "_head_"}, "Snapshots to search (workdir, stage, head)")
	searchCmd.Flags().Float32("min-score", 0.0, "Minimum similarity score (0.0 to 1.0)")
	searchCmd.Flags().Bool("json", false, "Output results in JSON format")
}
