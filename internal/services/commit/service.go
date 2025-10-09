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

// Package commitconfig provides services for commit command configuration resolution.
package commit

import (
	"fmt"

	"github.com/retran/meowg1k/internal/core/commit"
	"github.com/retran/meowg1k/internal/core/config"
	"github.com/retran/meowg1k/internal/core/profile"
)

// ConfigReader reads the application configuration.
type ConfigReader interface {
	GetConfig() (*config.Config, error)
}

// ProfileResolver resolves profile configurations.
type ProfileResolver interface {
	Get(profile profile.Profile) (*profile.ResolvedProfile, error)
}

// Service resolves commit configuration from application config and profiles.
type Service struct {
	configReader    ConfigReader
	profileResolver ProfileResolver
}

// NewService creates a new commit configuration service.
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

// GetCommitConfig resolves the commit configuration.
func (s *Service) GetCommitConfig() (*commit.ResolvedConfig, error) {
	if s == nil {
		return nil, fmt.Errorf("commit service is nil")
	}

	if s.configReader == nil {
		return nil, fmt.Errorf("config reader is nil")
	}

	if s.profileResolver == nil {
		return nil, fmt.Errorf("profile resolver is nil")
	}

	cfg, err := s.configReader.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get application cfg: %w", err)
	}

	var profileName string
	var systemPrompt string

	if cfg.Commit != nil {
		profileName = cfg.Commit.Profile
		systemPrompt = cfg.Commit.SystemPrompt
	}

	resolvedProfile, err := s.profileResolver.Get(profile.Profile(profileName))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve profile %q: %w", profileName, err)
	}

	if systemPrompt == "" {
		return nil, fmt.Errorf("system prompt is required")
	}

	return &commit.ResolvedConfig{
		Profile:      resolvedProfile,
		SystemPrompt: systemPrompt,
	}, nil
}
