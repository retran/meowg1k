// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package commit provides services for composing commit messages from file changes.
package commit

import (
	"fmt"

	commit2 "github.com/retran/meowg1k/internal/domain/commit"
	"github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/internal/ports"
)

// Service resolves commit configuration from application config and presets.
type Service struct {
	configResolver ports.ConfigResolver
	presetResolver ports.PresetResolver
}

// NewService creates a new commit configuration service.
func NewService(configResolver ports.ConfigResolver, presetResolver ports.PresetResolver) (*Service, error) {
	if configResolver == nil {
		return nil, fmt.Errorf("config resolver is nil")
	}

	if presetResolver == nil {
		return nil, fmt.Errorf("preset resolver is nil")
	}

	return &Service{
		configResolver: configResolver,
		presetResolver: presetResolver,
	}, nil
}

// Get resolves the commit configuration.
func (s *Service) Get() (*commit2.ResolvedConfig, error) {
	if s == nil {
		return nil, fmt.Errorf("commit service is nil")
	}

	if s.configResolver == nil {
		return nil, fmt.Errorf("config reader is nil")
	}

	if s.presetResolver == nil {
		return nil, fmt.Errorf("preset resolver is nil")
	}

	cfg, err := s.configResolver.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get application cfg: %w", err)
	}

	var presetName string
	var systemPrompt string
	var strategy string

	if cfg.Flows != nil && cfg.Flows.Draft != nil && cfg.Flows.Draft.Commit != nil {
		presetName = cfg.Flows.Draft.Commit.Preset
		systemPrompt = cfg.Flows.Draft.Commit.SystemPrompt
		strategy = cfg.Flows.Draft.Commit.Strategy
	}

	// Default to "summarize" if strategy is not specified
	if strategy == "" {
		strategy = "summarize"
	}

	resolvedPreset, err := s.presetResolver.Get(preset.Preset(presetName))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve preset %q: %w", presetName, err)
	}

	if systemPrompt == "" {
		return nil, fmt.Errorf("system prompt is required")
	}

	return &commit2.ResolvedConfig{
		Preset:       resolvedPreset,
		Strategy:     strategy,
		SystemPrompt: systemPrompt,
	}, nil
}
