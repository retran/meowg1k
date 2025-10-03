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

// Package commandmocks provides mock implementations for command service.
package commandmocks

import (
	"github.com/spf13/cobra"
)

// MockCommandService is a mock implementation of command.Service for testing.
type MockCommandService struct {
	Command         *cobra.Command
	CommandName     string
	ConfigPath      string
	TaskName        string
	UserPrompt      string
	Silent          bool
	Intent          string
	TargetBranch    string
	BaseBranch      string
	StdIn           string
	ConfigPathErr   error
	TaskNameErr     error
	UserPromptErr   error
	SilentErr       error
	IntentErr       error
	TargetBranchErr error
	BaseBranchErr   error
}

// GetCommand implements command.Service.
func (m *MockCommandService) GetCommand() *cobra.Command {
	return m.Command
}

// GetCommandName implements command.Service.
func (m *MockCommandService) GetCommandName() string {
	return m.CommandName
}

// GetConfigPath implements command.Service.
func (m *MockCommandService) GetConfigPath() (string, error) {
	return m.ConfigPath, m.ConfigPathErr
}

// GetTaskName implements command.Service.
func (m *MockCommandService) GetTaskName() (string, error) {
	return m.TaskName, m.TaskNameErr
}

// GetUserPrompt implements command.Service.
func (m *MockCommandService) GetUserPrompt() (string, error) {
	return m.UserPrompt, m.UserPromptErr
}

// GetSilentFlag implements command.Service.
func (m *MockCommandService) GetSilentFlag() (bool, error) {
	return m.Silent, m.SilentErr
}

// GetIntentFlag implements command.Service.
func (m *MockCommandService) GetIntentFlag() (string, error) {
	return m.Intent, m.IntentErr
}

// GetTargetBranchFlag implements command.Service.
func (m *MockCommandService) GetTargetBranchFlag() (string, error) {
	return m.TargetBranch, m.TargetBranchErr
}

// GetBaseBranchFlag implements command.Service.
func (m *MockCommandService) GetBaseBranchFlag() (string, error) {
	return m.BaseBranch, m.BaseBranchErr
}

// GetStdIn implements command.Service.
func (m *MockCommandService) GetStdIn() string {
	return m.StdIn
}
