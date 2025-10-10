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

package filter

import (
	"fmt"
	"testing"

	"github.com/retran/meowg1k/internal/domain/config"
)

// mockConfigResolver is a local mock implementation of configResolver for testing.
type mockConfigResolver struct {
	cfg *config.Config
}

func (m *mockConfigResolver) Get() (*config.Config, error) {
	return m.cfg, nil
}

func TestNewService(t *testing.T) {
	cfg := &config.Config{
		Filter: &config.FilterConfig{
			Ignore: []string{"*.tmp", ".git/**"},
		},
	}
	configResolver := &mockConfigResolver{cfg: cfg}

	service, err := NewService(configResolver)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	if service == nil {
		t.Error("NewService returned nil")
	}
}

func TestNewServiceNilFilter(t *testing.T) {
	cfg := &config.Config{
		Filter: nil,
	}
	configResolver := &mockConfigResolver{cfg: cfg}

	service, err := NewService(configResolver)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	if service == nil {
		t.Error("NewService returned nil")
	}
}

func TestIsIgnoredFile(t *testing.T) {
	cfg := &config.Config{
		Filter: &config.FilterConfig{
			Ignore: []string{"*.tmp", ".git/**"},
		},
	}
	configResolver := &mockConfigResolver{cfg: cfg}

	service, err := NewService(configResolver)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	tests := []struct {
		file     string
		expected bool
	}{
		{"file.txt", false},
		{"file.tmp", true},
		{".git/config", true},
		{"src/file.go", false},
	}

	for _, tt := range tests {
		result := service.IsIgnoredFile(tt.file)
		if result != tt.expected {
			t.Errorf("IsIgnoredFile(%s) = %v, expected %v", tt.file, result, tt.expected)
		}
	}
}

func TestIsIgnoredFileNoPatterns(t *testing.T) {
	cfg := &config.Config{
		Filter: &config.FilterConfig{
			Ignore: []string{},
		},
	}
	configResolver := &mockConfigResolver{cfg: cfg}

	service, err := NewService(configResolver)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	if service.IsIgnoredFile("anyfile.txt") {
		t.Error("Expected no files to be ignored when no patterns")
	}
}

func TestNewServiceWithNilConfigResolver(t *testing.T) {
	service, err := NewService(nil)
	if err == nil {
		t.Error("Expected error when config resolver is nil")
	}
	if service != nil {
		t.Error("Expected nil service when config resolver is nil")
	}

	expectedErrorMsg := "config resolver is nil"
	if err.Error() != expectedErrorMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedErrorMsg, err.Error())
	}
}

func TestNewServiceWithConfigError(t *testing.T) {
	cfg := &config.Config{
		Filter: &config.FilterConfig{
			Ignore: []string{"*.log"},
		},
	}
	configResolver := &mockConfigResolverWithError{cfg: cfg}

	service, err := NewService(configResolver)
	if err == nil {
		t.Error("Expected error when config resolver returns error")
	}
	if service != nil {
		t.Error("Expected nil service when config resolver returns error")
	}
}

func TestIsIgnoredFileWithNilService(t *testing.T) {
	var service *Service
	result := service.IsIgnoredFile("anyfile.txt")
	if result {
		t.Error("Expected false when service is nil")
	}
}

func TestIsIgnoredFileWithComplexPatterns(t *testing.T) {
	cfg := &config.Config{
		Filter: &config.FilterConfig{
			Ignore: []string{
				"*.log",
				"*.tmp",
				"node_modules/**",
				"build/**",
				"*.test.js",
				"**/temp/**",
			},
		},
	}
	configResolver := &mockConfigResolver{cfg: cfg}

	service, err := NewService(configResolver)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	tests := []struct {
		file     string
		expected bool
	}{
		{"app.log", true},
		{"data.tmp", true},
		{"src/app.js", false},
		{"node_modules/package/index.js", true},
		{"build/output.js", true},
		{"src/component.test.js", true},
		{"src/temp/file.txt", true},
		{"regular/path/file.go", false},
	}

	for _, tt := range tests {
		result := service.IsIgnoredFile(tt.file)
		if result != tt.expected {
			t.Errorf("IsIgnoredFile(%s) = %v, expected %v", tt.file, result, tt.expected)
		}
	}
}

type mockConfigResolverWithError struct {
	cfg *config.Config
}

func (m *mockConfigResolverWithError) Get() (*config.Config, error) {
	return nil, fmt.Errorf("config error")
}
