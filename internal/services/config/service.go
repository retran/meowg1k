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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/retran/meowg1k/internal/core/config"
	"github.com/retran/meowg1k/internal/core/ports"
)

const (
	projectName      = "meowg1k"
	projectConfigDir = "." + projectName
	configFileName   = "config"
)

// Service loads and provides application configuration.
type Service struct {
	config *config.Config
}

// NewService creates a new configuration service and loads configuration at creation time.
func NewService(filePathResolver ports.FilePathResolver) (*Service, error) {
	if filePathResolver == nil {
		return nil, fmt.Errorf("config path resolver is nil")
	}

	service := &Service{}
	v := viper.New()

	configPath, err := filePathResolver.GetConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path from command: %w", err)
	}

	if configPath != "" {
		err = loadSpecificConfigFile(v, configPath)
	} else {
		err = loadDefaultConfigFiles(v)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}

	var cfg config.Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	service.config = &cfg

	return service, nil
}

// loadSpecificConfigFile loads a specific config file path.
func loadSpecificConfigFile(v *viper.Viper, configPath string) error {
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			return fmt.Errorf("specified config file not found: %s", configPath)
		}

		return fmt.Errorf("failed to read config file: %w", err)
	}

	return nil
}

// loadDefaultConfigFiles loads configuration files from standard locations.
func loadDefaultConfigFiles(v *viper.Viper) error {
	v.SetConfigName(configFileName)
	v.SetConfigType("yaml")

	configPaths := getConfigPaths()
	foundAny := false

	for _, path := range configPaths {
		found, err := tryLoadConfigFromPath(v, path, !foundAny)
		if err != nil {
			return fmt.Errorf("failed to load config from path %q: %w", path, err)
		}

		if found {
			foundAny = true
		}
	}

	if !foundAny {
		return fmt.Errorf("no configuration file found in standard locations")
	}

	return nil
}

func tryLoadConfigFromPath(v *viper.Viper, path string, primary bool) (bool, error) {
	configFile := filepath.Join(path, configFileName+".yaml")

	if _, err := os.Stat(configFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}

		return false, fmt.Errorf("failed to access config file %s: %w", configFile, err)
	}

	if primary {
		v.AddConfigPath(path)

		if err := v.ReadInConfig(); err != nil {
			return false, fmt.Errorf("failed to read config from %s: %w", configFile, err)
		}

		return true, nil
	}

	v.SetConfigFile(configFile)

	if err := v.MergeInConfig(); err != nil {
		return false, fmt.Errorf("failed to merge config from %s: %w", configFile, err)
	}

	return true, nil
}

// getConfigPaths returns the standard configuration file search paths.
func getConfigPaths() []string {
	var configPaths []string

	systemConfigDirs := os.Getenv("XDG_CONFIG_DIRS")
	if systemConfigDirs == "" {
		systemConfigDirs = "/etc/xdg"
	}

	configPaths = append(configPaths, filepath.Join(systemConfigDirs, projectName))

	userConfigDir := os.Getenv("XDG_CONFIG_HOME")
	if userConfigDir == "" {
		if home := os.Getenv("HOME"); home != "" {
			userConfigDir = filepath.Join(home, ".config")
		}
	}

	if userConfigDir != "" {
		configPaths = append(configPaths, filepath.Join(userConfigDir, projectName))
	}

	if cwd, err := os.Getwd(); err == nil {
		configPaths = append(configPaths, filepath.Join(cwd, projectConfigDir))
	}

	return configPaths
}

// Get returns the loaded configuration.
func (s *Service) Get() (*config.Config, error) {
	if s == nil {
		return nil, fmt.Errorf("config service is nil")
	}

	return s.config, nil
}
