// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package config provides configuration management using Viper, supporting multiple config sources.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/retran/meowg1k/internal/domain/config"
)

const (
	projectName    = "meowg1k"
	configFileName = "config"
)

// Service loads and provides application configuration.
type Service struct {
	config *config.Config
}

// FilePathResolver resolves the configuration file path.
type FilePathResolver interface {
	GetConfigPath() (string, error)
}

// WorkspaceDirResolver resolves the workspace directory path.
type WorkspaceDirResolver interface {
	Get() (string, error)
}

// NewService creates a new configuration service and loads configuration at creation time.
func NewService(filePathResolver FilePathResolver, workspaceDirResolver WorkspaceDirResolver) (*Service, error) {
	if filePathResolver == nil {
		return nil, fmt.Errorf("config path resolver is nil")
	}

	if workspaceDirResolver == nil {
		return nil, fmt.Errorf("workspace dir resolver is nil")
	}

	service := &Service{}
	v := viper.New()

	configLoaded, err := loadBaseConfig(v)
	if err != nil {
		return nil, err
	}

	// 2. If a config is passed through a parameter - load it and merge it with the main one
	configPath, err := filePathResolver.GetConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path from command: %w", err)
	}

	if configPath != "" {
		if err := mergeSpecifiedConfig(v, configPath); err != nil {
			return nil, err
		}
		configLoaded = true
	} else if loaded, err := mergeWorkspaceConfig(v, workspaceDirResolver); err != nil {
		return nil, err
	} else if loaded {
		configLoaded = true
	}

	if !configLoaded {
		return nil, fmt.Errorf("no configuration file found in any standard location")
	}

	var cfg config.Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse configuration (this may be due to a schema mismatch or old config format): %w\nSuggestion: Run 'meow init' to generate a fresh valid configuration or check the documentation for migration.", err)
	}

	service.config = &cfg

	return service, nil
}

type noConfigurationFileFoundError struct {
	message string
}

func (e *noConfigurationFileFoundError) Error() string {
	return e.message
}

func loadBaseConfig(v *viper.Viper) (bool, error) {
	if err := loadDefaultConfigFiles(v); err != nil {
		var noConfigFoundErr *noConfigurationFileFoundError
		if errors.As(err, &noConfigFoundErr) {
			return false, nil
		}
		return false, fmt.Errorf("failed to load default config file: %w", err)
	}
	return true, nil
}

func mergeSpecifiedConfig(v *viper.Viper, configPath string) error {
	v.SetConfigFile(configPath)
	if err := v.MergeInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			return fmt.Errorf("specified config file not found: %s", configPath)
		}
		return fmt.Errorf("failed to merge specified config file: %w", err)
	}
	return nil
}

func mergeWorkspaceConfig(v *viper.Viper, workspaceDirResolver WorkspaceDirResolver) (bool, error) {
	workspaceDir, err := workspaceDirResolver.Get()
	if err != nil {
		return false, fmt.Errorf("failed to get workspace directory: %w", err)
	}
	if workspaceDir == "" {
		return false, nil
	}

	v.AddConfigPath(workspaceDir)
	v.SetConfigName("." + projectName)
	if err := v.MergeInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			return false, nil
		}
		return false, fmt.Errorf("failed to merge workspace config file: %w", err)
	}
	return true, nil
}

// loadDefaultConfigFiles loads configuration files from standard locations.
func loadDefaultConfigFiles(v *viper.Viper) error {
	configPaths := getConfigPaths()
	foundAny := false

	for _, path := range configPaths {
		found, err := tryLoadConfigFromPath(v, path)
		if err != nil {
			return fmt.Errorf("failed to load config from path %q: %w", path, err)
		}

		if found {
			foundAny = true
			// Load the first one found and break
			break
		}
	}

	if !foundAny {
		return &noConfigurationFileFoundError{message: "no configuration file found in standard locations"}
	}

	return nil
}

func tryLoadConfigFromPath(v *viper.Viper, path string) (bool, error) {
	v.AddConfigPath(path)
	v.SetConfigName(configFileName)

	// Viper will automatically look for yaml and yml
	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			return false, nil // Not found is not an error here
		}
		return false, fmt.Errorf("failed to read config from %s: %w", path, err)
	}

	return true, nil
}

// getConfigPaths returns the standard configuration file search paths.
func getConfigPaths() []string {
	var configPaths []string

	userConfigDir := os.Getenv("XDG_CONFIG_HOME")
	if userConfigDir == "" {
		if home := os.Getenv("HOME"); home != "" {
			userConfigDir = filepath.Join(home, ".config")
		}
	}

	if userConfigDir != "" {
		configPaths = append(configPaths, filepath.Join(userConfigDir, projectName))
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
