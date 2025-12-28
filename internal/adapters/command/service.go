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

// GetCommand retrieves the current executing command.
func (s *Service) GetCommand() (*cobra.Command, error) {
	if s == nil {
		return nil, fmt.Errorf("command service is nil")
	}

	return s.cmd, nil
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

// GetConfigPath retrieves the config path from command flags.
func (s *Service) GetConfigPath() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return "", fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetString("config")
	if err != nil {
		return "", fmt.Errorf("failed to get config flag: %w", err)
	}
	return val, nil
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

// GetIntentFlag retrieves the intent flag from command flags.
func (s *Service) GetIntentFlag() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return "", fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetString("intent")
	if err != nil {
		return "", fmt.Errorf("failed to get intent flag: %w", err)
	}
	return val, nil
}

// GetDiffFlag retrieves the diff flag from command flags.
func (s *Service) GetDiffFlag() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return "", fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetString("diff")
	if err != nil {
		return "", fmt.Errorf("failed to get diff flag: %w", err)
	}
	return val, nil
}

// GetBaseBranchFlag retrieves the base-branch flag from command flags.
func (s *Service) GetBaseBranchFlag() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return "", fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetString("base")
	if err != nil {
		return "", fmt.Errorf("failed to get base flag: %w", err)
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

// GetQueryTextFlag retrieves the searchindex text from command arguments or stdin.
func (s *Service) GetQueryTextFlag() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return "", fmt.Errorf("command is nil")
	}

	// Try to get from arguments first
	args := s.cmd.Flags().Args()
	if len(args) > 0 {
		return args[0], nil
	}

	// Fall back to stdin
	if s.stdin != "" {
		return s.stdin, nil
	}

	return "", fmt.Errorf("searchindex text is required (provide as argument or via stdin)")
}

// GetSnapshotsFlag retrieves the snapshots flag from command flags.
func (s *Service) GetSnapshotsFlag() ([]string, error) {
	if s == nil {
		return nil, fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return nil, fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetStringSlice("snapshots")
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshots flag: %w", err)
	}
	return val, nil
}

// GetTopKFlag retrieves the top-k flag from command flags.
func (s *Service) GetTopKFlag() (int, error) {
	if s == nil {
		return 0, fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return 0, fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetInt("top-k")
	if err != nil {
		return 0, fmt.Errorf("failed to get top-k flag: %w", err)
	}
	return val, nil
}

// GetMinScoreFlag retrieves the min-score flag from command flags.
func (s *Service) GetMinScoreFlag() (float32, error) {
	if s == nil {
		return 0, fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return 0, fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetFloat32("min-score")
	if err != nil {
		return 0, fmt.Errorf("failed to get min-score flag: %w", err)
	}

	return val, nil
}

// GetJSONFlag retrieves the json flag from command flags.
func (s *Service) GetJSONFlag() (bool, error) {
	if s == nil {
		return false, fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return false, fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetBool("json")
	if err != nil {
		return false, fmt.Errorf("failed to get json flag: %w", err)
	}
	return val, nil
}

// GetQuestionFlag retrieves the question from command arguments or stdin.
func (s *Service) GetQuestionFlag() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return "", fmt.Errorf("command is nil")
	}

	// Try to get from arguments first
	args := s.cmd.Flags().Args()
	if len(args) > 0 {
		return args[0], nil
	}

	// Fall back to stdin
	if s.stdin != "" {
		return s.stdin, nil
	}

	return "", fmt.Errorf("question is required (provide as argument or via stdin)")
}

// GetProfileFlag retrieves the profile flag from command flags.
func (s *Service) GetProfileFlag() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return "", fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetString("profile")
	if err != nil {
		return "", fmt.Errorf("failed to get profile flag: %w", err)
	}
	return val, nil
}

// GetShowContextFlag retrieves the show-context flag from command flags.
func (s *Service) GetShowContextFlag() (bool, error) {
	if s == nil {
		return false, fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return false, fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetBool("show-context")
	if err != nil {
		return false, fmt.Errorf("failed to get show-context flag: %w", err)
	}
	return val, nil
}

// GetSystemPromptFlag retrieves the system-prompt flag from command flags.
func (s *Service) GetSystemPromptFlag() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return "", fmt.Errorf("command is nil")
	}

	val, err := s.cmd.Flags().GetString("system-prompt")
	if err != nil {
		return "", fmt.Errorf("failed to get system-prompt flag: %w", err)
	}
	return val, nil
}

// GetTaskInput retrieves the task input from command arguments or stdin.
func (s *Service) GetTaskInput() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return "", fmt.Errorf("command is nil")
	}

	args := s.cmd.Flags().Args()
	if len(args) > 0 {
		return args[0], nil
	}

	if s.stdin != "" {
		return s.stdin, nil
	}

	return "", fmt.Errorf("task input is required (provide as argument or via stdin)")
}
