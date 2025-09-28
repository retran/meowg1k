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

	mdConfig "github.com/retran/meowg1k/internal/models/config"
	"github.com/retran/meowg1k/internal/services/command"
	"github.com/spf13/viper"
)

// Configuration errors
var (
	ErrSpecifiedConfigFileNotFound = errors.New("specified config file not found")
	ErrNoConfigFoundInStdLocations = errors.New("no configuration file found in standard locations")
)

const (
	projectName      = "meowg1k"
	projectConfigDir = "." + projectName
	configFileName   = "config"
)

// Service provides configuration loading and management capabilities.
type Service interface {
	// GetConfig returns the loaded configuration.
	GetConfig() *mdConfig.Config
}

// serviceImpl is the concrete implementation of the config service.
type serviceImpl struct {
	config *mdConfig.Config
}

// Compile-time interface satisfaction check
var _ Service = (*serviceImpl)(nil)

// NewService creates a new configuration service and loads configuration at creation time.
func NewService(commandSvc command.Service) (Service, error) {
	service := &serviceImpl{}
	v := viper.New()

	configPath, err := commandSvc.GetConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path from command: %w", err)
	}

	if configPath != "" {
		err = loadSpecificConfigFile(v, configPath)
	} else {
		err = loadDefaultConfigFiles(v)
	}
	if err != nil {
		return nil, err
	}

	// Unmarshal into config struct
	var cfg mdConfig.Config
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
			return fmt.Errorf("%w: %s", ErrSpecifiedConfigFileNotFound, configPath)
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

	for i, path := range configPaths {
		configFile := filepath.Join(path, configFileName+".yaml")
		if _, err := os.Stat(configFile); err == nil {
			if i == 0 {
				v.AddConfigPath(path)

				if err := v.ReadInConfig(); err == nil {
					foundAny = true
				}
			} else {
				v.SetConfigFile(configFile)
				if err := v.MergeInConfig(); err == nil {
					foundAny = true
				}
			}
		}
	}

	if !foundAny {
		return ErrNoConfigFoundInStdLocations
	}
	return nil
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

// GetConfig returns the loaded configuration.
func (s *serviceImpl) GetConfig() *mdConfig.Config {
	return s.config
}
