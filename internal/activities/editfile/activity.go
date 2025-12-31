// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package editfile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input for editing a file.
type Input struct {
	Path      string
	OldString string
	NewString string
}

// Output defines the output of the edit operation.
type Output struct {
	Applied bool
	Message string
}

// Factory builds editfile activities.
type Factory struct {
	workspaceService ports.WorkspaceService
	dryRun           bool
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new editfile activity factory.
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

		if input.OldString == "" {
			return nil, fmt.Errorf("old_string is required")
		}

		flowCtx.SendRunningWithDetails("Editing file", fmt.Sprintf("path=%s", cleanPath))

		contentBytes, err := os.ReadFile(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("file not found: %s", cleanPath)
			}
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
		content := string(contentBytes)

		count := strings.Count(content, input.OldString)
		if count == 0 {
			return nil, fmt.Errorf("original string not found in file")
		}
		if count > 1 {
			return nil, fmt.Errorf("ambiguous replacement: original string found %d times", count)
		}

		newContent := strings.Replace(content, input.OldString, input.NewString, 1)

		details := fmt.Sprintf("path=%s old_len=%d new_len=%d", cleanPath, len(input.OldString), len(input.NewString))
		if f.dryRun {
			details += " (DRY RUN)"
		}

		if f.dryRun {
			flowCtx.SendCompletedWithDetails("Skipped editing file (dry run)", details)
			return &Output{
				Applied: false,
				Message: fmt.Sprintf("Dry run: would have replaced text in %s", cleanPath),
			}, nil
		}

		if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}

		flowCtx.SendCompletedWithDetails("Edited file", details)

		return &Output{
			Applied: true,
			Message: fmt.Sprintf("Successfully edited %s", cleanPath),
		}, nil
	}
}
