// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

// Package sqlitepath provides services for resolving SQLite database file paths based on workspace location.
package sqlitepath

import (
	"fmt"
	"os"
	"path/filepath"
)

// WorkspaceService defines the interface for getting workspace root.
type WorkspaceService interface {
	Get() (string, error)
}

// Service is the concrete implementation of the database path service.
type Service struct {
	workspaceService WorkspaceService
	mainDBPath       string
}

// NewService creates a new database path service.
// It determines the appropriate location for the main database file
// based on XDG Base Directory specification or fallback to current directory.
// Returns an error if database directory creation fails in all locations.
func NewService(workspaceService WorkspaceService) (*Service, error) {
	mainDBPath, err := determineMainDBPath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine main database path: %w", err)
	}
	return &Service{
		mainDBPath:       mainDBPath,
		workspaceService: workspaceService,
	}, nil
}

// GetMainDBPath returns the path to the main database file.
func (s *Service) GetMainDBPath() (string, error) {
	if s == nil {
		return "", fmt.Errorf("database path service is nil")
	}

	return s.mainDBPath, nil
}

// GetProjectDBPath returns the path to the project database file.
// The project database is stored in <workspace root>/.meowg1k/.data/project.db.
func (s *Service) GetProjectDBPath() (string, error) {
	if s == nil {
		return "", fmt.Errorf("database path service is nil")
	}

	if s.workspaceService == nil {
		return "", fmt.Errorf("workspace service is nil")
	}

	workspaceRoot, err := s.workspaceService.Get()
	if err != nil {
		return "", fmt.Errorf("failed to get workspace root: %w", err)
	}

	projectDBDir := filepath.Join(workspaceRoot, ".meowg1k", ".data")
	if err := os.MkdirAll(projectDBDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create project database directory: %w", err)
	}

	return filepath.Join(projectDBDir, "project.db"), nil
}

// determineMainDBPath determines the appropriate location for the main database file.
// It follows XDG Base Directory specification:
// 1. $XDG_DATA_HOME/meowg1k/meowg1k.db
// 2. $HOME/.local/share/meowg1k/meowg1k.db
// 3. ./meowg1k.db (current directory as fallback)
// Returns an error if all directory creation attempts fail.
func determineMainDBPath() (string, error) {
	var lastErr error

	// Try XDG_DATA_HOME first
	if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
		dbDir := filepath.Join(xdgDataHome, "meowg1k")
		err := os.MkdirAll(dbDir, 0o750) //nolint:gosec // G703: path derived from env variable; user-controlled is acceptable
		if err == nil {
			return filepath.Join(dbDir, "meowg1k.db"), nil
		}
		lastErr = fmt.Errorf("failed to create XDG_DATA_HOME directory: %w", err)
	}

	// Try HOME/.local/share as fallback
	if home := os.Getenv("HOME"); home != "" {
		dbDir := filepath.Join(home, ".local", "share", "meowg1k")
		err := os.MkdirAll(dbDir, 0o750) //nolint:gosec // G703: path derived from HOME env variable; user-controlled is acceptable
		if err == nil {
			return filepath.Join(dbDir, "meowg1k.db"), nil
		}
		lastErr = fmt.Errorf("failed to create HOME/.local/share directory: %w", err)
	}

	// Last resort: use current directory
	return "meowg1k.db", lastErr
}
