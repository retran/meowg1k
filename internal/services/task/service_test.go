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

	mdConfig "github.com/retran/meowg1k/internal/models/config"
	mdGateway "github.com/retran/meowg1k/internal/models/gateway"
	mdProfile "github.com/retran/meowg1k/internal/models/profile"
	"github.com/spf13/cobra"
)

// Mock implementations for testing

type mockCommandService struct {
	taskName   string
	userPrompt string
}

func (m *mockCommandService) GetTaskName() (string, error) {
	return m.taskName, nil
}

func (m *mockCommandService) GetUserPrompt() (string, error) {
	return m.userPrompt, nil
}

func (m *mockCommandService) GetCommand() *cobra.Command {
	return nil
}

func (m *mockCommandService) GetCommandName() string {
	return "test"
}

func (m *mockCommandService) GetConfigPath() (string, error) {
	return "", nil
}

func (m *mockCommandService) GetSilentFlag() (bool, error) {
	return false, nil
}

func (m *mockCommandService) GetStdIn() string {
	return ""
}

type mockConfigService struct {
	config *mdConfig.Config
}

func (m *mockConfigService) GetConfig() *mdConfig.Config {
	return m.config
}

type mockProfileService struct {
	profiles map[mdProfile.Profile]*mdProfile.ResolvedProfile
}

var errMockProfileNotFound = errors.New("mock profile not found")

func (m *mockProfileService) Get(profile mdProfile.Profile) (*mdProfile.ResolvedProfile, error) {
	if resolved, exists := m.profiles[profile]; exists {
		return resolved, nil
	}
	return nil, fmt.Errorf("%w: %s", errMockProfileNotFound, profile)
}

func TestNewServiceSuccess(t *testing.T) {
	// Setup mocks with valid configuration
	config := &mdConfig.Config{
		Generate: &mdConfig.GenerateConfig{
			Default: &mdConfig.GenerateDefault{
				Profile:      "test-profile",
				SystemPrompt: "Default system prompt",
			},
		},
	}

	commandSvc := &mockCommandService{
		taskName:   "",
		userPrompt: "Test user prompt",
	}

	configSvc := &mockConfigService{config: config}

	resolvedProfile := &mdProfile.ResolvedProfile{
		Provider: mdGateway.OpenAI,
		Model:    "gpt-4",
		Timeout:  5 * time.Minute,
	}

	profileSvc := &mockProfileService{
		profiles: map[mdProfile.Profile]*mdProfile.ResolvedProfile{
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
	config := &mdConfig.Config{
		Generate: &mdConfig.GenerateConfig{
			Default: &mdConfig.GenerateDefault{
				Profile:      "default-profile",
				SystemPrompt: "Default system prompt",
			},
			Tasks: map[string]*mdConfig.GenerateTask{
				"specific-task": {
					Profile:      "task-profile",
					SystemPrompt: "Task system prompt",
					UserPrompt:   "Task user prompt",
				},
			},
		},
	}

	commandSvc := &mockCommandService{
		taskName:   "specific-task",
		userPrompt: "Command user prompt", // Should override task user prompt
	}

	configSvc := &mockConfigService{config: config}

	taskProfile := &mdProfile.ResolvedProfile{
		Provider: mdGateway.OpenAI,
		Model:    "gpt-3.5-turbo",
	}

	profileSvc := &mockProfileService{
		profiles: map[mdProfile.Profile]*mdProfile.ResolvedProfile{
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
	config := &mdConfig.Config{
		Generate: &mdConfig.GenerateConfig{
			Default: &mdConfig.GenerateDefault{
				Profile:      "default-profile",
				SystemPrompt: "Default system prompt",
			},
			Tasks: map[string]*mdConfig.GenerateTask{
				"task-no-profile": {
					UserPrompt: "Task user prompt",
					// No Profile or SystemPrompt - should use defaults
				},
			},
		},
	}

	commandSvc := &mockCommandService{
		taskName:   "task-no-profile",
		userPrompt: "",
	}

	configSvc := &mockConfigService{config: config}

	defaultProfile := &mdProfile.ResolvedProfile{
		Provider: mdGateway.OpenAI,
		Model:    "gpt-4",
	}

	profileSvc := &mockProfileService{
		profiles: map[mdProfile.Profile]*mdProfile.ResolvedProfile{
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
		config      *mdConfig.Config
		commandSvc  *mockCommandService
		profileSvc  *mockProfileService
		expectedErr string
	}{
		{
			name:        "nil config",
			config:      nil,
			commandSvc:  &mockCommandService{},
			profileSvc:  &mockProfileService{},
			expectedErr: "no configuration available",
		},
		{
			name: "task not found",
			config: &mdConfig.Config{
				Generate: &mdConfig.GenerateConfig{
					Tasks: map[string]*mdConfig.GenerateTask{},
				},
			},
			commandSvc:  &mockCommandService{taskName: "non-existent-task"},
			profileSvc:  &mockProfileService{},
			expectedErr: "task not found in configuration: non-existent-task",
		},
		{
			name:        "no default configuration",
			config:      &mdConfig.Config{},
			commandSvc:  &mockCommandService{taskName: "", userPrompt: "test"},
			profileSvc:  &mockProfileService{},
			expectedErr: "no default configuration available",
		},
		{
			name: "no profile configured",
			config: &mdConfig.Config{
				Generate: &mdConfig.GenerateConfig{
					Default: &mdConfig.GenerateDefault{
						Profile:      "", // Empty profile
						SystemPrompt: "System prompt",
					},
				},
			},
			commandSvc:  &mockCommandService{userPrompt: "test"},
			profileSvc:  &mockProfileService{},
			expectedErr: "no profile configured",
		},
		{
			name: "user prompt required",
			config: &mdConfig.Config{
				Generate: &mdConfig.GenerateConfig{
					Default: &mdConfig.GenerateDefault{
						Profile:      "test-profile",
						SystemPrompt: "System prompt",
					},
				},
			},
			commandSvc: &mockCommandService{taskName: "", userPrompt: ""}, // No task name and no user prompt
			profileSvc: &mockProfileService{
				profiles: map[mdProfile.Profile]*mdProfile.ResolvedProfile{
					"test-profile": {},
				},
			},
			expectedErr: "user prompt is required",
		},
		{
			name: "profile resolution error",
			config: &mdConfig.Config{
				Generate: &mdConfig.GenerateConfig{
					Default: &mdConfig.GenerateDefault{
						Profile:      "non-existent-profile",
						SystemPrompt: "System prompt",
					},
				},
			},
			commandSvc:  &mockCommandService{userPrompt: "test"},
			profileSvc:  &mockProfileService{profiles: map[mdProfile.Profile]*mdProfile.ResolvedProfile{}},
			expectedErr: "failed to resolve profile 'non-existent-profile'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configSvc := &mockConfigService{config: tc.config}

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
	profile := &mdProfile.ResolvedProfile{
		Provider: mdGateway.OpenAI,
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
