// Copyright © 2025 The meowg1k Authors.
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

	if !force {
		if _, err := os.Stat(initFile); err == nil {
			return fmt.Errorf("global config already exists: %s\nUse --force to overwrite", initFile)
		}
	}

	if err := os.MkdirAll(configDir, 0o750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(initFile, []byte(templates.GlobalInitTemplate), 0o600); err != nil {
		return fmt.Errorf("failed to write init.star: %w", err)
	}

	out := cmd.OutOrStdout()
	lines := []string{
		fmt.Sprintf("✓ Global configuration created: %s\n", initFile),
		"\nNext steps:\n",
		"1. Set your API key:\n",
		"   export OPENAI_API_KEY=\"sk-...\"\n",
		"2. Edit config to add more providers/models:\n",
		fmt.Sprintf("   %s\n", initFile),
		"3. Initialize a project:\n",
		"   cd your-project && meow init\n",
	}
	for _, line := range lines {
		if _, err := fmt.Fprint(out, line); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}

	return nil
}

func initProjectConfig(cmd *cobra.Command, force bool) error {
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		if _, printErr := fmt.Fprintf(cmd.OutOrStdout(), "⚠ Warning: Not in a git repository\n"); printErr != nil {
			return fmt.Errorf("failed to write output: %w", printErr)
		}
	}

	projectDir := ".meowg1k"
	initFile := filepath.Join(projectDir, "init.star")

	if !force {
		if _, err := os.Stat(initFile); err == nil {
			return fmt.Errorf("project config already exists: %s\nUse --force to overwrite", initFile)
		}
	}

	if err := os.MkdirAll(projectDir, 0o750); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	if err := os.WriteFile(initFile, []byte(templates.ProjectInitTemplate), 0o600); err != nil {
		return fmt.Errorf("failed to write init.star: %w", err)
	}

	if err := updateGitignore(); err != nil {
		if _, printErr := fmt.Fprintf(cmd.OutOrStderr(), "⚠ Warning: %v\n", err); printErr != nil {
			return fmt.Errorf("failed to write output: %w", printErr)
		}
	}

	out := cmd.OutOrStdout()
	lines := []string{
		fmt.Sprintf("✓ Project configuration created: %s\n", initFile),
		"\nExample commands available:\n",
		"  meow commit              # Generate commit message\n",
		"  meow search -q \"query\"   # Semantic search\n",
		fmt.Sprintf("\nEdit %s to customize!\n", initFile),
	}
	for _, line := range lines {
		if _, err := fmt.Fprint(out, line); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}

	return nil
}

func updateGitignore() error {
	gitignorePath := ".gitignore"

	var content string
	if data, err := os.ReadFile(gitignorePath); err == nil {
		content = string(data)
	}

	if strings.Contains(content, "# meowg1k") {
		return nil
	}

	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += templates.GitignoreEntries

	if err := os.WriteFile(gitignorePath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("failed to write .gitignore: %w", err)
	}
	return nil
}

func init() {
	initCmd.Flags().BoolP("global", "g", false, "Initialize global configuration (~/.config/meowg1k/)")
	initCmd.Flags().BoolP("force", "f", false, "Overwrite existing configuration")
	rootCmd.AddCommand(initCmd)
}
