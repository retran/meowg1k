// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package summarize provides services for generating summaries of file changes using LLMs.
package summarize

import (
	"fmt"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/preset"
	summarize2 "github.com/retran/meowg1k/internal/domain/summarize"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/gitignore"
)

// Service resolves file summarization configurations.
type Service struct {
	configResolver ports.ConfigResolver
	presetResolver ports.PresetResolver
}

// NewService creates a new file summarization configuration service.
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

// Get resolves the summarization configuration for a given file.
func (s *Service) Get(filename string) (*summarize2.ResolvedConfig, error) {
	if s == nil {
		return nil, fmt.Errorf("summarize service is nil")
	}

	currentConfig, err := s.configResolver.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get application config: %w", err)
	}

	if currentConfig.Activities == nil || currentConfig.Activities.Summarize == nil {
		return nil, nil
	}

	activityCfg := currentConfig.Activities.Summarize
	matchingRule := findMatchingRule(activityCfg, filename)

	if matchingRule != nil && matchingRule.Skip {
		return &summarize2.ResolvedConfig{
			Skip: true,
		}, nil
	}

	presetName, strategy, systemPrompt := resolveSettings(activityCfg, matchingRule)

	resolvedPreset, err := s.presetResolver.Get(preset.Preset(presetName))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve preset: %w", err)
	}

	if strategy == nil {
		strategy = defaultStrategy()
	}

	return &summarize2.ResolvedConfig{
		Preset:              resolvedPreset,
		Strategy:            strategy,
		SystemPrompt:        systemPrompt,
		Skip:                false,
		IncludeOriginalFile: strategy.IncludeOriginalFile,
		IncludeChangedFile:  strategy.IncludeChangedFile,
	}, nil
}

func findMatchingRule(cfg *config.SummarizeActivityConfig, filename string) *config.SummarizeRule {
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
	summarizeConfig *config.SummarizeActivityConfig,
	rule *config.SummarizeRule,
) (presetName string, strategy *config.StrategyConfig, systemPrompt string) {
	if rule != nil {
		applyRuleSettings(rule, &presetName, &strategy, &systemPrompt)
	}

	applyDefaultSettings(summarizeConfig, &presetName, &strategy, &systemPrompt)

	return presetName, strategy, systemPrompt
}

func applyRuleSettings(
	rule *config.SummarizeRule,
	presetName *string,
	strategy **config.StrategyConfig,
	systemPrompt *string,
) {
	if rule.Preset != "" {
		*presetName = rule.Preset
	}
	if rule.Strategy != nil {
		*strategy = rule.Strategy
	}
	if rule.SystemPrompt != "" {
		*systemPrompt = rule.SystemPrompt
	}
}

func applyDefaultSettings(
	defaults *config.SummarizeActivityConfig,
	presetName *string,
	strategy **config.StrategyConfig,
	systemPrompt *string,
) {
	if *presetName == "" {
		*presetName = defaults.Preset
	}
	if *strategy == nil {
		*strategy = defaults.Strategy
	}
	if *systemPrompt == "" {
		*systemPrompt = defaults.SystemPrompt
	}
}

func defaultStrategy() *config.StrategyConfig {
	return &config.StrategyConfig{
		Type:                "plain",
		IncludeOriginalFile: false,
		IncludeChangedFile:  false,
	}
}
