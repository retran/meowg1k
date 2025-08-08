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
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	projectName      = "meowg1k"
	projectConfigDir = "." + projectName
)

var rootCmd = &cobra.Command{
	Use:   "meow",
	Short: "'meow' — your fast, script-friendly AI companion",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return readConfiguration()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

// resolveUserConfigDir determines the user configuration directory based on the XDG Base Directory Specification.
func resolveUserConfigDir() (string, error) {
	var configPath string
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		configPath = filepath.Join(xdgConfigHome, projectName)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		configPath = filepath.Join(home, ".config", projectName)
	}
	return configPath, nil
}

// ignoreConfigFileNotFound checks if the error is a ConfigFileNotFoundError and returns nil if it is
// or wraps the error with a message if it is a different error.
func ignoreConfigFileNotFound(err error, configSource string) error {
	if err == nil {
		return nil
	}

	if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
		return fmt.Errorf("failed to load %s configuration: %w", configSource, err)
	}

	return nil
}

func readConfiguration() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	userConfigDir, err := resolveUserConfigDir()
	if err != nil {
		return err
	}
	viper.AddConfigPath(userConfigDir)

	// Read the user configuration file first, if it exists.
	if err := ignoreConfigFileNotFound(viper.ReadInConfig(), "user"); err != nil {
		return err
	}

	viper.AddConfigPath(projectConfigDir)

	// Merge the project configuration file, if it exists.
	if err := ignoreConfigFileNotFound(viper.MergeInConfig(), "project"); err != nil {
		return err
	}

	return nil
}
