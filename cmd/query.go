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

var queryCmd = &cobra.Command{
	Use:     "query <text>",
	Aliases: []string{"q"},
	Short:   "Search for code chunks similar to the query text",
	Long: `Search for code chunks similar to the query text using vector similarity.

This command performs semantic search over indexed code chunks and returns
the most relevant results. The query text can be provided as an argument
or via stdin.

The command searches across specified snapshots (workdir, stage, head by default)
and returns chunks with similarity scores above the minimum threshold.

Examples:
  # Search with query text as argument
  meow query "authentication logic"

  # Search with query text from stdin
  echo "error handling" | meow query

  # Search only in workdir and stage snapshots
  meow query "database connection" --snapshots workdir,stage

  # Get top 20 results in JSON format
  meow query "API endpoints" --top-k 20 --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		container, ok := ctx.Value(app.AppContainerKey).(*app.Container)
		if !ok || container == nil {
			return fmt.Errorf("application not initialized")
		}

		flow, err := container.CreateQueryFlow()
		if err != nil {
			return fmt.Errorf("failed to create query flow: %w", err)
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

		return orchestrator.Execute(ctx, "QueryFlow", flow, silent)
	},
}

func init() {
	rootCmd.AddCommand(queryCmd)
	queryCmd.Flags().IntP("top-k", "k", 10, "Number of top results to return")
	queryCmd.Flags().StringSliceP("snapshots", "s", []string{"_workdir_", "_stage_", "_head_"}, "Snapshots to search (workdir, stage, head)")
	queryCmd.Flags().Float32("min-score", 0.0, "Minimum similarity score (0.0 to 1.0)")
	queryCmd.Flags().Bool("json", false, "Output results in JSON format")
}
