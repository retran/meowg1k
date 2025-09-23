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
	"sort"
	"sync"

	"github.com/retran/meowg1k/internal/models/config"
	"github.com/retran/meowg1k/internal/services/command"
	"github.com/spf13/viper"
)

const (
	projectName      = "meowg1k"
	projectConfigDir = "." + projectName
	configFileName   = "config"
)

// Service provides unified configuration loading and management capabilities.
type Service interface {
	// GetConfig retrieves the loaded and validated configuration.
	GetConfig() *config.Config

	// LoadConfig loads configuration from command line parameters or standard locations.
	LoadConfig() error

	// LoadConfigFromPath loads configuration from a specific path.
	LoadConfigFromPath(configPath string) error

	// LoadFromSources loads configuration from multiple sources with priority.
	LoadFromSources(sources ...ConfigSource) error
}

// ConfigSource represents a configuration source with priority ordering.
type ConfigSource interface {
	// Load retrieves configuration data from this source.
	Load() (map[string]interface{}, error)

	// Priority returns the priority of this source (higher = more important).
	Priority() int

	// Name returns a human-readable name for this source.
	Name() string
}

// serviceImpl is the concrete implementation of the unified config service.
type serviceImpl struct {
	mu         sync.RWMutex
	config     *config.Config
	configPath string
	commandSvc command.Service
}

// Compile-time interface satisfaction check
var _ Service = (*serviceImpl)(nil)

// NewService creates a new unified configuration service with injected dependencies.
func NewService(commandSvc command.Service) Service {
	return &serviceImpl{
		commandSvc: commandSvc,
	}
}

// NewServiceWithConfig creates a new service with a pre-loaded configuration.
// This is useful for testing scenarios where you want to provide a specific config.
func NewServiceWithConfig(cfg *config.Config, configPath string, commandSvc command.Service) Service {
	if cfg == nil {
		panic("config cannot be nil")
	}

	return &serviceImpl{
		config:     cfg,
		configPath: configPath,
		commandSvc: commandSvc,
	}
}

// GetConfig retrieves the loaded and validated configuration.
func (s *serviceImpl) GetConfig() *config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// LoadConfig loads configuration from command line parameters or standard locations.
func (s *serviceImpl) LoadConfig() error {
	// Get config path from command service
	configPath, err := s.commandSvc.GetConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path from command: %w", err)
	}

	return s.LoadConfigFromPath(configPath)
}

// LoadConfigFromPath loads configuration from a specific path.
func (s *serviceImpl) LoadConfigFromPath(configPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Load configuration
	cfg, err := s.loadConfigInternal(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate configuration
	if err := s.validateConfig(cfg); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	s.config = cfg
	s.configPath = configPath
	return nil
}

// LoadFromSources loads configuration from multiple sources with priority.
func (s *serviceImpl) LoadFromSources(sources ...ConfigSource) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Sort sources by priority (higher priority first)
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Priority() > sources[j].Priority()
	})

	v := viper.New()

	// Load from each source in priority order
	for _, source := range sources {
		data, err := source.Load()
		if err != nil {
			return fmt.Errorf("failed to load from source %s: %w", source.Name(), err)
		}

		// Merge the data into viper
		for key, value := range data {
			v.Set(key, value)
		}
	}

	// Unmarshal into config struct
	var cfg config.Config
	if err := v.Unmarshal(&cfg); err != nil {
		return fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	// Validate configuration
	if err := s.validateConfig(&cfg); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	s.config = &cfg
	return nil
}

// loadConfigInternal loads configuration from a specific path (internal implementation)
func (s *serviceImpl) loadConfigInternal(configPath string) (*config.Config, error) {
	v := viper.New()

	if configPath != "" {
		// Use specific config file if provided
		v.SetConfigFile(configPath)
	} else {
		// Search in standard locations
		v.SetConfigName(configFileName)
		v.SetConfigType("yaml")

		// Look for config file in the following order:
		// 1. Current directory (.meowg1k/)
		// 2. Home directory (~/.meowg1k/)
		// 3. System config directory (/etc/meowg1k/)

		// Current directory project config
		if cwd, err := os.Getwd(); err == nil {
			v.AddConfigPath(filepath.Join(cwd, projectConfigDir))
		}

		// User home directory
		if home, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(filepath.Join(home, projectConfigDir))
		}

		// System config directory
		v.AddConfigPath("/etc/" + projectName)
	}

	// Set environment variable prefix
	v.SetEnvPrefix(projectName)
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, use defaults
			return s.getDefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg config.Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// getDefaultConfig returns a default configuration when no config file is found.
func (s *serviceImpl) getDefaultConfig() *config.Config {
	return &config.Config{
		// Default configuration values can be set here
	}
}

// validateConfig performs basic configuration validation.
func (s *serviceImpl) validateConfig(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	// Basic validation - check that we have at least one profile
	if len(cfg.Profiles) == 0 {
		return fmt.Errorf("at least one profile must be defined")
	}

	return nil
}
