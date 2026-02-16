// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/templates"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize meowg1k configuration",
	Long: `Initialize meowg1k configuration.

Two modes:

1. Global configuration (run once):
   meow init --global
   Creates: ~/.config/meowg1k/init.star

2. Project configuration (per repository):
   meow init
   Creates: ./.meowg1k/init.star and example commands

The global config defines providers, models, and presets that are
available in all projects. The project config can override these
settings and define project-specific commands.`,
	RunE: runInit,
}

func runInit(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true

	global, err := cmd.Flags().GetBool("global")
	if err != nil {
		return fmt.Errorf("failed to get global flag: %w", err)
	}

	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return fmt.Errorf("failed to get force flag: %w", err)
	}

	if global {
		return initGlobalConfig(cmd, force)
	}

	return initProjectConfig(cmd, force)
}

func initGlobalConfig(cmd *cobra.Command, force bool) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "meowg1k")
	initFile := filepath.Join(configDir, "init.star")

	// Check if already exists
	if !force {
		if _, err := os.Stat(initFile); err == nil {
			return fmt.Errorf("global config already exists: %s\nUse --force to overwrite", initFile)
		}
	}

	// Create directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write init.star
	if err := os.WriteFile(initFile, []byte(templates.GlobalInitTemplate), 0644); err != nil {
		return fmt.Errorf("failed to write init.star: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Global configuration created: %s\n", initFile)
	fmt.Fprintf(cmd.OutOrStdout(), "\nNext steps:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "1. Set your API key:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "   export OPENAI_API_KEY=\"sk-...\"\n")
	fmt.Fprintf(cmd.OutOrStdout(), "2. Edit config to add more providers/models:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "   %s\n", initFile)
	fmt.Fprintf(cmd.OutOrStdout(), "3. Initialize a project:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "   cd your-project && meow init\n")

	return nil
}

func initProjectConfig(cmd *cobra.Command, force bool) error {
	// Check if we're in a git repository
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		fmt.Fprintf(cmd.OutOrStdout(), "⚠ Warning: Not in a git repository\n")
	}

	projectDir := ".meowg1k"
	initFile := filepath.Join(projectDir, "init.star")

	// Check if already exists
	if !force {
		if _, err := os.Stat(initFile); err == nil {
			return fmt.Errorf("project config already exists: %s\nUse --force to overwrite", initFile)
		}
	}

	// Create directory
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// Write init.star
	if err := os.WriteFile(initFile, []byte(templates.ProjectInitTemplate), 0644); err != nil {
		return fmt.Errorf("failed to write init.star: %w", err)
	}

	// Update .gitignore
	if err := updateGitignore(); err != nil {
		fmt.Fprintf(cmd.OutOrStderr(), "⚠ Warning: %v\n", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Project configuration created: %s\n", initFile)
	fmt.Fprintf(cmd.OutOrStdout(), "\nExample commands available:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  meow commit              # Generate commit message\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  meow search -q \"query\"   # Semantic search\n")
	fmt.Fprintf(cmd.OutOrStdout(), "\nEdit %s to customize!\n", initFile)

	return nil
}

func updateGitignore() error {
	gitignorePath := ".gitignore"

	// Read existing .gitignore
	var content string
	if data, err := os.ReadFile(gitignorePath); err == nil {
		content = string(data)
	}

	// Check if already has our sentinel block
	if strings.Contains(content, "# meowg1k") {
		return nil // Already added
	}

	// Append our entries
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += templates.GitignoreEntries

	// Write back
	return os.WriteFile(gitignorePath, []byte(content), 0644)
}

func init() {
	initCmd.Flags().BoolP("global", "g", false, "Initialize global configuration (~/.config/meowg1k/)")
	initCmd.Flags().BoolP("force", "f", false, "Overwrite existing configuration")
	rootCmd.AddCommand(initCmd)
}
