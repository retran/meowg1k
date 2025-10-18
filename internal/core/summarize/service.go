// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package summarize provides services for generating summaries of file changes using LLMs.
package summarize

import (
	"fmt"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/profile"
	summarize2 "github.com/retran/meowg1k/internal/domain/summarize"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/gitignore"
)

// Service resolves file summarization configurations.
type Service struct {
	configResolver  ports.ConfigResolver
	profileResolver ports.ProfileResolver
}

// NewService creates a new file summarization configuration service.
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

// Get resolves the summarization configuration for a given file.
func (s *Service) Get(filename string) (*summarize2.ResolvedConfig, error) {
	if s == nil {
		return nil, fmt.Errorf("summarize service is nil")
	}

	currentConfig, err := s.configResolver.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get application config: %w", err)
	}

	if currentConfig.Summarize == nil {
		return nil, nil
	}

	var matchingRule *config.SummarizeRule
	for _, rule := range currentConfig.Summarize.Rules {
		if rule.Match != "" {
			matcher := gitignore.NewMatcher([]string{rule.Match})
			if matcher.Match(filename, false) {
				matchingRule = rule
				break
			}
		}
	}

	if matchingRule != nil && matchingRule.Skip {
		return &summarize2.ResolvedConfig{
			Skip: true,
		}, nil
	}

	var profileName string
	var strategy *config.Strategy
	var systemPrompt string

	if matchingRule != nil {
		if matchingRule.Profile != "" {
			profileName = matchingRule.Profile
		}
		if matchingRule.Strategy != nil {
			strategy = matchingRule.Strategy
		}
		if matchingRule.SystemPrompt != "" {
			systemPrompt = matchingRule.SystemPrompt
		}
	}

	if currentConfig.Summarize.Default != nil {
		if profileName == "" {
			profileName = currentConfig.Summarize.Default.Profile
		}
		if strategy == nil {
			strategy = currentConfig.Summarize.Default.Strategy
		}
		if systemPrompt == "" {
			systemPrompt = currentConfig.Summarize.Default.SystemPrompt
		}
	}

	resolvedProfile, err := s.profileResolver.Get(profile.Profile(profileName))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve profile: %w", err)
	}

	if strategy == nil {
		strategy = &config.Strategy{
			Type:                "plain",
			IncludeOriginalFile: false,
			IncludeChangedFile:  false,
		}
	}

	return &summarize2.ResolvedConfig{
		Profile:             resolvedProfile,
		Strategy:            strategy,
		SystemPrompt:        systemPrompt,
		Skip:                false,
		IncludeOriginalFile: strategy.IncludeOriginalFile,
		IncludeChangedFile:  strategy.IncludeChangedFile,
	}, nil
}
