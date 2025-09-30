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

// Service provides Git repository operations.
type Service interface {
	ReadStagedFiles() ([]string, error)
	ReadStagedChanges(filePath string) (string, error)
	ReadStagedFileContent(filePath string) (string, error)
	ReadOriginalFileContent(filePath string) (string, error)
}

// serviceImpl implements GitService.
type serviceImpl struct {
	worspaceService workspace.Service
}

// NewService creates a new Git service.
func NewService(workspaceService workspace.Service) Service {
	return &serviceImpl{
		worspaceService: workspaceService,
	}
}

// runGitCommand executes a git command with the provided arguments in the workspace directory.
func (g *serviceImpl) runGitCommand(args ...string) (string, error) {
	workspaceDir, err := g.worspaceService.GetWorkspaceDir()
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
	return g.runGitCommand("diff", "--cached", "--unified=0", "--", filePath)
}

// ReadStagedFileContent returns the current content of the specified file from the index (stage).
func (g *serviceImpl) ReadStagedFileContent(filePath string) (string, error) {
	return g.runGitCommand("show", ":"+filePath)
}

// ReadOriginalFileContent returns the content of the specified file from the HEAD commit.
func (g *serviceImpl) ReadOriginalFileContent(filePath string) (string, error) {
	return g.runGitCommand("show", "HEAD:"+filePath)
}
