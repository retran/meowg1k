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

package task

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/profile"
	"github.com/retran/meowg1k/internal/services/provider"
	"github.com/retran/meowg1k/internal/testutil/servicemocks"
)

// Mock implementations for testing

var errMockProfileNotFound = errors.New("mock profile not found")

type mockProfileService struct {
	profiles map[profile.Profile]*profile.ResolvedProfile
}

func (m *mockProfileService) Get(profile profile.Profile) (*profile.ResolvedProfile, error) {
	if resolved, exists := m.profiles[profile]; exists {
		return resolved, nil
	}
	return nil, fmt.Errorf("%w: %s", errMockProfileNotFound, profile)
}

func TestNewServiceSuccess(t *testing.T) {
	// Setup mocks with valid configuration
	config := &config.Config{
		Generate: &config.GenerateConfig{
			Default: &config.GenerateDefault{
				Profile:      "test-profile",
				SystemPrompt: "Default system prompt",
			},
		},
	}

	commandSvc := &servicemocks.MockCommandService{
		TaskName:   "",
		UserPrompt: "Test user prompt",
	}

	configSvc := &servicemocks.MockConfigService{Cfg: config}

	resolvedProfile := &profile.ResolvedProfile{
		Provider: provider.OpenAI,
		Model:    "gpt-4",
		Timeout:  5 * time.Minute,
	}

	profileSvc := &mockProfileService{
		profiles: map[profile.Profile]*profile.ResolvedProfile{
			"test-profile": resolvedProfile,
		},
	}

	// Test successful service creation
	service, err := NewService(commandSvc, configSvc, profileSvc)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if service == nil {
		t.Fatal("Service should not be nil")
	}

	// Test Get() method
	taskConfig := service.Get()
	if taskConfig == nil {
		t.Fatal("Task configuration should not be nil")
	}

	if taskConfig.Profile != resolvedProfile {
		t.Error("Expected resolved profile to be set")
	}

	if taskConfig.SystemPrompt != "Default system prompt" {
		t.Errorf("Expected system prompt 'Default system prompt', got '%s'", taskConfig.SystemPrompt)
	}

	if taskConfig.UserPrompt != "Test user prompt" {
		t.Errorf("Expected user prompt 'Test user prompt', got '%s'", taskConfig.UserPrompt)
	}
}

func TestNewServiceWithSpecificTask(t *testing.T) {
	// Setup configuration with a specific task
	config := &config.Config{
		Generate: &config.GenerateConfig{
			Default: &config.GenerateDefault{
				Profile:      "default-profile",
				SystemPrompt: "Default system prompt",
			},
			Tasks: map[string]*config.GenerateTask{
				"specific-task": {
					Profile:      "task-profile",
					SystemPrompt: "Task system prompt",
					UserPrompt:   "Task user prompt",
				},
			},
		},
	}

	commandSvc := &servicemocks.MockCommandService{
		TaskName:   "specific-task",
		UserPrompt: "Command user prompt", // Should override task user prompt
	}

	configSvc := &servicemocks.MockConfigService{Cfg: config}

	taskProfile := &profile.ResolvedProfile{
		Provider: provider.OpenAI,
		Model:    "gpt-3.5-turbo",
	}

	profileSvc := &mockProfileService{
		profiles: map[profile.Profile]*profile.ResolvedProfile{
			"task-profile": taskProfile,
		},
	}

	service, err := NewService(commandSvc, configSvc, profileSvc)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	taskConfig := service.Get()

	if taskConfig.Name != "specific-task" {
		t.Errorf("Expected task name 'specific-task', got '%s'", taskConfig.Name)
	}

	if taskConfig.SystemPrompt != "Task system prompt" {
		t.Errorf("Expected task system prompt, got '%s'", taskConfig.SystemPrompt)
	}

	if taskConfig.UserPrompt != "Command user prompt" {
		t.Errorf("Expected command user prompt to override task user prompt, got '%s'", taskConfig.UserPrompt)
	}

	if taskConfig.Profile != taskProfile {
		t.Error("Expected task profile to be used")
	}
}

func TestNewServiceWithTaskFallbackToDefault(t *testing.T) {
	// Test task that uses default profile when task doesn't specify one
	config := &config.Config{
		Generate: &config.GenerateConfig{
			Default: &config.GenerateDefault{
				Profile:      "default-profile",
				SystemPrompt: "Default system prompt",
			},
			Tasks: map[string]*config.GenerateTask{
				"task-no-profile": {
					UserPrompt: "Task user prompt",
					// No Profile or SystemPrompt - should use defaults
				},
			},
		},
	}

	commandSvc := &servicemocks.MockCommandService{
		TaskName:   "task-no-profile",
		UserPrompt: "",
	}

	configSvc := &servicemocks.MockConfigService{Cfg: config}

	defaultProfile := &profile.ResolvedProfile{
		Provider: provider.OpenAI,
		Model:    "gpt-4",
	}

	profileSvc := &mockProfileService{
		profiles: map[profile.Profile]*profile.ResolvedProfile{
			"default-profile": defaultProfile,
		},
	}

	service, err := NewService(commandSvc, configSvc, profileSvc)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	taskConfig := service.Get()

	if taskConfig.SystemPrompt != "Default system prompt" {
		t.Errorf("Expected default system prompt, got '%s'", taskConfig.SystemPrompt)
	}

	if taskConfig.UserPrompt != "Task user prompt" {
		t.Errorf("Expected task user prompt, got '%s'", taskConfig.UserPrompt)
	}

	if taskConfig.Profile != defaultProfile {
		t.Error("Expected default profile to be used")
	}
}

func TestNewServiceErrorCases(t *testing.T) {
	testCases := []struct {
		name        string
		config      *config.Config
		commandSvc  *servicemocks.MockCommandService
		profileSvc  *mockProfileService
		expectedErr string
	}{
		{
			name:        "nil config",
			config:      nil,
			commandSvc:  &servicemocks.MockCommandService{},
			profileSvc:  &mockProfileService{},
			expectedErr: "no configuration available",
		},
		{
			name: "task not found",
			config: &config.Config{
				Generate: &config.GenerateConfig{
					Tasks: map[string]*config.GenerateTask{},
				},
			},
			commandSvc:  &servicemocks.MockCommandService{TaskName: "non-existent-task"},
			profileSvc:  &mockProfileService{},
			expectedErr: "task not found in configuration: non-existent-task",
		},
		{
			name:        "no default configuration",
			config:      &config.Config{},
			commandSvc:  &servicemocks.MockCommandService{TaskName: "", UserPrompt: "test"},
			profileSvc:  &mockProfileService{},
			expectedErr: "no default configuration available",
		},
		{
			name: "no profile configured",
			config: &config.Config{
				Generate: &config.GenerateConfig{
					Default: &config.GenerateDefault{
						Profile:      "", // Empty profile
						SystemPrompt: "System prompt",
					},
				},
			},
			commandSvc:  &servicemocks.MockCommandService{UserPrompt: "test"},
			profileSvc:  &mockProfileService{},
			expectedErr: "no profile configured",
		},
		{
			name: "user prompt required",
			config: &config.Config{
				Generate: &config.GenerateConfig{
					Default: &config.GenerateDefault{
						Profile:      "test-profile",
						SystemPrompt: "System prompt",
					},
				},
			},
			commandSvc: &servicemocks.MockCommandService{TaskName: "", UserPrompt: ""}, // No task name and no user prompt
			profileSvc: &mockProfileService{
				profiles: map[profile.Profile]*profile.ResolvedProfile{
					"test-profile": {},
				},
			},
			expectedErr: "user prompt is required",
		},
		{
			name: "profile resolution error",
			config: &config.Config{
				Generate: &config.GenerateConfig{
					Default: &config.GenerateDefault{
						Profile:      "non-existent-profile",
						SystemPrompt: "System prompt",
					},
				},
			},
			commandSvc:  &servicemocks.MockCommandService{UserPrompt: "test"},
			profileSvc:  &mockProfileService{profiles: map[profile.Profile]*profile.ResolvedProfile{}},
			expectedErr: "failed to resolve profile 'non-existent-profile'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configSvc := &servicemocks.MockConfigService{Cfg: tc.config}

			_, err := NewService(tc.commandSvc, configSvc, tc.profileSvc)

			if err == nil {
				t.Errorf("Expected error for %s", tc.name)
				return
			}

			if tc.expectedErr != "" && !strings.Contains(err.Error(), tc.expectedErr) {
				t.Errorf("Expected error containing '%s', got '%s'", tc.expectedErr, err.Error())
			}
		})
	}
}

func TestConfigurationFields(t *testing.T) {
	// Test Configuration struct fields
	profile := &profile.ResolvedProfile{
		Provider: provider.OpenAI,
		Model:    "gpt-4",
	}

	config := &Configuration{
		Name:         "test-task",
		Profile:      profile,
		SystemPrompt: "Test system",
		UserPrompt:   "Test user",
	}

	if config.Name != "test-task" {
		t.Errorf("Expected Name 'test-task', got '%s'", config.Name)
	}

	if config.Profile != profile {
		t.Error("Expected Profile to be set correctly")
	}

	if config.SystemPrompt != "Test system" {
		t.Errorf("Expected SystemPrompt 'Test system', got '%s'", config.SystemPrompt)
	}

	if config.UserPrompt != "Test user" {
		t.Errorf("Expected UserPrompt 'Test user', got '%s'", config.UserPrompt)
	}
}

func TestServiceImplStructure(t *testing.T) {
	// Test that the service implements the interface
	var _ Service = (*serviceImpl)(nil)

	// Test basic service implementation structure
	impl := &serviceImpl{}
	if impl.Get() != nil {
		t.Error("Expected Get() to return nil for uninitialized service")
	}
}
