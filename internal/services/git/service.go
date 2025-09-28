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

package git

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
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
type serviceImpl struct{}

// NewService creates a new Git service.
func NewService() Service {
	return &serviceImpl{}
}

// runGitCommand is a helper function that executes any git command and returns its output.
func (g *serviceImpl) runGitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()

	if err != nil {
		// If an error occurs, try to get more detailed information from stderr.
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
	return g.runGitCommand("diff", "--cached", "--unified=0", filePath)
}

// ReadStagedFileContent returns the current content of the specified file from the index (stage).
func (g *serviceImpl) ReadStagedFileContent(filePath string) (string, error) {
	return g.runGitCommand("show", ":"+filePath)
}

// ReadOriginalFileContent returns the content of the specified file from the HEAD commit.
func (g *serviceImpl) ReadOriginalFileContent(filePath string) (string, error) {
	return g.runGitCommand("show", "HEAD:"+filePath)
}
