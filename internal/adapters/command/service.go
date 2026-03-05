// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package command provides services for accessing command-line flags and input streams.
package command

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// Service is the concrete implementation of the command service.
type Service struct {
	cmd         *cobra.Command
	stdinOnce   sync.Once
	stdinCached string
}

// NewService creates a new command context service with the provided command.
func NewService(cmd *cobra.Command) (*Service, error) {
	if cmd == nil {
		return nil, fmt.Errorf("command cannot be nil")
	}

	return &Service{
		cmd: cmd,
	}, nil
}

// GetCommandName retrieves the name of the current executing command.
func (s *Service) GetCommandName() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return "", fmt.Errorf("command is nil")
	}

	return s.cmd.Name(), nil
}

// GetWorkspacePath retrieves the workspace path from command flags.
func (s *Service) GetWorkspacePath() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return "", fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetString("workspace")
	if err != nil {
		return "", fmt.Errorf("failed to get workspace flag: %w", err)
	}
	return val, nil
}

// GetTaskName retrieves the task name from command flags.
func (s *Service) GetTaskName() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return "", fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetString("task")
	if err != nil {
		return "", fmt.Errorf("failed to get task flag: %w", err)
	}
	return val, nil
}

// GetUserPrompt retrieves the user prompt from command flags.
func (s *Service) GetUserPrompt() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return "", fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetString("user-prompt")
	if err != nil {
		return "", fmt.Errorf("failed to get user-prompt flag: %w", err)
	}
	return val, nil
}

// GetNoTUIFlag retrieves the no-tui flag from command flags.
func (s *Service) GetNoTUIFlag() (bool, error) {
	if s == nil {
		return false, fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return false, fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetBool("no-tui")
	if err != nil {
		return false, fmt.Errorf("failed to get no-tui flag: %w", err)
	}
	return val, nil
}

// GetStdIn retrieves the standard input sent to the command.
// It is read lazily on first call to avoid consuming stdin before
// the Starlark handler has a chance to read it via ctx.stdin.
func (s *Service) GetStdIn() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	s.stdinOnce.Do(func() {
		stat, err := os.Stdin.Stat()
		if err != nil {
			return
		}
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return
		}
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return
		}
		s.stdinCached = strings.TrimSpace(string(input))
	})

	return s.stdinCached, nil
}

// GetNoCacheFlag retrieves the no-cache flag from command flags.
func (s *Service) GetNoCacheFlag() (bool, error) {
	if s == nil {
		return false, fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return false, fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetBool("no-cache")
	if err != nil {
		return false, fmt.Errorf("failed to get no-cache flag: %w", err)
	}
	return val, nil
}

// GetUpdateCacheFlag retrieves the update-cache flag from command flags.
func (s *Service) GetUpdateCacheFlag() (bool, error) {
	if s == nil {
		return false, fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return false, fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetBool("update-cache")
	if err != nil {
		return false, fmt.Errorf("failed to get update-cache flag: %w", err)
	}
	return val, nil
}
