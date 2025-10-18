// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package commit

import (
	"fmt"
	"testing"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/profile"
)

// mockConfigResolver is a mock implementation of ConfigResolver for testing.
type mockConfigResolver struct {
	Cfg *config.Config
}

func (m *mockConfigResolver) Get() (*config.Config, error) {
	return m.Cfg, nil
}

// mockProfileResolver is a mock implementation of ProfileResolver for testing.
type mockProfileResolver struct {
	Profile *profile.ResolvedProfile
	Err     error
}

func (m *mockProfileResolver) Get(p profile.Profile) (*profile.ResolvedProfile, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Profile, nil
}

func TestNewService(t *testing.T) {
	configSvc := &mockConfigResolver{}
	profileSvc := &mockProfileResolver{}
	service, err := NewService(configSvc, profileSvc)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	if service == nil {
		t.Error("NewService returned nil")
	}
}

func TestGetCommitConfig(t *testing.T) {
	resolvedProfile := &profile.ResolvedProfile{
		Provider: "openai",
		Model:    "gpt-4",
	}

	configSvc := &mockConfigResolver{
		Cfg: &config.Config{
			Commit: &config.CommandConfig{
				Profile:      "test",
				SystemPrompt: "Test prompt",
			},
		},
	}
	profileSvc := &mockProfileResolver{
		Profile: resolvedProfile,
	}

	service, err := NewService(configSvc, profileSvc)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	result, err := service.Get()
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	if result.Profile != resolvedProfile {
		t.Error("Profile not set correctly")
	}

	if result.SystemPrompt != "Test prompt" {
		t.Errorf("Expected 'Test prompt', got '%s'", result.SystemPrompt)
	}
}

func TestGetCommitConfigDefault(t *testing.T) {
	resolvedProfile := &profile.ResolvedProfile{
		Provider: "openai",
		Model:    "gpt-4",
	}

	configSvc := &mockConfigResolver{
		Cfg: &config.Config{
			Commit: nil,
		},
	}

	profileSvc := &mockProfileResolver{
		Profile: resolvedProfile,
	}

	service, err := NewService(configSvc, profileSvc)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.Get()
	if err == nil {
		t.Error("Expected error when Commit config is nil, got nil")
	}
}

func TestGetCommitConfigProfileError(t *testing.T) {
	configSvc := &mockConfigResolver{
		Cfg: &config.Config{},
	}
	mockErr := fmt.Errorf("profile not found in configuration")
	profileSvc := &mockProfileResolver{
		Err: mockErr,
	}

	service, err := NewService(configSvc, profileSvc)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.Get()
	if err == nil {
		t.Error("Expected profile error, got nil")
	}
}

func TestNewServiceWithNilConfigResolver(t *testing.T) {
	profileSvc := &mockProfileResolver{}
	service, err := NewService(nil, profileSvc)
	if err == nil {
		t.Error("Expected error when config resolver is nil")
	}
	if service != nil {
		t.Error("Expected nil service when config resolver is nil")
	}
}

func TestNewServiceWithNilProfileResolver(t *testing.T) {
	configSvc := &mockConfigResolver{}
	service, err := NewService(configSvc, nil)
	if err == nil {
		t.Error("Expected error when profile resolver is nil")
	}
	if service != nil {
		t.Error("Expected nil service when profile resolver is nil")
	}
}

func TestGetWithNilService(t *testing.T) {
	var service *Service
	_, err := service.Get()
	if err == nil {
		t.Error("Expected error when service is nil")
	}
}

func TestGetWithConfigError(t *testing.T) {
	configSvc := &mockConfigResolverWithError{}
	profileSvc := &mockProfileResolver{
		Profile: &profile.ResolvedProfile{
			Provider: "openai",
			Model:    "gpt-4",
		},
	}

	service, err := NewService(configSvc, profileSvc)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.Get()
	if err == nil {
		t.Error("Expected error when config resolver returns error")
	}
}

func TestGetWithEmptySystemPrompt(t *testing.T) {
	resolvedProfile := &profile.ResolvedProfile{
		Provider: "openai",
		Model:    "gpt-4",
	}

	configSvc := &mockConfigResolver{
		Cfg: &config.Config{
			Commit: &config.CommandConfig{
				Profile:      "test",
				SystemPrompt: "", // Empty system prompt
			},
		},
	}
	profileSvc := &mockProfileResolver{
		Profile: resolvedProfile,
	}

	service, err := NewService(configSvc, profileSvc)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.Get()
	if err == nil {
		t.Error("Expected error when system prompt is empty")
	}

	expectedError := "system prompt is required"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

type mockConfigResolverWithError struct{}

func (m *mockConfigResolverWithError) Get() (*config.Config, error) {
	return nil, fmt.Errorf("config error")
}
