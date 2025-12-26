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

	matchingRule := findMatchingRule(currentConfig.Summarize, filename)

	if matchingRule != nil && matchingRule.Skip {
		return &summarize2.ResolvedConfig{
			Skip: true,
		}, nil
	}

	profileName, strategy, systemPrompt := resolveSettings(currentConfig.Summarize, matchingRule)

	resolvedProfile, err := s.profileResolver.Get(profile.Profile(profileName))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve profile: %w", err)
	}

	if strategy == nil {
		strategy = defaultStrategy()
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

func findMatchingRule(cfg *config.SummarizeConfig, filename string) *config.SummarizeRule {
	for _, rule := range cfg.Rules {
		if rule.Match == "" {
			continue
		}

		matcher := gitignore.NewMatcher([]string{rule.Match})
		if matcher.Match(filename, false) {
			return rule
		}
	}
	return nil
}

func resolveSettings(
	summarizeConfig *config.SummarizeConfig,
	rule *config.SummarizeRule,
) (profileName string, strategy *config.Strategy, systemPrompt string) {
	if rule != nil {
		applyRuleSettings(rule, &profileName, &strategy, &systemPrompt)
	}

	if summarizeConfig.Default != nil {
		applyDefaultSettings(summarizeConfig.Default, &profileName, &strategy, &systemPrompt)
	}

	return profileName, strategy, systemPrompt
}

func applyRuleSettings(
	rule *config.SummarizeRule,
	profileName *string,
	strategy **config.Strategy,
	systemPrompt *string,
) {
	if rule.Profile != "" {
		*profileName = rule.Profile
	}
	if rule.Strategy != nil {
		*strategy = rule.Strategy
	}
	if rule.SystemPrompt != "" {
		*systemPrompt = rule.SystemPrompt
	}
}

func applyDefaultSettings(
	defaults *config.SummarizeDefault,
	profileName *string,
	strategy **config.Strategy,
	systemPrompt *string,
) {
	if *profileName == "" {
		*profileName = defaults.Profile
	}
	if *strategy == nil {
		*strategy = defaults.Strategy
	}
	if *systemPrompt == "" {
		*systemPrompt = defaults.SystemPrompt
	}
}

func defaultStrategy() *config.Strategy {
	return &config.Strategy{
		Type:                "plain",
		IncludeOriginalFile: false,
		IncludeChangedFile:  false,
	}
}
