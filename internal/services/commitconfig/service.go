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
	"errors"
	"fmt"

	"github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/profile"
)

var (
	// ErrServiceIsNil indicates that the service is nil.
	ErrServiceIsNil = errors.New("service is nil")
	// ErrConfigReaderIsNil indicates that the config reader is nil.
	ErrConfigReaderIsNil = errors.New("config reader is nil")
	// ErrProfileResolverIsNil indicates that the profile resolver is nil.
	ErrProfileResolverIsNil = errors.New("profile resolver is nil")
)

// ConfigReader reads the application configuration.
type ConfigReader interface {
	GetConfig() (*config.Config, error)
}

// ProfileResolver resolves profile configurations.
type ProfileResolver interface {
	Get(profile profile.Profile) (*profile.ResolvedProfile, error)
}

// ResolvedCommitConfig represents the resolved configuration for generating a commit message.
type ResolvedCommitConfig struct {
	Profile      *profile.ResolvedProfile
	SystemPrompt string
}

// Service resolves commit configuration from application config and profiles.
type Service struct {
	configReader    ConfigReader
	profileResolver ProfileResolver
}

// NewService creates a new commit configuration service.
func NewService(configReader ConfigReader, profileResolver ProfileResolver) (*Service, error) {
	if configReader == nil {
		return nil, ErrConfigReaderIsNil
	}
	if profileResolver == nil {
		return nil, ErrProfileResolverIsNil
	}

	return &Service{
		configReader:    configReader,
		profileResolver: profileResolver,
	}, nil
}

// GetCommitConfig resolves the commit configuration.
func (s *Service) GetCommitConfig() (*ResolvedCommitConfig, error) {
	if s == nil {
		return nil, ErrServiceIsNil
	}
	if s.configReader == nil {
		return nil, ErrConfigReaderIsNil
	}
	if s.profileResolver == nil {
		return nil, ErrProfileResolverIsNil
	}

	config, err := s.configReader.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get application config: %w", err)
	}

	var profileName string
	var systemPrompt string

	if config.Commit != nil {
		profileName = config.Commit.Profile
		systemPrompt = config.Commit.SystemPrompt
	}

	resolvedProfile, err := s.profileResolver.Get(profile.Profile(profileName))
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
