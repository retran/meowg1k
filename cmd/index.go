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

var indexCmd = &cobra.Command{
	Use:     "index",
	Aliases: []string{"idx"},
	Short:   "Index workspace files for RAG-based queries",
	Long: `Index workspace files by computing embeddings and building vector indices.

This command processes all files in the workspace according to filter rules,
chunks them, computes embeddings, and builds vector indices for efficient
similarity search. The indices are used by 'query' and 'ask' commands.

The indexing process includes:
  - Scanning workspace state (workdir, stage, head)
  - Chunking files according to configuration
  - Computing embeddings using the configured profile
  - Building and saving vector indices

Example:
  meow index`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		container, ok := ctx.Value(app.AppContainerKey).(*app.Container)
		if !ok || container == nil {
			return fmt.Errorf("application not initialized")
		}

		flow, err := container.CreateIndexReconcileFlow()
		if err != nil {
			return fmt.Errorf("failed to create index flow: %w", err)
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

		return orchestrator.Execute(ctx, "IndexFlow", flow, silent)
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
}
