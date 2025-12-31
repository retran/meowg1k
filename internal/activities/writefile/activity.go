// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package writefile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input for writing a file.
type Input struct {
	Path    string
	Content string
}

// Output defines the output of the write operation.
type Output struct {
	Written bool
	Message string
}

// Factory builds writefile activities.
type Factory struct {
	workspaceService ports.WorkspaceService
	dryRun           bool
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new writefile activity factory.
func NewFactory(workspaceService ports.WorkspaceService, dryRun bool) *Factory {
	return &Factory{
		workspaceService: workspaceService,
		dryRun:           dryRun,
	}
}

// NewActivity creates the activity.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, flowCtx *executor.Context, input *Input) (*Output, error) {
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

		details := fmt.Sprintf("path=%s size=%d", cleanPath, len(input.Content))
		if f.dryRun {
			details += " (DRY RUN)"
		}
		flowCtx.SendRunningWithDetails("Writing file", details)

		if f.dryRun {
			flowCtx.SendCompletedWithDetails("Skipped writing file (dry run)", details)
			return &Output{
				Written: false,
				Message: fmt.Sprintf("Dry run: would have written %d bytes to %s", len(input.Content), cleanPath),
			}, nil
		}

		// Create directory if it doesn't exist
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		if err := os.WriteFile(fullPath, []byte(input.Content), 0644); err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}

		flowCtx.SendCompletedWithDetails("Wrote file", details)

		return &Output{
			Written: true,
			Message: fmt.Sprintf("Successfully wrote %d bytes to %s", len(input.Content), cleanPath),
		}, nil
	}
}
