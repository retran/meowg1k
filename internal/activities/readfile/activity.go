// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package readfile

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input for reading a file.
type Input struct {
	Path      string
	StartLine int // 1-based, inclusive
	EndLine   int // 1-based, inclusive. 0 means to the end.
}

// Output defines the output of the read operation.
type Output struct {
	Content     string
	IsTruncated bool
	TotalLines  int
}

// Factory builds readfile activities.
type Factory struct {
	workspaceService ports.WorkspaceService
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new readfile activity factory.
func NewFactory(workspaceService ports.WorkspaceService) *Factory {
	return &Factory{
		workspaceService: workspaceService,
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

		flowCtx.SendRunningWithDetails("Reading file", fmt.Sprintf("path=%s lines=%d-%d", cleanPath, input.StartLine, input.EndLine))

		file, err := os.Open(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("file not found: %s", cleanPath)
			}
			return nil, fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		var lines []string
		scanner := bufio.NewScanner(file)
		// Increase buffer size to handle long lines.
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		totalLines := len(lines)
		start := input.StartLine
		end := input.EndLine

		if start < 1 {
			start = 1
		}
		if end == 0 || end > totalLines {
			end = totalLines
		}

		if start > totalLines {
			return &Output{
					Content:     "",
					IsTruncated: true,
					TotalLines:  totalLines,
				},
				nil
		}

		// Adjust to 0-based index
		startIdx := start - 1
		endIdx := end

		content := strings.Join(lines[startIdx:endIdx], "\n")
		isTruncated := start > 1 || end < totalLines

		flowCtx.SendCompletedWithDetails("Read file", fmt.Sprintf("lines=%d total=%d", endIdx-startIdx, totalLines))

		return &Output{
				Content:     content,
				IsTruncated: isTruncated,
				TotalLines:  totalLines,
			},
			nil
	}
}
