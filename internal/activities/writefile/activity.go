// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package writefile implements an activity for creating or overwriting files.
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
	Message string
	Written bool
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
	return func(_ context.Context, flowCtx *executor.Context, input *Input) (*Output, error) {
		workspaceRoot, err := f.workspaceService.Get()
		if err != nil {
			return nil, fmt.Errorf("failed to get workspace root: %w", err)
		}

		fullPath, cleanPath, err := resolveAndValidatePath(workspaceRoot, input.Path)
		if err != nil {
			return nil, err
		}

		if f.dryRun {
			flowCtx.SendRunning(fmt.Sprintf("Writing %s (dry run)", cleanPath))
			flowCtx.SendCompleted(fmt.Sprintf("Skipped writing %s (dry run)", cleanPath))
			return &Output{
				Written: false,
				Message: fmt.Sprintf("Dry run: would have written %d bytes to %s", len(input.Content), cleanPath),
			}, nil
		}

		flowCtx.SendRunning(fmt.Sprintf("Writing %s", cleanPath))

		// Create directory if it doesn't exist
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}

		if err := os.WriteFile(fullPath, []byte(input.Content), 0o600); err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}

		flowCtx.SendCompleted(fmt.Sprintf("Wrote %s (%d bytes)", cleanPath, len(input.Content)))

		return &Output{
			Written: true,
			Message: fmt.Sprintf("Successfully wrote %d bytes to %s", len(input.Content), cleanPath),
		}, nil
	}
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
