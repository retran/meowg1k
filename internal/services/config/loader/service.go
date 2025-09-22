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

package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/retran/meowg1k/internal/config"
	"github.com/spf13/viper"
)

const (
	projectName      = "meowg1k"
	projectConfigDir = "." + projectName
	configFileName   = "config"
)

// Service provides configuration loading capabilities.
type Service interface {
	// LoadConfig loads configuration from a specific path or standard locations.
	LoadConfig(configPath string) (*config.Config, error)

	// LoadFromSources loads configuration from multiple sources with priority.
	LoadFromSources(sources ...ConfigSource) (*config.Config, error)
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

// serviceImpl is the concrete implementation of the loader service.
type serviceImpl struct{}

// Compile-time interface satisfaction check
var _ Service = (*serviceImpl)(nil)

// NewService creates a new configuration loader service.
func NewService() Service {
	return &serviceImpl{}
}

// LoadConfig loads configuration from a specific path or standard locations.
func (s *serviceImpl) LoadConfig(configPath string) (*config.Config, error) {
	if configPath != "" {
		// Load from specific path
		source := NewFileSource(configPath, 100, "explicit")
		return s.LoadFromSources(source)
	}

	// Load from standard locations
	userConfigDir, err := resolveUserConfigDir()
	if err != nil {
		return nil, err
	}

	sources := []ConfigSource{
		NewDirectorySource(userConfigDir, "config.yaml", 10, "user"),
		NewDirectorySource(projectConfigDir, "config.yaml", 20, "project"),
	}

	return s.LoadFromSources(sources...)
}

// LoadFromSources loads configuration from multiple sources with priority.
func (s *serviceImpl) LoadFromSources(sources ...ConfigSource) (*config.Config, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("no configuration sources provided")
	}

	// Sort sources by priority (lower priority first)
	sortedSources := make([]ConfigSource, len(sources))
	copy(sortedSources, sources)

	sort.Slice(sortedSources, func(i, j int) bool {
		return sortedSources[i].Priority() < sortedSources[j].Priority()
	})

	// Merge configurations from all sources
	v := viper.New()
	v.SetConfigType("yaml")

	foundConfig := false
	var loadErrors []error

	for _, source := range sortedSources {
		settings, err := source.Load()
		if err != nil {
			loadErrors = append(loadErrors, fmt.Errorf("source '%s': %w", source.Name(), err))
			continue
		}

		if len(settings) > 0 {
			foundConfig = true
			// Merge settings into viper
			if err := v.MergeConfigMap(settings); err != nil {
				loadErrors = append(loadErrors, fmt.Errorf("source '%s': failed to merge config: %w", source.Name(), err))
				continue
			}
		}
	}

	if !foundConfig {
		if len(loadErrors) > 0 {
			return nil, fmt.Errorf("failed to load configuration: %v", loadErrors)
		}
		return nil, fmt.Errorf("no configuration found in any source")
	}

	// Unmarshal into Config struct
	var cfg config.Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	// Basic validation after merging to catch obvious issues
	if err := s.validateMergedConfig(&cfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed after merging: %w", err)
	}

	return &cfg, nil
}

// fileSource represents a file-based configuration source.
type fileSource struct {
	path     string
	priority int
	name     string
}

// NewFileSource creates a new file-based configuration source.
func NewFileSource(path string, priority int, name string) ConfigSource {
	return &fileSource{
		path:     path,
		priority: priority,
		name:     name,
	}
}

// Load retrieves configuration data from this file source.
func (fs *fileSource) Load() (map[string]interface{}, error) {
	if _, err := os.Stat(fs.path); os.IsNotExist(err) {
		return make(map[string]interface{}), nil // Return empty config if file doesn't exist
	}

	v := viper.New()
	v.SetConfigFile(fs.path)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", fs.path, err)
	}

	return v.AllSettings(), nil
}

// Priority returns the priority of this source.
func (fs *fileSource) Priority() int {
	return fs.priority
}

// Name returns a human-readable name for this source.
func (fs *fileSource) Name() string {
	return fs.name
}

// directorySource represents a directory-based configuration source.
type directorySource struct {
	dir      string
	filename string
	priority int
	name     string
}

// NewDirectorySource creates a new directory-based configuration source.
func NewDirectorySource(dir, filename string, priority int, name string) ConfigSource {
	return &directorySource{
		dir:      dir,
		filename: filename,
		priority: priority,
		name:     name,
	}
}

// Load retrieves configuration data from this directory source.
func (ds *directorySource) Load() (map[string]interface{}, error) {
	configPath := filepath.Join(ds.dir, ds.filename)

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return make(map[string]interface{}), nil // Return empty config if file doesn't exist
	}

	v := viper.New()
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", configPath, err)
	}

	return v.AllSettings(), nil
}

// Priority returns the priority of this source.
func (ds *directorySource) Priority() int {
	return ds.priority
}

// Name returns a human-readable name for this source.
func (ds *directorySource) Name() string {
	return ds.name
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

// validateMergedConfig performs basic validation on merged configuration
// to catch obvious issues that might result from conflicting sources.
func (s *serviceImpl) validateMergedConfig(cfg *config.Config) error {
	var errors []error

	// Validate that profiles exist if referenced
	if cfg.Generate != nil {
		if cfg.Generate.Default != nil && cfg.Generate.Default.Profile != "" {
			if cfg.Profiles == nil || cfg.Profiles[cfg.Generate.Default.Profile] == nil {
				errors = append(errors, fmt.Errorf("default profile '%s' is not defined", cfg.Generate.Default.Profile))
			}
		}

		// Validate task profiles
		if cfg.Generate.Tasks != nil {
			for taskName, task := range cfg.Generate.Tasks {
				if task.Profile != "" {
					if cfg.Profiles == nil || cfg.Profiles[task.Profile] == nil {
						errors = append(errors, fmt.Errorf("task '%s' references undefined profile '%s'", taskName, task.Profile))
					}
				}
			}
		}
	}

	// Validate profile consistency
	if cfg.Profiles != nil {
		for profileName, profile := range cfg.Profiles {
			if profile == nil {
				errors = append(errors, fmt.Errorf("profile '%s' is nil", profileName))
				continue
			}
			if profile.Provider == "" {
				errors = append(errors, fmt.Errorf("profile '%s' has empty provider", profileName))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %v", errors)
	}

	return nil
}
