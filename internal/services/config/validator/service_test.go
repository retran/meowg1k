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

package validator

import (
	"testing"
	"time"

	"github.com/retran/meowg1k/internal/config"
	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock registry service for testing
type mockRegistryService struct {
	mock.Mock
}

func (m *mockRegistryService) RegisterProvider(name string, definition config.ProviderDefinition) error {
	args := m.Called(name, definition)
	return args.Error(0)
}

func (m *mockRegistryService) GetProvider(name string) (config.ProviderDefinition, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return config.ProviderDefinition{}, args.Error(1)
	}
	return args.Get(0).(config.ProviderDefinition), args.Error(1)
}

func (m *mockRegistryService) ListProviders() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *mockRegistryService) HasProvider(name string) bool {
	args := m.Called(name)
	return args.Bool(0)
}

func (m *mockRegistryService) GetDefaultProfile(providerType gateway.Provider) config.Profile {
	args := m.Called(providerType)
	return args.Get(0).(config.Profile)
}

func TestNewService(t *testing.T) {
	mockRegistry := &mockRegistryService{}
	service := NewService(mockRegistry)

	assert.NotNil(t, service)
	assert.IsType(t, &serviceImpl{}, service)
}

func TestValidateConfig(t *testing.T) {
	mockRegistry := &mockRegistryService{}
	service := NewService(mockRegistry)

	tests := []struct {
		name        string
		config      *config.Config
		expectError bool
		errorMsg    string
		setupMocks  func()
	}{
		{
			name:        "Nil config",
			config:      nil,
			expectError: true,
			errorMsg:    "configuration cannot be nil",
			setupMocks:  func() {},
		},
		{
			name: "Config with no profiles",
			config: &config.Config{
				Profiles: make(map[string]*config.Profile),
			},
			expectError: true,
			errorMsg:    "at least one profile must be defined",
			setupMocks:  func() {},
		},
		{
			name: "Valid minimal config",
			config: &config.Config{
				Profiles: map[string]*config.Profile{
					"test": {
						Provider: "openai",
						Model:    "gpt-4",
					},
				},
			},
			expectError: false,
			setupMocks: func() {
				mockRegistry.On("HasProvider", "openai").Return(true)
			},
		},
		{
			name: "Config with invalid profile",
			config: &config.Config{
				Profiles: map[string]*config.Profile{
					"invalid": {
						Provider: "invalid-provider",
					},
				},
			},
			expectError: true,
			errorMsg:    "invalid provider",
			setupMocks: func() {
				mockRegistry.On("HasProvider", "invalid-provider").Return(false)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock expectations
			mockRegistry.ExpectedCalls = nil
			tt.setupMocks()

			err := service.ValidateConfig(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}

			mockRegistry.AssertExpectations(t)
		})
	}
}

func TestValidateProfile(t *testing.T) {
	mockRegistry := &mockRegistryService{}
	service := NewService(mockRegistry)

	tests := []struct {
		name        string
		profile     *config.Profile
		profileName string
		expectError bool
		errorMsg    string
		setupMocks  func()
	}{
		{
			name:        "Nil profile",
			profile:     nil,
			profileName: "test",
			expectError: true,
			errorMsg:    "profile cannot be nil",
			setupMocks:  func() {},
		},
		{
			name: "Profile without provider",
			profile: &config.Profile{
				Model: "gpt-4",
			},
			profileName: "test",
			expectError: true,
			errorMsg:    "provider is required",
			setupMocks:  func() {},
		},
		{
			name: "Profile with invalid provider",
			profile: &config.Profile{
				Provider: "invalid",
				Model:    "some-model",
			},
			profileName: "test",
			expectError: true,
			errorMsg:    "invalid provider",
			setupMocks: func() {
				mockRegistry.On("HasProvider", "invalid").Return(false)
			},
		},
		{
			name: "Valid profile",
			profile: &config.Profile{
				Provider: "openai",
				Model:    "gpt-4",
			},
			profileName: "test",
			expectError: false,
			setupMocks: func() {
				mockRegistry.On("HasProvider", "openai").Return(true)
			},
		},
		{
			name: "Profile with invalid timeout (too short)",
			profile: &config.Profile{
				Provider: "openai",
				Model:    "gpt-4",
				Timeout:  500 * time.Millisecond,
			},
			profileName: "test",
			expectError: true,
			errorMsg:    "timeout must be at least 1 second",
			setupMocks: func() {
				mockRegistry.On("HasProvider", "openai").Return(true)
			},
		},
		{
			name: "Profile with invalid timeout (too long)",
			profile: &config.Profile{
				Provider: "openai",
				Model:    "gpt-4",
				Timeout:  35 * time.Minute,
			},
			profileName: "test",
			expectError: true,
			errorMsg:    "timeout is too large",
			setupMocks: func() {
				mockRegistry.On("HasProvider", "openai").Return(true)
			},
		},
		{
			name: "Profile with invalid max input tokens (negative)",
			profile: &config.Profile{
				Provider:       "openai",
				Model:          "gpt-4",
				MaxInputTokens: -100,
			},
			profileName: "test",
			expectError: true,
			errorMsg:    "max input tokens must be positive",
			setupMocks: func() {
				mockRegistry.On("HasProvider", "openai").Return(true)
			},
		},
		{
			name: "Profile with invalid max input tokens (too large)",
			profile: &config.Profile{
				Provider:       "openai",
				Model:          "gpt-4",
				MaxInputTokens: 3000000,
			},
			profileName: "test",
			expectError: true,
			errorMsg:    "max input tokens is too large",
			setupMocks: func() {
				mockRegistry.On("HasProvider", "openai").Return(true)
			},
		},
		{
			name: "Profile with invalid max output tokens (negative)",
			profile: &config.Profile{
				Provider:        "openai",
				Model:           "gpt-4",
				MaxOutputTokens: -50,
			},
			profileName: "test",
			expectError: true,
			errorMsg:    "max output tokens must be positive",
			setupMocks: func() {
				mockRegistry.On("HasProvider", "openai").Return(true)
			},
		},
		{
			name: "Profile with invalid max output tokens (too large)",
			profile: &config.Profile{
				Provider:        "openai",
				Model:           "gpt-4",
				MaxOutputTokens: 300000,
			},
			profileName: "test",
			expectError: true,
			errorMsg:    "max output tokens is too large",
			setupMocks: func() {
				mockRegistry.On("HasProvider", "openai").Return(true)
			},
		},
		{
			name: "Profile without model but provider has default",
			profile: &config.Profile{
				Provider: "openai",
			},
			profileName: "test",
			expectError: false,
			setupMocks: func() {
				mockRegistry.On("HasProvider", "openai").Return(true)
				mockRegistry.On("GetDefaultProfile", gateway.OpenAI).Return(config.Profile{
					Model: "gpt-4",
				})
			},
		},
		{
			name: "Profile without model and provider has no default",
			profile: &config.Profile{
				Provider: "openai",
			},
			profileName: "test",
			expectError: true,
			errorMsg:    "model is required",
			setupMocks: func() {
				mockRegistry.On("HasProvider", "openai").Return(true)
				mockRegistry.On("GetDefaultProfile", gateway.OpenAI).Return(config.Profile{
					Model: "", // No default model
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock expectations
			mockRegistry.ExpectedCalls = nil
			tt.setupMocks()

			err := service.ValidateProfile(tt.profile, tt.profileName)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}

			mockRegistry.AssertExpectations(t)
		})
	}
}

func TestValidateResolvedProfile(t *testing.T) {
	mockRegistry := &mockRegistryService{}
	service := NewService(mockRegistry)

	tests := []struct {
		name        string
		resolved    *config.ResolvedProfile
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Nil resolved profile",
			resolved:    nil,
			expectError: true,
			errorMsg:    "resolved profile cannot be nil",
		},
		{
			name: "Resolved profile without model",
			resolved: &config.ResolvedProfile{
				MaxInputTokens:  100000,
				MaxOutputTokens: 4096,
				Timeout:         5 * time.Minute,
			},
			expectError: true,
			errorMsg:    "model name is required",
		},
		{
			name: "Valid resolved profile",
			resolved: &config.ResolvedProfile{
				Model:           "gpt-4",
				MaxInputTokens:  100000,
				MaxOutputTokens: 4096,
				Timeout:         5 * time.Minute,
			},
			expectError: false,
		},
		{
			name: "Resolved profile with invalid timeout",
			resolved: &config.ResolvedProfile{
				Model:           "gpt-4",
				MaxInputTokens:  100000,
				MaxOutputTokens: 4096,
				Timeout:         500 * time.Millisecond,
			},
			expectError: true,
			errorMsg:    "timeout must be at least 1 second",
		},
		{
			name: "Resolved profile with too large max output tokens",
			resolved: &config.ResolvedProfile{
				Model:           "gpt-4",
				MaxInputTokens:  100000,
				MaxOutputTokens: 300000,
				Timeout:         5 * time.Minute,
			},
			expectError: true,
			errorMsg:    "max output tokens too large",
		},
		{
			name: "Resolved profile with too large max input tokens",
			resolved: &config.ResolvedProfile{
				Model:           "gpt-4",
				MaxInputTokens:  3000000,
				MaxOutputTokens: 4096,
				Timeout:         5 * time.Minute,
			},
			expectError: true,
			errorMsg:    "max input tokens too large",
		},
		{
			name: "Resolved profile with zero values (should get defaults)",
			resolved: &config.ResolvedProfile{
				Model:           "gpt-4",
				MaxInputTokens:  0,
				MaxOutputTokens: 0,
				Timeout:         0,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateResolvedProfile(tt.resolved)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				// Check that defaults were applied for zero values
				if tt.resolved != nil && !tt.expectError {
					if tt.resolved.Timeout == 0 {
						assert.Equal(t, 5*time.Minute, tt.resolved.Timeout)
					}
					if tt.resolved.MaxOutputTokens == 0 {
						assert.Equal(t, 4096, tt.resolved.MaxOutputTokens)
					}
					if tt.resolved.MaxInputTokens == 0 {
						assert.Equal(t, 128000, tt.resolved.MaxInputTokens)
					}
				}
			}
		})
	}
}

func TestValidateGenerateConfig(t *testing.T) {
	mockRegistry := &mockRegistryService{}
	service := NewService(mockRegistry)

	tests := []struct {
		name        string
		config      *config.GenerateConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Nil generate config",
			config:      nil,
			expectError: false,
		},
		{
			name: "Valid generate config with default",
			config: &config.GenerateConfig{
				Default: &config.GenerateDefault{
					Profile: "test-profile",
				},
			},
			expectError: false,
		},
		{
			name: "Valid generate config with tasks",
			config: &config.GenerateConfig{
				Tasks: map[string]*config.GenerateTask{
					"test-task": {
						UserPrompt: "Test prompt",
					},
				},
			},
			expectError: false,
		},
		{
			name: "Generate config with invalid default",
			config: &config.GenerateConfig{
				Default: &config.GenerateDefault{
					Profile: "", // Empty profile
				},
			},
			expectError: true,
			errorMsg:    "profile reference is required",
		},
		{
			name: "Generate config with invalid task",
			config: &config.GenerateConfig{
				Tasks: map[string]*config.GenerateTask{
					"test-task": {
						UserPrompt: "", // Empty prompt
					},
				},
			},
			expectError: true,
			errorMsg:    "user prompt is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.(*serviceImpl).validateGenerateConfig(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateGenerateDefault(t *testing.T) {
	mockRegistry := &mockRegistryService{}
	service := NewService(mockRegistry)

	tests := []struct {
		name        string
		config      *config.GenerateDefault
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Nil default config",
			config:      nil,
			expectError: false,
		},
		{
			name: "Valid default config",
			config: &config.GenerateDefault{
				Profile: "test-profile",
			},
			expectError: false,
		},
		{
			name: "Default config with empty profile",
			config: &config.GenerateDefault{
				Profile: "",
			},
			expectError: true,
			errorMsg:    "profile reference is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.(*serviceImpl).validateGenerateDefault(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateGenerateTask(t *testing.T) {
	mockRegistry := &mockRegistryService{}
	service := NewService(mockRegistry)

	tests := []struct {
		name        string
		task        *config.GenerateTask
		taskName    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Nil task",
			task:        nil,
			taskName:    "test-task",
			expectError: true,
			errorMsg:    "task cannot be nil",
		},
		{
			name: "Valid task",
			task: &config.GenerateTask{
				UserPrompt: "Test prompt",
			},
			taskName:    "test-task",
			expectError: false,
		},
		{
			name: "Task with empty user prompt",
			task: &config.GenerateTask{
				UserPrompt: "",
			},
			taskName:    "test-task",
			expectError: true,
			errorMsg:    "user prompt is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.(*serviceImpl).validateGenerateTask(tt.task, tt.taskName)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFilterConfig(t *testing.T) {
	mockRegistry := &mockRegistryService{}
	service := NewService(mockRegistry)

	tests := []struct {
		name        string
		config      *config.FilterConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Nil filter config",
			config:      nil,
			expectError: false,
		},
		{
			name: "Valid filter config",
			config: &config.FilterConfig{
				Ignore: []string{"*.tmp", "*.log"},
			},
			expectError: false,
		},
		{
			name: "Filter config with empty ignore patterns",
			config: &config.FilterConfig{
				Ignore: []string{},
			},
			expectError: true,
			errorMsg:    "at least one ignore pattern should be specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.(*serviceImpl).validateFilterConfig(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateSummarizeConfig(t *testing.T) {
	mockRegistry := &mockRegistryService{}
	service := NewService(mockRegistry)

	tests := []struct {
		name        string
		config      *config.SummarizeConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Nil summarize config",
			config:      nil,
			expectError: false,
		},
		{
			name: "Valid summarize config with default",
			config: &config.SummarizeConfig{
				Default: &config.SummarizeDefault{
					Profile: "test-profile",
				},
			},
			expectError: false,
		},
		{
			name: "Valid summarize config with rules",
			config: &config.SummarizeConfig{
				Rules: []*config.SummarizeRule{
					{
						Match: "*.go",
						Strategy: &config.Strategy{
							Type: "structured",
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.(*serviceImpl).validateSummarizeConfig(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCommandConfig(t *testing.T) {
	mockRegistry := &mockRegistryService{}
	service := NewService(mockRegistry)

	tests := []struct {
		name        string
		config      *config.CommandConfig
		configType  string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Nil command config",
			config:      nil,
			configType:  "commit",
			expectError: false,
		},
		{
			name: "Valid command config",
			config: &config.CommandConfig{
				Profile:      "test-profile",
				SystemPrompt: "test system prompt",
			},
			configType:  "commit",
			expectError: false,
		},
		{
			name: "Command config with empty profile",
			config: &config.CommandConfig{
				Profile: "",
			},
			configType:  "commit",
			expectError: true,
			errorMsg:    "profile reference is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.(*serviceImpl).validateCommandConfig(tt.config, tt.configType)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateSummarizeDefault(t *testing.T) {
	mockRegistry := &mockRegistryService{}
	service := NewService(mockRegistry)

	tests := []struct {
		name        string
		config      *config.SummarizeDefault
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Nil summarize default config",
			config:      nil,
			expectError: false,
		},
		{
			name: "Valid summarize default config",
			config: &config.SummarizeDefault{
				Profile: "test-profile",
			},
			expectError: false,
		},
		{
			name: "Valid summarize default config with strategy",
			config: &config.SummarizeDefault{
				Profile: "test-profile",
				Strategy: &config.Strategy{
					Type: "structured",
				},
			},
			expectError: false,
		},
		{
			name: "Summarize default with empty profile",
			config: &config.SummarizeDefault{
				Profile: "",
			},
			expectError: true,
			errorMsg:    "profile reference is required",
		},
		{
			name: "Summarize default with invalid strategy",
			config: &config.SummarizeDefault{
				Profile: "test-profile",
				Strategy: &config.Strategy{
					Type: "", // Empty type
				},
			},
			expectError: true,
			errorMsg:    "strategy type is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.(*serviceImpl).validateSummarizeDefault(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateSummarizeRule(t *testing.T) {
	mockRegistry := &mockRegistryService{}
	service := NewService(mockRegistry)

	tests := []struct {
		name        string
		rule        *config.SummarizeRule
		index       int
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Nil summarize rule",
			rule:        nil,
			index:       0,
			expectError: true,
			errorMsg:    "rule cannot be nil",
		},
		{
			name: "Valid summarize rule",
			rule: &config.SummarizeRule{
				Match: "*.go",
			},
			index:       0,
			expectError: false,
		},
		{
			name: "Valid summarize rule with strategy",
			rule: &config.SummarizeRule{
				Match: "*.go",
				Strategy: &config.Strategy{
					Type: "structured",
				},
			},
			index:       0,
			expectError: false,
		},
		{
			name: "Summarize rule with empty match",
			rule: &config.SummarizeRule{
				Match: "",
			},
			index:       0,
			expectError: true,
			errorMsg:    "match pattern is required",
		},
		{
			name: "Summarize rule with invalid strategy",
			rule: &config.SummarizeRule{
				Match: "*.go",
				Strategy: &config.Strategy{
					Type: "invalid",
				},
			},
			index:       0,
			expectError: true,
			errorMsg:    "invalid strategy type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.(*serviceImpl).validateSummarizeRule(tt.rule, tt.index)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateStrategy(t *testing.T) {
	mockRegistry := &mockRegistryService{}
	service := NewService(mockRegistry)

	tests := []struct {
		name        string
		strategy    *config.Strategy
		fieldPath   string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Nil strategy",
			strategy:    nil,
			fieldPath:   "test.strategy",
			expectError: false,
		},
		{
			name: "Valid strategy - plaintext",
			strategy: &config.Strategy{
				Type: "plaintext",
			},
			fieldPath:   "test.strategy",
			expectError: false,
		},
		{
			name: "Valid strategy - diff",
			strategy: &config.Strategy{
				Type: "diff",
			},
			fieldPath:   "test.strategy",
			expectError: false,
		},
		{
			name: "Valid strategy - structured",
			strategy: &config.Strategy{
				Type: "structured",
			},
			fieldPath:   "test.strategy",
			expectError: false,
		},
		{
			name: "Strategy with empty type",
			strategy: &config.Strategy{
				Type: "",
			},
			fieldPath:   "test.strategy",
			expectError: true,
			errorMsg:    "strategy type is required",
		},
		{
			name: "Strategy with invalid type",
			strategy: &config.Strategy{
				Type: "invalid",
			},
			fieldPath:   "test.strategy",
			expectError: true,
			errorMsg:    "invalid strategy type 'invalid'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.(*serviceImpl).validateStrategy(tt.strategy, tt.fieldPath)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
