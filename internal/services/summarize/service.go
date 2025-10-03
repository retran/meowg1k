/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package summarize provides services for file summarization configuration and rule matching.
package summarize

import (
	"github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/profile"
	"github.com/retran/meowg1k/pkg/gitignore"
)

// ResolvedSummarizationConfig holds the resolved summarization configuration for a specific file.
type ResolvedSummarizationConfig struct {
	Profile             *profile.ResolvedProfile
	Strategy            *config.Strategy
	SystemPrompt        string
	Skip                bool
	IncludeOriginalFile bool
	IncludeChangedFile  bool
}

// Service provides functionality for resolving file summarization configuration.
type Service interface {
	// GetSummarizationConfig returns the resolved configuration for summarizing a specific file.
	GetSummarizationConfig(filename string) (*ResolvedSummarizationConfig, error)
}

// serviceImpl is the concrete implementation of the Service interface.
type serviceImpl struct {
	configService  config.Service
	profileService profile.Service
}

// NewService creates a new instance of the summarization service.
func NewService(configService config.Service, profileService profile.Service) Service {
	return &serviceImpl{
		configService:  configService,
		profileService: profileService,
	}
}

// GetSummarizationConfig resolves the summarization configuration for a given file.
func (s *serviceImpl) GetSummarizationConfig(filename string) (*ResolvedSummarizationConfig, error) {
	currentConfig := s.configService.GetConfig()

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
		return &ResolvedSummarizationConfig{
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

	resolvedProfile, err := s.profileService.Get(profile.Profile(profileName))
	if err != nil {
		return nil, err
	}

	if strategy == nil {
		strategy = &config.Strategy{
			Type:                "plain",
			IncludeOriginalFile: false,
			IncludeChangedFile:  false,
		}
	}

	return &ResolvedSummarizationConfig{
		Profile:             resolvedProfile,
		Strategy:            strategy,
		SystemPrompt:        systemPrompt,
		Skip:                false,
		IncludeOriginalFile: strategy.IncludeOriginalFile,
		IncludeChangedFile:  strategy.IncludeChangedFile,
	}, nil
}
