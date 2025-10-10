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

// Package command provides command context capabilities.
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

	return s.cmd.Flags().GetString("config")
}

// GetTaskName retrieves the task name from command flags.
func (s *Service) GetTaskName() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return "", fmt.Errorf("command is nil")
	}

	return s.cmd.Flags().GetString("task")
}

// GetUserPrompt retrieves the user prompt from command flags.
func (s *Service) GetUserPrompt() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return "", fmt.Errorf("command is nil")
	}

	return s.cmd.Flags().GetString("user-prompt")
}

// GetSilentFlag retrieves the silent flag from command flags.
func (s *Service) GetSilentFlag() (bool, error) {
	if s == nil {
		return false, fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return false, fmt.Errorf("command is nil")
	}

	return s.cmd.Flags().GetBool("silent")
}

// GetIntentFlag retrieves the intent flag from command flags.
func (s *Service) GetIntentFlag() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return "", fmt.Errorf("command is nil")
	}

	return s.cmd.Flags().GetString("intent")
}

// GetTargetBranchFlag retrieves the target-branch flag from command flags.
func (s *Service) GetTargetBranchFlag() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return "", fmt.Errorf("command is nil")
	}

	return s.cmd.Flags().GetString("target-branch")
}

// GetBaseBranchFlag retrieves the base-branch flag from command flags.
func (s *Service) GetBaseBranchFlag() (string, error) {
	if s == nil {
		return "", fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return "", fmt.Errorf("command is nil")
	}

	return s.cmd.Flags().GetString("base")
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

	return s.cmd.Flags().GetBool("no-cache")
}

// GetUpdateCacheFlag retrieves the update-cache flag from command flags.
func (s *Service) GetUpdateCacheFlag() (bool, error) {
	if s == nil {
		return false, fmt.Errorf("command service is nil")
	}

	if s.cmd == nil {
		return false, fmt.Errorf("command is nil")
	}

	return s.cmd.Flags().GetBool("update-cache")
}
