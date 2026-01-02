// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package index

import (
	"fmt"

	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/internal/ports"
)

// ConfigService resolves index configuration from application config and presets.
type ConfigService struct {
	configResolver ports.ConfigResolver
	presetResolver ports.PresetResolver
}

// NewConfigService creates a new index configuration service.
func NewConfigService(configResolver ports.ConfigResolver, presetResolver ports.PresetResolver) (*ConfigService, error) {
	if configResolver == nil {
		return nil, fmt.Errorf("config resolver is nil")
	}

	if presetResolver == nil {
		return nil, fmt.Errorf("preset resolver is nil")
	}

	return &ConfigService{
		configResolver: configResolver,
		presetResolver: presetResolver,
	}, nil
}

// Get resolves the index configuration.
func (s *ConfigService) Get() (*domainindex.ResolvedConfig, error) {
	if s == nil {
		return nil, fmt.Errorf("index config service is nil")
	}

	if s.configResolver == nil {
		return nil, fmt.Errorf("config resolver is nil")
	}

	if s.presetResolver == nil {
		return nil, fmt.Errorf("preset resolver is nil")
	}

	cfg, err := s.configResolver.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get application config: %w", err)
	}

	// Validate index configuration
	if cfg.Flows == nil || cfg.Flows.Index == nil {
		return nil, fmt.Errorf("index configuration is missing")
	}
	if cfg.Flows.Index.Preset == "" {
		return nil, fmt.Errorf("index.preset is required in configuration")
	}
	if cfg.Flows.Index.Chunker == nil {
		return nil, fmt.Errorf("index.chunker configuration is missing")
	}

	// Resolve preset
	resolvedPreset, err := s.presetResolver.Get(preset.Preset(cfg.Flows.Index.Preset))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve preset %q: %w", cfg.Flows.Index.Preset, err)
	}

	// Set default batch size if not specified
	batchSize := cfg.Flows.Index.BatchSize
	if batchSize <= 0 {
		batchSize = 32 // Default batch size
	}

	return &domainindex.ResolvedConfig{
		Preset:              resolvedPreset,
		ChunkerMaxRunes:     cfg.Flows.Index.Chunker.MaxRunes,
		ChunkerOverlapRunes: cfg.Flows.Index.Chunker.OverlapRunes,
		BatchSize:           batchSize,
	}, nil
}
