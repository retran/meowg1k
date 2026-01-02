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
	"github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/internal/domain/provider"
	task2 "github.com/retran/meowg1k/internal/domain/task"
)

// Mock implementations for testing

var errMockPresetNotFound = errors.New("mock preset not found")

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

// mockPresetResolver is a mock implementation of PresetResolver for testing.
type mockPresetResolver struct {
	presets map[preset.Preset]*preset.ResolvedPreset
}

func (m *mockPresetResolver) Get(presetID preset.Preset) (*preset.ResolvedPreset, error) {
	if resolved, exists := m.presets[presetID]; exists {
		return resolved, nil
	}
	return nil, fmt.Errorf("%w: %s", errMockPresetNotFound, presetID)
}

func TestNewServiceSuccess(t *testing.T) {
	// Setup mocks with valid configuration
	cfg := &config.Config{
		Flows: &config.FlowsConfig{
			Write: &config.WriteFlowConfig{
				Preset:       "test-preset",
				SystemPrompt: "Default system prompt",
			},
		},
	}

	commandSvc := &mockTaskParametersReader{
		TaskName:   "",
		UserPrompt: "Test user prompt",
	}

	configSvc := &mockConfigResolver{Cfg: cfg}

	resolvedPreset := &preset.ResolvedPreset{
		Provider: provider.OpenAI,
		Model:    "gpt-4",
		Timeout:  5 * time.Minute,
	}

	presetSvc := &mockPresetResolver{
		presets: map[preset.Preset]*preset.ResolvedPreset{
			"test-preset": resolvedPreset,
		},
	}

	// Test successful service creation
	service, err := NewService(commandSvc, configSvc, presetSvc)
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

	if taskConfig.Preset != resolvedPreset {
		t.Error("Expected resolved preset to be set")
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
		Flows: &config.FlowsConfig{
			Write: &config.WriteFlowConfig{
				Preset:       "default-preset",
				SystemPrompt: "Default system prompt",
				Tasks: map[string]*config.WriteTask{
					"specific-task": {
						Preset:       "task-preset",
						SystemPrompt: "Task system prompt",
						UserPrompt:   "Task user prompt",
					},
				},
			},
		},
	}

	commandSvc := &mockTaskParametersReader{
		TaskName:   "specific-task",
		UserPrompt: "Command user prompt", // Should override task user prompt
	}

	configSvc := &mockConfigResolver{Cfg: cfg}

	taskPreset := &preset.ResolvedPreset{
		Provider: provider.OpenAI,
		Model:    "gpt-3.5-turbo",
	}

	presetSvc := &mockPresetResolver{
		presets: map[preset.Preset]*preset.ResolvedPreset{
			"task-preset": taskPreset,
		},
	}

	service, err := NewService(commandSvc, configSvc, presetSvc)
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

	if taskConfig.Preset != taskPreset {
		t.Error("Expected task preset to be used")
	}
}

func TestNewServiceWithTaskFallbackToDefault(t *testing.T) {
	// Test task that uses default preset when task doesn't specify one
	cfg := &config.Config{
		Flows: &config.FlowsConfig{
			Write: &config.WriteFlowConfig{
				Preset:       "default-preset",
				SystemPrompt: "Default system prompt",
				Tasks: map[string]*config.WriteTask{
					"task-no-preset": {
						UserPrompt: "Task user prompt",
						// No Preset or SystemPrompt - should use defaults
					},
				},
			},
		},
	}

	commandSvc := &mockTaskParametersReader{
		TaskName:   "task-no-preset",
		UserPrompt: "",
	}

	configSvc := &mockConfigResolver{Cfg: cfg}

	defaultPreset := &preset.ResolvedPreset{
		Provider: provider.OpenAI,
		Model:    "gpt-4",
	}

	presetSvc := &mockPresetResolver{
		presets: map[preset.Preset]*preset.ResolvedPreset{
			"default-preset": defaultPreset,
		},
	}

	service, err := NewService(commandSvc, configSvc, presetSvc)
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

	if taskConfig.Preset != defaultPreset {
		t.Error("Expected default preset to be used")
	}
}

func TestNewServiceErrorCases(t *testing.T) {
	testCases := []struct {
		name        string
		config      *config.Config
		commandSvc  *mockTaskParametersReader
		presetSvc   *mockPresetResolver
		expectedErr string
	}{
		{
			name:        "nil config",
			config:      nil,
			commandSvc:  &mockTaskParametersReader{},
			presetSvc:   &mockPresetResolver{},
			expectedErr: "no configuration available",
		},
		{
			name: "task not found",
			config: &config.Config{
				Flows: &config.FlowsConfig{
					Write: &config.WriteFlowConfig{
						Tasks: map[string]*config.WriteTask{},
					},
				},
			},
			commandSvc:  &mockTaskParametersReader{TaskName: "non-existent-task"},
			presetSvc:   &mockPresetResolver{},
			expectedErr: "task not found in configuration: non-existent-task",
		},
		{
			name:        "no default configuration",
			config:      &config.Config{},
			commandSvc:  &mockTaskParametersReader{TaskName: "", UserPrompt: "test"},
			presetSvc:   &mockPresetResolver{},
			expectedErr: "no default write configuration available",
		},
		{
			name: "no preset configured",
			config: &config.Config{
				Flows: &config.FlowsConfig{
					Write: &config.WriteFlowConfig{
						Preset:       "", // Empty preset
						SystemPrompt: "System prompt",
					},
				},
			},
			commandSvc:  &mockTaskParametersReader{UserPrompt: "test"},
			presetSvc:   &mockPresetResolver{},
			expectedErr: "no preset configured",
		},
		{
			name: "user prompt required",
			config: &config.Config{
				Flows: &config.FlowsConfig{
					Write: &config.WriteFlowConfig{
						Preset:       "test-preset",
						SystemPrompt: "System prompt",
					},
				},
			},
			commandSvc: &mockTaskParametersReader{TaskName: "", UserPrompt: ""}, // No task name and no user prompt
			presetSvc: &mockPresetResolver{
				presets: map[preset.Preset]*preset.ResolvedPreset{
					"test-preset": {},
				},
			},
			expectedErr: "user prompt is required",
		},
		{
			name: "preset resolution error",
			config: &config.Config{
				Flows: &config.FlowsConfig{
					Write: &config.WriteFlowConfig{
						Preset:       "non-existent-preset",
						SystemPrompt: "System prompt",
					},
				},
			},
			commandSvc:  &mockTaskParametersReader{UserPrompt: "test"},
			presetSvc:   &mockPresetResolver{presets: map[preset.Preset]*preset.ResolvedPreset{}},
			expectedErr: "failed to resolve preset \"non-existent-preset\"",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configSvc := &mockConfigResolver{Cfg: tc.config}

			_, err := NewService(tc.commandSvc, configSvc, tc.presetSvc)

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
	resolvedPreset := &preset.ResolvedPreset{
		Model: "gpt-4",
	}

	resolvedConfig := &task2.ResolvedConfig{
		Name:         "test-task",
		Preset:       resolvedPreset,
		SystemPrompt: "Test system",
		UserPrompt:   "Test user",
	}

	if resolvedConfig.Name != "test-task" {
		t.Errorf("Expected Name 'test-task', got '%s'", resolvedConfig.Name)
	}

	if resolvedConfig.Preset != resolvedPreset {
		t.Error("Expected Preset to be set correctly")
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
