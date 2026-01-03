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
		cleanSourcePath := filepath.Clean(input.SourcePath)
		if cleanSourcePath == "." || cleanSourcePath == "" {
			return nil, fmt.Errorf("source path is required")
		}
		if filepath.IsAbs(cleanSourcePath) {
			return nil, fmt.Errorf("absolute paths are not allowed: %s", input.SourcePath)
		}

		fullSourcePath := filepath.Join(absRoot, cleanSourcePath)
		absSourcePath, err := filepath.Abs(fullSourcePath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve source path: %w", err)
		}
		relSource, err := filepath.Rel(absRoot, absSourcePath)
		if err != nil {
			return nil, fmt.Errorf("failed to compute relative source path: %w", err)
		}
		relSource = filepath.Clean(relSource)
		if relSource == ".." || strings.HasPrefix(relSource, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("path traversal attempt: %s", input.SourcePath)
		}

		// Validate destination path
		cleanDestPath := filepath.Clean(input.DestPath)
		if cleanDestPath == "." || cleanDestPath == "" {
			return nil, fmt.Errorf("destination path is required")
		}
		if filepath.IsAbs(cleanDestPath) {
			return nil, fmt.Errorf("absolute paths are not allowed: %s", input.DestPath)
		}

		fullDestPath := filepath.Join(absRoot, cleanDestPath)
		absDestPath, err := filepath.Abs(fullDestPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve destination path: %w", err)
		}
		relDest, err := filepath.Rel(absRoot, absDestPath)
		if err != nil {
			return nil, fmt.Errorf("failed to compute relative destination path: %w", err)
		}
		relDest = filepath.Clean(relDest)
		if relDest == ".." || strings.HasPrefix(relDest, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("path traversal attempt: %s", input.DestPath)
		}

		details := fmt.Sprintf("source=%s dest=%s", cleanSourcePath, cleanDestPath)
		if f.dryRun {
			return &Output{
				Message: fmt.Sprintf("DRY RUN: would move %s", details),
				Moved:   false,
			}, nil
		}

		// Check if source exists
		if _, err := os.Stat(absSourcePath); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("source file not found: %s", cleanSourcePath)
			}
			return nil, fmt.Errorf("failed to stat source file: %w", err)
		}

		// Ensure destination directory exists
		destDir := filepath.Dir(absDestPath)
		if err := os.MkdirAll(destDir, 0o750); err != nil {
			return nil, fmt.Errorf("failed to create destination directory: %w", err)
		}

		// Check if .git exists to decide whether to use git mv
		gitDir := filepath.Join(absRoot, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			// Use git mv
			cmd := exec.CommandContext(ctx, "git", "mv", absSourcePath, absDestPath) // #nosec G204
			cmd.Dir = absRoot
			if output, err := cmd.CombinedOutput(); err != nil {
				return nil, fmt.Errorf("git mv failed: %w\nOutput: %s", err, string(output))
			}
			return &Output{
				Message: fmt.Sprintf("Moved with git: %s", details),
				Moved:   true,
			}, nil
		}

		// Use os.Rename
		if err := os.Rename(absSourcePath, absDestPath); err != nil {
			return nil, fmt.Errorf("failed to move file: %w", err)
		}

		return &Output{
			Message: fmt.Sprintf("Moved: %s", details),
			Moved:   true,
		}, nil
	}
}
