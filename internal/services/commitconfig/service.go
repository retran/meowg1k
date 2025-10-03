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
package commitconfig

import (
	"github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/profile"
)

// ResolvedCommitConfig represents the resolved configuration for generating a commit message.
type ResolvedCommitConfig struct {
	Profile      *profile.ResolvedProfile
	SystemPrompt string
}

// Service provides functionality for resolving commit configuration.
type Service interface {
	// GetCommitConfig returns the resolved configuration for generating commit messages.
	GetCommitConfig() (*ResolvedCommitConfig, error)
}

// serviceImpl is the concrete implementation of the Service interface.
type serviceImpl struct {
	configService  config.Service
	profileService profile.Service
}

// NewService creates a new instance of the commit config service.
func NewService(configService config.Service, profileService profile.Service) Service {
	return &serviceImpl{
		configService:  configService,
		profileService: profileService,
	}
}

// GetCommitConfig resolves the commit configuration.
func (s *serviceImpl) GetCommitConfig() (*ResolvedCommitConfig, error) {
	config := s.configService.GetConfig()

	var profileName string
	var systemPrompt string

	if config.Commit != nil {
		profileName = config.Commit.Profile
		systemPrompt = config.Commit.SystemPrompt
	}

	resolvedProfile, err := s.profileService.Get(profile.Profile(profileName))
	if err != nil {
		return nil, err
	}

	if systemPrompt == "" {
		systemPrompt = "You are an expert software engineer. Write a clear and descriptive commit message in the Conventional Commits format based on the provided change summaries."
	}

	return &ResolvedCommitConfig{
		Profile:      resolvedProfile,
		SystemPrompt: systemPrompt,
	}, nil
}
