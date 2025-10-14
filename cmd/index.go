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
		if cmd == nil {
			return fmt.Errorf("command is nil")
		}

		ctx := cmd.Context()

		container, ok := ctx.Value(app.AppContainerKey).(*app.Container)
		if !ok || container == nil {
			return fmt.Errorf("application not initialized")
		}

		flow, err := container.CreateIndexReconcileFlow()
		if err != nil {
			return fmt.Errorf("failed to create index flow: %w", err)
		}

		orchestrator, err := executor.NewOrchestrator(container.OutputService, container.TraceLogger)
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
