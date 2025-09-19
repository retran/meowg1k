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

// Package cmd provides commands for the meow CLI application.
package cmd

import (
	"fmt"

	"github.com/retran/meowg1k/internal/config"
	"github.com/spf13/cobra"
)

var (
	configPath string
	appConfig  *config.Config
)

func Execute() error {
	return rootCmd.Execute()
}

var rootCmd = &cobra.Command{
	Use:   "meow",
	Short: "'meow' — your fast, script-friendly AI companion",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for commands that don't need it
		if cmd.Name() == "version" || cmd.Name() == "help" || cmd.Name() == "meow" || cmd.Name() == "completion" {
			return nil
		}

		var err error
		appConfig, err = config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file path (overrides project/user configs when specified)")
}
