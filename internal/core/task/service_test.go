// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/domain/provider"
	task2 "github.com/retran/meowg1k/internal/domain/task"
)

// Mock implementations for testing

var errMockProfileNotFound = errors.New("mock profile not found")

// mockTaskParametersReader is a mock implementation of ParametersReader for testing.
type mockTaskParametersReader struct {
	Err        error
	TaskName   string
	UserPrompt string
}

func (m *mockTaskParametersReader) GetTaskName() (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	return m.TaskName, nil
}

func (m *mockTaskParametersReader) GetUserPrompt() (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	return m.UserPrompt, nil
}

// mockConfigResolver is a mock implementation of ConfigResolver for testing.
type mockConfigResolver struct {
	Cfg *config.Config
}

func (m *mockConfigResolver) Get() (*config.Config, error) {
	return m.Cfg, nil
}

// mockProfileResolver is a mock implementation of ProfileResolver for testing.
type mockProfileResolver struct {
	profiles map[profile.Profile]*profile.ResolvedProfile
}

func (m *mockProfileResolver) Get(profileID profile.Profile) (*profile.ResolvedProfile, error) {
	if resolved, exists := m.profiles[profileID]; exists {
		return resolved, nil
	}
	return nil, fmt.Errorf("%w: %s", errMockProfileNotFound, profileID)
}

func TestNewServiceSuccess(t *testing.T) {
	// Setup mocks with valid configuration
	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Default: &config.GenerateDefault{
				Profile:      "test-profile",
				SystemPrompt: "Default system prompt",
			},
		},
	}

	commandSvc := &mockTaskParametersReader{
		TaskName:   "",
		UserPrompt: "Test user prompt",
	}

	configSvc := &mockConfigResolver{Cfg: cfg}

	resolvedProfile := &profile.ResolvedProfile{
		Provider: provider.OpenAI,
		Model:    "gpt-4",
		Timeout:  5 * time.Minute,
	}

	profileSvc := &mockProfileResolver{
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
	taskConfig, err := service.Get()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

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
	cfg := &config.Config{
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

	commandSvc := &mockTaskParametersReader{
		TaskName:   "specific-task",
		UserPrompt: "Command user prompt", // Should override task user prompt
	}

	configSvc := &mockConfigResolver{Cfg: cfg}

	taskProfile := &profile.ResolvedProfile{
		Provider: provider.OpenAI,
		Model:    "gpt-3.5-turbo",
	}

	profileSvc := &mockProfileResolver{
		profiles: map[profile.Profile]*profile.ResolvedProfile{
			"task-profile": taskProfile,
		},
	}

	service, err := NewService(commandSvc, configSvc, profileSvc)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	taskConfig, err := service.Get()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

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
	cfg := &config.Config{
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

	commandSvc := &mockTaskParametersReader{
		TaskName:   "task-no-profile",
		UserPrompt: "",
	}

	configSvc := &mockConfigResolver{Cfg: cfg}

	defaultProfile := &profile.ResolvedProfile{
		Provider: provider.OpenAI,
		Model:    "gpt-4",
	}

	profileSvc := &mockProfileResolver{
		profiles: map[profile.Profile]*profile.ResolvedProfile{
			"default-profile": defaultProfile,
		},
	}

	service, err := NewService(commandSvc, configSvc, profileSvc)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	taskConfig, err := service.Get()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

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
		commandSvc  *mockTaskParametersReader
		profileSvc  *mockProfileResolver
		expectedErr string
	}{
		{
			name:        "nil config",
			config:      nil,
			commandSvc:  &mockTaskParametersReader{},
			profileSvc:  &mockProfileResolver{},
			expectedErr: "no configuration available",
		},
		{
			name: "task not found",
			config: &config.Config{
				Generate: &config.GenerateConfig{
					Tasks: map[string]*config.GenerateTask{},
				},
			},
			commandSvc:  &mockTaskParametersReader{TaskName: "non-existent-task"},
			profileSvc:  &mockProfileResolver{},
			expectedErr: "task not found in configuration: non-existent-task",
		},
		{
			name:        "no default configuration",
			config:      &config.Config{},
			commandSvc:  &mockTaskParametersReader{TaskName: "", UserPrompt: "test"},
			profileSvc:  &mockProfileResolver{},
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
			commandSvc:  &mockTaskParametersReader{UserPrompt: "test"},
			profileSvc:  &mockProfileResolver{},
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
			commandSvc: &mockTaskParametersReader{TaskName: "", UserPrompt: ""}, // No task name and no user prompt
			profileSvc: &mockProfileResolver{
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
			commandSvc:  &mockTaskParametersReader{UserPrompt: "test"},
			profileSvc:  &mockProfileResolver{profiles: map[profile.Profile]*profile.ResolvedProfile{}},
			expectedErr: "failed to resolve profile 'non-existent-profile'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configSvc := &mockConfigResolver{Cfg: tc.config}

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
	resolvedProfile := &profile.ResolvedProfile{
		Model: "gpt-4",
	}

	resolvedConfig := &task2.ResolvedConfig{
		Name:         "test-task",
		Profile:      resolvedProfile,
		SystemPrompt: "Test system",
		UserPrompt:   "Test user",
	}

	if resolvedConfig.Name != "test-task" {
		t.Errorf("Expected Name 'test-task', got '%s'", resolvedConfig.Name)
	}

	if resolvedConfig.Profile != resolvedProfile {
		t.Error("Expected Profile to be set correctly")
	}

	if resolvedConfig.SystemPrompt != "Test system" {
		t.Errorf("Expected SystemPrompt 'Test system', got '%s'", resolvedConfig.SystemPrompt)
	}

	if resolvedConfig.UserPrompt != "Test user" {
		t.Errorf("Expected UserPrompt 'Test user', got '%s'", resolvedConfig.UserPrompt)
	}
}

func TestServiceImplStructure(t *testing.T) {
	// Test basic service implementation structure
	impl := &Service{}
	result, err := impl.Get()

	if err == nil {
		t.Error("Expected error for uninitialized service")
	}

	if result != nil {
		t.Error("Expected Get() to return nil for uninitialized service")
	}
}
