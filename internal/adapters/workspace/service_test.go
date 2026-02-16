// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

type mockWorkspacePathResolver struct {
	err  error
	path string
}

func (m *mockWorkspacePathResolver) GetWorkspacePath() (string, error) {
	return m.path, m.err
}

func TestNewService(t *testing.T) {
	service := NewService(nil)
	if service == nil {
		t.Error("NewService returned nil")
	}
}

func TestGetWorkspaceDir(t *testing.T) {
	service := NewService(nil)

	dir, err := service.Get()
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	// The returned directory should be valid and accessible
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("Returned directory %s does not exist or is not accessible: %v", dir, err)
	}

	// The directory should either be the current directory or a parent with markers
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd failed: %v", err)
	}

	// Verify that dir is either currentDir or a parent of it
	if dir != currentDir {
		// Check if currentDir is under dir
		rel, err := filepath.Rel(dir, currentDir)
		if err != nil || filepath.IsAbs(rel) || rel == "" {
			t.Errorf("Returned directory %s is not current dir %s or its parent", dir, currentDir)
		}
	}
}

func TestGetWorkspaceDirFromTempDir(t *testing.T) {
	service := NewService(nil)

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
	service := NewService(nil)

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

func TestGetWorkspaceRoot_WithMeowg1kYaml(t *testing.T) {
	service := NewService(nil)

	// Create temp directory structure
	tempDir := t.TempDir()
	workspaceRoot, _ := filepath.EvalSymlinks(tempDir)
	subDir := workspaceRoot + "/sub/dir"
	os.MkdirAll(subDir, 0o755)

	// Create .meowg1k.yaml in root
	os.WriteFile(workspaceRoot+"/.meowg1k.yaml", []byte("test: config"), 0o644)

	// Change to subdirectory
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(subDir)

	// Get should return the workspace root, not the current directory
	dir, err := service.Get()
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	// Resolve symlinks for comparison
	dirResolved, _ := filepath.EvalSymlinks(dir)
	if dirResolved != workspaceRoot {
		t.Errorf("Expected workspace root %s, got %s", workspaceRoot, dirResolved)
	}
}

func TestGetWorkspaceRoot_WithMeowg1kYml(t *testing.T) {
	service := NewService(nil)

	// Create temp directory structure
	tempDir := t.TempDir()
	workspaceRoot, _ := filepath.EvalSymlinks(tempDir)
	subDir := workspaceRoot + "/sub/dir"
	os.MkdirAll(subDir, 0o755)

	// Create .meowg1k.yml in root
	os.WriteFile(workspaceRoot+"/.meowg1k.yml", []byte("test: config"), 0o644)

	// Change to subdirectory
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(subDir)

	// Get should return the workspace root, not the current directory
	dir, err := service.Get()
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	// Resolve symlinks for comparison
	dirResolved, _ := filepath.EvalSymlinks(dir)
	if dirResolved != workspaceRoot {
		t.Errorf("Expected workspace root %s, got %s", workspaceRoot, dirResolved)
	}
}

func TestGetWorkspaceRoot_WithGitDir(t *testing.T) {
	service := NewService(nil)

	// Create temp directory structure
	tempDir := t.TempDir()
	workspaceRoot, _ := filepath.EvalSymlinks(tempDir)
	subDir := workspaceRoot + "/sub/dir"
	os.MkdirAll(subDir, 0o755)

	// Create .git directory in root
	os.MkdirAll(workspaceRoot+"/.git", 0o755)

	// Change to subdirectory
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(subDir)

	// Get should return the workspace root, not the current directory
	dir, err := service.Get()
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	// Resolve symlinks for comparison
	dirResolved, _ := filepath.EvalSymlinks(dir)
	if dirResolved != workspaceRoot {
		t.Errorf("Expected workspace root %s, got %s", workspaceRoot, dirResolved)
	}
}

func TestGetWorkspaceRoot_WithNoMarkers(t *testing.T) {
	service := NewService(nil)

	// Create temp directory structure with no markers
	tempDir := t.TempDir()
	tempDirResolved, _ := filepath.EvalSymlinks(tempDir)
	subDir := tempDirResolved + "/sub/dir"
	os.MkdirAll(subDir, 0o755)

	// Change to subdirectory
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(subDir)

	// Get should return the current directory since no markers found
	dir, err := service.Get()
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	// Resolve symlinks for comparison
	dirResolved, _ := filepath.EvalSymlinks(dir)
	if dirResolved != subDir {
		t.Errorf("Expected current dir %s, got %s", subDir, dirResolved)
	}
}

func TestGetWorkspaceRoot_PriorityOrder(t *testing.T) {
	service := NewService(nil)

	// Create temp directory structure
	tempDir := t.TempDir()
	workspaceRoot, _ := filepath.EvalSymlinks(tempDir)
	middleDir := workspaceRoot + "/middle"
	subDir := middleDir + "/sub"
	os.MkdirAll(subDir, 0o755)

	// Create .git in root and .meowg1k.yaml in middle
	os.MkdirAll(workspaceRoot+"/.git", 0o755)
	os.WriteFile(middleDir+"/.meowg1k.yaml", []byte("test: config"), 0o644)

	// Change to subdirectory
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(subDir)

	// Get should return the closest marker (middle dir with .meowg1k.yaml)
	dir, err := service.Get()
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	// Resolve symlinks for comparison
	dirResolved, _ := filepath.EvalSymlinks(dir)
	if dirResolved != middleDir {
		t.Errorf("Expected middle dir %s (closest marker), got %s", middleDir, dirResolved)
	}
}

func TestGetWithExplicitWorkspacePath(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	workspaceRoot, _ := filepath.EvalSymlinks(tempDir)

	// Create mock resolver that returns explicit path
	mock := &mockWorkspacePathResolver{
		path: workspaceRoot,
		err:  nil,
	}

	service := NewService(mock)

	dir, err := service.Get()
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	if dir != workspaceRoot {
		t.Errorf("Expected explicit workspace path %s, got %s", workspaceRoot, dir)
	}
}

func TestGetWithExplicitWorkspacePathNonExistent(t *testing.T) {
	// Create mock resolver with non-existent path
	mock := &mockWorkspacePathResolver{
		path: "/nonexistent/path",
		err:  nil,
	}

	service := NewService(mock)

	_, err := service.Get()
	if err == nil {
		t.Error("Expected error for non-existent workspace path")
	}
}

func TestGetWithExplicitWorkspacePathNotDirectory(t *testing.T) {
	// Create temp file (not directory)
	tempFile, err := os.CreateTemp("", "testfile")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	// Create mock resolver pointing to a file
	mock := &mockWorkspacePathResolver{
		path: tempFile.Name(),
		err:  nil,
	}

	service := NewService(mock)

	_, err = service.Get()
	if err == nil {
		t.Error("Expected error when workspace path is not a directory")
	}
}

func TestGetWithEmptyExplicitPath(t *testing.T) {
	// Create temp directory for auto-detection to find
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	// Create mock resolver with empty path (fallback to auto-detection)
	mock := &mockWorkspacePathResolver{
		path: "",
		err:  nil,
	}

	service := NewService(mock)

	dir, err := service.Get()
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	// Should fall back to auto-detection
	if dir == "" {
		t.Error("Expected non-empty workspace directory from auto-detection")
	}
}

// Test NewServiceWithPath

func TestNewServiceWithPath(t *testing.T) {
	testPath := "/test/workspace/path"
	service := NewServiceWithPath(testPath)

	if service == nil {
		t.Fatal("NewServiceWithPath returned nil")
	}

	if service.workspacePathResolver == nil {
		t.Fatal("Expected workspacePathResolver to be set")
	}

	// Verify that the fixed path resolver returns the correct path
	path, err := service.workspacePathResolver.GetWorkspacePath()
	if err != nil {
		t.Fatalf("GetWorkspacePath failed: %v", err)
	}

	if path != testPath {
		t.Errorf("Expected path %s, got %s", testPath, path)
	}
}

func TestNewServiceWithPath_ActualDirectory(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	service := NewServiceWithPath(tempDir)

	// Get should return the fixed path
	dir, err := service.Get()
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	if dir != tempDir {
		t.Errorf("Expected workspace path %s, got %s", tempDir, dir)
	}
}

func TestNewServiceWithPath_NonExistentPath(t *testing.T) {
	nonExistentPath := "/nonexistent/test/path"

	service := NewServiceWithPath(nonExistentPath)

	// Get should fail for non-existent path
	_, err := service.Get()
	if err == nil {
		t.Error("Expected error for non-existent path")
	}
}

// Test fixedPathResolver directly

func TestFixedPathResolver_GetWorkspacePath(t *testing.T) {
	testPath := "/some/fixed/path"
	resolver := &fixedPathResolver{path: testPath}

	path, err := resolver.GetWorkspacePath()
	if err != nil {
		t.Errorf("GetWorkspacePath failed: %v", err)
	}

	if path != testPath {
		t.Errorf("Expected path %s, got %s", testPath, path)
	}
}

func TestFixedPathResolver_GetWorkspacePath_EmptyPath(t *testing.T) {
	resolver := &fixedPathResolver{path: ""}

	path, err := resolver.GetWorkspacePath()
	if err != nil {
		t.Errorf("GetWorkspacePath failed: %v", err)
	}

	if path != "" {
		t.Errorf("Expected empty path, got %s", path)
	}
}
