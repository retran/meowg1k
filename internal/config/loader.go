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

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	projectName      = "meowg1k"
	projectConfigDir = "." + projectName
	configFileName   = "config"
)

// LoadConfig loads configuration with the following precedence:
// 1. --config flag (if provided)
// 2. Project config: ./.meowg1k/config.yaml
// 3. User config: ~/.config/meowg1k/config.yaml
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigName(configFileName)
	v.SetConfigType("yaml")

	// If a specific config path is provided, use it exclusively
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
		}
	} else {
		// Set up standard configuration search paths
		userConfigDir, err := resolveUserConfigDir()
		if err != nil {
			return nil, err
		}
		v.AddConfigPath(userConfigDir)

		// Track if any config file was found
		foundConfig := false

		// Try to read the user configuration file first
		err = v.ReadInConfig()
		if err == nil {
			foundConfig = true
		} else if !isConfigFileNotFoundError(err) {
			return nil, fmt.Errorf("failed to load user configuration: %w", err)
		}

		// Add project config path and try to merge
		v.AddConfigPath(projectConfigDir)
		err = v.MergeInConfig()
		if err == nil {
			foundConfig = true
		} else if !isConfigFileNotFoundError(err) {
			return nil, fmt.Errorf("failed to load project configuration: %w", err)
		}

		// If no config file was found, return an error
		if !foundConfig {
			return nil, fmt.Errorf("no configuration file found. Create one of:\n  - %s/config.yaml (user config)\n  - %s/config.yaml (project config)\n\nOr use --config flag to specify a custom config file", userConfigDir, projectConfigDir)
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	// Ensure configuration has at least one profile
	if len(config.Profiles) == 0 {
		return nil, fmt.Errorf("no profiles defined in configuration. Create a config file with at least one profile")
	}

	return &config, nil
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

// isConfigFileNotFoundError checks if the error is a ConfigFileNotFoundError.
func isConfigFileNotFoundError(err error) bool {
	_, ok := err.(viper.ConfigFileNotFoundError)
	return ok
}
