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

var generateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"gen", "g"},
	Short:   "Generate any content based on input — code, text, or docs",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		container, ok := ctx.Value(app.AppContainerKey).(*app.Container)
		if !ok || container == nil {
			return fmt.Errorf("application not initialized")
		}

		flow, err := container.CreateGenerateFlow()
		if err != nil {
			return fmt.Errorf("failed to create generate flow: %w", err)
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

		return orchestrator.Execute(ctx, "GenerateFlow", flow, silent)
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringP("task", "t", "", "Run a predefined task from config")
	generateCmd.Flags().StringP("system-prompt", "s", "", "System prompt for generation")
	generateCmd.Flags().StringP("user-prompt", "u", "", "User prompt for generation. Can be combined with stdin")
}
