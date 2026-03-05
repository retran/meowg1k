// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package workspace provides services for resolving and managing the workspace directory path.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// PathResolver resolves the explicitly set workspace path from command flags.
type PathResolver interface {
	GetWorkspacePath() (string, error)
}

// Service resolves workspace paths based on flags and local markers.
type Service struct {
	workspacePathResolver PathResolver
}

// NewService creates a new workspace service instance.
func NewService(workspacePathResolver PathResolver) *Service {
	return &Service{
		workspacePathResolver: workspacePathResolver,
	}
}

// NewServiceWithPath creates a workspace service with a fixed path.
// This is used during Starlark initialization when command flags are not available.
func NewServiceWithPath(path string) *Service {
	return &Service{
		workspacePathResolver: &fixedPathResolver{path: path},
	}
}

// fixedPathResolver always returns a fixed workspace path
type fixedPathResolver struct {
	path string
}

func (f *fixedPathResolver) GetWorkspacePath() (string, error) {
	return f.path, nil
}

// Get returns the workspace root directory.
// If --workspace flag is set, returns that path.
// Otherwise, walks up from the current directory looking for .meowg1k.yaml, .meowg1k.yml, or .git directory.
// If none are found, returns the current working directory.
func (g *Service) Get() (string, error) {
	if g == nil {
		return "", fmt.Errorf("workspace service is nil")
	}

	explicitPath, ok, err := g.resolveExplicitPath()
	if err != nil {
		return "", err
	}
	if ok {
		return explicitPath, nil
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	foundRoot, err := findWorkspaceRoot(currentDir)
	if err != nil {
		return "", err
	}
	if foundRoot != "" {
		return foundRoot, nil
	}
	return currentDir, nil
}

func (g *Service) resolveExplicitPath() (explicitPath string, resolved bool, err error) {
	if g.workspacePathResolver == nil {
		return "", false, nil
	}

	explicitPath, err = g.workspacePathResolver.GetWorkspacePath()
	if err != nil || explicitPath == "" {
		if err != nil {
			return "", false, fmt.Errorf("failed to resolve workspace path: %w", err)
		}
		return "", false, nil
	}

	info, err := os.Stat(explicitPath)
	if err != nil {
		return "", false, fmt.Errorf("workspace path does not exist: %s", explicitPath)
	}
	if !info.IsDir() {
		return "", false, fmt.Errorf("workspace path is not a directory: %s", explicitPath)
	}

	return explicitPath, true, nil
}

func findWorkspaceRoot(startDir string) (string, error) {
	dir := startDir
	for {
		found, err := hasWorkspaceMarker(dir)
		if err != nil {
			return "", err
		}
		if found {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

func hasWorkspaceMarker(dir string) (bool, error) {
	if _, err := os.Stat(filepath.Join(dir, ".meowg1k.yaml")); err == nil {
		return true, nil
	}
	if _, err := os.Stat(filepath.Join(dir, ".meowg1k.yml")); err == nil {
		return true, nil
	}

	// .meowg1k directory (with init.star) is also a valid workspace marker
	if info, err := os.Stat(filepath.Join(dir, ".meowg1k")); err == nil && info.IsDir() {
		if _, err2 := os.Stat(filepath.Join(dir, ".meowg1k", "init.star")); err2 == nil {
			return true, nil
		}
	}

	info, err := os.Stat(filepath.Join(dir, ".git"))
	if err == nil && info.IsDir() {
		return true, nil
	}
	if err == nil || os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to stat workspace marker in %s: %w", dir, err)
}
