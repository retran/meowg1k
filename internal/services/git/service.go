/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package git provides functionalities to interact with Git repositories.
package git

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/retran/meowg1k/internal/services/workspace"
)

// Git command errors
var (
	ErrGitCommandFailed = errors.New("git command failed")
)

// FileChange defines the output structure for the FetchFileDiff activity.
type FileChange struct {
	Filename            string
	Change              string
	OriginalFileContent string
	StagedFileContent   string
}

// Service provides Git repository operations.
type Service interface {
	ReadStagedFiles() ([]string, error)
	ReadStagedChanges(filePath string) (string, error)
	ReadStagedFileContent(filePath string) (string, error)
	ReadOriginalFileContent(filePath string) (string, error)
	GetCurrentBranch() (string, error)
	GetChangedFilesInBranch(targetBranch string) ([]string, error)
	GetBranchDiff(filePath, targetBranch string) (string, error)
}

// serviceImpl implements GitService.
type serviceImpl struct {
	workspaceService workspace.Service
	semaphore        chan struct{} // Worker pool with 1 worker for sequential execution
}

// NewService creates a new Git service.
// Git commands will be executed sequentially using a worker pool (semaphore with capacity 1)
// to prevent race conditions while keeping OS threads free.
func NewService(workspaceService workspace.Service) Service {
	return &serviceImpl{
		workspaceService: workspaceService,
		semaphore:        make(chan struct{}, 1), // Only 1 concurrent git command
	}
}

// runGitCommand executes a git command with the provided arguments in the workspace directory.
func (g *serviceImpl) runGitCommand(args ...string) (string, error) {
	workspaceDir, err := g.workspaceService.GetWorkspaceDir()
	if err != nil {
		return "", fmt.Errorf("could not get workspace directory: %w", err)
	}

	finalArgs := append([]string{"-C", workspaceDir}, args...)
	cmd := exec.Command("git", finalArgs...)

	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := string(exitErr.Stderr)
			return "", fmt.Errorf("%w: %s\nargs: %v", ErrGitCommandFailed, strings.TrimSpace(stderr), args)
		}
		return "", fmt.Errorf("failed to run git command: %w", err)
	}

	return string(out), nil
}

// ReadStagedFiles returns a list of files that are currently staged.
func (g *serviceImpl) ReadStagedFiles() ([]string, error) {
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
func (g *serviceImpl) ReadStagedChanges(filePath string) (string, error) {
	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	return g.runGitCommand("diff", "--cached", "--unified=0", "--", filePath)
}

// ReadStagedFileContent returns the current content of the specified file from the index (stage).
func (g *serviceImpl) ReadStagedFileContent(filePath string) (string, error) {
	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	return g.runGitCommand("show", ":"+filePath)
}

// ReadOriginalFileContent returns the content of the specified file from the HEAD commit.
func (g *serviceImpl) ReadOriginalFileContent(filePath string) (string, error) {
	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	return g.runGitCommand("show", "HEAD:"+filePath)
}

// GetCurrentBranch returns the name of the current Git branch.
func (g *serviceImpl) GetCurrentBranch() (string, error) {
	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	branch, err := g.runGitCommand("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(branch), nil
}

// GetChangedFilesInBranch returns the list of files that differ between the current branch and the target branch.
func (g *serviceImpl) GetChangedFilesInBranch(targetBranch string) ([]string, error) {
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

// GetBranchDiff returns the diff of a specific file between the current branch and the target branch.
func (g *serviceImpl) GetBranchDiff(filePath, targetBranch string) (string, error) {
	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	diff, err := g.runGitCommand("diff", "--unified=0", targetBranch+"...HEAD", "--", filePath)
	if err != nil {
		return "", fmt.Errorf("failed to get branch diff for %s: %w", filePath, err)
	}
	return diff, nil
}
