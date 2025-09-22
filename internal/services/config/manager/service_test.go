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

package manager

import (
	"errors"
	"testing"

	"github.com/retran/meowg1k/internal/config"
	"github.com/retran/meowg1k/internal/services/config/loader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewServiceWithConfig(t *testing.T) {
	// Create a minimal config
	cfg := &config.Config{
		Profiles: map[string]*config.Profile{
			"test": {
				Provider: "openai",
				Model:    "gpt-4",
			},
		},
	}

	service := NewServiceWithConfig(cfg, "/test/config.yaml")

	if service == nil {
		t.Errorf("NewServiceWithConfig() returned nil")
	}

	// Verify interface compliance
	var _ Service = service
}

func TestNewServiceWithConfig_NilConfig(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewServiceWithConfig() with nil config should panic")
		}
	}()

	NewServiceWithConfig(nil, "/test/config.yaml")
}

func TestServiceImpl_GetConfig(t *testing.T) {
	// Create a test config
	cfg := &config.Config{
		Profiles: map[string]*config.Profile{
			"test": {
				Provider: "openai",
				Model:    "gpt-4",
			},
		},
	}

	service := NewServiceWithConfig(cfg, "/test/config.yaml").(*serviceImpl)

	result := service.GetConfig()
	if result == nil {
		t.Errorf("GetConfig() returned nil")
		return
	}

	if len(result.Profiles) != 1 {
		t.Errorf("GetConfig() returned config with %d profiles, want 1", len(result.Profiles))
	}

	if result.Profiles["test"].Provider != "openai" {
		t.Errorf("GetConfig() profile provider = %s, want openai", result.Profiles["test"].Provider)
	}
}

func TestServiceImpl_GetConfigPath(t *testing.T) {
	cfg := &config.Config{}
	expectedPath := "/custom/path/config.yaml"

	service := NewServiceWithConfig(cfg, expectedPath).(*serviceImpl)

	result := service.GetConfigPath()
	if result != expectedPath {
		t.Errorf("GetConfigPath() = %s, want %s", result, expectedPath)
	}
}

// Mock services for testing NewService
type mockLoaderService struct {
	mock.Mock
}

func (m *mockLoaderService) LoadConfig(configPath string) (*config.Config, error) {
	args := m.Called(configPath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*config.Config), args.Error(1)
}

func (m *mockLoaderService) LoadFromSources(sources ...loader.ConfigSource) (*config.Config, error) {
	args := m.Called(sources)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*config.Config), args.Error(1)
}

type mockValidatorService struct {
	mock.Mock
}

func (m *mockValidatorService) ValidateConfig(cfg *config.Config) error {
	args := m.Called(cfg)
	return args.Error(0)
}

func (m *mockValidatorService) ValidateProfile(profile *config.Profile, profileName string) error {
	args := m.Called(profile, profileName)
	return args.Error(0)
}

func (m *mockValidatorService) ValidateResolvedProfile(resolved *config.ResolvedProfile) error {
	args := m.Called(resolved)
	return args.Error(0)
}

func TestNewService(t *testing.T) {
	tests := []struct {
		name        string
		configPath  string
		setupMocks  func(*mockLoaderService, *mockValidatorService)
		expectError bool
		errorMsg    string
	}{
		{
			name:       "Successful service creation",
			configPath: "/test/config.yaml",
			setupMocks: func(loader *mockLoaderService, validator *mockValidatorService) {
				cfg := &config.Config{
					Profiles: map[string]*config.Profile{
						"test": {
							Provider: "openai",
							Model:    "gpt-4",
						},
					},
				}
				loader.On("LoadConfig", "/test/config.yaml").Return(cfg, nil)
				validator.On("ValidateConfig", cfg).Return(nil)
			},
			expectError: false,
		},
		{
			name:       "Config loading fails",
			configPath: "/invalid/config.yaml",
			setupMocks: func(loader *mockLoaderService, validator *mockValidatorService) {
				loader.On("LoadConfig", "/invalid/config.yaml").Return(nil, errors.New("file not found"))
			},
			expectError: true,
			errorMsg:    "failed to load configuration",
		},
		{
			name:       "Config validation fails",
			configPath: "/test/config.yaml",
			setupMocks: func(loader *mockLoaderService, validator *mockValidatorService) {
				cfg := &config.Config{
					Profiles: make(map[string]*config.Profile), // Empty profiles
				}
				loader.On("LoadConfig", "/test/config.yaml").Return(cfg, nil)
				validator.On("ValidateConfig", cfg).Return(errors.New("validation failed"))
			},
			expectError: true,
			errorMsg:    "configuration validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLoader := &mockLoaderService{}
			mockValidator := &mockValidatorService{}
			tt.setupMocks(mockLoader, mockValidator)

			service, err := NewService(tt.configPath, mockLoader, mockValidator)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, service)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, service)
				// Cast to implementation to access GetConfigPath
				impl := service.(*serviceImpl)
				assert.Equal(t, tt.configPath, impl.GetConfigPath())
				assert.NotNil(t, service.GetConfig())
			}

			mockLoader.AssertExpectations(t)
			mockValidator.AssertExpectations(t)
		})
	}
}
