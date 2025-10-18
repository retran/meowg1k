// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package pullrequest provides services for composing pull request descriptions from file changes.
package pullrequest

import (
	"fmt"

	"github.com/retran/meowg1k/internal/domain/profile"
	pullrequest2 "github.com/retran/meowg1k/internal/domain/pullrequest"
	"github.com/retran/meowg1k/internal/ports"
)

// Service resolves PR configuration from application config and profiles.
type Service struct {
	configResolver  ports.ConfigResolver
	profileResolver ports.ProfileResolver
}

// NewService creates a new PR configuration service.
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

// Get resolves the PR configuration.
func (s *Service) Get() (*pullrequest2.ResolvedConfig, error) {
	if s == nil {
		return nil, fmt.Errorf("pull request service is nil")
	}

	config, err := s.configResolver.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get application config: %w", err)
	}

	var profileName string
	var systemPrompt string
	var strategy string

	if config.PullRequest != nil {
		profileName = config.PullRequest.Profile
		systemPrompt = config.PullRequest.SystemPrompt
		strategy = config.PullRequest.Strategy
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
		return nil, fmt.Errorf("failed to get system prompt")
	}

	return &pullrequest2.ResolvedConfig{
		Profile:      resolvedProfile,
		Strategy:     strategy,
		SystemPrompt: systemPrompt,
	}, nil
}
