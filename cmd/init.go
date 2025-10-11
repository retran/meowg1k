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
	RunE: func(cmd *cobra.Command, args []string) error {
		force, err := cmd.Flags().GetBool("force")
		if err != nil {
			return fmt.Errorf("failed to get force flag: %w", err)
		}

		targetDir := ""
		if cmd.Root() != nil && cmd.Root().PersistentFlags() != nil {
			targetDir, _ = cmd.Root().PersistentFlags().GetString("workspace")
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
			silent, _ = cmd.Root().PersistentFlags().GetBool("silent")
		}

		if _, err := os.Stat(configPath); err == nil {
			if !force {
				return fmt.Errorf("configuration file already exists: %s\nUse --force to overwrite", configPath)
			}
			if !silent {
				fmt.Fprintf(cmd.OutOrStdout(), "Overwriting existing configuration file...\n")
			}
		}

		if err := os.WriteFile(configPath, []byte(defaultConfig), 0o644); err != nil {
			return fmt.Errorf("failed to create configuration file: %w", err)
		}

		if silent {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", configPath)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Configuration file created: %s\n", configPath)
			fmt.Fprintf(cmd.OutOrStdout(), "\nNext steps:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "1. Get a free API key from: https://aistudio.google.com/app/apikey\n")
			fmt.Fprintf(cmd.OutOrStdout(), "2. Set the environment variable:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "   export MEOW_GEMINI_API_KEY=\"your-api-key-here\"\n")
			fmt.Fprintf(cmd.OutOrStdout(), "3. Try it out:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "   echo \"Create a hello world function\" | meow generate\n")
		}

		return nil
	},
}

func init() {
	initCmd.Flags().BoolP("force", "f", false, "overwrite existing configuration file")
	rootCmd.AddCommand(initCmd)
}
