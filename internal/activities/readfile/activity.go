// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package readfile implements an activity for reading file contents.
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
	return func(_ context.Context, flowCtx *executor.Context, input *Input) (*Output, error) {
		workspaceRoot, err := f.workspaceService.Get()
		if err != nil {
			return nil, fmt.Errorf("failed to get workspace root: %w", err)
		}

		fullPath, cleanPath, err := resolveAndValidatePath(workspaceRoot, input.Path)
		if err != nil {
			return nil, err
		}

		f.sendRunningStatus(flowCtx, cleanPath, input.StartLine, input.EndLine)

		lines, err := readLines(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, executor.Expected(fmt.Errorf("file not found: %s", cleanPath))
			}
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		totalLines := len(lines)
		start, end := resolveLineRange(input.StartLine, input.EndLine, totalLines)

		if start > totalLines {
			return &Output{
				Content:     "",
				IsTruncated: true,
				TotalLines:  totalLines,
			}, nil
		}

		startIdx := start - 1
		endIdx := end
		content := strings.Join(lines[startIdx:endIdx], "\n")
		isTruncated := start > 1 || end < totalLines

		readLinesCount := endIdx - startIdx
		flowCtx.SendCompleted(fmt.Sprintf("Read %s (%d lines)", cleanPath, readLinesCount))

		return &Output{
			Content:     content,
			IsTruncated: isTruncated,
			TotalLines:  totalLines,
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

func (f *Factory) sendRunningStatus(flowCtx *executor.Context, cleanPath string, startLine, endLine int) {
	readTitle := fmt.Sprintf("Reading %s", cleanPath)
	if startLine > 0 || endLine > 0 {
		start := startLine
		if start < 1 {
			start = 1
		}
		if endLine > 0 {
			readTitle = fmt.Sprintf("Reading %s (lines %d-%d)", cleanPath, start, endLine)
		} else {
			readTitle = fmt.Sprintf("Reading %s (from line %d)", cleanPath, start)
		}
	}
	flowCtx.SendRunning(readTitle)
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Explicitly ignore close error during read
			_ = closeErr
		}
	}()

	var lines []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}
	return lines, nil
}

func resolveLineRange(inputStart, inputEnd, totalLines int) (start, end int) {
	start = inputStart
	end = inputEnd
	if start < 1 {
		start = 1
	}
	if end == 0 || end > totalLines {
		end = totalLines
	}
	return start, end
}
