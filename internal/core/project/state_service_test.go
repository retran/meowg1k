// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	domainindex "github.com/retran/meowg1k/internal/domain/index"
)

// Mock implementations for testing.
const ignoredFilename = "ignored.txt"

type mockGitService struct {
	ListFilesFunc             func(ref string) ([]string, error)
	ReadFileAtCommitFunc      func(ref, filePath string) (string, error)
	ReadStagedFilesFunc       func() ([]string, error)
	ReadStagedFileContentFunc func(filePath string) (string, error)
}

func (m *mockGitService) ListFiles(ref string) ([]string, error) {
	if m.ListFilesFunc != nil {
		return m.ListFilesFunc(ref)
	}
	return []string{}, nil
}

func (m *mockGitService) ReadFileAtCommit(ref, filePath string) (string, error) {
	if m.ReadFileAtCommitFunc != nil {
		return m.ReadFileAtCommitFunc(ref, filePath)
	}
	return "", nil
}

func (m *mockGitService) ReadStagedFiles() ([]string, error) {
	if m.ReadStagedFilesFunc != nil {
		return m.ReadStagedFilesFunc()
	}
	return []string{}, nil
}

func (m *mockGitService) ReadStagedFileContent(filePath string) (string, error) {
	if m.ReadStagedFileContentFunc != nil {
		return m.ReadStagedFileContentFunc(filePath)
	}
	return "", nil
}

type mockFilterService struct {
	IsIgnoredFileFunc func(filePath string) bool
}

func (m *mockFilterService) IsIgnoredFile(filePath string) bool {
	if m.IsIgnoredFileFunc != nil {
		return m.IsIgnoredFileFunc(filePath)
	}
	return false
}

type mockWorkspaceService struct {
	GetFunc func() (string, error)
}

func (m *mockWorkspaceService) Get() (string, error) {
	if m.GetFunc != nil {
		return m.GetFunc()
	}
	return "/tmp/test-workspace", nil
}

func TestNewStateService(t *testing.T) {
	t.Run("Creates service successfully", func(t *testing.T) {
		gitSvc := &mockGitService{}
		filterSvc := &mockFilterService{}
		workspaceSvc := &mockWorkspaceService{}

		service := NewStateService(gitSvc, filterSvc, workspaceSvc)
		if service == nil {
			t.Fatal("Expected service to be non-nil")
		}
		if service.gitService != gitSvc {
			t.Error("Expected gitService to be set")
		}
		if service.filterService != filterSvc {
			t.Error("Expected filterService to be set")
		}
		if service.workspaceService != workspaceSvc {
			t.Error("Expected workspaceService to be set")
		}
	})
}

func TestStateService_GetHeadState(t *testing.T) {
	t.Run("Successful retrieval", func(t *testing.T) {
		gitSvc := &mockGitService{
			ListFilesFunc: func(ref string) ([]string, error) {
				if ref != "HEAD" {
					t.Errorf("Expected ref 'HEAD', got %s", ref)
				}
				return []string{"file1.go", "file2.go"}, nil
			},
			ReadFileAtCommitFunc: func(ref, filePath string) (string, error) {
				return "package main", nil
			},
		}
		filterSvc := &mockFilterService{}
		workspaceSvc := &mockWorkspaceService{}

		service := NewStateService(gitSvc, filterSvc, workspaceSvc)
		state, err := service.GetHeadState(context.Background())
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(state) != 2 {
			t.Errorf("Expected 2 files in state, got %d", len(state))
		}
		if _, ok := state["file1.go"]; !ok {
			t.Error("Expected file1.go in state")
		}
		if _, ok := state["file2.go"]; !ok {
			t.Error("Expected file2.go in state")
		}
	})

	t.Run("ListFiles error propagates", func(t *testing.T) {
		gitSvc := &mockGitService{
			ListFilesFunc: func(ref string) ([]string, error) {
				return nil, errors.New("git error")
			},
		}
		service := NewStateService(gitSvc, &mockFilterService{}, &mockWorkspaceService{})

		_, err := service.GetHeadState(context.Background())
		if err == nil {
			t.Fatal("Expected error from ListFiles")
		}
	})

	t.Run("ReadFileAtCommit error propagates", func(t *testing.T) {
		gitSvc := &mockGitService{
			ListFilesFunc: func(ref string) ([]string, error) {
				return []string{"file1.go"}, nil
			},
			ReadFileAtCommitFunc: func(ref, filePath string) (string, error) {
				return "", errors.New("read error")
			},
		}
		service := NewStateService(gitSvc, &mockFilterService{}, &mockWorkspaceService{})

		_, err := service.GetHeadState(context.Background())
		if err == nil {
			t.Fatal("Expected error from ReadFileAtCommit")
		}
	})

	t.Run("Filters ignored files", func(t *testing.T) {
		gitSvc := &mockGitService{
			ListFilesFunc: func(ref string) ([]string, error) {
				return []string{"file1.go", ignoredFilename, "file2.go"}, nil
			},
			ReadFileAtCommitFunc: func(ref, filePath string) (string, error) {
				return "content", nil
			},
		}
		filterSvc := &mockFilterService{
			IsIgnoredFileFunc: func(filePath string) bool {
				return filePath == ignoredFilename
			},
		}
		service := NewStateService(gitSvc, filterSvc, &mockWorkspaceService{})

		state, err := service.GetHeadState(context.Background())
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(state) != 2 {
			t.Errorf("Expected 2 files (filtered 1), got %d", len(state))
		}
		if _, ok := state[ignoredFilename]; ok {
			t.Errorf("Expected %s to be filtered out", ignoredFilename)
		}
	})

	t.Run("Context cancellation", func(t *testing.T) {
		gitSvc := &mockGitService{
			ListFilesFunc: func(ref string) ([]string, error) {
				return []string{"file1.go", "file2.go"}, nil
			},
		}
		service := NewStateService(gitSvc, &mockFilterService{}, &mockWorkspaceService{})

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := service.GetHeadState(ctx)
		if err == nil {
			t.Fatal("Expected context cancellation error")
		}
	})

	t.Run("Computes content hash correctly", func(t *testing.T) {
		content := "test content"
		gitSvc := &mockGitService{
			ListFilesFunc: func(ref string) ([]string, error) {
				return []string{"file1.go"}, nil
			},
			ReadFileAtCommitFunc: func(ref, filePath string) (string, error) {
				return content, nil
			},
		}
		service := NewStateService(gitSvc, &mockFilterService{}, &mockWorkspaceService{})

		state, err := service.GetHeadState(context.Background())
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		fileState := state["file1.go"]
		if fileState.ContentHash == "" {
			t.Error("Expected content hash to be computed")
		}
		// Verify content is stored
		if !bytes.Equal(fileState.Content, []byte(content)) {
			t.Errorf("Expected content %q, got %q", content, string(fileState.Content))
		}
	})
}

func TestStateService_GetStagingState(t *testing.T) {
	t.Run("Successful retrieval", func(t *testing.T) {
		gitSvc := &mockGitService{
			ReadStagedFilesFunc: func() ([]string, error) {
				return []string{"staged1.go", "staged2.go"}, nil
			},
			ReadStagedFileContentFunc: func(filePath string) (string, error) {
				return "staged content", nil
			},
		}
		service := NewStateService(gitSvc, &mockFilterService{}, &mockWorkspaceService{})

		state, err := service.GetStagingState(context.Background())
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(state) != 2 {
			t.Errorf("Expected 2 files in state, got %d", len(state))
		}
	})

	t.Run("ReadStagedFiles error propagates", func(t *testing.T) {
		gitSvc := &mockGitService{
			ReadStagedFilesFunc: func() ([]string, error) {
				return nil, errors.New("staging error")
			},
		}
		service := NewStateService(gitSvc, &mockFilterService{}, &mockWorkspaceService{})

		_, err := service.GetStagingState(context.Background())
		if err == nil {
			t.Fatal("Expected error from ReadStagedFiles")
		}
	})

	t.Run("ReadStagedFileContent error propagates", func(t *testing.T) {
		gitSvc := &mockGitService{
			ReadStagedFilesFunc: func() ([]string, error) {
				return []string{"file.go"}, nil
			},
			ReadStagedFileContentFunc: func(filePath string) (string, error) {
				return "", errors.New("read error")
			},
		}
		service := NewStateService(gitSvc, &mockFilterService{}, &mockWorkspaceService{})

		_, err := service.GetStagingState(context.Background())
		if err == nil {
			t.Fatal("Expected error from ReadStagedFileContent")
		}
	})

	t.Run("Filters ignored files", func(t *testing.T) {
		gitSvc := &mockGitService{
			ReadStagedFilesFunc: func() ([]string, error) {
				return []string{"file1.go", "ignored.log"}, nil
			},
			ReadStagedFileContentFunc: func(filePath string) (string, error) {
				return "content", nil
			},
		}
		filterSvc := &mockFilterService{
			IsIgnoredFileFunc: func(filePath string) bool {
				return filePath == "ignored.log"
			},
		}
		service := NewStateService(gitSvc, filterSvc, &mockWorkspaceService{})

		state, err := service.GetStagingState(context.Background())
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(state) != 1 {
			t.Errorf("Expected 1 file (filtered 1), got %d", len(state))
		}
	})

	t.Run("Context cancellation", func(t *testing.T) {
		gitSvc := &mockGitService{
			ReadStagedFilesFunc: func() ([]string, error) {
				return []string{"file1.go"}, nil
			},
		}
		service := NewStateService(gitSvc, &mockFilterService{}, &mockWorkspaceService{})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := service.GetStagingState(ctx)
		if err == nil {
			t.Fatal("Expected context cancellation error")
		}
	})
}

func TestStateService_GetWorkdirState(t *testing.T) {
	t.Run("Workspace error propagates", func(t *testing.T) {
		workspaceSvc := &mockWorkspaceService{
			GetFunc: func() (string, error) {
				return "", errors.New("workspace error")
			},
		}
		service := NewStateService(&mockGitService{}, &mockFilterService{}, workspaceSvc)

		_, err := service.GetWorkdirState(context.Background())
		if err == nil {
			t.Fatal("Expected error from workspace service")
		}
	})

	t.Run("Context cancellation", func(t *testing.T) {
		service := NewStateService(&mockGitService{}, &mockFilterService{}, &mockWorkspaceService{})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := service.GetWorkdirState(ctx)
		if err == nil {
			t.Fatal("Expected context cancellation error")
		}
	})
}

func TestComputeContentHash(t *testing.T) {
	t.Run("Same content produces same hash", func(t *testing.T) {
		content := []byte("test content")
		hash1 := computeContentHash(content)
		hash2 := computeContentHash(content)

		if hash1 != hash2 {
			t.Error("Expected same hash for same content")
		}
	})

	t.Run("Different content produces different hash", func(t *testing.T) {
		content1 := []byte("content 1")
		content2 := []byte("content 2")
		hash1 := computeContentHash(content1)
		hash2 := computeContentHash(content2)

		if hash1 == hash2 {
			t.Error("Expected different hashes for different content")
		}
	})

	t.Run("Empty content produces valid hash", func(t *testing.T) {
		hash := computeContentHash([]byte{})
		if hash == "" {
			t.Error("Expected non-empty hash for empty content")
		}
		// SHA-256 hash should be 64 hex characters
		if len(hash) != 64 {
			t.Errorf("Expected hash length 64, got %d", len(hash))
		}
	})

	t.Run("Hash is deterministic", func(t *testing.T) {
		content := []byte("deterministic test")
		// Compute hash multiple times
		hashes := make([]string, 10)
		for i := 0; i < 10; i++ {
			hashes[i] = computeContentHash(content)
		}

		// All should be identical
		firstHash := hashes[0]
		for i, h := range hashes {
			if h != firstHash {
				t.Errorf("Hash %d differs: expected %s, got %s", i, firstHash, h)
			}
		}
	})
}

func TestEnsurePathWithinRoot(t *testing.T) {
	t.Run("accepts paths inside root", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "root")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatalf("failed to create root: %v", err)
		}

		path := filepath.Join(root, "file.txt")
		cleaned, err := ensurePathWithinRoot(path, root)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if cleaned != path {
			t.Errorf("expected cleaned path %s, got %s", path, cleaned)
		}
	})

	t.Run("rejects paths outside root", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "root")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatalf("failed to create root: %v", err)
		}

		outside := filepath.Join(t.TempDir(), "outside.txt")
		if _, err := ensurePathWithinRoot(outside, root); err == nil {
			t.Fatal("expected error for path outside root")
		}
	})
}

func TestReadFileState(t *testing.T) {
	t.Run("reads file and computes hash", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "file.txt")
		content := []byte("hello")
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		state, err := readFileState(path)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !bytes.Equal(state.Content, content) {
			t.Errorf("expected content %q, got %q", content, state.Content)
		}
		if state.ContentHash != computeContentHash(content) {
			t.Errorf("expected hash %s, got %s", computeContentHash(content), state.ContentHash)
		}
	})

	t.Run("returns error when file missing", func(t *testing.T) {
		if _, err := readFileState(filepath.Join(t.TempDir(), "missing.txt")); err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}

func TestCheckWalkContext(t *testing.T) {
	t.Run("returns context error when cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		if err := checkWalkContext(ctx, nil); err == nil {
			t.Fatal("expected context cancellation error")
		}
	})

	t.Run("returns walk error when provided", func(t *testing.T) {
		walkErr := errors.New("walk error")
		if err := checkWalkContext(context.Background(), walkErr); err == nil || !errors.Is(err, walkErr) {
			t.Fatalf("expected walk error, got %v", err)
		}
	})
}

func TestHandleWorkdirEntry(t *testing.T) {
	makeEntry := func(t *testing.T, path string) fs.DirEntry {
		t.Helper()
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat failed: %v", err)
		}
		return fs.FileInfoToDirEntry(info)
	}

	t.Run("adds file state for valid file", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "keep.txt")
		if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
			t.Fatalf("write failed: %v", err)
		}

		filterSvc := &mockFilterService{}
		service := NewStateService(&mockGitService{}, filterSvc, &mockWorkspaceService{})
		state := map[string]domainindex.FileState{}

		err := service.handleWorkdirEntry(context.Background(), root, state, path, makeEntry(t, path), nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if _, ok := state["keep.txt"]; !ok {
			t.Fatal("expected keep.txt to be tracked")
		}
	})

	t.Run("skips ignored files", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, ignoredFilename)
		if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
			t.Fatalf("write failed: %v", err)
		}

		filterSvc := &mockFilterService{
			IsIgnoredFileFunc: func(filePath string) bool {
				return filePath == ignoredFilename
			},
		}
		service := NewStateService(&mockGitService{}, filterSvc, &mockWorkspaceService{})
		state := map[string]domainindex.FileState{}

		err := service.handleWorkdirEntry(context.Background(), root, state, path, makeEntry(t, path), nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if _, ok := state[ignoredFilename]; ok {
			t.Fatalf("expected %s to be skipped", ignoredFilename)
		}
	})

	t.Run("rejects files outside root", func(t *testing.T) {
		root := t.TempDir()
		outsideDir := t.TempDir()
		path := filepath.Join(outsideDir, "outside.txt")
		if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
			t.Fatalf("write failed: %v", err)
		}

		service := NewStateService(&mockGitService{}, &mockFilterService{}, &mockWorkspaceService{})
		state := map[string]domainindex.FileState{}

		err := service.handleWorkdirEntry(context.Background(), root, state, path, makeEntry(t, path), nil)
		if err == nil {
			t.Fatal("expected error for file outside root")
		}
	})
}

func TestStateService_GetWorkdirState_Success(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ignoredFilename), []byte("ignore"), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	service := NewStateService(
		&mockGitService{},
		&mockFilterService{
			IsIgnoredFileFunc: func(filePath string) bool { return filePath == ignoredFilename },
		},
		&mockWorkspaceService{
			GetFunc: func() (string, error) {
				return root, nil
			},
		},
	)

	state, err := service.GetWorkdirState(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, ok := state["keep.txt"]; !ok {
		t.Fatal("expected keep.txt to be tracked")
	}
	if _, ok := state[ignoredFilename]; ok {
		t.Fatalf("expected %s to be filtered out", ignoredFilename)
	}
}
