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

package prconfig

import (
	"testing"

	"github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/profile"
)

// mockApplicationConfigReader is a mock implementation of ApplicationConfigReader for testing.
type mockApplicationConfigReader struct {
	Cfg *config.Config
}

func (m *mockApplicationConfigReader) GetConfig() *config.Config {
	return m.Cfg
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
	configSvc := &mockApplicationConfigReader{}
	profileSvc := &mockProfileResolver{}
	service := NewService(configSvc, profileSvc)

	if service == nil {
		t.Error("NewService returned nil")
	}
}

func TestGetPRConfig(t *testing.T) {
	resolvedProfile := &profile.ResolvedProfile{
		Provider: "openai",
		Model:    "gpt-4",
	}

	configSvc := &mockApplicationConfigReader{
		Cfg: &config.Config{
			PR: &config.CommandConfig{
				Profile:      "test",
				SystemPrompt: "Test PR prompt",
			},
		},
	}
	profileSvc := &mockProfileResolver{
		Profile: resolvedProfile,
	}

	service := NewService(configSvc, profileSvc)

	result, err := service.GetPRConfig()
	if err != nil {
		t.Errorf("GetPRConfig failed: %v", err)
	}

	if result.Profile != resolvedProfile {
		t.Error("Profile not set correctly")
	}

	if result.SystemPrompt != "Test PR prompt" {
		t.Errorf("Expected 'Test PR prompt', got '%s'", result.SystemPrompt)
	}
}

func TestGetPRConfigDefault(t *testing.T) {
	resolvedProfile := &profile.ResolvedProfile{
		Provider: "openai",
		Model:    "gpt-4",
	}

	configSvc := &mockApplicationConfigReader{
		Cfg: &config.Config{
			PR: nil,
		},
	}
	profileSvc := &mockProfileResolver{
		Profile: resolvedProfile,
	}

	service := NewService(configSvc, profileSvc)

	result, err := service.GetPRConfig()
	if err != nil {
		t.Errorf("GetPRConfig failed: %v", err)
	}

	expectedPrompt := "You are an expert software engineer. Write a clear and detailed Pull Request description based on the provided change summaries. Include a concise title and a detailed description explaining what changed and why."
	if result.SystemPrompt != expectedPrompt {
		t.Errorf("Expected default prompt, got '%s'", result.SystemPrompt)
	}
}

func TestGetPRConfigProfileError(t *testing.T) {
	configSvc := &mockApplicationConfigReader{
		Cfg: &config.Config{},
	}
	profileSvc := &mockProfileResolver{
		Err: profile.ErrProfileNotFound,
	}

	service := NewService(configSvc, profileSvc)

	_, err := service.GetPRConfig()
	if err != profile.ErrProfileNotFound {
		t.Errorf("Expected ErrProfileNotFound, got %v", err)
	}
}
