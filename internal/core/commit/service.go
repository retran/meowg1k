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

// Package commit provides adapters for commit command configuration resolution.
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
