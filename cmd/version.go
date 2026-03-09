// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version info",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if cmd == nil {
			return nil
		}

		out := cmd.OutOrStdout()
		lines := []string{
			fmt.Sprintf("meow version %s\n", version.Version),
			fmt.Sprintf("Build Date: %s\n", version.BuildDate),
			fmt.Sprintf("Git Commit: %s\n", version.GitCommit),
		}
		for _, line := range lines {
			if _, err := fmt.Fprint(out, line); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
