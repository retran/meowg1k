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

package manager

import (
	"fmt"
	"sync"

	"github.com/retran/meowg1k/internal/config"
	"github.com/retran/meowg1k/internal/services/config/loader"
	"github.com/retran/meowg1k/internal/services/config/validator"
)

// Service provides configuration management capabilities.
type Service interface {
	// GetConfig retrieves the loaded and validated configuration.
	GetConfig() *config.Config
}

// serviceImpl is the concrete implementation of the manager service.
type serviceImpl struct {
	mu         sync.RWMutex
	config     *config.Config
	configPath string
}

// Compile-time interface satisfaction check
var _ Service = (*serviceImpl)(nil)

// NewService creates a new configuration manager service with injected dependencies.
// This is useful for testing or when you want to provide custom implementations.
func NewService(configPath string, loaderService loader.Service, validatorService validator.Service) (Service, error) {
	// Load configuration
	cfg, err := loaderService.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate configuration
	if err := validatorService.ValidateConfig(cfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &serviceImpl{
		config:     cfg,
		configPath: configPath,
	}, nil
}

// NewServiceWithConfig creates a new manager service with a pre-loaded configuration.
// This is useful for testing scenarios where you want to provide a specific config.
func NewServiceWithConfig(cfg *config.Config, configPath string) Service {
	if cfg == nil {
		panic("config cannot be nil")
	}

	return &serviceImpl{
		config:     cfg,
		configPath: configPath,
	}
}

// GetConfig retrieves the loaded and validated configuration.
func (s *serviceImpl) GetConfig() *config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// GetConfigPath returns the path that was used to load the configuration.
func (s *serviceImpl) GetConfigPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.configPath
}
