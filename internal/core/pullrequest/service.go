// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package pullrequest provides services for composing pull request descriptions from file changes.
package pullrequest

import (
	"fmt"

	"github.com/retran/meowg1k/internal/domain/preset"
	domainpullrequest "github.com/retran/meowg1k/internal/domain/pullrequest"
	"github.com/retran/meowg1k/internal/ports"
)

// Service resolves PR configuration from application config and presets.
type Service struct {
	configResolver ports.ConfigResolver
	presetResolver ports.PresetResolver
}

// NewService creates a new PR configuration service.
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

// Get resolves the PR configuration.
func (s *Service) Get() (*domainpullrequest.ResolvedConfig, error) {
	if s == nil {
		return nil, fmt.Errorf("pull request service is nil")
	}

	config, err := s.configResolver.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get application config: %w", err)
	}

	var presetName string
	var systemPrompt string
	var strategy string

	if config.Flows != nil && config.Flows.Draft != nil && config.Flows.Draft.Pr != nil {
		presetName = config.Flows.Draft.Pr.Preset
		systemPrompt = config.Flows.Draft.Pr.SystemPrompt
		strategy = config.Flows.Draft.Pr.Strategy
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
		return nil, fmt.Errorf("failed to get system prompt")
	}

	return &domainpullrequest.ResolvedConfig{
		Preset:       resolvedPreset,
		Strategy:     strategy,
		SystemPrompt: systemPrompt,
	}, nil
}
