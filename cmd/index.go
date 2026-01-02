// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
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
similarity search. The indices are used by 'search' and 'ask' commands.

The indexing process includes:
  - Scanning workspace state (workdir, stage, head)
  - Chunking files according to configuration
  - Computing embeddings using the configured preset
  - Building and saving vector indices

Example:
  meow index`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runFlowCommand(cmd, "IndexFlow", func(container *app.Container) (executor.Flow, error) {
			return container.CreateIndexReconcileFlow()
		})
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
}
