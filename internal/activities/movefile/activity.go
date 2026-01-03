// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package movefile implements an activity for moving or renaming files.
package movefile

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input for moving a file.
type Input struct {
	SourcePath string `json:"source_path"`
	DestPath   string `json:"dest_path"`
}

// Output defines the output of the move operation.
type Output struct {
	Message string
	Moved   bool
}

// Factory builds movefile activities.
type Factory struct {
	workspaceService ports.WorkspaceService
	dryRun           bool
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new movefile activity factory.
func NewFactory(workspaceService ports.WorkspaceService, dryRun bool) *Factory {
	return &Factory{
		workspaceService: workspaceService,
		dryRun:           dryRun,
	}
}

// NewActivity creates the activity.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, _ *executor.Context, input *Input) (*Output, error) {
		workspaceRoot, err := f.workspaceService.Get()
		if err != nil {
			return nil, fmt.Errorf("failed to get workspace root: %w", err)
		}

		absRoot, err := filepath.Abs(workspaceRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve workspace root: %w", err)
		}

		// Validate source path
		fullSourcePath, cleanSourcePath, err := resolveAndValidatePath(workspaceRoot, input.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("source path error: %w", err)
		}

		// Validate destination path
		fullDestPath, cleanDestPath, err := resolveAndValidatePath(workspaceRoot, input.DestPath)
		if err != nil {
			return nil, fmt.Errorf("destination path error: %w", err)
		}

		details := fmt.Sprintf("source=%s dest=%s", cleanSourcePath, cleanDestPath)
		if f.dryRun {
			return &Output{
				Message: fmt.Sprintf("DRY RUN: would move %s", details),
				Moved:   false,
			}, nil
		}

		message, err := f.performMove(ctx, absRoot, fullSourcePath, fullDestPath, cleanSourcePath, details)
		if err != nil {
			return nil, err
		}

		return &Output{
			Message: message,
			Moved:   true,
		}, nil
	}
}

func (f *Factory) performMove(ctx context.Context, absRoot, fullSourcePath, fullDestPath, cleanSourcePath, details string) (string, error) {
	// Check if source exists
	if _, err := os.Stat(fullSourcePath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("source file not found: %s", cleanSourcePath)
		}
		return "", fmt.Errorf("failed to stat source file: %w", err)
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(fullDestPath)
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Check if .git exists to decide whether to use git mv
	gitDir := filepath.Join(absRoot, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		// Use git mv
		cmd := exec.CommandContext(ctx, "git", "mv", fullSourcePath, fullDestPath) // #nosec G204
		cmd.Dir = absRoot
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git mv failed: %w\nOutput: %s", err, string(output))
		}
		return fmt.Sprintf("Moved with git: %s", details), nil
	}

	// Use os.Rename
	if err := os.Rename(fullSourcePath, fullDestPath); err != nil {
		return "", fmt.Errorf("failed to move file: %w", err)
	}

	return fmt.Sprintf("Moved: %s", details), nil
}

func resolveAndValidatePath(workspaceRoot, inputPath string) (fullPath, cleanPath string, err error) {
	cleanPath = filepath.Clean(inputPath)
	if cleanPath == "." || cleanPath == "" {
		return "", "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(cleanPath) {
		return "", "", fmt.Errorf("absolute paths are not allowed: %s", inputPath)
	}

	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve workspace root: %w", err)
	}
	fullPath = filepath.Join(absRoot, cleanPath)
	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve path: %w", err)
	}
	rel, err := filepath.Rel(absRoot, absFull)
	if err != nil {
		return "", "", fmt.Errorf("failed to compute relative path: %w", err)
	}
	rel = filepath.Clean(rel)
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("path traversal attempt: %s", inputPath)
	}
	return fullPath, cleanPath, nil
}
