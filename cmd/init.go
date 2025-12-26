// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// defaultConfig contains the default configuration template for new projects.
// The content is embedded at compile time from default_config.yaml file.
//
//go:embed default_config.yaml
var defaultConfig string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new meowg1k project configuration",
	Long: `Initialize creates a .meowg1k.yaml configuration file in the current directory.

This command checks if a project configuration file already exists. If not,
it creates one with sensible defaults using Google Gemini as the default provider.

After initialization, you should set the MEOW_GEMINI_API_KEY environment variable:
  export MEOW_GEMINI_API_KEY="your-api-key-here"

You can get a free API key from: https://aistudio.google.com/app/apikey`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		force, err := cmd.Flags().GetBool("force")
		if err != nil {
			return fmt.Errorf("failed to get force flag: %w", err)
		}

		targetDir := ""
		if cmd.Root() != nil && cmd.Root().PersistentFlags() != nil {
			targetDir, _ = cmd.Root().PersistentFlags().GetString("workspace") //nolint:errcheck // Fall back to cwd on error
		}

		if targetDir == "" {
			targetDir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		configPath := filepath.Join(targetDir, ".meowg1k.yaml")

		silent := false
		if cmd.Root() != nil && cmd.Root().PersistentFlags() != nil {
			silent, _ = cmd.Root().PersistentFlags().GetBool("silent") //nolint:errcheck // Default to false on error
		}

		if _, err := os.Stat(configPath); err == nil {
			if !force {
				return fmt.Errorf("configuration file already exists: %s\nUse --force to overwrite", configPath)
			}
			if !silent {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Overwriting existing configuration file...\n") //nolint:errcheck // Output errors are not critical
			}
		}

		if err := os.WriteFile(configPath, []byte(defaultConfig), 0o600); err != nil {
			return fmt.Errorf("failed to create configuration file: %w", err)
		}

		if silent {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", configPath) //nolint:errcheck // Output errors are not critical
		} else {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Configuration file created: %s\n", configPath)                       //nolint:errcheck // Output errors are not critical
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nNext steps:\n")                                                      //nolint:errcheck // Output errors are not critical
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "1. Get a free API key from: https://aistudio.google.com/app/apikey\n") //nolint:errcheck // Output errors are not critical
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "2. Set the environment variable:\n")                                   //nolint:errcheck // Output errors are not critical
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   export MEOW_GEMINI_API_KEY=\"your-api-key-here\"\n")                //nolint:errcheck // Output errors are not critical
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "3. Try it out:\n")                                                     //nolint:errcheck // Output errors are not critical
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   echo \"Create a hello world function\" | meow generate\n")          //nolint:errcheck // Output errors are not critical
		}

		return nil
	},
}

func init() {
	initCmd.Flags().BoolP("force", "f", false, "overwrite existing configuration file")
	rootCmd.AddCommand(initCmd)
}
