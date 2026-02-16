// Copyright © 2025 The meowg1k Authors
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
	Run: func(cmd *cobra.Command, _ []string) {
		if cmd == nil {
			return
		}

		fmt.Fprintf(cmd.OutOrStdout(), "meow version %s\n", version.Version)
		fmt.Fprintf(cmd.OutOrStdout(), "Build Date: %s\n", version.BuildDate)
		fmt.Fprintf(cmd.OutOrStdout(), "Git Commit: %s\n", version.GitCommit)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
