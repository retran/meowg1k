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
	"fmt"

	"github.com/retran/meowg1k/internal/core/config"
	"github.com/retran/meowg1k/internal/core/profile"
	"github.com/retran/meowg1k/internal/core/summarize"
	"github.com/retran/meowg1k/pkg/gitignore"
)

// ConfigReader reads the application configuration.
type ConfigReader interface {
	GetConfig() (*config.Config, error)
}

// ProfileResolver resolves profile configurations.
type ProfileResolver interface {
	Get(profile profile.Profile) (*profile.ResolvedProfile, error)
}

// Service resolves file summarization configurations.
type Service struct {
	configReader    ConfigReader
	profileResolver ProfileResolver
}

// NewService creates a new file summarization configuration service.
func NewService(configReader ConfigReader, profileResolver ProfileResolver) (*Service, error) {
	if configReader == nil {
		return nil, fmt.Errorf("config reader is nil")
	}

	if profileResolver == nil {
		return nil, fmt.Errorf("profile resolver is nil")
	}

	return &Service{
		configReader:    configReader,
		profileResolver: profileResolver,
	}, nil
}

// GetSummarizationConfig resolves the summarization configuration for a given file.
func (s *Service) GetSummarizationConfig(filename string) (*summarize.ResolvedConfig, error) {
	if s == nil {
		return nil, fmt.Errorf("summarize service is nil")
	}

	currentConfig, err := s.configReader.GetConfig()
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
		return &summarize.ResolvedConfig{
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

	return &summarize.ResolvedConfig{
		Profile:             resolvedProfile,
		Strategy:            strategy,
		SystemPrompt:        systemPrompt,
		Skip:                false,
		IncludeOriginalFile: strategy.IncludeOriginalFile,
		IncludeChangedFile:  strategy.IncludeChangedFile,
	}, nil
}
