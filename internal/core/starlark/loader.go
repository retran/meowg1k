// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"
	"os"
	"path/filepath"
)

// LoaderService handles loading Starlark scripts from the config hierarchy
type LoaderService struct {
	runtime *Runtime
}

// NewLoaderService creates a new script loader
func NewLoaderService(runtime *Runtime) *LoaderService {
	return &LoaderService{
		runtime: runtime,
	}
}

// LoadAll loads config with priority: project init.star if found, otherwise system init.star.
// If neither exists, nothing is loaded.
func (l *LoaderService) LoadAll() error {
	// 1. Try project init.star first
	projectInit := l.getProjectInitPath()
	if _, err := os.Stat(projectInit); err == nil {
		if err := l.loadScript(projectInit); err != nil {
			return fmt.Errorf("failed to load project init: %w", err)
		}
		return nil
	}

	// 2. Fall back to system init.star
	systemInit := l.getSystemInitPath()
	if _, err := os.Stat(systemInit); err == nil {
		if err := l.loadScript(systemInit); err != nil {
			return fmt.Errorf("failed to load system init: %w", err)
		}
	}

	return nil
}

// getSystemInitPath returns ~/.config/meowg1k/init.star
func (l *LoaderService) getSystemInitPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".config", "meowg1k", "init.star")
}

// getProjectInitPath returns ./.meowg1k/init.star
func (l *LoaderService) getProjectInitPath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Join(cwd, ".meowg1k", "init.star")
}

// loadScript loads and executes a single Starlark script
func (l *LoaderService) loadScript(path string) error {
	return l.runtime.LoadScript(path)
}
