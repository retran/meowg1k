// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package command provides services for accessing command-line flags and input streams.
package command

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Service is the concrete implementation of the command service.
type Service struct {
	cmd   *cobra.Command
	stdin string
}

// NewService creates a new command context service with the provided command.
func NewService(cmd *cobra.Command) (*Service, error) {
	if cmd == nil {
		return nil, fmt.Errorf("command cannot be nil")
	}

	stdin := ""

	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat stdin: %w", err)
	}

	if (stat.Mode() & os.ModeCharDevice) == 0 {
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read stdin: %w", err)
		}

		stdin = strings.TrimSpace(string(input))
	}

	return &Service{
		cmd:   cmd,
		stdin: stdin,
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

// GetSilentFlag retrieves the silent flag from command flags.
func (s *Service) GetSilentFlag() (bool, error) {
	if s == nil {
		return false, fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return false, fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetBool("silent")
	if err != nil {
		return false, fmt.Errorf("failed to get silent flag: %w", err)
	}
	return val, nil
}

// GetStdIn retrieves the standard input sent to the command.
func (s *Service) GetStdIn() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	return s.stdin, nil
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

// GetPlainFlag retrieves the plain flag from command flags.
func (s *Service) GetPlainFlag() (bool, error) {
	if s == nil {
		return false, fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return false, fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetBool("plain")
	if err != nil {
		return false, fmt.Errorf("failed to get plain flag: %w", err)
	}
	return val, nil
}

// GetNoColorFlag retrieves the no-color flag from command flags.
func (s *Service) GetNoColorFlag() (bool, error) {
	if s == nil {
		return false, fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return false, fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetBool("no-color")
	if err != nil {
		return false, fmt.Errorf("failed to get no-color flag: %w", err)
	}
	return val, nil
}
