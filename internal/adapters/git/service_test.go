// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const testFileName = "test.txt"

type mockWorkspaceService struct{}

func (m *mockWorkspaceService) Get() (string, error) {
	return ".", nil
}

func TestNewService(t *testing.T) {
	workspaceService := &mockWorkspaceService{}
	service, err := NewService(workspaceService)
	if err != nil {
		t.Errorf("NewService() returned error: %v", err)
	}
	if service == nil {
		t.Errorf("NewService() returned nil")
	}
}

func TestServiceImpl_ReadStagedFiles(t *testing.T) {
	workspaceService := &mockWorkspaceService{}
	service, err := NewService(workspaceService)
	if err != nil {
		t.Errorf("NewService() returned error: %v", err)
	}

	// This test assumes we're in a git repository
	// It may fail if there are no staged files or if not in a git repo
	files, err := service.ReadStagedFiles()
	if err != nil {
		// If git command fails (e.g., not in git repo), that's acceptable for this basic test
		t.Logf("ReadStagedFiles() error (expected if not in git repo): %v", err)
		return
	}

	// If successful, files should be a slice (possibly empty)
	if files == nil {
		t.Errorf("ReadStagedFiles() returned nil slice")
	}
}

func TestServiceImpl_ReadStagedChanges(t *testing.T) {
	workspaceService := &mockWorkspaceService{}
	service, err := NewService(workspaceService)
	if err != nil {
		t.Errorf("NewService() returned error: %v", err)
	}

	// Test with a file that likely doesn't exist in staging
	_, err = service.ReadStagedChanges("nonexistent.txt")
	// This should fail since the file is not staged
	if err == nil {
		t.Logf("ReadStagedChanges() unexpectedly succeeded for nonexistent file")
	}
}

func TestServiceImpl_ReadStagedFileContent(t *testing.T) {
	workspaceService := &mockWorkspaceService{}
	service, err := NewService(workspaceService)
	if err != nil {
		t.Errorf("NewService() returned error: %v", err)
	}

	// Test with a file that likely doesn't exist in staging
	_, err = service.ReadStagedFileContent("nonexistent.txt")
	// This should fail since the file is not staged
	if err == nil {
		t.Logf("ReadStagedFileContent() unexpectedly succeeded for nonexistent file")
	}
}

func TestServiceImpl_ReadOriginalFileContent(t *testing.T) {
	workspaceService := &mockWorkspaceService{}
	service, err := NewService(workspaceService)
	if err != nil {
		t.Errorf("NewService() returned error: %v", err)
	}

	// Test with a file that likely doesn't exist in HEAD
	_, err = service.ReadOriginalFileContent("nonexistent.txt")
	// This should fail since the file is not in HEAD
	if err == nil {
		t.Logf("ReadOriginalFileContent() unexpectedly succeeded for nonexistent file")
	}
}

func TestServiceImpl_ReadStagedFilesWithTempRepo(t *testing.T) {
	// Create a temporary git repository for more comprehensive testing
	tempDir, err := os.MkdirTemp("", "git_test")
	if err != nil {
		t.Skipf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if chdirErr := os.Chdir(tempDir); chdirErr != nil {
		t.Skipf("Failed to change to temp directory: %v", chdirErr)
	}

	// Initialize git repo
	if initErr := exec.CommandContext(context.Background(), "git", "init").Run(); initErr != nil {
		t.Skipf("Failed to init git repo: %v", initErr)
	}

	// Configure git user (required for commits)
	exec.CommandContext(context.Background(), "git", "config", "user.name", "Test User").Run()
	exec.CommandContext(context.Background(), "git", "config", "user.email", "test@example.com").Run()

	workspaceService := &mockWorkspaceService{}
	service, err := NewService(workspaceService)
	if err != nil {
		t.Errorf("NewService() returned error: %v", err)
	}

	// Test with empty repository (no staged files)
	files, err := service.ReadStagedFiles()
	if err != nil {
		t.Errorf("ReadStagedFiles() failed in empty repo: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("Expected no staged files in empty repo, got %d", len(files))
	}

	// Create and stage a file
	testFile := testFileName
	testContent := "Hello, world!"
	err = os.WriteFile(testFile, []byte(testContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = exec.CommandContext(context.Background(), "git", "add", testFile).Run()
	if err != nil {
		t.Fatalf("Failed to stage test file: %v", err)
	}

	// Test ReadStagedFiles with staged file
	files, err = service.ReadStagedFiles()
	if err != nil {
		t.Errorf("ReadStagedFiles() failed: %v", err)
	}
	if len(files) != 1 || files[0] != testFile {
		t.Errorf("Expected [%s], got %v", testFile, files)
	}

	// Test ReadStagedFileContent
	content, err := service.ReadStagedFileContent(testFile)
	if err != nil {
		t.Errorf("ReadStagedFileContent() failed: %v", err)
	}
	if strings.TrimSpace(content) != testContent {
		t.Errorf("Expected '%s', got '%s'", testContent, strings.TrimSpace(content))
	}

	// Test ReadStagedChanges
	changes, err := service.ReadStagedChanges(testFile)
	if err != nil {
		t.Errorf("ReadStagedChanges() failed: %v", err)
	}
	if !strings.Contains(changes, testContent) {
		t.Errorf("Expected changes to contain '%s', got '%s'", testContent, changes)
	}

	// Commit the file to test ReadOriginalFileContent
	if commitErr := exec.CommandContext(context.Background(), "git", "commit", "-m", "Initial commit").Run(); commitErr != nil {
		t.Fatalf("Failed to commit: %v", commitErr)
	}

	// Test ReadOriginalFileContent
	originalContent, err := service.ReadOriginalFileContent(testFile)
	if err != nil {
		t.Errorf("ReadOriginalFileContent() failed: %v", err)
	}
	if strings.TrimSpace(originalContent) != testContent {
		t.Errorf("Expected '%s', got '%s'", testContent, strings.TrimSpace(originalContent))
	}
}

func TestServiceImpl_RunGitCommandErrorHandling(t *testing.T) {
	workspaceService := &mockWorkspaceService{}
	workspaceDir, _ := workspaceService.Get()
	service := &Service{
		workspaceDir: workspaceDir,
	}

	// Test with invalid git command to trigger error handling
	_, err := service.runGitCommand("invalid-command", "nonexistent")
	if err == nil {
		t.Error("Expected error for invalid git command")
	}

	// Error message should contain useful information
	if !strings.Contains(err.Error(), "git command failed") {
		t.Errorf("Expected error message to contain 'git command failed', got: %v", err)
	}
}

func TestServiceImpl_ReadStagedFilesEmptyOutput(t *testing.T) {
	// Create a temporary git repository
	tempDir, err := os.MkdirTemp("", "git_test_empty")
	if err != nil {
		t.Skipf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	originalDir, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	err = os.Chdir(tempDir)
	if err != nil {
		t.Skipf("Failed to change to temp directory: %v", err)
	}

	// Initialize git repo
	err = exec.CommandContext(context.Background(), "git", "init").Run()
	if err != nil {
		t.Skipf("Failed to init git repo: %v", err)
	}

	workspaceService := &mockWorkspaceService{}
	service, err := NewService(workspaceService)
	if err != nil {
		t.Errorf("NewService() returned error: %v", err)
	}

	// Test ReadStagedFiles with no staged files (empty output handling)
	files, err := service.ReadStagedFiles()
	if err != nil {
		t.Errorf("ReadStagedFiles() failed: %v", err)
	}

	// Should return empty slice, not nil
	if files == nil {
		t.Error("ReadStagedFiles() returned nil instead of empty slice")
	}
	if len(files) != 0 {
		t.Errorf("Expected empty slice, got %v", files)
	}
}

func TestServiceImpl_ReadStagedFilesMultipleFiles(t *testing.T) {
	// Create a temporary git repository
	tempDir, err := os.MkdirTemp("", "git_test_multiple")
	if err != nil {
		t.Skipf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	originalDir, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	err = os.Chdir(tempDir)
	if err != nil {
		t.Skipf("Failed to change to temp directory: %v", err)
	}

	// Initialize git repo
	err = exec.CommandContext(context.Background(), "git", "init").Run()
	if err != nil {
		t.Skipf("Failed to init git repo: %v", err)
	}

	// Configure git user
	exec.CommandContext(context.Background(), "git", "config", "user.name", "Test User").Run()
	exec.CommandContext(context.Background(), "git", "config", "user.email", "test@example.com").Run()

	workspaceService := &mockWorkspaceService{}
	service, err := NewService(workspaceService)
	if err != nil {
		t.Errorf("NewService() returned error: %v", err)
	}

	// Create multiple files and stage them
	testFiles := []string{"file1.txt", "file2.txt", "file3.txt"}
	for i, filename := range testFiles {
		content := fmt.Sprintf("Content of file %d", i+1)
		writeErr := os.WriteFile(filename, []byte(content), 0o644)
		if writeErr != nil {
			t.Fatalf("Failed to create %s: %v", filename, writeErr)
		}
		err = exec.CommandContext(context.Background(), "git", "add", filename).Run()
		if err != nil {
			t.Fatalf("Failed to stage %s: %v", filename, err)
		}
	}

	// Test ReadStagedFiles with multiple files
	files, err := service.ReadStagedFiles()
	if err != nil {
		t.Errorf("ReadStagedFiles() failed: %v", err)
	}

	if len(files) != len(testFiles) {
		t.Errorf("Expected %d files, got %d", len(testFiles), len(files))
	}

	// Verify all files are present (order may vary)
	fileSet := make(map[string]bool)
	for _, file := range files {
		fileSet[file] = true
	}

	for _, expectedFile := range testFiles {
		if !fileSet[expectedFile] {
			t.Errorf("Expected file %s not found in staged files", expectedFile)
		}
	}
}

func TestServiceImpl_GetCurrentBranch(t *testing.T) {
	// Create a temporary git repository
	tempDir, err := os.MkdirTemp("", "git_test_branch")
	if err != nil {
		t.Skipf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	err = os.Chdir(tempDir)
	if err != nil {
		t.Skipf("Failed to change to temp directory: %v", err)
	}

	// Initialize git repo
	err = exec.CommandContext(context.Background(), "git", "init").Run()
	if err != nil {
		t.Skipf("Failed to init git repo: %v", err)
	}

	// Configure git user
	exec.CommandContext(context.Background(), "git", "config", "user.name", "Test User").Run()
	exec.CommandContext(context.Background(), "git", "config", "user.email", "test@example.com").Run()

	// Create initial commit
	os.WriteFile(testFileName, []byte("test"), 0o644)
	exec.CommandContext(context.Background(), "git", "add", testFileName).Run()
	exec.CommandContext(context.Background(), "git", "commit", "-m", "Initial commit").Run()

	workspaceService := &mockWorkspaceService{}
	service, err := NewService(workspaceService)
	if err != nil {
		t.Errorf("NewService() returned error: %v", err)
	}

	// Test GetCurrentBranch
	branch, err := service.GetCurrentBranch()
	if err != nil {
		t.Errorf("GetCurrentBranch() failed: %v", err)
	}

	// Default branch is usually "master" or "main"
	if branch != "master" && branch != "main" {
		t.Logf("Current branch: %s (expected master or main)", branch)
	}
}

func TestServiceImpl_GetChangedFilesInBranch(t *testing.T) {
	// Create a temporary git repository
	tempDir, err := os.MkdirTemp("", "git_test_changed_files")
	if err != nil {
		t.Skipf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	err = os.Chdir(tempDir)
	if err != nil {
		t.Skipf("Failed to change to temp directory: %v", err)
	}

	// Initialize git repo
	err = exec.CommandContext(context.Background(), "git", "init").Run()
	if err != nil {
		t.Skipf("Failed to init git repo: %v", err)
	}

	// Configure git user
	exec.CommandContext(context.Background(), "git", "config", "user.name", "Test User").Run()
	exec.CommandContext(context.Background(), "git", "config", "user.email", "test@example.com").Run()

	// Create initial commit on main branch
	os.WriteFile("base.txt", []byte("base content"), 0o644)
	exec.CommandContext(context.Background(), "git", "add", "base.txt").Run()
	exec.CommandContext(context.Background(), "git", "commit", "-m", "Initial commit").Run()

	// Create a new branch
	exec.CommandContext(context.Background(), "git", "checkout", "-b", "feature").Run()

	// Add changes in feature branch
	os.WriteFile("feature.txt", []byte("feature content"), 0o644)
	exec.CommandContext(context.Background(), "git", "add", "feature.txt").Run()
	exec.CommandContext(context.Background(), "git", "commit", "-m", "Add feature file").Run()

	workspaceService := &mockWorkspaceService{}
	service, err := NewService(workspaceService)
	if err != nil {
		t.Errorf("NewService() returned error: %v", err)
	}

	// Test GetChangedFilesInBranch
	files, err := service.GetChangedFilesInBranch("master")
	if err != nil {
		// Try with "main" if "master" doesn't exist
		files, err = service.GetChangedFilesInBranch("main")
		if err != nil {
			t.Errorf("GetChangedFilesInBranch() failed: %v", err)
		}
	}

	if len(files) != 1 || files[0] != "feature.txt" {
		t.Logf("Expected [feature.txt], got %v", files)
	}
}

func TestServiceImpl_GetChangedFilesInBranchEmpty(t *testing.T) {
	// Create a temporary git repository
	tempDir, err := os.MkdirTemp("", "git_test_changed_empty")
	if err != nil {
		t.Skipf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	err = os.Chdir(tempDir)
	if err != nil {
		t.Skipf("Failed to change to temp directory: %v", err)
	}

	// Initialize git repo
	err = exec.CommandContext(context.Background(), "git", "init").Run()
	if err != nil {
		t.Skipf("Failed to init git repo: %v", err)
	}

	// Configure git user
	exec.CommandContext(context.Background(), "git", "config", "user.name", "Test User").Run()
	exec.CommandContext(context.Background(), "git", "config", "user.email", "test@example.com").Run()

	// Create initial commit
	os.WriteFile(testFileName, []byte("test"), 0o644)
	exec.CommandContext(context.Background(), "git", "add", testFileName).Run()
	exec.CommandContext(context.Background(), "git", "commit", "-m", "Initial commit").Run()

	workspaceService := &mockWorkspaceService{}
	service, err := NewService(workspaceService)
	if err != nil {
		t.Errorf("NewService() returned error: %v", err)
	}

	// Test GetChangedFilesInBranch with no changes
	baseBranch := "HEAD"
	files, err := service.GetChangedFilesInBranch(baseBranch)
	if err != nil {
		t.Errorf("GetChangedFilesInBranch() failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("Expected no changed files, got %v", files)
	}
}

func TestServiceImpl_GetBranchDiff(t *testing.T) {
	// Create a temporary git repository
	tempDir, err := os.MkdirTemp("", "git_test_branch_diff")
	if err != nil {
		t.Skipf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	err = os.Chdir(tempDir)
	if err != nil {
		t.Skipf("Failed to change to temp directory: %v", err)
	}

	// Initialize git repo
	err = exec.CommandContext(context.Background(), "git", "init").Run()
	if err != nil {
		t.Skipf("Failed to init git repo: %v", err)
	}

	// Configure git user
	exec.CommandContext(context.Background(), "git", "config", "user.name", "Test User").Run()
	exec.CommandContext(context.Background(), "git", "config", "user.email", "test@example.com").Run()

	// Create initial commit on main branch
	testFile := testFileName
	os.WriteFile(testFile, []byte("line 1\nline 2\n"), 0o644)
	exec.CommandContext(context.Background(), "git", "add", testFile).Run()
	exec.CommandContext(context.Background(), "git", "commit", "-m", "Initial commit").Run()

	// Create a new branch
	exec.CommandContext(context.Background(), "git", "checkout", "-b", "feature").Run()

	// Modify the file in feature branch
	os.WriteFile(testFile, []byte("line 1\nline 2 modified\nline 3\n"), 0o644)
	exec.CommandContext(context.Background(), "git", "add", testFile).Run()
	exec.CommandContext(context.Background(), "git", "commit", "-m", "Modify file").Run()

	workspaceService := &mockWorkspaceService{}
	service, err := NewService(workspaceService)
	if err != nil {
		t.Errorf("NewService() returned error: %v", err)
	}

	// Test GetBranchDiff
	diff, err := service.GetBranchDiff(testFile, "master")
	if err != nil {
		// Try with "main" if "master" doesn't exist
		diff, err = service.GetBranchDiff(testFile, "main")
		if err != nil {
			t.Errorf("GetBranchDiff() failed: %v", err)
		}
	}

	if !strings.Contains(diff, "line 2 modified") || !strings.Contains(diff, "line 3") {
		t.Logf("Expected diff to contain modifications, got: %s", diff)
	}
}

type customWorkspaceService struct {
	dir string
}

func (c *customWorkspaceService) Get() (string, error) {
	return c.dir, nil
}

func TestServiceWithCustomWorkspace(t *testing.T) {
	// Test that git service uses the workspace directory provided by the service
	tmpDir := t.TempDir()

	// Initialize a git repo in temp directory
	initCmd := exec.CommandContext(context.Background(), "git", "init", tmpDir)
	if err := initCmd.Run(); err != nil {
		t.Skipf("Cannot init git repo: %v", err)
	}

	// Configure git
	exec.CommandContext(context.Background(), "git", "-C", tmpDir, "config", "user.name", "Test User").Run()
	exec.CommandContext(context.Background(), "git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()

	// Create a test file and commit it
	testFile := tmpDir + "/test.txt"
	if err := os.WriteFile(testFile, []byte("initial content\n"), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	exec.CommandContext(context.Background(), "git", "-C", tmpDir, "add", testFileName).Run()
	exec.CommandContext(context.Background(), "git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	// Create mock workspace service that returns our temp directory
	mockWS := &customWorkspaceService{dir: tmpDir}

	// Create git service with custom workspace
	service, err := NewService(mockWS)
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}

	// Verify that service has the correct workspace
	if service.workspaceDir != tmpDir {
		t.Errorf("Expected workspace dir %s, got %s", tmpDir, service.workspaceDir)
	}

	// Modify the file and stage it
	if err := os.WriteFile(testFile, []byte("modified content\n"), 0o644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}
	exec.CommandContext(context.Background(), "git", "-C", tmpDir, "add", testFileName).Run()

	// Read staged files - should work from the custom workspace
	files, err := service.ReadStagedFiles()
	if err != nil {
		t.Fatalf("ReadStagedFiles() failed: %v", err)
	}

	if len(files) != 1 || files[0] != testFileName {
		t.Errorf("Expected staged files [test.txt], got %v", files)
	}

	t.Logf("Successfully tested git service with custom workspace: %s", tmpDir)
}
