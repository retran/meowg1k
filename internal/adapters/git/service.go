// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package git provides a service for executing git commands to read staged files, diffs, and branch information.
package git

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Service implements GitService.
type Service struct {
	workspaceDir string
	semaphore    chan struct{} // Worker pool with 1 worker for sequential execution
}

// WorkspaceDirProvider provides the workspace directory path.
type WorkspaceDirProvider interface {
	Get() (string, error)
}

// NewService creates a new Git service.
// Git commands will be executed sequentially using a worker pool (semaphore with capacity 1)
// to prevent race conditions while keeping OS threads free.
func NewService(workspaceDirProvider WorkspaceDirProvider) (*Service, error) {
	if workspaceDirProvider == nil {
		return nil, fmt.Errorf("workspace dir provider is nil")
	}

	workspaceDir, err := workspaceDirProvider.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace directory: %w", err)
	}

	return &Service{
		workspaceDir: workspaceDir,
		semaphore:    make(chan struct{}, 1), // Only 1 concurrent git command
	}, nil
}

// runGitCommand executes a git command with the provided arguments in the workspace directory.
func (g *Service) runGitCommand(args ...string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("git service is nil")
	}

	finalArgs := append([]string{"-C", g.workspaceDir}, args...)
	// #nosec G204 - git command with controlled arguments, not shell execution
	cmd := exec.Command("git", finalArgs...)

	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := string(exitErr.Stderr)
			return "", fmt.Errorf("git command failed: %s\nargs: %v", strings.TrimSpace(stderr), args)
		}
		return "", fmt.Errorf("failed to run git command: %w", err)
	}

	return string(out), nil
}

// ReadStagedFiles returns a list of files that are currently staged.
func (g *Service) ReadStagedFiles() ([]string, error) {
	if g == nil {
		return nil, fmt.Errorf("git service is nil")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	out, err := g.runGitCommand("diff", "--cached", "--name-only")
	if err != nil {
		return nil, fmt.Errorf("failed to read staged files: %w", err)
	}

	output := strings.TrimSpace(out)
	if output == "" {
		return []string{}, nil
	}

	return strings.Split(output, "\n"), nil
}

// ReadStagedChanges returns the staged changes (diff) for a specific file.
func (g *Service) ReadStagedChanges(filePath string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("git service is nil")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	return g.runGitCommand("diff", "--cached", "--unified=0", "--", filePath)
}

// ReadStagedFileContent returns the current content of the specified file from the index (stage).
func (g *Service) ReadStagedFileContent(filePath string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("git service is nil")
	}

	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	return g.runGitCommand("show", ":"+filePath)
}

// ReadOriginalFileContent returns the content of the specified file from the HEAD commit.
func (g *Service) ReadOriginalFileContent(filePath string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("git service is nil")
	}

	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	return g.runGitCommand("show", "HEAD:"+filePath)
}

// GetCurrentBranch returns the name of the current Git branch.
func (g *Service) GetCurrentBranch() (string, error) {
	if g == nil {
		return "", fmt.Errorf("git service is nil")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	branch, err := g.runGitCommand("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(branch), nil
}

// GetChangedFilesInBranch returns the list of files that differ between the current branch and the target branch.
func (g *Service) GetChangedFilesInBranch(targetBranch string) ([]string, error) {
	if g == nil {
		return nil, fmt.Errorf("git service is nil")
	}

	if strings.TrimSpace(targetBranch) == "" {
		return nil, fmt.Errorf("target branch cannot be empty")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	output, err := g.runGitCommand("diff", "--name-only", targetBranch+"...HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files in branch: %w", err)
	}

	if output == "" {
		return []string{}, nil
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			files = append(files, trimmed)
		}
	}

	return files, nil
}

// ListFiles returns a list of all files in the specified commit/ref.
func (g *Service) ListFiles(ref string) ([]string, error) {
	if g == nil {
		return nil, fmt.Errorf("git service is nil")
	}

	if strings.TrimSpace(ref) == "" {
		return nil, fmt.Errorf("ref cannot be empty")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	output, err := g.runGitCommand("ls-tree", "-r", "--name-only", ref)
	if err != nil {
		return nil, fmt.Errorf("failed to list files at ref %s: %w", ref, err)
	}

	if output == "" {
		return []string{}, nil
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			files = append(files, trimmed)
		}
	}

	return files, nil
}

// ReadFileAtCommit reads the content of a file at a specific commit/ref.
func (g *Service) ReadFileAtCommit(ref, filePath string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("git service is nil")
	}

	if strings.TrimSpace(ref) == "" {
		return "", fmt.Errorf("ref cannot be empty")
	}

	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	content, err := g.runGitCommand("show", ref+":"+filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s at ref %s: %w", filePath, ref, err)
	}

	return content, nil
}

// GetBranchDiff returns the diff of a specific file between the current branch and the target branch.
func (g *Service) GetBranchDiff(filePath, targetBranch string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("git service is nil")
	}

	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	diff, err := g.runGitCommand("diff", "--unified=0", targetBranch+"...HEAD", "--", filePath)
	if err != nil {
		return "", fmt.Errorf("failed to get branch diff for %s: %w", filePath, err)
	}

	return diff, nil
}
