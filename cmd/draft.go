// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import "github.com/spf13/cobra"

var draftCmd = &cobra.Command{
	Use:   "draft",
	Short: "Draft commit messages and pull request descriptions",
	Long: `Draft content based on code changes.

Examples:
  meow draft commit --diff staged
  meow draft commit --diff branch --base main
  meow draft pr --base main`,
}

func init() {
	rootCmd.AddCommand(draftCmd)
}
