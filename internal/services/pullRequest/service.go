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

// Package pullRequest provides services for PR command configuration resolution.
package pullRequest

import (
	"fmt"

	"github.com/retran/meowg1k/internal/core/config"
	"github.com/retran/meowg1k/internal/core/profile"
	"github.com/retran/meowg1k/internal/core/pullRequest"
)

// ConfigReader reads the application configuration.
type ConfigReader interface {
	GetConfig() (*config.Config, error)
}

// ProfileResolver resolves profile configurations.
type ProfileResolver interface {
	Get(profile profile.Profile) (*profile.ResolvedProfile, error)
}

// Service resolves PR configuration from application config and profiles.
type Service struct {
	configReader    ConfigReader
	profileResolver ProfileResolver
}

// NewService creates a new PR configuration service.
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

// GetPRConfig resolves the PR configuration.
func (s *Service) GetPRConfig() (*pullRequest.ResolvedConfig, error) {
	if s == nil {
		return nil, fmt.Errorf("pull request service is nil")
	}

	config, err := s.configReader.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get application config: %w", err)
	}

	var profileName string
	var systemPrompt string

	if config.PR != nil {
		profileName = config.PR.Profile
		systemPrompt = config.PR.SystemPrompt
	}

	resolvedProfile, err := s.profileResolver.Get(profile.Profile(profileName))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve profile %q: %w", profileName, err)
	}

	if systemPrompt == "" {
		return nil, fmt.Errorf("failed to get system prompt")
	}

	return &pullRequest.ResolvedConfig{
		Profile:      resolvedProfile,
		SystemPrompt: systemPrompt,
	}, nil
}
