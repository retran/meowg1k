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

// Package workspace provides services for resolving and managing the workspace directory path.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// WorkspacePathResolver resolves the explicitly set workspace path from command flags.
type WorkspacePathResolver interface {
	GetWorkspacePath() (string, error)
}

type Service struct {
	workspacePathResolver WorkspacePathResolver
}

// NewService creates a new workspace service instance.
func NewService(workspacePathResolver WorkspacePathResolver) *Service {
	return &Service{
		workspacePathResolver: workspacePathResolver,
	}
}

// Get returns the workspace root directory.
// If --workspace flag is set, returns that path.
// Otherwise, walks up from the current directory looking for .meowg1k.yaml, .meowg1k.yml, or .git directory.
// If none are found, returns the current working directory.
func (g *Service) Get() (string, error) {
	if g == nil {
		return "", fmt.Errorf("workspace service is nil")
	}

	// Check if workspace path is explicitly set via flag
	if g.workspacePathResolver != nil {
		explicitPath, err := g.workspacePathResolver.GetWorkspacePath()
		if err == nil && explicitPath != "" {
			// Verify the path exists and is a directory
			info, err := os.Stat(explicitPath)
			if err != nil {
				return "", fmt.Errorf("workspace path does not exist: %s", explicitPath)
			}
			if !info.IsDir() {
				return "", fmt.Errorf("workspace path is not a directory: %s", explicitPath)
			}
			return explicitPath, nil
		}
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Walk up the directory tree looking for workspace markers
	dir := currentDir
	for {
		// Check for .meowg1k.yaml
		if _, err := os.Stat(filepath.Join(dir, ".meowg1k.yaml")); err == nil {
			return dir, nil
		}

		// Check for .meowg1k.yml
		if _, err := os.Stat(filepath.Join(dir, ".meowg1k.yml")); err == nil {
			return dir, nil
		}

		// Check for .git directory
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
			return dir, nil
		}

		// Move to parent directory
		parent := filepath.Dir(dir)

		// Check if we've reached the root
		if parent == dir {
			// We've reached the filesystem root without finding any markers
			// Return the original current directory
			return currentDir, nil
		}

		dir = parent
	}
}
