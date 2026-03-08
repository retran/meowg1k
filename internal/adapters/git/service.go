// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

// Package git provides a service for executing git commands to read staged files, diffs, and branch information.
package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Service implements GitService.
type Service struct {
	semaphore    chan struct{}
	workspaceDir string
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
		semaphore:    make(chan struct{}, 1), // Only 1 in-flight git command.
	}, nil
}

// runGitCommand executes a git command with the provided arguments in the workspace directory.
func (g *Service) runGitCommand(args ...string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("git service is nil")
	}

	finalArgs := append([]string{"-C", g.workspaceDir}, args...)
	// #nosec G204 - git command with controlled arguments, not shell execution
	cmd := exec.CommandContext(context.Background(), "git", finalArgs...)

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
// For renamed files, returns the new filename. Excludes deleted files.
func (g *Service) ReadStagedFiles() ([]string, error) {
	if g == nil {
		return nil, fmt.Errorf("git service is nil")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	out, err := g.runGitCommand("diff", "--cached", "--name-status", "-M", "--diff-filter=d")
	if err != nil {
		return nil, fmt.Errorf("failed to read staged files: %w", err)
	}

	output := strings.TrimSpace(out)
	if output == "" {
		return []string{}, nil
	}

	return parseNameStatus(output), nil
}

// ReadStagedChanges returns the staged changes (diff) for a specific file.
func (g *Service) ReadStagedChanges(filePath string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("git service is nil")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	return g.runGitCommand("diff", "--cached", "--unified=0", "-M", "--", filePath)
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

// parseNameStatus parses the output of git diff --name-status.
// Format: "<status>\t<filename>" or "R<similarity>\t<old>\t<new>" for renames.
// Returns the list of affected filenames (new name for renames).
func parseNameStatus(output string) []string {
	files, _ := parseNameStatusWithRenames(output)
	return files
}

// parseNameStatusWithRenames parses the output of git diff --name-status.
// Returns both the list of files and a map of rename pairs (old -> new).
func parseNameStatusWithRenames(output string) (files []string, renames map[string]string) {
	lines := strings.Split(output, "\n")
	files = make([]string, 0, len(lines))
	renames = make(map[string]string)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Split by tab
		parts := strings.Split(trimmed, "\t")
		if len(parts) < 2 {
			continue
		}

		status := parts[0]

		if strings.HasPrefix(status, "R") {
			// For renames: R<similarity>\told\tnew
			if len(parts) >= 3 {
				oldPath := parts[1]
				newPath := parts[2]
				files = append(files, newPath)
				renames[newPath] = oldPath
			}
		} else {
			files = append(files, parts[1])
		}
	}

	return files, renames
}

// GetChangedFilesInBranch returns the list of files that differ between the current branch and the target branch.
// For renamed files, returns the new filename.
func (g *Service) GetChangedFilesInBranch(targetBranch string) ([]string, error) {
	files, _, err := g.GetChangedFilesInBranchWithRenames(targetBranch)
	return files, err
}

// GetChangedFilesInBranchWithRenames returns the list of files that differ between the current branch and the target branch.
// Also returns a map of renames (new filename -> old filename).
func (g *Service) GetChangedFilesInBranchWithRenames(targetBranch string) (files []string, renames map[string]string, err error) {
	if g == nil {
		return nil, nil, fmt.Errorf("git service is nil")
	}

	if strings.TrimSpace(targetBranch) == "" {
		return nil, nil, fmt.Errorf("target branch cannot be empty")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	output, err := g.runGitCommand("diff", "--name-status", "-M", targetBranch+"...HEAD")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get changed files in branch: %w", err)
	}

	if output == "" {
		return []string{}, make(map[string]string), nil
	}

	files, renames = parseNameStatusWithRenames(strings.TrimSpace(output))

	return files, renames, nil
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
	return g.GetBranchDiffWithOldPath(filePath, targetBranch, "")
}

// GetBranchDiffWithOldPath returns the diff of a specific file between the current branch and the target branch.
// If oldPath is provided, it will be used to fetch the diff (for renamed files).
func (g *Service) GetBranchDiffWithOldPath(filePath, targetBranch, oldPath string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("git service is nil")
	}

	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}

	if strings.TrimSpace(targetBranch) == "" {
		return "", fmt.Errorf("target branch cannot be empty")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	// For renamed files, query both old and new paths to get rename headers
	var diff string
	var err error
	if oldPath != "" {
		diff, err = g.runGitCommand("diff", "--unified=0", "-M", targetBranch+"...HEAD", "--", oldPath, filePath)
	} else {
		diff, err = g.runGitCommand("diff", "--unified=0", "-M", targetBranch+"...HEAD", "--", filePath)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get branch diff for %s: %w", filePath, err)
	}

	return diff, nil
}

// Status returns git status in porcelain format.
func (g *Service) Status() (string, error) {
	if g == nil {
		return "", fmt.Errorf("git service is nil")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	output, err := g.runGitCommand("status", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w", err)
	}

	return strings.TrimSpace(output), nil
}

// Diff returns git diff for a ref and optional path.
func (g *Service) Diff(ref, path string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("git service is nil")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	args := []string{"diff"}
	if strings.TrimSpace(ref) != "" {
		args = append(args, ref)
	}
	if strings.TrimSpace(path) != "" {
		args = append(args, "--", path)
	}

	output, err := g.runGitCommand(args...)
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %w", err)
	}

	return output, nil
}

// Show returns git show output for a ref.
func (g *Service) Show(ref string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("git service is nil")
	}

	if strings.TrimSpace(ref) == "" {
		return "", fmt.Errorf("ref cannot be empty")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	output, err := g.runGitCommand("show", ref)
	if err != nil {
		return "", fmt.Errorf("failed to show ref %s: %w", ref, err)
	}

	return output, nil
}

// Log returns git log output with optional limit and path.
func (g *Service) Log(limit int, path string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("git service is nil")
	}

	if limit <= 0 {
		limit = 10
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	args := []string{"log", fmt.Sprintf("-n%d", limit), "--oneline"}
	if strings.TrimSpace(path) != "" {
		args = append(args, "--", path)
	}

	output, err := g.runGitCommand(args...)
	if err != nil {
		return "", fmt.Errorf("failed to get log: %w", err)
	}

	return strings.TrimSpace(output), nil
}

// Branches returns a list of local branches.
func (g *Service) Branches() ([]string, error) {
	if g == nil {
		return nil, fmt.Errorf("git service is nil")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	output, err := g.runGitCommand("branch", "--list")
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	branches := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "*"))
		if trimmed != "" {
			branches = append(branches, trimmed)
		}
	}

	return branches, nil
}

// CurrentBranch returns the current branch name.
func (g *Service) CurrentBranch() (string, error) {
	return g.GetCurrentBranch()
}

// Stage stages the provided paths or all changes if none are provided.
func (g *Service) Stage(paths []string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("git service is nil")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	args := []string{"add"}
	if len(paths) == 0 {
		args = append(args, "-A")
	} else {
		args = append(args, "--")
		args = append(args, paths...)
	}

	output, err := g.runGitCommand(args...)
	if err != nil {
		return "", fmt.Errorf("failed to stage files: %w", err)
	}

	return strings.TrimSpace(output), nil
}

// Commit creates a new commit with the provided message.
func (g *Service) Commit(message string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("git service is nil")
	}

	if strings.TrimSpace(message) == "" {
		return "", fmt.Errorf("commit message cannot be empty")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	output, err := g.runGitCommand("commit", "-m", message)
	if err != nil {
		return "", fmt.Errorf("failed to commit: %w", err)
	}

	return strings.TrimSpace(output), nil
}

// HeadHash returns the current HEAD hash.
func (g *Service) HeadHash() (string, error) {
	if g == nil {
		return "", fmt.Errorf("git service is nil")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	output, err := g.runGitCommand("rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD hash: %w", err)
	}

	return strings.TrimSpace(output), nil
}
