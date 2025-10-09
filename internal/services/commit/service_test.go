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

package commit

import (
	"fmt"
	"testing"

	"github.com/retran/meowg1k/internal/core/config"
	coreProfile "github.com/retran/meowg1k/internal/core/profile"
)

// mockConfigReader is a mock implementation of ConfigResolver for testing.
type mockConfigReader struct {
	Cfg *config.Config
}

func (m *mockConfigReader) Get() (*config.Config, error) {
	return m.Cfg, nil
}

// mockProfileResolver is a mock implementation of ProfileResolver for testing.
type mockProfileResolver struct {
	Profile *coreProfile.ResolvedProfile
	Err     error
}

func (m *mockProfileResolver) Get(p coreProfile.Profile) (*coreProfile.ResolvedProfile, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Profile, nil
}

func TestNewService(t *testing.T) {
	configSvc := &mockConfigReader{}
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
	resolvedProfile := &coreProfile.ResolvedProfile{
		Provider: "openai",
		Model:    "gpt-4",
	}

	configSvc := &mockConfigReader{
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

	result, err := service.GetCommitConfig()
	if err != nil {
		t.Errorf("GetCommitConfig failed: %v", err)
	}

	if result.Profile != resolvedProfile {
		t.Error("Profile not set correctly")
	}

	if result.SystemPrompt != "Test prompt" {
		t.Errorf("Expected 'Test prompt', got '%s'", result.SystemPrompt)
	}
}

func TestGetCommitConfigDefault(t *testing.T) {
	resolvedProfile := &coreProfile.ResolvedProfile{
		Provider: "openai",
		Model:    "gpt-4",
	}

	configSvc := &mockConfigReader{
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

	_, err = service.GetCommitConfig()
	if err == nil {
		t.Error("Expected error when Commit config is nil, got nil")
	}
}

func TestGetCommitConfigProfileError(t *testing.T) {
	configSvc := &mockConfigReader{
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

	_, err = service.GetCommitConfig()
	if err == nil {
		t.Error("Expected profile error, got nil")
	}
}
