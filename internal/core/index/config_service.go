// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package index

import (
	"fmt"

	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/ports"
)

// ConfigService resolves index configuration from application config and profiles.
type ConfigService struct {
	configResolver  ports.ConfigResolver
	profileResolver ports.ProfileResolver
}

// NewConfigService creates a new index configuration service.
func NewConfigService(configResolver ports.ConfigResolver, profileResolver ports.ProfileResolver) (*ConfigService, error) {
	if configResolver == nil {
		return nil, fmt.Errorf("config resolver is nil")
	}

	if profileResolver == nil {
		return nil, fmt.Errorf("profile resolver is nil")
	}

	return &ConfigService{
		configResolver:  configResolver,
		profileResolver: profileResolver,
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

	if s.profileResolver == nil {
		return nil, fmt.Errorf("profile resolver is nil")
	}

	cfg, err := s.configResolver.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get application config: %w", err)
	}

	// Validate index configuration
	if cfg.Index == nil {
		return nil, fmt.Errorf("index configuration is missing")
	}
	if cfg.Index.Profile == "" {
		return nil, fmt.Errorf("index.profile is required in configuration")
	}
	if cfg.Index.Chunker == nil {
		return nil, fmt.Errorf("index.chunker configuration is missing")
	}

	// Resolve profile
	resolvedProfile, err := s.profileResolver.Get(profile.Profile(cfg.Index.Profile))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve profile %q: %w", cfg.Index.Profile, err)
	}

	// Set default batch size if not specified
	batchSize := cfg.Index.BatchSize
	if batchSize <= 0 {
		batchSize = 32 // Default batch size
	}

	return &domainindex.ResolvedConfig{
		Profile:             resolvedProfile,
		ChunkerMaxRunes:     cfg.Index.Chunker.MaxRunes,
		ChunkerOverlapRunes: cfg.Index.Chunker.OverlapRunes,
		BatchSize:           batchSize,
	}, nil
}
