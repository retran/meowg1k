// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package config provides configuration management using Starlark scripts only.
package config

import (
	"fmt"

	"github.com/retran/meowg1k/internal/domain/config"
)

// Service provides application configuration from Starlark scripts.
// YAML configuration has been removed - all configuration is done via Starlark.
type Service struct {
	config *config.Config
}

// NewService creates a new configuration service with empty config.
// Configuration will be populated by Starlark scripts via Override().
func NewService() (*Service, error) {
	return &Service{
		config: &config.Config{
			Providers: make(map[string]*config.ProviderConfig),
			Models:    make(map[string]*config.ModelConfig),
			Presets:   make(map[string]*config.PresetConfig),
		},
	}, nil
}

// Get returns the loaded configuration.
func (s *Service) Get() (*config.Config, error) {
	if s == nil {
		return nil, fmt.Errorf("config service is nil")
	}

	return s.config, nil
}

// Override replaces the current configuration with a new one.
// This is used to apply runtime configuration from Starlark scripts.
func (s *Service) Override(cfg *config.Config) error {
	if s == nil {
		return fmt.Errorf("config service is nil")
	}
	if cfg == nil {
		return fmt.Errorf("configuration cannot be nil")
	}
	s.config = cfg
	return nil
}
