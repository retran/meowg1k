// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"errors"
	"testing"
)

// Mock implementations for testing

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
				return []string{"file1.go", "ignored.txt", "file2.go"}, nil
			},
			ReadFileAtCommitFunc: func(ref, filePath string) (string, error) {
				return "content", nil
			},
		}
		filterSvc := &mockFilterService{
			IsIgnoredFileFunc: func(filePath string) bool {
				return filePath == "ignored.txt"
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
		if _, ok := state["ignored.txt"]; ok {
			t.Error("Expected ignored.txt to be filtered out")
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
		if string(fileState.Content) != content {
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
