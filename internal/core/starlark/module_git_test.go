// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// Helper to initialize a git repository for testing.
func initTestGitRepo(t *testing.T) string {
	t.Helper()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "meowg1k-git-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	// Initialize git repo
	cmd := exec.CommandContext(context.Background(), "git", "init")
	cmd.Dir = tmpDir
	err = cmd.Run()
	require.NoError(t, err, "failed to initialize git repo")

	// Configure git user (required for commits)
	cmd = exec.CommandContext(context.Background(), "git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.CommandContext(context.Background(), "git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	err = cmd.Run()
	require.NoError(t, err)

	return tmpDir
}

// Helper to create a commit in the test repo.
func createTestCommit(t *testing.T, repoDir, filename, content, message string) {
	t.Helper()

	// Create a test file
	testFile := filepath.Join(repoDir, filename)
	err := os.WriteFile(testFile, []byte(content), 0o644)
	require.NoError(t, err)

	// Stage the file
	cmd := exec.CommandContext(context.Background(), "git", "add", filename)
	cmd.Dir = repoDir
	err = cmd.Run()
	require.NoError(t, err)

	// Commit
	cmd = exec.CommandContext(context.Background(), "git", "commit", "-m", message)
	cmd.Dir = repoDir
	err = cmd.Run()
	require.NoError(t, err)
}

func TestGitModule_Branch(t *testing.T) {
	t.Run("returns current branch name", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "test.txt", "content", "Initial commit")

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get the branch function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		branchVal, err := gitStruct.Attr("branch")
		require.NoError(t, err)
		branchFunc := branchVal.(starlark.Callable)

		// Call git.branch()
		thread := &starlark.Thread{Name: "test"}
		result, err := starlark.Call(thread, branchFunc, nil, nil)

		require.NoError(t, err)
		branch, ok := starlark.AsString(result)
		require.True(t, ok)
		// Git 2.28+ uses "main" by default, older versions use "master"
		assert.True(t, branch == "main" || branch == "master", "expected main or master, got %s", branch)
	})
}

// TestGitModule_Push tests git.push() function.
func TestGitModule_Push(t *testing.T) {
	t.Run("push without remote fails gracefully", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "content", "Initial commit")

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get the push function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		pushVal, err := gitStruct.Attr("push")
		require.NoError(t, err)
		pushFunc := pushVal.(starlark.Callable)

		// Call git.push() without configured remote
		thread := &starlark.Thread{Name: "test"}
		_, err = starlark.Call(thread, pushFunc, nil, nil)

		// Should fail because there's no remote configured
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "git push failed")
	})

	t.Run("push with explicit remote and branch fails gracefully", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "content", "Initial commit")

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get the push function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		pushVal, err := gitStruct.Attr("push")
		require.NoError(t, err)
		pushFunc := pushVal.(starlark.Callable)

		// Call git.push(remote="nonexistent", branch="main")
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("remote"), starlark.String("nonexistent")},
			{starlark.String("branch"), starlark.String("main")},
		}
		_, err = starlark.Call(thread, pushFunc, nil, kwargs)

		// Should fail because remote doesn't exist
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "git push failed")
	})
}

// TestGitModule_ErrorPaths tests error handling in git operations.
func TestGitModule_ErrorPaths(t *testing.T) {
	t.Run("git.branch() with invalid repository", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Don't initialize git repo - this should cause errors

		runtime := NewRuntime(tmpDir)
		gitModule := runtime.CreateGitModuleForCtx()

		gitStruct := gitModule.(*starlarkstruct.Struct)
		branchVal, err := gitStruct.Attr("branch")
		require.NoError(t, err)
		branchFunc := branchVal.(starlark.Callable)

		thread := &starlark.Thread{Name: "test"}
		_, err = starlark.Call(thread, branchFunc, nil, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "git")
	})

	t.Run("git.commit() with no staged changes", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "content", "Initial commit")

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		gitStruct := gitModule.(*starlarkstruct.Struct)
		commitVal, err := gitStruct.Attr("commit")
		require.NoError(t, err)
		commitFunc := commitVal.(starlark.Callable)

		// Try to commit with no changes
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("message"), starlark.String("Empty commit")},
		}
		_, err = starlark.Call(thread, commitFunc, nil, kwargs)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "git commit failed")
	})

	t.Run("git.checkout() to non-existent branch", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "content", "Initial commit")

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		gitStruct := gitModule.(*starlarkstruct.Struct)
		checkoutVal, err := gitStruct.Attr("checkout")
		require.NoError(t, err)
		checkoutFunc := checkoutVal.(starlark.Callable)

		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("target"), starlark.String("nonexistent-branch")},
		}
		_, err = starlark.Call(thread, checkoutFunc, nil, kwargs)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "git checkout failed")
	})

	t.Run("git.create_branch() with existing name", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "content", "Initial commit")

		// Create a branch
		cmd := exec.CommandContext(context.Background(), "git", "branch", "existing-branch")
		cmd.Dir = repoDir
		cmd.Run()

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		gitStruct := gitModule.(*starlarkstruct.Struct)
		createBranchVal, err := gitStruct.Attr("create_branch")
		require.NoError(t, err)
		createBranchFunc := createBranchVal.(starlark.Callable)

		// Try to create the same branch again
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("name"), starlark.String("existing-branch")},
		}
		_, err = starlark.Call(thread, createBranchFunc, nil, kwargs)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "git branch failed")
	})

	t.Run("git.read() with invalid ref", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "content", "Initial commit")

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		gitStruct := gitModule.(*starlarkstruct.Struct)
		readVal, err := gitStruct.Attr("read")
		require.NoError(t, err)
		readFunc := readVal.(starlark.Callable)

		// Try to read from non-existent ref
		thread := &starlark.Thread{Name: "test"}
		args := starlark.Tuple{starlark.String("nonexistent-ref:file1.txt")}
		_, err = starlark.Call(thread, readFunc, args, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "git show failed")
	})

	t.Run("git.add() with non-existent file", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "content", "Initial commit")

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		gitStruct := gitModule.(*starlarkstruct.Struct)
		addVal, err := gitStruct.Attr("add")
		require.NoError(t, err)
		addFunc := addVal.(starlark.Callable)

		// Try to add non-existent file
		thread := &starlark.Thread{Name: "test"}
		files := starlark.NewList([]starlark.Value{starlark.String("nonexistent.txt")})
		args := starlark.Tuple{files}
		_, err = starlark.Call(thread, addFunc, args, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "git add failed")
	})
}

// TestGitModule_ShouldIgnoreFile tests the shouldIgnoreFile helper.
func TestGitModule_ShouldIgnoreFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		patterns []string
		expected bool
	}{
		{
			name:     "no patterns returns false",
			path:     "file.txt",
			patterns: []string{},
			expected: false,
		},
		{
			name:     "exact match returns true",
			path:     ".git/config",
			patterns: []string{".git/**"},
			expected: true,
		},
		{
			name:     "wildcard match returns true",
			path:     "node_modules/package/index.js",
			patterns: []string{"node_modules/**"},
			expected: true,
		},
		{
			name:     "no match returns false",
			path:     "src/main.go",
			patterns: []string{".git/**", "node_modules/**"},
			expected: false,
		},
		{
			name:     "extension match returns true",
			path:     "test.log",
			patterns: []string{"*.log"},
			expected: true,
		},
		{
			name:     "multiple patterns, first matches",
			path:     ".env",
			patterns: []string{".env", "*.log", "build/**"},
			expected: true,
		},
		{
			name:     "multiple patterns, last matches",
			path:     "build/output/binary",
			patterns: []string{".env", "*.log", "build/**"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldIgnoreFile(tt.path, tt.patterns)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGitModule_Log(t *testing.T) {
	t.Run("returns commit history", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "content1", "First commit")
		createTestCommit(t, repoDir, "file2.txt", "content2", "Second commit")

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get the log function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		logVal, err := gitStruct.Attr("log")
		require.NoError(t, err)
		logFunc := logVal.(starlark.Callable)

		// Call git.log(count=10)
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("count"), starlark.MakeInt(10)},
		}
		result, err := starlark.Call(thread, logFunc, nil, kwargs)

		require.NoError(t, err)
		logList, ok := result.(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 2, logList.Len())

		// Check first commit (most recent) - it's a Struct, not a Dict
		commit := logList.Index(0).(*starlarkstruct.Struct)
		messageVal, err := commit.Attr("message")
		require.NoError(t, err)
		message, _ := starlark.AsString(messageVal)
		assert.Equal(t, "Second commit", message)
	})
}

func TestGitModule_Diff(t *testing.T) {
	t.Run("returns diff for staged changes", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "original content", "Initial commit")

		// Modify and stage the file
		os.WriteFile(filepath.Join(repoDir, "file1.txt"), []byte("modified content"), 0o644)
		cmd := exec.CommandContext(context.Background(), "git", "add", "file1.txt")
		cmd.Dir = repoDir
		cmd.Run()

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get the diff function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		diffVal, err := gitStruct.Attr("diff")
		require.NoError(t, err)
		diffFunc := diffVal.(starlark.Callable)

		// Call git.diff("staged")
		thread := &starlark.Thread{Name: "test"}
		args := starlark.Tuple{starlark.String("staged")}
		result, err := starlark.Call(thread, diffFunc, args, nil)

		require.NoError(t, err)
		diffStruct, ok := result.(*starlarkstruct.Struct)
		require.True(t, ok, "git.diff() should return a struct")

		// Check raw diff content
		rawVal, err := diffStruct.Attr("raw")
		require.NoError(t, err)
		diff, _ := starlark.AsString(rawVal)
		assert.Contains(t, diff, "file1.txt")
		assert.Contains(t, diff, "modified content")
	})
}

func TestGitModule_Add(t *testing.T) {
	t.Run("stages files", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "content", "Initial commit")

		// Create new file
		os.WriteFile(filepath.Join(repoDir, "file2.txt"), []byte("new content"), 0o644)

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get the add function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		addVal, err := gitStruct.Attr("add")
		require.NoError(t, err)
		addFunc := addVal.(starlark.Callable)

		// Call git.add(["file2.txt"])
		thread := &starlark.Thread{Name: "test"}
		pathsList := starlark.NewList([]starlark.Value{starlark.String("file2.txt")})
		args := starlark.Tuple{pathsList}
		_, err = starlark.Call(thread, addFunc, args, nil)

		require.NoError(t, err)

		// Verify file is staged using git status
		cmd := exec.CommandContext(context.Background(), "git", "diff", "--cached", "--name-only")
		cmd.Dir = repoDir
		output, err := cmd.Output()
		require.NoError(t, err)
		assert.Contains(t, string(output), "file2.txt")
	})
}

func TestGitModule_StagedFiles(t *testing.T) {
	t.Run("returns list of staged files", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "content", "Initial commit")

		// Create and stage new files
		os.WriteFile(filepath.Join(repoDir, "file2.txt"), []byte("new content"), 0o644)
		cmd := exec.CommandContext(context.Background(), "git", "add", "file2.txt")
		cmd.Dir = repoDir
		cmd.Run()

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get the staged_files function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		stagedVal, err := gitStruct.Attr("staged_files")
		require.NoError(t, err)
		stagedFunc := stagedVal.(starlark.Callable)

		// Call git.staged_files()
		thread := &starlark.Thread{Name: "test"}
		result, err := starlark.Call(thread, stagedFunc, nil, nil)

		require.NoError(t, err)
		stagedList, ok := result.(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 1, stagedList.Len())

		// Check file name
		fileName, _ := starlark.AsString(stagedList.Index(0))
		assert.Equal(t, "file2.txt", fileName)
	})
}

func TestGitModule_ModifiedFiles(t *testing.T) {
	t.Run("returns list of modified files", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "original", "Initial commit")

		// Modify file (but don't stage)
		os.WriteFile(filepath.Join(repoDir, "file1.txt"), []byte("modified"), 0o644)

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get the modified_files function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		modifiedVal, err := gitStruct.Attr("modified_files")
		require.NoError(t, err)
		modifiedFunc := modifiedVal.(starlark.Callable)

		// Call git.modified_files()
		thread := &starlark.Thread{Name: "test"}
		result, err := starlark.Call(thread, modifiedFunc, nil, nil)

		require.NoError(t, err)
		modifiedList, ok := result.(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 1, modifiedList.Len())

		// Check file name
		fileName, _ := starlark.AsString(modifiedList.Index(0))
		assert.Equal(t, "file1.txt", fileName)
	})
}

func TestGitModule_UntrackedFiles(t *testing.T) {
	t.Run("returns list of untracked files", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "content", "Initial commit")

		// Create untracked file
		os.WriteFile(filepath.Join(repoDir, "untracked.txt"), []byte("untracked"), 0o644)

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get the untracked_files function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		untrackedVal, err := gitStruct.Attr("untracked_files")
		require.NoError(t, err)
		untrackedFunc := untrackedVal.(starlark.Callable)

		// Call git.untracked_files()
		thread := &starlark.Thread{Name: "test"}
		result, err := starlark.Call(thread, untrackedFunc, nil, nil)

		require.NoError(t, err)
		untrackedList, ok := result.(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 1, untrackedList.Len())

		// Check file name
		fileName, _ := starlark.AsString(untrackedList.Index(0))
		assert.Equal(t, "untracked.txt", fileName)
	})
}

func TestGitModule_DiffFile(t *testing.T) {
	t.Run("returns diff for specific staged file", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "original content\n", "Initial commit")

		// Modify and stage the file
		os.WriteFile(filepath.Join(repoDir, "file1.txt"), []byte("modified content\n"), 0o644)
		cmd := exec.CommandContext(context.Background(), "git", "add", "file1.txt")
		cmd.Dir = repoDir
		cmd.Run()

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get the diff_file function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		diffFileVal, err := gitStruct.Attr("diff_file")
		require.NoError(t, err)
		diffFileFunc := diffFileVal.(starlark.Callable)

		// Call git.diff_file(file="file1.txt", target="staged")
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("file"), starlark.String("file1.txt")},
			{starlark.String("target"), starlark.String("staged")},
		}
		result, err := starlark.Call(thread, diffFileFunc, nil, kwargs)

		require.NoError(t, err)
		diffStruct, ok := result.(*starlarkstruct.Struct)
		require.True(t, ok, "git.diff_file() should return a struct")

		// Check raw diff content
		rawVal, err := diffStruct.Attr("raw")
		require.NoError(t, err)
		diff, _ := starlark.AsString(rawVal)
		assert.Contains(t, diff, "file1.txt")

		// Check file path
		fileVal, err := diffStruct.Attr("file")
		require.NoError(t, err)
		file, _ := starlark.AsString(fileVal)
		assert.Equal(t, "file1.txt", file)

		// Check additions/deletions
		additionsVal, err := diffStruct.Attr("additions")
		require.NoError(t, err)
		additionsInt, ok := additionsVal.(starlark.Int)
		require.True(t, ok)
		additions, _ := additionsInt.Int64()
		assert.GreaterOrEqual(t, additions, int64(1))

		deletionsVal, err := diffStruct.Attr("deletions")
		require.NoError(t, err)
		deletionsInt, ok := deletionsVal.(starlark.Int)
		require.True(t, ok)
		deletions, _ := deletionsInt.Int64()
		assert.GreaterOrEqual(t, deletions, int64(1))
	})
}

func TestGitModule_Commit(t *testing.T) {
	t.Run("creates a commit", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "content", "Initial commit")

		// Create and stage new file
		os.WriteFile(filepath.Join(repoDir, "file2.txt"), []byte("new content"), 0o644)
		cmd := exec.CommandContext(context.Background(), "git", "add", "file2.txt")
		cmd.Dir = repoDir
		cmd.Run()

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get the commit function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		commitVal, err := gitStruct.Attr("commit")
		require.NoError(t, err)
		commitFunc := commitVal.(starlark.Callable)

		// Call git.commit(message="Test commit")
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("message"), starlark.String("Test commit")},
		}
		result, err := starlark.Call(thread, commitFunc, nil, kwargs)

		require.NoError(t, err)
		commitStruct, ok := result.(*starlarkstruct.Struct)
		require.True(t, ok, "git.commit() should return a struct")

		// Check success
		successVal, err := commitStruct.Attr("success")
		require.NoError(t, err)
		assert.True(t, bool(successVal.(starlark.Bool)))

		// Check message
		messageVal, err := commitStruct.Attr("message")
		require.NoError(t, err)
		message, _ := starlark.AsString(messageVal)
		assert.Equal(t, "Test commit", message)

		// Check hash is not empty
		hashVal, err := commitStruct.Attr("hash")
		require.NoError(t, err)
		hash, _ := starlark.AsString(hashVal)
		assert.NotEmpty(t, hash)
	})
}

func TestGitModule_CreateBranch(t *testing.T) {
	t.Run("creates a new branch without checkout", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "content", "Initial commit")

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get the create_branch function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		createBranchVal, err := gitStruct.Attr("create_branch")
		require.NoError(t, err)
		createBranchFunc := createBranchVal.(starlark.Callable)

		// Call git.create_branch(name="test-branch")
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("name"), starlark.String("test-branch")},
		}
		result, err := starlark.Call(thread, createBranchFunc, nil, kwargs)

		require.NoError(t, err)
		branchStruct, ok := result.(*starlarkstruct.Struct)
		require.True(t, ok, "git.create_branch() should return a struct")

		// Check success
		successVal, err := branchStruct.Attr("success")
		require.NoError(t, err)
		assert.True(t, bool(successVal.(starlark.Bool)))

		// Check name
		nameVal, err := branchStruct.Attr("name")
		require.NoError(t, err)
		name, _ := starlark.AsString(nameVal)
		assert.Equal(t, "test-branch", name)

		// Check checked_out is false
		checkedOutVal, err := branchStruct.Attr("checked_out")
		require.NoError(t, err)
		assert.False(t, bool(checkedOutVal.(starlark.Bool)))

		// Verify branch exists using git
		cmd := exec.CommandContext(context.Background(), "git", "branch", "--list", "test-branch")
		cmd.Dir = repoDir
		output, err := cmd.Output()
		require.NoError(t, err)
		assert.Contains(t, string(output), "test-branch")
	})

	t.Run("creates and checks out new branch", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "content", "Initial commit")

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get the create_branch function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		createBranchVal, err := gitStruct.Attr("create_branch")
		require.NoError(t, err)
		createBranchFunc := createBranchVal.(starlark.Callable)

		// Call git.create_branch(name="test-branch2", should_checkout=True)
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("name"), starlark.String("test-branch2")},
			{starlark.String("should_checkout"), starlark.Bool(true)},
		}
		result, err := starlark.Call(thread, createBranchFunc, nil, kwargs)

		require.NoError(t, err)
		branchStruct, ok := result.(*starlarkstruct.Struct)
		require.True(t, ok)

		// Check checked_out is true
		checkedOutVal, err := branchStruct.Attr("checked_out")
		require.NoError(t, err)
		assert.True(t, bool(checkedOutVal.(starlark.Bool)))

		// Verify we're on the new branch
		cmd := exec.CommandContext(context.Background(), "git", "branch", "--show-current")
		cmd.Dir = repoDir
		output, err := cmd.Output()
		require.NoError(t, err)
		assert.Equal(t, "test-branch2", string(output[:len(output)-1]))
	})
}

func TestGitModule_Checkout(t *testing.T) {
	t.Run("checks out existing branch", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "content", "Initial commit")

		// Create a new branch
		cmd := exec.CommandContext(context.Background(), "git", "branch", "test-branch")
		cmd.Dir = repoDir
		cmd.Run()

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get the checkout function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		checkoutVal, err := gitStruct.Attr("checkout")
		require.NoError(t, err)
		checkoutFunc := checkoutVal.(starlark.Callable)

		// Call git.checkout(target="test-branch")
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("target"), starlark.String("test-branch")},
		}
		result, err := starlark.Call(thread, checkoutFunc, nil, kwargs)

		require.NoError(t, err)
		checkoutStruct, ok := result.(*starlarkstruct.Struct)
		require.True(t, ok, "git.checkout() should return a struct")

		// Check success
		successVal, err := checkoutStruct.Attr("success")
		require.NoError(t, err)
		assert.True(t, bool(successVal.(starlark.Bool)))

		// Check target
		targetVal, err := checkoutStruct.Attr("target")
		require.NoError(t, err)
		target, _ := starlark.AsString(targetVal)
		assert.Equal(t, "test-branch", target)

		// Verify we're on the new branch
		cmd = exec.CommandContext(context.Background(), "git", "branch", "--show-current")
		cmd.Dir = repoDir
		output, err := cmd.Output()
		require.NoError(t, err)
		assert.Equal(t, "test-branch", string(output[:len(output)-1]))
	})
}

func TestGitModule_Read(t *testing.T) {
	t.Run("reads file from HEAD", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "test.txt", "test content", "Initial commit")

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get the read function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		readVal, err := gitStruct.Attr("read")
		require.NoError(t, err)
		readFunc := readVal.(starlark.Callable)

		// Call git.read("HEAD:test.txt")
		thread := &starlark.Thread{Name: "test"}
		args := starlark.Tuple{starlark.String("HEAD:test.txt")}
		result, err := starlark.Call(thread, readFunc, args, nil)

		require.NoError(t, err)
		content, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Equal(t, "test content", content)
	})

	t.Run("reads file using keyword args", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "test2.txt", "test content 2", "Initial commit")

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get the read function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		readVal, err := gitStruct.Attr("read")
		require.NoError(t, err)
		readFunc := readVal.(starlark.Callable)

		// Call git.read(ref="HEAD", path="test2.txt")
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("ref"), starlark.String("HEAD")},
			{starlark.String("path"), starlark.String("test2.txt")},
		}
		result, err := starlark.Call(thread, readFunc, nil, kwargs)

		require.NoError(t, err)
		content, ok := starlark.AsString(result)
		require.True(t, ok)
		assert.Equal(t, "test content 2", content)
	})
}

func TestGitStatus(t *testing.T) {
	t.Run("returns empty list for clean repository", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get git.status function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		statusVal, err := gitStruct.Attr("status")
		require.NoError(t, err)
		require.NotNil(t, statusVal)
		statusFunc := statusVal.(starlark.Callable)

		// Call git.status()
		thread := &starlark.Thread{Name: "test"}
		result, err := starlark.Call(thread, statusFunc, nil, nil)

		require.NoError(t, err)
		statusList, ok := result.(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 0, statusList.Len())
	})

	t.Run("returns modified files", func(t *testing.T) {
		repoDir := initTestGitRepo(t)

		// Create and commit a file first
		createTestCommit(t, repoDir, "modified.txt", "initial content", "Add modified.txt")

		// Modify the file
		testFile := filepath.Join(repoDir, "modified.txt")
		err := os.WriteFile(testFile, []byte("modified content"), 0o644)
		require.NoError(t, err)

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get git.status function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		statusVal, err := gitStruct.Attr("status")
		require.NoError(t, err)
		statusFunc := statusVal.(starlark.Callable)

		// Call git.status()
		thread := &starlark.Thread{Name: "test"}
		result, err := starlark.Call(thread, statusFunc, nil, nil)

		require.NoError(t, err)
		statusList, ok := result.(*starlark.List)
		require.True(t, ok)
		assert.Greater(t, statusList.Len(), 0)

		// Verify the status contains information about the modified file
		firstStatus, ok := starlark.AsString(statusList.Index(0))
		require.True(t, ok)
		assert.Contains(t, firstStatus, "modified.txt")
	})

	t.Run("returns untracked files", func(t *testing.T) {
		repoDir := initTestGitRepo(t)

		// Create an untracked file
		untrackedFile := filepath.Join(repoDir, "untracked.txt")
		err := os.WriteFile(untrackedFile, []byte("untracked content"), 0o644)
		require.NoError(t, err)

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get git.status function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		statusVal, err := gitStruct.Attr("status")
		require.NoError(t, err)
		statusFunc := statusVal.(starlark.Callable)

		// Call git.status()
		thread := &starlark.Thread{Name: "test"}
		result, err := starlark.Call(thread, statusFunc, nil, nil)

		require.NoError(t, err)
		statusList, ok := result.(*starlark.List)
		require.True(t, ok)
		assert.Greater(t, statusList.Len(), 0)

		// Verify the status contains information about the untracked file
		firstStatus, ok := starlark.AsString(statusList.Index(0))
		require.True(t, ok)
		assert.Contains(t, firstStatus, "untracked.txt")
	})

	t.Run("fails when not in git repository", func(t *testing.T) {
		nonGitDir := t.TempDir()
		runtime := NewRuntime(nonGitDir)
		gitModule := runtime.CreateGitModuleForCtx()

		// Get git.status function
		gitStruct := gitModule.(*starlarkstruct.Struct)
		statusVal, err := gitStruct.Attr("status")
		require.NoError(t, err)
		statusFunc := statusVal.(starlark.Callable)

		// Call git.status()
		thread := &starlark.Thread{Name: "test"}
		_, err = starlark.Call(thread, statusFunc, nil, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "git status failed")
	})
}

// ============================================================================
// Phase 4b: High-value edge case tests for moderate-coverage functions
// ============================================================================

// TestGitModule_Glob tests git.glob() function.
func TestGitModule_Glob(t *testing.T) {
	t.Run("glob files at HEAD with default pattern", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "content1", "Add file1")

		// Create subdirectory
		os.MkdirAll(filepath.Join(repoDir, "dir"), 0o755)
		createTestCommit(t, repoDir, "dir/file2.go", "content2", "Add file2")
		createTestCommit(t, repoDir, "README.md", "readme", "Add README")

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		gitStruct := gitModule.(*starlarkstruct.Struct)
		globVal, err := gitStruct.Attr("glob")
		require.NoError(t, err)
		globFunc := globVal.(starlark.Callable)

		thread := &starlark.Thread{Name: "test"}
		result, err := starlark.Call(thread, globFunc, nil, nil)

		require.NoError(t, err)
		fileList, ok := result.(*starlark.List)
		require.True(t, ok)
		assert.Greater(t, fileList.Len(), 0)
	})

	t.Run("glob with specific pattern", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "content1", "Add file1")
		createTestCommit(t, repoDir, "file2.go", "content2", "Add file2")
		createTestCommit(t, repoDir, "file3.txt", "content3", "Add file3")

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		gitStruct := gitModule.(*starlarkstruct.Struct)
		globVal, err := gitStruct.Attr("glob")
		require.NoError(t, err)
		globFunc := globVal.(starlark.Callable)

		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("pattern"), starlark.String("*.txt")},
		}
		result, err := starlark.Call(thread, globFunc, nil, kwargs)

		require.NoError(t, err)
		fileList, ok := result.(*starlark.List)
		require.True(t, ok)

		// Should only match .txt files
		for i := 0; i < fileList.Len(); i++ {
			filename, _ := starlark.AsString(fileList.Index(i))
			assert.Contains(t, filename, ".txt")
		}
	})

	t.Run("glob with ignore patterns", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file1.txt", "content1", "Add file1")
		createTestCommit(t, repoDir, "file2.txt", "content2", "Add file2")
		createTestCommit(t, repoDir, "ignore.txt", "content3", "Add ignore")

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		gitStruct := gitModule.(*starlarkstruct.Struct)
		globVal, err := gitStruct.Attr("glob")
		require.NoError(t, err)
		globFunc := globVal.(starlark.Callable)

		thread := &starlark.Thread{Name: "test"}
		ignoreList := starlark.NewList([]starlark.Value{
			starlark.String("ignore.txt"),
		})
		kwargs := []starlark.Tuple{
			{starlark.String("ignore"), ignoreList},
		}
		result, err := starlark.Call(thread, globFunc, nil, kwargs)

		require.NoError(t, err)
		fileList, ok := result.(*starlark.List)
		require.True(t, ok)

		// Should not include ignore.txt
		for i := 0; i < fileList.Len(); i++ {
			filename, _ := starlark.AsString(fileList.Index(i))
			assert.NotEqual(t, "ignore.txt", filename)
		}
	})

	t.Run("glob staged files", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "committed.txt", "content", "Initial")

		// Create and stage a new file
		newFile := filepath.Join(repoDir, "staged.txt")
		err := os.WriteFile(newFile, []byte("staged content"), 0o644)
		require.NoError(t, err)

		cmd := exec.CommandContext(context.Background(), "git", "add", "staged.txt")
		cmd.Dir = repoDir
		err = cmd.Run()
		require.NoError(t, err)

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		gitStruct := gitModule.(*starlarkstruct.Struct)
		globVal, err := gitStruct.Attr("glob")
		require.NoError(t, err)
		globFunc := globVal.(starlark.Callable)

		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("ref"), starlark.String("staged")},
		}
		result, err := starlark.Call(thread, globFunc, nil, kwargs)

		require.NoError(t, err)
		fileList, ok := result.(*starlark.List)
		require.True(t, ok)

		// Should include staged.txt
		found := false
		for i := 0; i < fileList.Len(); i++ {
			filename, _ := starlark.AsString(fileList.Index(i))
			if filename == "staged.txt" {
				found = true
				break
			}
		}
		assert.True(t, found, "staged.txt should be in results")
	})

	t.Run("glob fails with workdir ref", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file.txt", "content", "Initial")

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		gitStruct := gitModule.(*starlarkstruct.Struct)
		globVal, err := gitStruct.Attr("glob")
		require.NoError(t, err)
		globFunc := globVal.(starlark.Callable)

		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("ref"), starlark.String("workdir")},
		}
		_, err = starlark.Call(thread, globFunc, nil, kwargs)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not support ref='workdir'")
	})

	t.Run("glob fails with invalid pattern", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file.txt", "content", "Initial")

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		gitStruct := gitModule.(*starlarkstruct.Struct)
		globVal, err := gitStruct.Attr("glob")
		require.NoError(t, err)
		globFunc := globVal.(starlark.Callable)

		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("pattern"), starlark.String("[invalid")},
		}
		_, err = starlark.Call(thread, globFunc, nil, kwargs)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pattern")
	})
}

// TestGitModule_Push_EdgeCases tests additional git.push() scenarios.
func TestGitModule_Push_EdgeCases(t *testing.T) {
	t.Run("push with remote and branch specified", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file.txt", "content", "Initial")

		// Get current branch name
		cmd := exec.CommandContext(context.Background(), "git", "branch", "--show-current")
		cmd.Dir = repoDir
		branchBytes, err := cmd.Output()
		require.NoError(t, err)
		branchName := strings.TrimSpace(string(branchBytes))

		// Create a bare remote repo
		remoteDir := t.TempDir()
		cmd = exec.CommandContext(context.Background(), "git", "init", "--bare")
		cmd.Dir = remoteDir
		err = cmd.Run()
		require.NoError(t, err)

		// Add remote
		cmd = exec.CommandContext(context.Background(), "git", "remote", "add", "origin", remoteDir)
		cmd.Dir = repoDir
		err = cmd.Run()
		require.NoError(t, err)

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		gitStruct := gitModule.(*starlarkstruct.Struct)
		pushVal, err := gitStruct.Attr("push")
		require.NoError(t, err)
		pushFunc := pushVal.(starlark.Callable)

		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("remote"), starlark.String("origin")},
			{starlark.String("branch"), starlark.String(branchName)},
		}
		result, err := starlark.Call(thread, pushFunc, nil, kwargs)

		require.NoError(t, err)
		resultStruct, ok := result.(*starlarkstruct.Struct)
		require.True(t, ok)

		// Verify success field
		successVal, err := resultStruct.Attr("success")
		require.NoError(t, err)
		assert.Equal(t, starlark.Bool(true), successVal)
	})

	t.Run("push without parameters uses defaults", func(t *testing.T) {
		repoDir := initTestGitRepo(t)
		createTestCommit(t, repoDir, "file.txt", "content", "Initial")

		// Get current branch name
		cmd := exec.CommandContext(context.Background(), "git", "branch", "--show-current")
		cmd.Dir = repoDir
		branchBytes, err := cmd.Output()
		require.NoError(t, err)
		branchName := strings.TrimSpace(string(branchBytes))

		// Create a bare remote repo
		remoteDir := t.TempDir()
		cmd = exec.CommandContext(context.Background(), "git", "init", "--bare")
		cmd.Dir = remoteDir
		err = cmd.Run()
		require.NoError(t, err)

		// Add remote
		cmd = exec.CommandContext(context.Background(), "git", "remote", "add", "origin", remoteDir)
		cmd.Dir = repoDir
		err = cmd.Run()
		require.NoError(t, err)

		// Set upstream for current branch
		cmd = exec.CommandContext(context.Background(), "git", "push", "-u", "origin", branchName)
		cmd.Dir = repoDir
		err = cmd.Run()
		require.NoError(t, err)

		// Make another commit
		createTestCommit(t, repoDir, "file2.txt", "content2", "Second commit")

		runtime := NewRuntime(repoDir)
		gitModule := runtime.CreateGitModuleForCtx()

		gitStruct := gitModule.(*starlarkstruct.Struct)
		pushVal, err := gitStruct.Attr("push")
		require.NoError(t, err)
		pushFunc := pushVal.(starlark.Callable)

		thread := &starlark.Thread{Name: "test"}
		result, err := starlark.Call(thread, pushFunc, nil, nil)

		require.NoError(t, err)
		resultStruct, ok := result.(*starlarkstruct.Struct)
		require.True(t, ok)

		// Verify success field
		successVal, err := resultStruct.Attr("success")
		require.NoError(t, err)
		assert.Equal(t, starlark.Bool(true), successVal)
	})
}
