// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package editfile implements an activity for editing files by string replacement.
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
	Message string
	Applied bool
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
	return func(_ context.Context, flowCtx *executor.Context, input *Input) (*Output, error) {
		workspaceRoot, err := f.workspaceService.Get()
		if err != nil {
			return nil, fmt.Errorf("failed to get workspace root: %w", err)
		}

		fullPath, cleanPath, err := resolveAndValidatePath(workspaceRoot, input.Path)
		if err != nil {
			return nil, err
		}

		if input.OldString == "" {
			return nil, fmt.Errorf("old_string is required")
		}

		if f.dryRun {
			flowCtx.SendRunning(fmt.Sprintf("Editing %s (dry run)", cleanPath))
		} else {
			flowCtx.SendRunning(fmt.Sprintf("Editing %s", cleanPath))
		}

		newContent, err := f.performEdit(fullPath, cleanPath, input.OldString, input.NewString)
		if err != nil {
			return nil, err
		}

		if f.dryRun {
			flowCtx.SendCompleted(fmt.Sprintf("Skipped editing %s (dry run)", cleanPath))
			return &Output{
				Applied: false,
				Message: fmt.Sprintf("Dry run: would have replaced text in %s", cleanPath),
			}, nil
		}

		if err := os.WriteFile(fullPath, []byte(newContent), 0o600); err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}

		flowCtx.SendCompleted(fmt.Sprintf("Edited %s", cleanPath))

		return &Output{
			Applied: true,
			Message: fmt.Sprintf("Successfully edited %s", cleanPath),
		}, nil
	}
}

func (f *Factory) performEdit(fullPath, cleanPath, oldString, newString string) (string, error) {
	contentBytes, err := os.ReadFile(fullPath) // #nosec G304
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("edit error: %w", executor.Expected(fmt.Errorf("file not found: %s", cleanPath)))
		}
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	content := string(contentBytes)

	count := strings.Count(content, oldString)
	if count == 0 {
		return "", fmt.Errorf("edit error: %w", executor.Expected(fmt.Errorf(
			"edit failed for %s: old_string not found (it must match the file content exactly, including spaces/newlines)",
			cleanPath,
		)))
	}
	if count > 1 {
		return "", fmt.Errorf("edit error: %w", executor.Expected(fmt.Errorf(
			"edit failed for %s: old_string matched %d times (make old_string longer/more specific)",
			cleanPath,
			count,
		)))
	}

	return strings.Replace(content, oldString, newString, 1), nil
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
