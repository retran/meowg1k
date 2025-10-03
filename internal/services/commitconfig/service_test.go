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

package commitconfig

import (
	"testing"

	"github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/profile"
	"github.com/retran/meowg1k/internal/testutil/servicemocks"
)

func TestNewService(t *testing.T) {
	configSvc := &servicemocks.MockConfigService{}
	profileSvc := &servicemocks.MockProfileService{}
	service := NewService(configSvc, profileSvc)

	if service == nil {
		t.Error("NewService returned nil")
	}
}

func TestGetCommitConfig(t *testing.T) {
	resolvedProfile := &profile.ResolvedProfile{
		Provider: "openai",
		Model:    "gpt-4",
	}

	configSvc := &servicemocks.MockConfigService{
		Cfg: &config.Config{
			Commit: &config.CommandConfig{
				Profile:      "test",
				SystemPrompt: "Test prompt",
			},
		},
	}
	profileSvc := &servicemocks.MockProfileService{
		Profile: resolvedProfile,
	}

	service := NewService(configSvc, profileSvc)

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
	resolvedProfile := &profile.ResolvedProfile{
		Provider: "openai",
		Model:    "gpt-4",
	}

	configSvc := &servicemocks.MockConfigService{
		Cfg: &config.Config{
			Commit: nil,
		},
	}
	profileSvc := &servicemocks.MockProfileService{
		Profile: resolvedProfile,
	}

	service := NewService(configSvc, profileSvc)

	result, err := service.GetCommitConfig()
	if err != nil {
		t.Errorf("GetCommitConfig failed: %v", err)
	}

	expectedPrompt := "You are an expert software engineer. Write a clear and descriptive commit message in the Conventional Commits format based on the provided change summaries."
	if result.SystemPrompt != expectedPrompt {
		t.Errorf("Expected default prompt, got '%s'", result.SystemPrompt)
	}
}

func TestGetCommitConfigProfileError(t *testing.T) {
	configSvc := &servicemocks.MockConfigService{
		Cfg: &config.Config{},
	}
	profileSvc := &servicemocks.MockProfileService{
		Err: profile.ErrProfileNotFound,
	}

	service := NewService(configSvc, profileSvc)

	_, err := service.GetCommitConfig()
	if err != profile.ErrProfileNotFound {
		t.Errorf("Expected ErrProfileNotFound, got %v", err)
	}
}
