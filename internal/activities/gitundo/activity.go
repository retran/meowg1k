// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package gitundo implements an activity for discarding uncommitted changes.
package gitundo

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

// Input defines the input for git undo operation.
type Input struct {
	Path string `json:"path"`
}

// Output defines the output of the git undo operation.
type Output struct {
	Message  string
	Restored bool
}

// Factory builds gitundo activities.
type Factory struct {
	workspaceService ports.WorkspaceService
	dryRun           bool
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new gitundo activity factory.
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

		// Sanitize and resolve path
		cleanPath := filepath.Clean(input.Path)
		if cleanPath == "." || cleanPath == "" {
			return nil, fmt.Errorf("path is required")
		}
		if filepath.IsAbs(cleanPath) {
			return nil, fmt.Errorf("absolute paths are not allowed: %s", input.Path)
		}

		absRoot, err := filepath.Abs(workspaceRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve workspace root: %w", err)
		}
		fullPath := filepath.Join(absRoot, cleanPath)
		absFull, err := filepath.Abs(fullPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path: %w", err)
		}
		rel, err := filepath.Rel(absRoot, absFull)
		if err != nil {
			return nil, fmt.Errorf("failed to compute relative path: %w", err)
		}
		rel = filepath.Clean(rel)
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("path traversal attempt: %s", input.Path)
		}

		// Check if .git exists
		gitDir := filepath.Join(absRoot, ".git")
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			return nil, fmt.Errorf("not a git repository")
		}

		details := fmt.Sprintf("path=%s", cleanPath)
		if f.dryRun {
			return &Output{
				Message:  fmt.Sprintf("DRY RUN: would restore %s", details),
				Restored: false,
			}, nil
		}

		// Execute git checkout HEAD -- <file>
		cmd := exec.CommandContext(ctx, "git", "checkout", "HEAD", "--", absFull) // #nosec G204
		cmd.Dir = absRoot
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("git checkout failed: %w\nOutput: %s", err, string(output))
		}

		return &Output{
			Message:  fmt.Sprintf("Restored from git: %s", details),
			Restored: true,
		}, nil
	}
}
