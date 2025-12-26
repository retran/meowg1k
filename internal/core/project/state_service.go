// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

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

type StateService struct {
	gitService       ports.GitService
	filterService    ports.FilterService
	workspaceService ports.WorkspaceService
}

func NewStateService(gitService ports.GitService, filterService ports.FilterService, workspaceService ports.WorkspaceService) *StateService {
	return &StateService{
		gitService:       gitService,
		filterService:    filterService,
		workspaceService: workspaceService,
	}
}

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

	// Walk through workspace directory
	err = filepath.WalkDir(workspaceRoot, func(path string, d fs.DirEntry, err error) error {
		// Check context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(workspaceRoot, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}

		// Check if file should be ignored
		ignored := s.filterService.IsIgnoredFile(relPath)

		if ignored {
			return nil
		}

		// Verify the path is within workspace root to prevent directory traversal
		cleanPath := filepath.Clean(path)
		cleanRoot := filepath.Clean(workspaceRoot)
		// Ensure cleanRoot ends with separator for proper prefix matching
		if !strings.HasSuffix(cleanRoot, string(filepath.Separator)) {
			cleanRoot += string(filepath.Separator)
		}
		if !strings.HasPrefix(cleanPath+string(filepath.Separator), cleanRoot) {
			return fmt.Errorf("path %s is outside workspace root %s", path, workspaceRoot)
		}

		// Read file content
		content, err := os.ReadFile(cleanPath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		// Compute content hash
		hash := computeContentHash(content)

		state[relPath] = domainindex.FileState{
			ContentHash: hash,
			Content:     content,
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk workspace directory: %w", err)
	}

	return state, nil
}

// computeContentHash computes SHA-256 hash of content.
func computeContentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
