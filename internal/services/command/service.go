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
		panic("command cannot be nil")
	}

	stdin := ""

	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat stdin: %w", err)
	}

	if (stat.Mode() & os.ModeCharDevice) == 0 {
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read from stdin: %w", err)
		}

		stdin = strings.TrimSpace(string(input))
	}

	return &Service{
		cmd:   cmd,
		stdin: stdin,
	}, nil
}

// GetCommand retrieves the current executing command.
func (s *Service) GetCommand() *cobra.Command {
	return s.cmd
}

// GetCommandName retrieves the name of the current executing command.
func (s *Service) GetCommandName() string {
	return s.cmd.Name()
}

// GetConfigPath retrieves the config path from command flags.
func (s *Service) GetConfigPath() (string, error) {
	return s.cmd.Flags().GetString("config")
}

// GetTaskName retrieves the task name from command flags.
func (s *Service) GetTaskName() (string, error) {
	return s.cmd.Flags().GetString("task")
}

// GetUserPrompt retrieves the user prompt from command flags.
func (s *Service) GetUserPrompt() (string, error) {
	return s.cmd.Flags().GetString("user-prompt")
}

// GetSilentFlag retrieves the silent flag from command flags.
func (s *Service) GetSilentFlag() (bool, error) {
	return s.cmd.Flags().GetBool("silent")
}

// GetIntentFlag retrieves the intent flag from command flags.
func (s *Service) GetIntentFlag() (string, error) {
	return s.cmd.Flags().GetString("intent")
}

// GetTargetBranchFlag retrieves the target-branch flag from command flags.
func (s *Service) GetTargetBranchFlag() (string, error) {
	return s.cmd.Flags().GetString("target-branch")
}

// GetBaseBranchFlag retrieves the base-branch flag from command flags.
func (s *Service) GetBaseBranchFlag() (string, error) {
	return s.cmd.Flags().GetString("base")
}

// GetStdIn retrieves the standard input sent to the command.
func (s *Service) GetStdIn() string {
	return s.stdin
}
