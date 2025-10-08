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

// Package prconfig provides services for PR command configuration resolution.
package prconfig

import (
	"github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/profile"
)

// ResolvedPRConfig represents the resolved configuration for generating a PR description.
type ResolvedPRConfig struct {
	Profile      *profile.ResolvedProfile
	SystemPrompt string
}

// ApplicationConfigReader reads the application configuration.
type ApplicationConfigReader interface {
	GetConfig() *config.Config
}

// ProfileResolver resolves profile configurations.
type ProfileResolver interface {
	Get(profile profile.Profile) (*profile.ResolvedProfile, error)
}

// Service resolves PR configuration from application config and profiles.
type Service struct {
	configReader    ApplicationConfigReader
	profileResolver ProfileResolver
}

// NewService creates a new PR configuration service.
func NewService(configReader ApplicationConfigReader, profileResolver ProfileResolver) *Service {
	return &Service{
		configReader:    configReader,
		profileResolver: profileResolver,
	}
}

// GetPRConfig resolves the PR configuration.
func (s *Service) GetPRConfig() (*ResolvedPRConfig, error) {
	config := s.configReader.GetConfig()

	var profileName string
	var systemPrompt string

	if config.PR != nil {
		profileName = config.PR.Profile
		systemPrompt = config.PR.SystemPrompt
	}

	resolvedProfile, err := s.profileResolver.Get(profile.Profile(profileName))
	if err != nil {
		return nil, err
	}

	if systemPrompt == "" {
		systemPrompt = "You are an expert software engineer. Write a clear and detailed Pull Request description based on the provided change summaries. Include a concise title and a detailed description explaining what changed and why."
	}

	return &ResolvedPRConfig{
		Profile:      resolvedProfile,
		SystemPrompt: systemPrompt,
	}, nil
}
