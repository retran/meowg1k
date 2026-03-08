// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package sqlitepath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testWorkspacePath = "/test/workspace"

// mockWorkspaceService is a mock implementation of WorkspaceService for testing.
type mockWorkspaceService struct {
	getFunc func() (string, error)
}

func (m *mockWorkspaceService) Get() (string, error) {
	if m.getFunc != nil {
		return m.getFunc()
	}
	return "", nil
}

func TestNewService(t *testing.T) {
	mockWs := &mockWorkspaceService{
		getFunc: func() (string, error) {
			return testWorkspacePath, nil
		},
	}
	service, err := NewService(mockWs)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	if service == nil {
		t.Fatal("Service should not be nil")
	}
}

func TestGetMainDBPath(t *testing.T) {
	mockWs := &mockWorkspaceService{
		getFunc: func() (string, error) {
			return testWorkspacePath, nil
		},
	}
	service, err := NewService(mockWs)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
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

	mockWs := &mockWorkspaceService{
		getFunc: func() (string, error) {
			return testWorkspacePath, nil
		},
	}
	service, err := NewService(mockWs)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
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

	mockWs := &mockWorkspaceService{
		getFunc: func() (string, error) {
			return testWorkspacePath, nil
		},
	}
	service, err := NewService(mockWs)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
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

	mockWs := &mockWorkspaceService{
		getFunc: func() (string, error) {
			return testWorkspacePath, nil
		},
	}
	service, err := NewService(mockWs)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	path, err := service.GetMainDBPath()
	if err != nil {
		t.Fatalf("GetMainDBPath failed: %v", err)
	}

	// Should fallback to current directory
	if path != "meowg1k.db" {
		t.Errorf("Expected fallback path meowg1k.db, got %s", path)
	}
}

func TestGetProjectDBPath(t *testing.T) {
	tempDir := t.TempDir()
	mockWs := &mockWorkspaceService{
		getFunc: func() (string, error) {
			return tempDir, nil
		},
	}

	service, err := NewService(mockWs)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	path, err := service.GetProjectDBPath()
	if err != nil {
		t.Fatalf("GetProjectDBPath failed: %v", err)
	}

	expectedPath := filepath.Join(tempDir, ".meowg1k", ".data", "project.db")
	if path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, path)
	}

	// Verify directory was created
	dbDir := filepath.Dir(path)
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		t.Errorf("DB directory was not created: %s", dbDir)
	}
}

func TestGetProjectDBPathNilService(t *testing.T) {
	var service *Service
	_, err := service.GetProjectDBPath()
	if err == nil {
		t.Fatal("Expected error for nil service, got nil")
	}
}
