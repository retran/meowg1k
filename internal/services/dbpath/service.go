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

// Package dbpath provides services for determining database file paths.
package dbpath

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrServiceIsNil indicates that the service is nil.
var ErrServiceIsNil = errors.New("service is nil")

// Service is the concrete implementation of the database path service.
type Service struct {
	mainDBPath string
}

// NewService creates a new database path service.
// It determines the appropriate location for the main database file
// based on XDG Base Directory specification or fallback to current directory.
func NewService() *Service {
	return &Service{
		mainDBPath: determineMainDBPath(),
	}
}

// GetMainDBPath returns the path to the main database file.
func (s *Service) GetMainDBPath() (string, error) {
	if s == nil {
		return "", ErrServiceIsNil
	}
	return s.mainDBPath, nil
}

// determineMainDBPath determines the appropriate location for the main database file.
// It follows XDG Base Directory specification:
// 1. $XDG_DATA_HOME/meowg1k/meowg1k.db
// 2. $HOME/.local/share/meowg1k/meowg1k.db
// 3. ./meowg1k.db (current directory as fallback)
func determineMainDBPath() string {
	// Try XDG_DATA_HOME first
	if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
		dbDir := filepath.Join(xdgDataHome, "meowg1k")
		if err := os.MkdirAll(dbDir, 0o755); err == nil {
			return filepath.Join(dbDir, "meowg1k.db")
		}
	}

	// Try HOME/.local/share as fallback
	if home := os.Getenv("HOME"); home != "" {
		dbDir := filepath.Join(home, ".local", "share", "meowg1k")
		if err := os.MkdirAll(dbDir, 0o755); err == nil {
			return filepath.Join(dbDir, "meowg1k.db")
		}
	}

	// Fallback to current directory
	return "meowg1k.db"
}
