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

package dbpath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewService(t *testing.T) {
	service := NewService()
	if service == nil {
		t.Fatal("Service should not be nil")
	}
}

func TestGetMainDBPath(t *testing.T) {
	service := NewService()
	path, err := service.GetMainDBPath()
	if err != nil {
		t.Fatalf("GetMainDBPath failed: %v", err)
	}

	if path == "" {
		t.Fatal("DB path should not be empty")
	}

	// Path should end with meowg1k.db
	if !strings.HasSuffix(path, "meowg1k.db") {
		t.Errorf("DB path should end with meowg1k.db, got %s", path)
	}
}

func TestGetMainDBPathWithXDGDataHome(t *testing.T) {
	// Save original environment
	originalXDGDataHome := os.Getenv("XDG_DATA_HOME")
	defer os.Setenv("XDG_DATA_HOME", originalXDGDataHome)

	// Create temporary directory
	tempDir := t.TempDir()

	// Set XDG_DATA_HOME to temp directory
	os.Setenv("XDG_DATA_HOME", tempDir)

	service := NewService()
	path, err := service.GetMainDBPath()
	if err != nil {
		t.Fatalf("GetMainDBPath failed: %v", err)
	}

	expectedPath := filepath.Join(tempDir, "meowg1k", "meowg1k.db")
	if path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, path)
	}

	// Verify directory was created
	dbDir := filepath.Dir(path)
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		t.Errorf("DB directory was not created: %s", dbDir)
	}
}

func TestGetMainDBPathWithHomeDirectory(t *testing.T) {
	// Save original environment
	originalXDGDataHome := os.Getenv("XDG_DATA_HOME")
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("XDG_DATA_HOME", originalXDGDataHome)
		os.Setenv("HOME", originalHome)
	}()

	// Clear XDG_DATA_HOME and set HOME
	os.Setenv("XDG_DATA_HOME", "")
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)

	service := NewService()
	path, err := service.GetMainDBPath()
	if err != nil {
		t.Fatalf("GetMainDBPath failed: %v", err)
	}

	expectedPath := filepath.Join(tempDir, ".local", "share", "meowg1k", "meowg1k.db")
	if path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, path)
	}

	// Verify directory was created
	dbDir := filepath.Dir(path)
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		t.Errorf("DB directory was not created: %s", dbDir)
	}
}

func TestGetMainDBPathFallbackToCurrentDirectory(t *testing.T) {
	// Save original environment
	originalXDGDataHome := os.Getenv("XDG_DATA_HOME")
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("XDG_DATA_HOME", originalXDGDataHome)
		os.Setenv("HOME", originalHome)
	}()

	// Clear both XDG_DATA_HOME and HOME to force fallback
	os.Setenv("XDG_DATA_HOME", "")
	os.Setenv("HOME", "")

	service := NewService()
	path, err := service.GetMainDBPath()
	if err != nil {
		t.Fatalf("GetMainDBPath failed: %v", err)
	}

	// Should fallback to current directory
	if path != "meowg1k.db" {
		t.Errorf("Expected fallback path meowg1k.db, got %s", path)
	}
}
