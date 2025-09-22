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
	"testing"

	"github.com/spf13/cobra"
)

func TestNewService(t *testing.T) {
	tests := []struct {
		name    string
		cmd     *cobra.Command
		wantErr bool
	}{
		{
			name: "valid command",
			cmd: &cobra.Command{
				Use: "test",
			},
			wantErr: false,
		},
		{
			name:    "nil command should panic",
			cmd:     nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("NewService() should have panicked with nil command")
					}
				}()
			}

			service := NewService(tt.cmd)

			if !tt.wantErr {
				if service == nil {
					t.Errorf("NewService() returned nil service")
				}
			}
		})
	}
}

func TestServiceImpl_GetCommand(t *testing.T) {
	testCmd := &cobra.Command{
		Use:   "test",
		Short: "A test command",
	}

	service := NewService(testCmd)

	retrievedCmd := service.GetCommand()

	if retrievedCmd != testCmd {
		t.Errorf("GetCommand() returned different command than provided")
	}

	if retrievedCmd.Use != "test" {
		t.Errorf("GetCommand() returned command with wrong Use field, got %s, want test", retrievedCmd.Use)
	}

	if retrievedCmd.Short != "A test command" {
		t.Errorf("GetCommand() returned command with wrong Short field, got %s, want 'A test command'", retrievedCmd.Short)
	}
}

func TestServiceImpl_InterfaceCompliance(t *testing.T) {
	// Compile-time check that serviceImpl implements Service interface
	var _ Service = (*serviceImpl)(nil)

	// Runtime verification with actual service
	testCmd := &cobra.Command{Use: "test"}
	service := NewService(testCmd)

	// Verify service provides expected interface methods
	if service.GetCommand() != testCmd {
		t.Errorf("Service does not properly implement GetCommand method")
	}
}
