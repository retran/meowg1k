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

package command

import (
	"github.com/spf13/cobra"
)

// Service provides command context capabilities.
type Service interface {
	// GetCommand retrieves the current executing command.
	GetCommand() *cobra.Command

	// GetConfigPath retrieves the config path from command flags or global variable.
	GetConfigPath() (string, error)

	// GetTaskName retrieves the task name from command flags.
	GetTaskName() (string, error)

	// GetUserPrompt retrieves the user prompt from command flags.
	GetUserPrompt() (string, error)

	// GetSilentFlag retrieves the silent flag from command flags.
	GetSilentFlag() (bool, error)
}

// serviceImpl is the concrete implementation of the command service.
type serviceImpl struct {
	cmd *cobra.Command
}

// Compile-time interface satisfaction check
var _ Service = (*serviceImpl)(nil)

// NewService creates a new command context service with the provided command.
func NewService(cmd *cobra.Command) Service {
	if cmd == nil {
		panic("command cannot be nil")
	}
	return &serviceImpl{
		cmd: cmd,
	}
}

func (s *serviceImpl) GetCommand() *cobra.Command {
	return s.cmd
}

// GetConfigPath retrieves the config path from command flags.
func (s *serviceImpl) GetConfigPath() (string, error) {
	return s.cmd.Flags().GetString("config")
}

// GetTaskName retrieves the task name from command flags.
func (s *serviceImpl) GetTaskName() (string, error) {
	return s.cmd.Flags().GetString("task")
}

// GetUserPrompt retrieves the user prompt from command flags.
func (s *serviceImpl) GetUserPrompt() (string, error) {
	return s.cmd.Flags().GetString("user-prompt")
}

// GetSilentFlag retrieves the silent flag from command flags.
func (s *serviceImpl) GetSilentFlag() (bool, error) {
	return s.cmd.Flags().GetBool("silent")
}
