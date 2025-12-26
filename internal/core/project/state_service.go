// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package project provides services for inspecting git and workspace state.
package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
)

// StateService computes file states for git and workspace snapshots.
type StateService struct {
	gitService       ports.GitService
	filterService    ports.FilterService
	workspaceService ports.WorkspaceService
}

// NewStateService creates a new StateService.
func NewStateService(gitService ports.GitService, filterService ports.FilterService, workspaceService ports.WorkspaceService) *StateService {
	return &StateService{
		gitService:       gitService,
		filterService:    filterService,
		workspaceService: workspaceService,
	}
}

// GetHeadState returns the state of files in HEAD.
func (s *StateService) GetHeadState(ctx context.Context) (map[string]domainindex.FileState, error) {
	// Get list of files in HEAD
	files, err := s.gitService.ListFiles("HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to list files in HEAD: %w", err)
	}

	state := make(map[string]domainindex.FileState, len(files))

	for _, filePath := range files {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled while processing HEAD state: %w", err)
		}

		// Check if file should be ignored
		if s.filterService.IsIgnoredFile(filePath) {
			continue
		}

		// Read file content from HEAD
		content, err := s.gitService.ReadFileAtCommit("HEAD", filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s from HEAD: %w", filePath, err)
		}

		// Convert string to []byte for hash computation
		contentBytes := []byte(content)

		// Compute content hash
		hash := computeContentHash(contentBytes)

		state[filePath] = domainindex.FileState{
			ContentHash: hash,
			Content:     contentBytes,
		}
	}

	return state, nil
}

// GetStagingState returns the state of files in staging area.
func (s *StateService) GetStagingState(ctx context.Context) (map[string]domainindex.FileState, error) {
	// Get list of staged files
	files, err := s.gitService.ReadStagedFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to list staged files: %w", err)
	}

	state := make(map[string]domainindex.FileState, len(files))

	for _, filePath := range files {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled while processing staging state: %w", err)
		}

		// Check if file should be ignored
		if s.filterService.IsIgnoredFile(filePath) {
			continue
		}

		// Read file content from staging
		content, err := s.gitService.ReadStagedFileContent(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read staged file %s: %w", filePath, err)
		}

		// Convert string to []byte for hash computation
		contentBytes := []byte(content)

		// Compute content hash
		hash := computeContentHash(contentBytes)

		state[filePath] = domainindex.FileState{
			ContentHash: hash,
			Content:     contentBytes,
		}
	}

	return state, nil
}

// GetWorkdirState returns the state of files in working directory.
func (s *StateService) GetWorkdirState(ctx context.Context) (map[string]domainindex.FileState, error) {
	state := make(map[string]domainindex.FileState)

	// Get workspace root
	workspaceRoot, err := s.workspaceService.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace root: %w", err)
	}

	err = filepath.WalkDir(workspaceRoot, func(path string, d fs.DirEntry, walkErr error) error {
		return s.handleWorkdirEntry(ctx, workspaceRoot, state, path, d, walkErr)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk workspace directory: %w", err)
	}

	return state, nil
}

func (s *StateService) handleWorkdirEntry(
	ctx context.Context,
	workspaceRoot string,
	state map[string]domainindex.FileState,
	path string,
	d fs.DirEntry,
	walkErr error,
) error {
	if err := checkWalkContext(ctx, walkErr); err != nil {
		return err
	}

	if d.IsDir() {
		return nil
	}

	relPath, err := filepath.Rel(workspaceRoot, path)
	if err != nil {
		return fmt.Errorf("failed to get relative path for %s: %w", path, err)
	}

	if s.filterService.IsIgnoredFile(relPath) {
		return nil
	}

	cleanPath, err := ensurePathWithinRoot(path, workspaceRoot)
	if err != nil {
		return err
	}

	fileState, err := readFileState(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	state[relPath] = fileState
	return nil
}

func checkWalkContext(ctx context.Context, walkErr error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("context cancelled while walking workspace: %w", ctx.Err())
	}
	return walkErr
}

func ensurePathWithinRoot(path, workspaceRoot string) (string, error) {
	cleanPath := filepath.Clean(path)
	cleanRoot := filepath.Clean(workspaceRoot)
	if !strings.HasSuffix(cleanRoot, string(filepath.Separator)) {
		cleanRoot += string(filepath.Separator)
	}
	if !strings.HasPrefix(cleanPath+string(filepath.Separator), cleanRoot) {
		return "", fmt.Errorf("path %s is outside workspace root %s", path, workspaceRoot)
	}
	return cleanPath, nil
}

func readFileState(path string) (domainindex.FileState, error) {
	// #nosec G304 -- path is validated before this call.
	content, err := os.ReadFile(path)
	if err != nil {
		return domainindex.FileState{}, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	return domainindex.FileState{
		ContentHash: computeContentHash(content),
		Content:     content,
	}, nil
}

// computeContentHash computes SHA-256 hash of content.
func computeContentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
