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

// Package cmd contains the command-line interface for meowg1k.
package cmd

import (
	"fmt"
	"io"
	"os"

	generateFlows "github.com/retran/meowg1k/flows/generate"
	utilsio "github.com/retran/meowg1k/internal/utils/io"
	"github.com/retran/meowg1k/internal/utils/ui"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"gen", "g"},
	Short:   "Generate any content based on input — code, text, or docs",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGenerate(cmd)
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().StringP("task", "t", "", "Run a predefined task from config")
	generateCmd.Flags().StringP("user-prompt", "p", "", "User prompt for generation. Can be combined with stdin")
	generateCmd.Flags().Bool("silent", false, "Silent mode - only output the result without progress indicators")
}

// runGenerate executes the main logic of the generate command.
func runGenerate(cmd *cobra.Command) error {
	silent, err := cmd.Flags().GetBool("silent")
	if err != nil {
		return fmt.Errorf("failed to get silent flag: %w", err)
	}
	ctx := cmd.Context()

	content, err := ui.RunFlowWithProgress(silent, "Generating", func(tracker *ui.FlowProgressTracker) (string, error) {
		return generateFlows.ExecuteGenerate(ctx, cmd, appConfig, tracker.FeedbackHandler())
	})
	if err != nil {
		return fmt.Errorf("failed to execute generation flow: %w", err)
	}

	_, err = io.WriteString(os.Stdout, utilsio.FinalizeOutput(content))
	if err != nil {
		return fmt.Errorf("failed to write response to stdout: %w", err)
	}

	return nil
}
