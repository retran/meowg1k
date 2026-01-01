// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package deletefile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input for deleting a file.
type Input struct {
	Path string `json:"path"`
}

// Output defines the output of the delete operation.
type Output struct {
	Message string
	Deleted bool
}

// Factory builds deletefile activities.
type Factory struct {
	workspaceService ports.WorkspaceService
	dryRun           bool
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new deletefile activity factory.
func NewFactory(workspaceService ports.WorkspaceService, dryRun bool) *Factory {
	return &Factory{
		workspaceService: workspaceService,
		dryRun:           dryRun,
	}
}

// NewActivity creates the activity.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(_ context.Context, flowCtx *executor.Context, input *Input) (*Output, error) {
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

		details := fmt.Sprintf("path=%s", cleanPath)
		if f.dryRun {
			return &Output{
				Message: fmt.Sprintf("DRY RUN: would delete %s", details),
				Deleted: false,
			}, nil
		}

		// Check if file exists
		if _, err := os.Stat(absFull); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("file not found: %s", cleanPath)
			}
			return nil, fmt.Errorf("failed to stat file: %w", err)
		}

		// Delete the file
		if err := os.Remove(absFull); err != nil {
			return nil, fmt.Errorf("failed to delete file: %w", err)
		}

		return &Output{
			Message: fmt.Sprintf("Deleted: %s", details),
			Deleted: true,
		}, nil
	}
}
