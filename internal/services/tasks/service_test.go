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

package tasks

import (
	"testing"

	"github.com/retran/meowg1k/internal/models/config"
	configservice "github.com/retran/meowg1k/internal/services/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock services for testing
type mockCommandService struct {
	mock.Mock
}

func (m *mockCommandService) GetCommand() *cobra.Command {
	args := m.Called()
	return args.Get(0).(*cobra.Command)
}

func (m *mockCommandService) GetConfigPath() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *mockCommandService) GetTaskName() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *mockCommandService) GetUserPrompt() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *mockCommandService) GetSilentFlag() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

type mockManagerService struct {
	mock.Mock
}

func (m *mockManagerService) GetConfig() *config.Config {
	args := m.Called()
	return args.Get(0).(*config.Config)
}

func (m *mockManagerService) LoadConfig() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockManagerService) LoadConfigFromPath(configPath string) error {
	args := m.Called(configPath)
	return args.Error(0)
}

func (m *mockManagerService) LoadFromSources(sources ...configservice.ConfigSource) error {
	args := m.Called(sources)
	return args.Error(0)
}

func TestResolveTaskConfiguration_WithTask(t *testing.T) {
	mockCommand := &mockCommandService{}
	mockManager := &mockManagerService{}

	service := NewService(mockCommand, mockManager)

	// Mock command service methods
	mockCommand.On("GetTaskName").Return("test-task", nil)
	mockCommand.On("GetUserPrompt").Return("custom user prompt", nil)

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Tasks: map[string]*config.GenerateTask{
				"test-task": {
					Profile:      "test-profile",
					SystemPrompt: "Test system prompt",
					UserPrompt:   "Task user prompt",
				},
			},
		},
	}

	mockManager.On("GetConfig").Return(cfg)

	// Execute
	profileName, systemPrompt, userPrompt, err := service.ResolveTaskConfiguration()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "test-profile", profileName)
	assert.Equal(t, "Test system prompt", systemPrompt)
	assert.Equal(t, "custom user prompt", userPrompt) // Command-line override

	mockManager.AssertExpectations(t)
	mockCommand.AssertExpectations(t)
}

func TestResolveTaskConfiguration_WithTask_NoCommandLineOverride(t *testing.T) {
	mockCommand := &mockCommandService{}
	mockManager := &mockManagerService{}

	service := NewService(mockCommand, mockManager)

	// Mock command service methods with no user-prompt override
	mockCommand.On("GetTaskName").Return("test-task", nil)
	mockCommand.On("GetUserPrompt").Return("", nil)

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Tasks: map[string]*config.GenerateTask{
				"test-task": {
					Profile:      "test-profile",
					SystemPrompt: "Test system prompt",
					UserPrompt:   "Task user prompt",
				},
			},
		},
	}

	mockManager.On("GetConfig").Return(cfg)

	// Execute
	profileName, systemPrompt, userPrompt, err := service.ResolveTaskConfiguration()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "test-profile", profileName)
	assert.Equal(t, "Test system prompt", systemPrompt)
	assert.Equal(t, "Task user prompt", userPrompt) // From task config

	mockManager.AssertExpectations(t)
	mockCommand.AssertExpectations(t)
}

func TestResolveTaskConfiguration_WithTask_FallbackToDefault(t *testing.T) {
	mockCommand := &mockCommandService{}
	mockManager := &mockManagerService{}

	service := NewService(mockCommand, mockManager)

	mockCommand.On("GetTaskName").Return("test-task", nil)
	mockCommand.On("GetUserPrompt").Return("", nil) // No command-line override

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Default: &config.GenerateDefault{
				Profile:      "default-profile",
				SystemPrompt: "Default system prompt",
			},
			Tasks: map[string]*config.GenerateTask{
				"test-task": {
					// No profile specified - should fall back to default
					UserPrompt: "Task user prompt",
				},
			},
		},
	}

	mockManager.On("GetConfig").Return(cfg)

	// Execute
	profileName, systemPrompt, userPrompt, err := service.ResolveTaskConfiguration()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "default-profile", profileName)
	assert.Equal(t, "Default system prompt", systemPrompt) // From default (task didn't override)
	assert.Equal(t, "Task user prompt", userPrompt)

	mockManager.AssertExpectations(t)
	mockCommand.AssertExpectations(t)
}

func TestResolveTaskConfiguration_TaskNotFound(t *testing.T) {
	mockCommand := &mockCommandService{}
	mockManager := &mockManagerService{}

	service := NewService(mockCommand, mockManager)

	mockCommand.On("GetTaskName").Return("nonexistent-task", nil)

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Tasks: map[string]*config.GenerateTask{
				"test-task": {
					Profile: "test-profile",
				},
			},
		},
	}

	mockManager.On("GetConfig").Return(cfg)

	// Execute
	_, _, _, err := service.ResolveTaskConfiguration()

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task 'nonexistent-task' not found")

	mockManager.AssertExpectations(t)
	mockCommand.AssertExpectations(t)
}

func TestResolveTaskConfiguration_NoTasksConfigured(t *testing.T) {
	mockCommand := &mockCommandService{}
	mockManager := &mockManagerService{}

	service := NewService(mockCommand, mockManager)

	mockCommand.On("GetTaskName").Return("any-task", nil)

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Tasks: nil,
		},
	}

	mockManager.On("GetConfig").Return(cfg)

	// Execute
	_, _, _, err := service.ResolveTaskConfiguration()

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no tasks configured")

	mockManager.AssertExpectations(t)
	mockCommand.AssertExpectations(t)
}

func TestResolveTaskConfiguration_DefaultConfiguration(t *testing.T) {
	mockCommand := &mockCommandService{}
	mockManager := &mockManagerService{}

	service := NewService(mockCommand, mockManager)

	mockCommand.On("GetTaskName").Return("", nil) // No task specified
	mockCommand.On("GetUserPrompt").Return("Command line user prompt", nil)

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Default: &config.GenerateDefault{
				Profile:      "default-profile",
				SystemPrompt: "Default system prompt",
			},
		},
	}

	mockManager.On("GetConfig").Return(cfg)

	// Execute
	profileName, systemPrompt, userPrompt, err := service.ResolveTaskConfiguration()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "default-profile", profileName)
	assert.Equal(t, "Default system prompt", systemPrompt)
	assert.Equal(t, "Command line user prompt", userPrompt)

	mockManager.AssertExpectations(t)
	mockCommand.AssertExpectations(t)
}

func TestResolveTaskConfiguration_DefaultConfiguration_NoUserPrompt(t *testing.T) {
	mockCommand := &mockCommandService{}
	mockManager := &mockManagerService{}

	service := NewService(mockCommand, mockManager)

	mockCommand.On("GetTaskName").Return("", nil)   // No task specified
	mockCommand.On("GetUserPrompt").Return("", nil) // No user prompt

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Default: &config.GenerateDefault{
				Profile: "default-profile",
			},
		},
	}

	mockManager.On("GetConfig").Return(cfg)

	// Execute
	_, _, _, err := service.ResolveTaskConfiguration()

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user prompt is required")

	mockManager.AssertExpectations(t)
	mockCommand.AssertExpectations(t)
}

func TestResolveTaskConfiguration_NoDefaultProfile(t *testing.T) {
	mockCommand := &mockCommandService{}
	mockManager := &mockManagerService{}

	service := NewService(mockCommand, mockManager)

	mockCommand.On("GetTaskName").Return("", nil) // No task specified

	cfg := &config.Config{
		Generate: &config.GenerateConfig{
			Default: nil,
		},
	}

	mockManager.On("GetConfig").Return(cfg)

	// Execute
	_, _, _, err := service.ResolveTaskConfiguration()

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no default profile configured")

	mockManager.AssertExpectations(t)
	mockCommand.AssertExpectations(t)
}
