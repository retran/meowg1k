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

package loader

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewService(t *testing.T) {
	service := NewService()
	if service == nil {
		t.Errorf("NewService() returned nil")
	}

	// Verify interface compliance
	var _ Service = service
}

func TestServiceImpl_LoadFromSources_NoSources(t *testing.T) {
	service := NewService()
	_, err := service.LoadFromSources()
	if err == nil {
		t.Errorf("LoadFromSources() with no sources should return error")
	}
	expectedErrMsg := "no configuration sources provided"
	if err.Error() != expectedErrMsg {
		t.Errorf("LoadFromSources() error = %v, want %v", err.Error(), expectedErrMsg)
	}
}

func TestServiceImpl_LoadFromSources_WithValidSource(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	configContent := `
profiles:
  test:
    provider: "openai"
    model: "gpt-4"
    maxOutputTokens: 1000
    timeout: "5m"

generate:
  default:
    profile: "test"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	service := NewService()
	source := NewFileSource(configFile, 10, "test")

	cfg, err := service.LoadFromSources(source)
	if err != nil {
		t.Errorf("LoadFromSources() error = %v", err)
		return
	}

	if cfg == nil {
		t.Errorf("LoadFromSources() returned nil config")
		return
	}

	// Verify the configuration was loaded correctly
	if cfg.Profiles == nil || cfg.Profiles["test"] == nil {
		t.Errorf("LoadFromSources() did not load profiles correctly")
		return
	}

	profile := cfg.Profiles["test"]
	if profile.Provider != "openai" {
		t.Errorf("LoadFromSources() profile provider = %v, want openai", profile.Provider)
	}
	if profile.Model != "gpt-4" {
		t.Errorf("LoadFromSources() profile model = %v, want gpt-4", profile.Model)
	}
	if profile.MaxOutputTokens != 1000 {
		t.Errorf("LoadFromSources() profile maxOutputTokens = %v, want 1000", profile.MaxOutputTokens)
	}
	if profile.Timeout != 5*time.Minute {
		t.Errorf("LoadFromSources() profile timeout = %v, want 5m", profile.Timeout)
	}
}

func TestServiceImpl_LoadFromSources_MultipleSources(t *testing.T) {
	tempDir := t.TempDir()

	// Create base config
	baseConfigFile := filepath.Join(tempDir, "base.yaml")
	baseContent := `
profiles:
  base:
    provider: "openai"
    model: "gpt-3.5-turbo"
`
	err := os.WriteFile(baseConfigFile, []byte(baseContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create base config file: %v", err)
	}

	// Create override config
	overrideConfigFile := filepath.Join(tempDir, "override.yaml")
	overrideContent := `
profiles:
  base:
    model: "gpt-4"  # Override the model
  premium:
    provider: "anthropic"
    model: "claude-3-sonnet"
`
	err = os.WriteFile(overrideConfigFile, []byte(overrideContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create override config file: %v", err)
	}

	service := NewService()

	// Lower priority source
	baseSource := NewFileSource(baseConfigFile, 10, "base")
	// Higher priority source
	overrideSource := NewFileSource(overrideConfigFile, 20, "override")

	cfg, err := service.LoadFromSources(baseSource, overrideSource)
	if err != nil {
		t.Errorf("LoadFromSources() error = %v", err)
		return
	}

	// Verify the base profile was overridden
	if cfg.Profiles["base"].Model != "gpt-4" {
		t.Errorf("LoadFromSources() base profile model = %v, want gpt-4 (should be overridden)", cfg.Profiles["base"].Model)
	}

	// Verify the premium profile was added
	if cfg.Profiles["premium"] == nil {
		t.Errorf("LoadFromSources() premium profile not found")
	} else if cfg.Profiles["premium"].Provider != "anthropic" {
		t.Errorf("LoadFromSources() premium profile provider = %v, want anthropic", cfg.Profiles["premium"].Provider)
	}
}

func TestServiceImpl_LoadFromSources_ValidationError(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "invalid.yaml")

	// Create config with invalid reference
	configContent := `
generate:
  default:
    profile: "nonexistent"  # This profile doesn't exist
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	service := NewService()
	source := NewFileSource(configFile, 10, "test")

	_, err = service.LoadFromSources(source)
	if err == nil {
		t.Errorf("LoadFromSources() should return validation error for nonexistent profile reference")
	}
}

func TestServiceImpl_LoadConfig_ExplicitPath(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "explicit.yaml")

	configContent := `
profiles:
  explicit:
    provider: "anthropic"
    model: "claude-3-haiku"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	service := NewService()
	cfg, err := service.LoadConfig(configFile)
	if err != nil {
		t.Errorf("LoadConfig() error = %v", err)
		return
	}

	if cfg.Profiles["explicit"] == nil {
		t.Errorf("LoadConfig() did not load explicit config")
	}
}

func TestServiceImpl_LoadConfig_StandardLocations(t *testing.T) {
	service := NewService()

	// This will attempt to load from standard locations
	// Since we can't guarantee the existence of config files,
	// we just test that it doesn't panic and handles the case gracefully
	_, err := service.LoadConfig("")

	// We expect either a successful load or a "no configuration found" error
	// Both are acceptable since we don't control the environment
	if err != nil {
		// Check that it's the expected type of error
		if err.Error() != "no configuration found in any source" &&
			!containsString(err.Error(), "failed to load configuration") {
			t.Errorf("LoadConfig() unexpected error type: %v", err)
		}
	}
}

func TestFileSource(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test.yaml")

	configContent := `
test:
  value: "hello"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	source := NewFileSource(configFile, 15, "testfile")

	// Test methods
	if source.Priority() != 15 {
		t.Errorf("FileSource Priority() = %v, want 15", source.Priority())
	}
	if source.Name() != "testfile" {
		t.Errorf("FileSource Name() = %v, want testfile", source.Name())
	}

	// Test loading
	settings, err := source.Load()
	if err != nil {
		t.Errorf("FileSource Load() error = %v", err)
		return
	}

	if settings["test"] == nil {
		t.Errorf("FileSource Load() did not load test section")
	}
}

func TestFileSource_NonexistentFile(t *testing.T) {
	source := NewFileSource("/nonexistent/path/config.yaml", 10, "nonexistent")

	settings, err := source.Load()
	if err != nil {
		t.Errorf("FileSource Load() with nonexistent file should not error, got: %v", err)
		return
	}

	if len(settings) != 0 {
		t.Errorf("FileSource Load() with nonexistent file should return empty settings, got: %v", settings)
	}
}

func TestDirectorySource(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	configContent := `
directory:
  test: true
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	source := NewDirectorySource(tempDir, "config.yaml", 25, "testdir")

	// Test methods
	if source.Priority() != 25 {
		t.Errorf("DirectorySource Priority() = %v, want 25", source.Priority())
	}
	if source.Name() != "testdir" {
		t.Errorf("DirectorySource Name() = %v, want testdir", source.Name())
	}

	// Test loading
	settings, err := source.Load()
	if err != nil {
		t.Errorf("DirectorySource Load() error = %v", err)
		return
	}

	if settings["directory"] == nil {
		t.Errorf("DirectorySource Load() did not load directory section")
	}
}

func TestDirectorySource_NonexistentDirectory(t *testing.T) {
	source := NewDirectorySource("/nonexistent/path", "config.yaml", 10, "nonexistent")

	settings, err := source.Load()
	if err != nil {
		t.Errorf("DirectorySource Load() with nonexistent directory should not error, got: %v", err)
		return
	}

	if len(settings) != 0 {
		t.Errorf("DirectorySource Load() with nonexistent directory should return empty settings, got: %v", settings)
	}
}

func TestConfigSourceInterface(t *testing.T) {
	// Test that our concrete types implement the interface
	var _ ConfigSource = &fileSource{}
	var _ ConfigSource = &directorySource{}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) &&
		(len(substr) == 0 ||
			s[:len(substr)] == substr ||
			containsString(s[1:], substr))
}
