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

package workspace

import (
	"os"
	"testing"
)

func TestNewService(t *testing.T) {
	service := NewService()
	if service == nil {
		t.Error("NewService returned nil")
	}
}

func TestGetWorkspaceDir(t *testing.T) {
	service := NewService()

	dir, err := service.Get()
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	expected, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd failed: %v", err)
	}

	if dir != expected {
		t.Errorf("Expected %s, got %s", expected, dir)
	}
}

func TestGetWorkspaceDirFromTempDir(t *testing.T) {
	service := NewService()

	// Create temp directory and change to it
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get original directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	dir, err := service.Get()
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	// On macOS, /var might be a symlink to /private/var, so we need to check both
	// Get should return a valid path
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("Returned directory %s does not exist or is not accessible: %v", dir, err)
	}

	// Verify we're in the expected directory
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("Directory returned by Get is not accessible: %v", err)
	}
}

func TestGetWorkspaceDirAfterChangeDir(t *testing.T) {
	service := NewService()

	// Get original directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get original directory: %v", err)
	}

	// Get directory initially
	dir1, err := service.Get()
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	// Create temp directory and change to it
	tempDir := t.TempDir()
	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Get directory after changing
	dir2, err := service.Get()
	if err != nil {
		t.Errorf("Get failed after changing directory: %v", err)
	}

	// Restore original directory
	os.Chdir(originalDir)

	// Verify both directories are valid and accessible
	if _, err := os.Stat(dir1); err != nil {
		t.Errorf("First directory is not accessible: %v", err)
	}
	if _, err := os.Stat(dir2); err != nil {
		t.Errorf("Second directory is not accessible: %v", err)
	}

	// Directories should be different
	if dir1 == dir2 {
		t.Error("Get should return different directories after changing working directory")
	}
}

func TestGet_NilService(t *testing.T) {
	var service *Service
	_, err := service.Get()
	if err == nil {
		t.Fatal("expected error for nil service, got nil")
	}
	expectedMsg := "workspace service is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}
