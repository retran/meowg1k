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
	"testing"

	"github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/testutil/servicemocks"
)

func TestNewService(t *testing.T) {
	cfg := &config.Config{
		Filter: &config.FilterConfig{
			Ignore: []string{"*.tmp", ".git/**"},
		},
	}
	configSvc := &servicemocks.MockConfigService{Cfg: cfg}

	service := NewService(configSvc)

	if service == nil {
		t.Error("NewService returned nil")
	}
}

func TestNewServiceNilFilter(t *testing.T) {
	cfg := &config.Config{
		Filter: nil,
	}
	configSvc := &servicemocks.MockConfigService{Cfg: cfg}

	service := NewService(configSvc)

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
	configSvc := &servicemocks.MockConfigService{Cfg: cfg}

	service := NewService(configSvc)

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
	configSvc := &servicemocks.MockConfigService{Cfg: cfg}

	service := NewService(configSvc)

	if service.IsIgnoredFile("anyfile.txt") {
		t.Error("Expected no files to be ignored when no patterns")
	}
}
