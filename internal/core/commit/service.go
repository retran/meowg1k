// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package commit provides services for composing commit messages from file changes.
package commit

import (
	"fmt"

	commit2 "github.com/retran/meowg1k/internal/domain/commit"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/ports"
)

// Service resolves commit configuration from application config and profiles.
type Service struct {
	configResolver  ports.ConfigResolver
	profileResolver ports.ProfileResolver
}

// NewService creates a new commit configuration service.
func NewService(configResolver ports.ConfigResolver, profileResolver ports.ProfileResolver) (*Service, error) {
	if configResolver == nil {
		return nil, fmt.Errorf("config resolver is nil")
	}

	if profileResolver == nil {
		return nil, fmt.Errorf("profile resolver is nil")
	}

	return &Service{
		configResolver:  configResolver,
		profileResolver: profileResolver,
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

	if s.profileResolver == nil {
		return nil, fmt.Errorf("profile resolver is nil")
	}

	cfg, err := s.configResolver.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get application cfg: %w", err)
	}

	var profileName string
	var systemPrompt string
	var strategy string

	if cfg.Commit != nil {
		profileName = cfg.Commit.Profile
		systemPrompt = cfg.Commit.SystemPrompt
		strategy = cfg.Commit.Strategy
	}

	// Default to "summarize" if strategy is not specified
	if strategy == "" {
		strategy = "summarize"
	}

	resolvedProfile, err := s.profileResolver.Get(profile.Profile(profileName))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve profile %q: %w", profileName, err)
	}

	if systemPrompt == "" {
		return nil, fmt.Errorf("system prompt is required")
	}

	return &commit2.ResolvedConfig{
		Profile:      resolvedProfile,
		Strategy:     strategy,
		SystemPrompt: systemPrompt,
	}, nil
}
