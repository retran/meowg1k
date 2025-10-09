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
)

// Git command errors
var (
	ErrGitCommandFailed          = errors.New("git command failed")
	ErrServiceIsNil              = errors.New("git service is nil")
	ErrFilePathEmpty             = errors.New("file path cannot be empty")
	ErrWorkspaceDirProviderIsNil = errors.New("workspace dir provider is nil")
)

// FileChange defines the output structure for the FetchFileDiff activity.
type FileChange struct {
	Filename            string
	Change              string
	OriginalFileContent string
	ChangedFileContent  string
}

type WorkspaceDirProvider interface {
	GetWorkspaceDir() (string, error)
}

// Service implements GitService.
type Service struct {
	workspaceDir string
	semaphore    chan struct{} // Worker pool with 1 worker for sequential execution
}

// NewService creates a new Git service.
// Git commands will be executed sequentially using a worker pool (semaphore with capacity 1)
// to prevent race conditions while keeping OS threads free.
func NewService(workspaceDirProvider WorkspaceDirProvider) (*Service, error) {
	if workspaceDirProvider == nil {
		return nil, ErrWorkspaceDirProviderIsNil
	}

	workspaceDir, err := workspaceDirProvider.GetWorkspaceDir()
	if err != nil {
		// TODO proper error
		return nil, err
	}

	return &Service{
		workspaceDir: workspaceDir,
		semaphore:    make(chan struct{}, 1), // Only 1 concurrent git command
	}, nil
}

// runGitCommand executes a git command with the provided arguments in the workspace directory.
func (g *Service) runGitCommand(args ...string) (string, error) {
	if g == nil {
		return "", ErrServiceIsNil
	}

	finalArgs := append([]string{"-C", g.workspaceDir}, args...)
	// #nosec G204 - git command with controlled arguments, not shell execution
	cmd := exec.Command("git", finalArgs...)

	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := string(exitErr.Stderr)
			// TODO proper error
			return "", fmt.Errorf("%w: %s\nargs: %v", ErrGitCommandFailed, strings.TrimSpace(stderr), args)
		}
		// TODO proper error
		return "", fmt.Errorf("failed to run git command: %w", err)
	}

	return string(out), nil
}

// ReadStagedFiles returns a list of files that are currently staged.
func (g *Service) ReadStagedFiles() ([]string, error) {
	if g == nil {
		return nil, ErrServiceIsNil
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	out, err := g.runGitCommand("diff", "--cached", "--name-only")
	if err != nil {
		// TODO proper error
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
		return "", ErrServiceIsNil
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	return g.runGitCommand("diff", "--cached", "--unified=0", "--", filePath)
}

// ReadStagedFileContent returns the current content of the specified file from the index (stage).
func (g *Service) ReadStagedFileContent(filePath string) (string, error) {
	if g == nil {
		return "", ErrServiceIsNil
	}

	if strings.TrimSpace(filePath) == "" {
		return "", ErrFilePathEmpty
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	return g.runGitCommand("show", ":"+filePath)
}

// ReadOriginalFileContent returns the content of the specified file from the HEAD commit.
func (g *Service) ReadOriginalFileContent(filePath string) (string, error) {
	if g == nil {
		return "", ErrServiceIsNil
	}

	if strings.TrimSpace(filePath) == "" {
		return "", ErrFilePathEmpty
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	return g.runGitCommand("show", "HEAD:"+filePath)
}

// GetCurrentBranch returns the name of the current Git branch.
func (g *Service) GetCurrentBranch() (string, error) {
	if g == nil {
		return "", ErrServiceIsNil
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	branch, err := g.runGitCommand("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		// TODO proper error
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(branch), nil
}

// GetChangedFilesInBranch returns the list of files that differ between the current branch and the target branch.
func (g *Service) GetChangedFilesInBranch(targetBranch string) ([]string, error) {
	if g == nil {
		return nil, ErrServiceIsNil
	}

	if strings.TrimSpace(targetBranch) == "" {
		// TODO proper error
		return nil, errors.New("target branch cannot be empty")
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	output, err := g.runGitCommand("diff", "--name-only", targetBranch+"...HEAD")
	if err != nil {
		// TODO proper error
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
func (g *Service) GetBranchDiff(filePath, targetBranch string) (string, error) {
	if g == nil {
		return "", ErrServiceIsNil
	}

	if strings.TrimSpace(filePath) == "" {
		return "", ErrFilePathEmpty
	}

	g.semaphore <- struct{}{}
	defer func() { <-g.semaphore }()

	diff, err := g.runGitCommand("diff", "--unified=0", targetBranch+"...HEAD", "--", filePath)
	if err != nil {
		// TODO proper error
		return "", fmt.Errorf("failed to get branch diff for %s: %w", filePath, err)
	}

	return diff, nil
}
