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

package index

import (
	"errors"
	"strings"
	"testing"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/profile"
)

// Mock implementations for testing

type mockConfigResolver struct {
	GetFunc func() (*config.Config, error)
}

func (m *mockConfigResolver) Get() (*config.Config, error) {
	if m.GetFunc != nil {
		return m.GetFunc()
	}
	return &config.Config{
		Index: &config.IndexConfig{
			Profile: "test-profile",
			Chunker: &config.ChunkerConfig{
				MaxRunes:     1000,
				OverlapRunes: 100,
			},
			BatchSize: 32,
		},
	}, nil
}

type mockProfileResolver struct {
	GetFunc func(prof profile.Profile) (*profile.ResolvedProfile, error)
}

func (m *mockProfileResolver) Get(prof profile.Profile) (*profile.ResolvedProfile, error) {
	if m.GetFunc != nil {
		return m.GetFunc(prof)
	}
	return &profile.ResolvedProfile{
		Name: string(prof),
	}, nil
}

func TestNewConfigService(t *testing.T) {
	t.Run("Valid parameters", func(t *testing.T) {
		configResolver := &mockConfigResolver{}
		profileResolver := &mockProfileResolver{}

		service, err := NewConfigService(configResolver, profileResolver)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if service == nil {
			t.Fatal("Expected service to be non-nil")
		}
	})

	t.Run("Nil configResolver", func(t *testing.T) {
		profileResolver := &mockProfileResolver{}

		service, err := NewConfigService(nil, profileResolver)
		if err == nil {
			t.Fatal("Expected error for nil configResolver")
		}
		if service != nil {
			t.Fatal("Expected service to be nil when error occurs")
		}
		if !strings.Contains(err.Error(), "config resolver is nil") {
			t.Errorf("Expected config resolver error, got: %v", err)
		}
	})

	t.Run("Nil profileResolver", func(t *testing.T) {
		configResolver := &mockConfigResolver{}

		service, err := NewConfigService(configResolver, nil)
		if err == nil {
			t.Fatal("Expected error for nil profileResolver")
		}
		if service != nil {
			t.Fatal("Expected service to be nil when error occurs")
		}
		if !strings.Contains(err.Error(), "profile resolver is nil") {
			t.Errorf("Expected profile resolver error, got: %v", err)
		}
	})
}

func TestConfigService_Get(t *testing.T) {
	t.Run("Successful configuration retrieval", func(t *testing.T) {
		configResolver := &mockConfigResolver{}
		profileResolver := &mockProfileResolver{}

		service, _ := NewConfigService(configResolver, profileResolver)
		cfg, err := service.Get()

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if cfg == nil {
			t.Fatal("Expected config to be non-nil")
		}
		if cfg.Profile == nil {
			t.Fatal("Expected profile to be non-nil")
		}
		if cfg.ChunkerMaxRunes != 1000 {
			t.Errorf("Expected ChunkerMaxRunes=1000, got %d", cfg.ChunkerMaxRunes)
		}
		if cfg.ChunkerOverlapRunes != 100 {
			t.Errorf("Expected ChunkerOverlapRunes=100, got %d", cfg.ChunkerOverlapRunes)
		}
		if cfg.BatchSize != 32 {
			t.Errorf("Expected BatchSize=32, got %d", cfg.BatchSize)
		}
	})

	t.Run("Nil service", func(t *testing.T) {
		var service *ConfigService = nil

		_, err := service.Get()
		if err == nil {
			t.Fatal("Expected error for nil service")
		}
		if !strings.Contains(err.Error(), "index config service is nil") {
			t.Errorf("Expected service nil error, got: %v", err)
		}
	})

	t.Run("ConfigResolver returns error", func(t *testing.T) {
		configResolver := &mockConfigResolver{
			GetFunc: func() (*config.Config, error) {
				return nil, errors.New("config error")
			},
		}
		service, _ := NewConfigService(configResolver, &mockProfileResolver{})

		_, err := service.Get()
		if err == nil {
			t.Fatal("Expected error from configResolver")
		}
		if !strings.Contains(err.Error(), "failed to get application config") {
			t.Errorf("Expected config error, got: %v", err)
		}
	})

	t.Run("Missing index configuration", func(t *testing.T) {
		configResolver := &mockConfigResolver{
			GetFunc: func() (*config.Config, error) {
				return &config.Config{
					Index: nil,
				}, nil
			},
		}
		service, _ := NewConfigService(configResolver, &mockProfileResolver{})

		_, err := service.Get()
		if err == nil {
			t.Fatal("Expected error for missing index config")
		}
		if !strings.Contains(err.Error(), "index configuration is missing") {
			t.Errorf("Expected missing index config error, got: %v", err)
		}
	})

	t.Run("Empty profile name", func(t *testing.T) {
		configResolver := &mockConfigResolver{
			GetFunc: func() (*config.Config, error) {
				return &config.Config{
					Index: &config.IndexConfig{
						Profile: "",
						Chunker: &config.ChunkerConfig{
							MaxRunes:     1000,
							OverlapRunes: 100,
						},
					},
				}, nil
			},
		}
		service, _ := NewConfigService(configResolver, &mockProfileResolver{})

		_, err := service.Get()
		if err == nil {
			t.Fatal("Expected error for empty profile")
		}
		if !strings.Contains(err.Error(), "index.profile is required") {
			t.Errorf("Expected profile required error, got: %v", err)
		}
	})

	t.Run("Missing chunker configuration", func(t *testing.T) {
		configResolver := &mockConfigResolver{
			GetFunc: func() (*config.Config, error) {
				return &config.Config{
					Index: &config.IndexConfig{
						Profile: "test-profile",
						Chunker: nil,
					},
				}, nil
			},
		}
		service, _ := NewConfigService(configResolver, &mockProfileResolver{})

		_, err := service.Get()
		if err == nil {
			t.Fatal("Expected error for missing chunker config")
		}
		if !strings.Contains(err.Error(), "index.chunker configuration is missing") {
			t.Errorf("Expected chunker config error, got: %v", err)
		}
	})

	t.Run("ProfileResolver returns error", func(t *testing.T) {
		profileResolver := &mockProfileResolver{
			GetFunc: func(prof profile.Profile) (*profile.ResolvedProfile, error) {
				return nil, errors.New("profile error")
			},
		}
		service, _ := NewConfigService(&mockConfigResolver{}, profileResolver)

		_, err := service.Get()
		if err == nil {
			t.Fatal("Expected error from profileResolver")
		}
		if !strings.Contains(err.Error(), "failed to resolve profile") {
			t.Errorf("Expected profile resolve error, got: %v", err)
		}
	})

	t.Run("Default batch size when zero", func(t *testing.T) {
		configResolver := &mockConfigResolver{
			GetFunc: func() (*config.Config, error) {
				return &config.Config{
					Index: &config.IndexConfig{
						Profile: "test-profile",
						Chunker: &config.ChunkerConfig{
							MaxRunes:     1000,
							OverlapRunes: 100,
						},
						BatchSize: 0, // Should default to 32
					},
				}, nil
			},
		}
		service, _ := NewConfigService(configResolver, &mockProfileResolver{})

		cfg, err := service.Get()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if cfg.BatchSize != 32 {
			t.Errorf("Expected default BatchSize=32, got %d", cfg.BatchSize)
		}
	})

	t.Run("Default batch size when negative", func(t *testing.T) {
		configResolver := &mockConfigResolver{
			GetFunc: func() (*config.Config, error) {
				return &config.Config{
					Index: &config.IndexConfig{
						Profile: "test-profile",
						Chunker: &config.ChunkerConfig{
							MaxRunes:     1000,
							OverlapRunes: 100,
						},
						BatchSize: -10, // Should default to 32
					},
				}, nil
			},
		}
		service, _ := NewConfigService(configResolver, &mockProfileResolver{})

		cfg, err := service.Get()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if cfg.BatchSize != 32 {
			t.Errorf("Expected default BatchSize=32, got %d", cfg.BatchSize)
		}
	})

	t.Run("Custom batch size", func(t *testing.T) {
		configResolver := &mockConfigResolver{
			GetFunc: func() (*config.Config, error) {
				return &config.Config{
					Index: &config.IndexConfig{
						Profile: "test-profile",
						Chunker: &config.ChunkerConfig{
							MaxRunes:     1000,
							OverlapRunes: 100,
						},
						BatchSize: 64,
					},
				}, nil
			},
		}
		service, _ := NewConfigService(configResolver, &mockProfileResolver{})

		cfg, err := service.Get()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if cfg.BatchSize != 64 {
			t.Errorf("Expected BatchSize=64, got %d", cfg.BatchSize)
		}
	})

	t.Run("Preserves chunker configuration", func(t *testing.T) {
		expectedMaxRunes := 2000
		expectedOverlapRunes := 200

		configResolver := &mockConfigResolver{
			GetFunc: func() (*config.Config, error) {
				return &config.Config{
					Index: &config.IndexConfig{
						Profile: "test-profile",
						Chunker: &config.ChunkerConfig{
							MaxRunes:     expectedMaxRunes,
							OverlapRunes: expectedOverlapRunes,
						},
						BatchSize: 32,
					},
				}, nil
			},
		}
		service, _ := NewConfigService(configResolver, &mockProfileResolver{})

		cfg, err := service.Get()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if cfg.ChunkerMaxRunes != expectedMaxRunes {
			t.Errorf("Expected ChunkerMaxRunes=%d, got %d", expectedMaxRunes, cfg.ChunkerMaxRunes)
		}
		if cfg.ChunkerOverlapRunes != expectedOverlapRunes {
			t.Errorf("Expected ChunkerOverlapRunes=%d, got %d", expectedOverlapRunes, cfg.ChunkerOverlapRunes)
		}
	})
}

func TestConfigService_Get_ReturnsResolvedConfig(t *testing.T) {
	t.Run("Returns valid ResolvedConfig structure", func(t *testing.T) {
		expectedProfile := &profile.ResolvedProfile{
			Name: "test-profile",
		}
		profileResolver := &mockProfileResolver{
			GetFunc: func(prof profile.Profile) (*profile.ResolvedProfile, error) {
				return expectedProfile, nil
			},
		}
		service, _ := NewConfigService(&mockConfigResolver{}, profileResolver)

		cfg, err := service.Get()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if cfg == nil {
			t.Fatal("Expected ResolvedConfig to be non-nil")
		}

		if cfg.Profile != expectedProfile {
			t.Error("Expected profile to match")
		}
	})
}
